package ws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aratan/dbp2p/pkg/binary"
)

// BinaryUploadMessage representa un mensaje para subir un archivo binario
type BinaryUploadMessage struct {
	Collection string            `json:"collection"`
	Filename   string            `json:"filename"`
	MimeType   string            `json:"mimetype"`
	Data       string            `json:"data"` // Contenido en base64
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// BinaryDownloadRequest representa una solicitud para descargar un archivo binario
type BinaryDownloadRequest struct {
	ID string `json:"id"`
}

// BinaryDownloadResponse representa una respuesta con un archivo binario
type BinaryDownloadResponse struct {
	ID       string            `json:"id"`
	Filename string            `json:"filename"`
	MimeType string            `json:"mimetype"`
	Size     int64             `json:"size"`
	Data     string            `json:"data"` // Contenido en base64
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BinaryListRequest representa una solicitud para listar archivos binarios
type BinaryListRequest struct {
	Collection string `json:"collection,omitempty"`
}

// handleBinaryUpload maneja la subida de un archivo binario a través de WebSocket
func (c *Client) handleBinaryUpload(message []byte) {
	// Decodificar mensaje
	var uploadMsg BinaryUploadMessage
	if err := json.Unmarshal(message, &uploadMsg); err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al decodificar mensaje: %v", err))
		return
	}

	// Validar datos
	if uploadMsg.Collection == "" {
		c.sendBinaryErrorMessage("Se requiere una colección")
		return
	}
	if uploadMsg.Filename == "" {
		c.sendBinaryErrorMessage("Se requiere un nombre de archivo")
		return
	}
	if uploadMsg.Data == "" {
		c.sendBinaryErrorMessage("Se requieren datos")
		return
	}

	// Decodificar datos de base64
	data, err := base64.StdEncoding.DecodeString(uploadMsg.Data)
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al decodificar datos base64: %v", err))
		return
	}

	// Crear archivo temporal
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "ws_upload_*")
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al crear archivo temporal: %v", err))
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Escribir datos al archivo temporal
	if _, err := tempFile.Write(data); err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al escribir datos: %v", err))
		return
	}

	// Rebobinar el archivo
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al rebobinar archivo: %v", err))
		return
	}

	// Determinar tipo MIME si no se proporcionó
	mimeType := uploadMsg.MimeType
	if mimeType == "" {
		mimeType = getMimeType(uploadMsg.Filename)
	}

	// Configurar opciones
	options := []binary.StoreOption{
		binary.WithCollection(uploadMsg.Collection),
		binary.WithMetadata(uploadMsg.Metadata),
	}

	// Almacenar archivo
	binaryManager := c.server.getBinaryManager()
	if binaryManager == nil {
		c.sendBinaryErrorMessage("Gestor de binarios no disponible")
		return
	}

	fileMetadata, err := binaryManager.StoreFile(tempFile, uploadMsg.Filename, mimeType, options...)
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al almacenar archivo: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":    "binary_upload_response",
		"success": true,
		"id":      fileMetadata.ID,
		"message": fmt.Sprintf("Archivo %s subido exitosamente", uploadMsg.Filename),
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al codificar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleBinaryDownload maneja la descarga de un archivo binario a través de WebSocket
func (c *Client) handleBinaryDownload(message []byte) {
	// Decodificar mensaje
	var downloadReq BinaryDownloadRequest
	if err := json.Unmarshal(message, &downloadReq); err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al decodificar mensaje: %v", err))
		return
	}

	// Validar datos
	if downloadReq.ID == "" {
		c.sendBinaryErrorMessage("Se requiere un ID")
		return
	}

	// Obtener gestor de binarios
	binaryManager := c.server.getBinaryManager()
	if binaryManager == nil {
		c.sendBinaryErrorMessage("Gestor de binarios no disponible")
		return
	}

	// Recuperar archivo
	fileReader, metadata, err := binaryManager.GetFile(downloadReq.ID)
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al recuperar archivo: %v", err))
		return
	}
	defer fileReader.Close()

	// Leer datos
	data, err := io.ReadAll(fileReader)
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al leer datos: %v", err))
		return
	}

	// Codificar en base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// Crear respuesta
	response := BinaryDownloadResponse{
		ID:       downloadReq.ID,
		Filename: metadata.Filename,
		MimeType: metadata.MimeType,
		Size:     metadata.Size,
		Data:     base64Data,
		Metadata: metadata.Metadata,
	}

	// Envolver en mensaje de tipo
	wrappedResponse := map[string]interface{}{
		"type":     "binary_download_response",
		"success":  true,
		"response": response,
	}

	// Codificar respuesta
	responseJSON, err := json.Marshal(wrappedResponse)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al codificar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleBinaryList maneja la lista de archivos binarios a través de WebSocket
func (c *Client) handleBinaryList(message []byte) {
	// Decodificar mensaje
	var listReq BinaryListRequest
	if err := json.Unmarshal(message, &listReq); err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al decodificar mensaje: %v", err))
		return
	}

	// Obtener gestor de binarios
	binaryManager := c.server.getBinaryManager()
	if binaryManager == nil {
		c.sendBinaryErrorMessage("Gestor de binarios no disponible")
		return
	}

	// Listar archivos
	files, err := binaryManager.ListFiles(listReq.Collection)
	if err != nil {
		c.sendBinaryErrorMessage(fmt.Sprintf("Error al listar archivos: %v", err))
		return
	}

	// Crear respuesta
	response := map[string]interface{}{
		"type":    "binary_list_response",
		"success": true,
		"files":   files,
		"count":   len(files),
	}

	// Codificar respuesta
	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al codificar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// sendBinaryErrorMessage envía un mensaje de error al cliente para operaciones binarias
func (c *Client) sendBinaryErrorMessage(message string) {
	response := map[string]interface{}{
		"type":    "binary_error",
		"success": false,
		"message": message,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return
	}

	c.send <- responseJSON
}

// getMimeType determina el tipo MIME basado en la extensión del archivo
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
