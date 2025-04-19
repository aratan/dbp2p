package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"dbp2p/pkg/binary"

	"github.com/gorilla/mux"
)

// setupBinaryRoutes configura las rutas para manejar archivos binarios
func (s *APIServer) setupBinaryRoutes() {
	// Inicializar el gestor de binarios
	binaryManager, err := binary.NewBinaryManager(
		s.db,
		filepath.Join("./data", "binaries"),
		binary.WithCompression(true),
	)
	if err != nil {
		panic(fmt.Sprintf("Error al inicializar gestor de binarios: %v", err))
	}
	s.binaryManager = binaryManager

	// Rutas para archivos binarios
	fileApi := s.router.PathPrefix("/api/files").Subrouter()
	fileApi.Use(s.authMiddleware)
	fileApi.HandleFunc("", s.handleListFiles).Methods("GET")
	fileApi.HandleFunc("", s.handleUploadFile).Methods("POST")
	fileApi.HandleFunc("/{id}", s.handleGetFile).Methods("GET")
	fileApi.HandleFunc("/{id}", s.handleDeleteFile).Methods("DELETE")
	fileApi.HandleFunc("/{id}/metadata", s.handleGetFileMetadata).Methods("GET")

	// Rutas para archivos de colecciones
	s.router.PathPrefix("/api/collections/{collection}/files").Handler(
		s.authMiddleware(http.HandlerFunc(s.handleListCollectionFiles))).Methods("GET")
}

// handleUploadFile maneja la subida de archivos
func (s *APIServer) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	// Limitar el tamaño máximo de la solicitud (100MB)
	r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024)

	// Parsear formulario multipart
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error al parsear formulario: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Obtener archivo
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error al obtener archivo: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Obtener parámetros adicionales
	collection := r.FormValue("collection")
	documentID := r.FormValue("document_id")
	tagsStr := r.FormValue("tags")

	// Parsear tags
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	// Parsear metadatos personalizados
	metadata := make(map[string]string)
	for key, values := range r.Form {
		if strings.HasPrefix(key, "meta_") && len(values) > 0 {
			metaKey := strings.TrimPrefix(key, "meta_")
			metadata[metaKey] = values[0]
		}
	}

	// Determinar tipo MIME
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = binary.GetMimeType(header.Filename)
	}

	// Configurar opciones de almacenamiento
	options := []binary.StoreOption{
		binary.WithTags(tags),
		binary.WithMetadata(metadata),
	}

	// Añadir colección y documentID si se proporcionaron
	if collection != "" {
		options = append(options, binary.WithCollection(collection))
	}
	if documentID != "" {
		options = append(options, binary.WithDocumentID(documentID))
	}

	// Almacenar archivo
	fileMetadata, err := s.binaryManager.StoreFile(file, header.Filename, mimeType, options...)
	if err != nil {
		http.Error(w, "Error al almacenar archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Devolver metadatos
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fileMetadata)
}

// handleGetFile maneja la descarga de archivos
func (s *APIServer) handleGetFile(w http.ResponseWriter, r *http.Request) {
	// Obtener ID del archivo
	vars := mux.Vars(r)
	fileID := vars["id"]

	// Recuperar archivo
	fileReader, metadata, err := s.binaryManager.GetFile(fileID)
	if err != nil {
		if err == binary.ErrFileNotFound {
			http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		} else {
			http.Error(w, "Error al recuperar archivo: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer fileReader.Close()

	// Configurar cabeceras
	w.Header().Set("Content-Type", metadata.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", metadata.Filename))
	w.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))

	// Enviar archivo
	if _, err := io.Copy(w, fileReader); err != nil {
		// No podemos enviar un error HTTP aquí porque ya hemos empezado a enviar la respuesta
		fmt.Printf("Error al enviar archivo: %v\n", err)
	}
}

// handleDeleteFile maneja la eliminación de archivos
func (s *APIServer) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	// Obtener ID del archivo
	vars := mux.Vars(r)
	fileID := vars["id"]

	// Eliminar archivo
	if err := s.binaryManager.DeleteFile(fileID); err != nil {
		if err == binary.ErrFileNotFound {
			http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		} else {
			http.Error(w, "Error al eliminar archivo: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Responder con éxito
	w.WriteHeader(http.StatusNoContent)
}

// handleGetFileMetadata maneja la obtención de metadatos de archivos
func (s *APIServer) handleGetFileMetadata(w http.ResponseWriter, r *http.Request) {
	// Obtener ID del archivo
	vars := mux.Vars(r)
	fileID := vars["id"]

	// Obtener metadatos
	doc, err := s.db.GetDocument(fileID)
	if err != nil {
		http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		return
	}

	// Verificar que sea un documento binario
	if doc.Collection != binary.BinaryCollection {
		http.Error(w, "El documento no es un archivo binario", http.StatusBadRequest)
		return
	}

	// Devolver metadatos
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc.Data)
}

// handleListFiles maneja la lista de todos los archivos
func (s *APIServer) handleListFiles(w http.ResponseWriter, r *http.Request) {
	// Obtener parámetros de consulta
	collection := r.URL.Query().Get("collection")

	// Listar archivos
	files, err := s.binaryManager.ListFiles(collection)
	if err != nil {
		http.Error(w, "Error al listar archivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Devolver lista
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// handleListCollectionFiles maneja la lista de archivos de una colección
func (s *APIServer) handleListCollectionFiles(w http.ResponseWriter, r *http.Request) {
	// Obtener colección
	vars := mux.Vars(r)
	collection := vars["collection"]

	// Listar archivos
	files, err := s.binaryManager.ListFiles(collection)
	if err != nil {
		http.Error(w, "Error al listar archivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Devolver lista
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
