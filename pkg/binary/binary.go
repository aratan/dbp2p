package binary

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// FileMetadata representa los metadatos de un archivo binario
type FileMetadata struct {
	ID         string            `json:"id"`
	Filename   string            `json:"filename"`
	MimeType   string            `json:"mimetype"`
	Size       int64             `json:"size"`
	Collection string            `json:"collection"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// BinaryManager gestiona el almacenamiento de archivos binarios
type BinaryManager struct {
	storageDir  string
	metadataDir string
	compression bool
}

// StoreOption representa una opción para almacenar un archivo
type StoreOption func(*storeOptions)

type storeOptions struct {
	collection string
	metadata   map[string]string
}

// WithCollection establece la colección para un archivo
func WithCollection(collection string) StoreOption {
	return func(o *storeOptions) {
		o.collection = collection
	}
}

// WithMetadata establece los metadatos para un archivo
func WithMetadata(metadata map[string]string) StoreOption {
	return func(o *storeOptions) {
		o.metadata = metadata
	}
}

// WithCompression establece si se debe comprimir el archivo
func WithCompression(enabled bool) BinaryManagerOption {
	return func(m *BinaryManager) {
		m.compression = enabled
	}
}

// BinaryManagerOption representa una opción para el gestor de binarios
type BinaryManagerOption func(*BinaryManager)

// NewBinaryManager crea un nuevo gestor de binarios
func NewBinaryManager(storageDir string, opts ...BinaryManagerOption) (*BinaryManager, error) {
	// Crear directorios si no existen
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de almacenamiento: %v", err)
	}

	metadataDir := filepath.Join(storageDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de metadatos: %v", err)
	}

	manager := &BinaryManager{
		storageDir:  storageDir,
		metadataDir: metadataDir,
		compression: false,
	}

	// Aplicar opciones
	for _, opt := range opts {
		opt(manager)
	}

	return manager, nil
}

// StoreFile almacena un archivo y devuelve sus metadatos
func (m *BinaryManager) StoreFile(reader io.Reader, filename, mimeType string, opts ...StoreOption) (*FileMetadata, error) {
	// Aplicar opciones
	options := &storeOptions{
		collection: "default",
		metadata:   make(map[string]string),
	}
	for _, opt := range opts {
		opt(options)
	}

	// Generar ID único
	id := uuid.New().String()

	// Crear directorio de colección si no existe
	collectionDir := filepath.Join(m.storageDir, options.collection)
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de colección: %v", err)
	}

	// Ruta del archivo
	filePath := filepath.Join(collectionDir, id)

	// Crear archivo
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("error al crear archivo: %v", err)
	}
	defer file.Close()

	// Preparar escritor (con compresión si está habilitada)
	var writer io.Writer = file
	var gzipWriter *gzip.Writer
	if m.compression {
		gzipWriter = gzip.NewWriter(file)
		writer = gzipWriter
		defer gzipWriter.Close()
	}

	// Copiar contenido
	size, err := io.Copy(writer, reader)
	if err != nil {
		return nil, fmt.Errorf("error al escribir archivo: %v", err)
	}

	// Cerrar escritor gzip si se está usando
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			return nil, fmt.Errorf("error al cerrar escritor gzip: %v", err)
		}
	}

	// Crear metadatos
	now := time.Now()
	metadata := &FileMetadata{
		ID:         id,
		Filename:   filename,
		MimeType:   mimeType,
		Size:       size,
		Collection: options.collection,
		Metadata:   options.metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Guardar metadatos
	if err := m.saveMetadata(metadata); err != nil {
		return nil, fmt.Errorf("error al guardar metadatos: %v", err)
	}

	return metadata, nil
}

// GetFile recupera un archivo por su ID
func (m *BinaryManager) GetFile(id string) (io.ReadCloser, *FileMetadata, error) {
	// Obtener metadatos
	metadata, err := m.getMetadata(id)
	if err != nil {
		return nil, nil, err
	}

	// Ruta del archivo
	filePath := filepath.Join(m.storageDir, metadata.Collection, id)

	// Abrir archivo
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error al abrir archivo: %v", err)
	}

	// Si está comprimido, devolver un lector gzip
	if m.compression {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("error al crear lector gzip: %v", err)
		}
		return &gzipReadCloser{gzipReader, file}, metadata, nil
	}

	return file, metadata, nil
}

// gzipReadCloser combina un gzip.Reader y un os.File para implementar io.ReadCloser
type gzipReadCloser struct {
	*gzip.Reader
	file *os.File
}

// Close cierra tanto el lector gzip como el archivo
func (g *gzipReadCloser) Close() error {
	gzipErr := g.Reader.Close()
	fileErr := g.file.Close()
	if gzipErr != nil {
		return gzipErr
	}
	return fileErr
}

// DeleteFile elimina un archivo por su ID
func (m *BinaryManager) DeleteFile(id string) error {
	// Obtener metadatos
	metadata, err := m.getMetadata(id)
	if err != nil {
		return err
	}

	// Ruta del archivo
	filePath := filepath.Join(m.storageDir, metadata.Collection, id)

	// Eliminar archivo
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error al eliminar archivo: %v", err)
	}

	// Eliminar metadatos
	metadataPath := filepath.Join(m.metadataDir, id+".json")
	if err := os.Remove(metadataPath); err != nil {
		return fmt.Errorf("error al eliminar metadatos: %v", err)
	}

	return nil
}

// ListFiles lista todos los archivos de una colección
func (m *BinaryManager) ListFiles(collection string) ([]*FileMetadata, error) {
	var files []*FileMetadata

	// Listar todos los archivos de metadatos
	metadataFiles, err := filepath.Glob(filepath.Join(m.metadataDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("error al listar archivos de metadatos: %v", err)
	}

	// Leer cada archivo de metadatos
	for _, metadataFile := range metadataFiles {
		data, err := os.ReadFile(metadataFile)
		if err != nil {
			continue
		}

		var metadata FileMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}

		// Filtrar por colección si se especifica
		if collection == "" || metadata.Collection == collection {
			files = append(files, &metadata)
		}
	}

	return files, nil
}

// saveMetadata guarda los metadatos de un archivo
func (m *BinaryManager) saveMetadata(metadata *FileMetadata) error {
	// Serializar metadatos
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	// Guardar en archivo
	metadataPath := filepath.Join(m.metadataDir, metadata.ID+".json")
	return os.WriteFile(metadataPath, data, 0644)
}

// getMetadata obtiene los metadatos de un archivo
func (m *BinaryManager) getMetadata(id string) (*FileMetadata, error) {
	// Ruta del archivo de metadatos
	metadataPath := filepath.Join(m.metadataDir, id+".json")

	// Leer archivo
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("error al leer metadatos: %v", err)
	}

	// Deserializar metadatos
	var metadata FileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("error al deserializar metadatos: %v", err)
	}

	return &metadata, nil
}

// UpdateMetadata actualiza los metadatos de un archivo
func (m *BinaryManager) UpdateMetadata(id string, updates map[string]string) (*FileMetadata, error) {
	// Obtener metadatos actuales
	metadata, err := m.getMetadata(id)
	if err != nil {
		return nil, err
	}

	// Actualizar metadatos
	if metadata.Metadata == nil {
		metadata.Metadata = make(map[string]string)
	}
	for k, v := range updates {
		metadata.Metadata[k] = v
	}
	metadata.UpdatedAt = time.Now()

	// Guardar metadatos actualizados
	if err := m.saveMetadata(metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}
