package db

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

// CompressionType define el tipo de compresión
type CompressionType string

const (
	// CompressionNone sin compresión
	CompressionNone CompressionType = "none"
	// CompressionGzip compresión gzip
	CompressionGzip CompressionType = "gzip"
	// CompressionZlib compresión zlib
	CompressionZlib CompressionType = "zlib"
)

// CompressionLevel define el nivel de compresión
type CompressionLevel int

const (
	// CompressionLevelDefault nivel por defecto
	CompressionLevelDefault CompressionLevel = 0
	// CompressionLevelBestSpeed optimizado para velocidad
	CompressionLevelBestSpeed CompressionLevel = 1
	// CompressionLevelBestCompression optimizado para tamaño
	CompressionLevelBestCompression CompressionLevel = 9
)

// CompressedData representa datos comprimidos
type CompressedData struct {
	Type       CompressionType `json:"type"`
	Data       []byte          `json:"data"`
	OrigSize   int             `json:"orig_size"`
	CompSize   int             `json:"comp_size"`
	Ratio      float64         `json:"ratio"`
	Collection string          `json:"collection,omitempty"`
	DocumentID string          `json:"document_id,omitempty"`
}

// CompressionOptions opciones de compresión
type CompressionOptions struct {
	Type  CompressionType
	Level CompressionLevel
}

// DefaultCompressionOptions opciones por defecto
var DefaultCompressionOptions = CompressionOptions{
	Type:  CompressionGzip,
	Level: CompressionLevelDefault,
}

// CompressJSON comprime datos JSON
func CompressJSON(data interface{}, options ...CompressionOptions) (*CompressedData, error) {
	// Serializar a JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error al serializar JSON: %v", err)
	}

	// Usar opciones por defecto si no se especifican
	opts := DefaultCompressionOptions
	if len(options) > 0 {
		opts = options[0]
	}

	// Si no se requiere compresión, devolver los datos sin comprimir
	if opts.Type == CompressionNone {
		return &CompressedData{
			Type:     CompressionNone,
			Data:     jsonData,
			OrigSize: len(jsonData),
			CompSize: len(jsonData),
			Ratio:    1.0,
		}, nil
	}

	// Comprimir datos
	var compressedData []byte
	var compressErr error

	switch opts.Type {
	case CompressionGzip:
		compressedData, compressErr = compressGzip(jsonData, int(opts.Level))
	case CompressionZlib:
		compressedData, compressErr = compressZlib(jsonData, int(opts.Level))
	default:
		return nil, fmt.Errorf("tipo de compresión no soportado: %s", opts.Type)
	}

	if compressErr != nil {
		return nil, fmt.Errorf("error al comprimir datos: %v", compressErr)
	}

	// Calcular ratio de compresión
	ratio := float64(len(compressedData)) / float64(len(jsonData))

	return &CompressedData{
		Type:     opts.Type,
		Data:     compressedData,
		OrigSize: len(jsonData),
		CompSize: len(compressedData),
		Ratio:    ratio,
	}, nil
}

// DecompressJSON descomprime datos JSON
func DecompressJSON(compressed *CompressedData, target interface{}) error {
	var jsonData []byte
	var err error

	// Si no hay compresión, usar los datos directamente
	if compressed.Type == CompressionNone {
		jsonData = compressed.Data
	} else {
		// Descomprimir según el tipo
		switch compressed.Type {
		case CompressionGzip:
			jsonData, err = decompressGzip(compressed.Data)
		case CompressionZlib:
			jsonData, err = decompressZlib(compressed.Data)
		default:
			return fmt.Errorf("tipo de compresión no soportado: %s", compressed.Type)
		}

		if err != nil {
			return fmt.Errorf("error al descomprimir datos: %v", err)
		}
	}

	// Deserializar JSON
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("error al deserializar JSON: %v", err)
	}

	return nil
}

// compressGzip comprime datos con gzip
func compressGzip(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer

	// Crear writer gzip
	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	// Escribir datos
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	// Cerrar writer
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompressGzip descomprime datos gzip
func decompressGzip(data []byte) ([]byte, error) {
	// Crear reader gzip
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Leer datos descomprimidos
	return ioutil.ReadAll(reader)
}

// compressZlib comprime datos con zlib
func compressZlib(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer

	// Crear writer zlib
	writer, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	// Escribir datos
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	// Cerrar writer
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompressZlib descomprime datos zlib
func decompressZlib(data []byte) ([]byte, error) {
	// Crear reader zlib
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Leer datos descomprimidos
	return ioutil.ReadAll(reader)
}

// CompressDocument comprime un documento
func CompressDocument(doc *Document, options ...CompressionOptions) (*CompressedData, error) {
	compressed, err := CompressJSON(doc.Data, options...)
	if err != nil {
		return nil, err
	}

	compressed.Collection = doc.Collection
	compressed.DocumentID = doc.ID

	return compressed, nil
}

// DecompressDocument descomprime un documento
func DecompressDocument(compressed *CompressedData) (*Document, error) {
	var data map[string]interface{}
	if err := DecompressJSON(compressed, &data); err != nil {
		return nil, err
	}

	return &Document{
		ID:         compressed.DocumentID,
		Collection: compressed.Collection,
		Data:       data,
	}, nil
}

// CompressReader comprime datos de un reader
func CompressReader(reader io.Reader, compressionType CompressionType, level CompressionLevel) (io.ReadCloser, error) {
	// Leer todos los datos
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Comprimir según el tipo
	var compressedData []byte
	switch compressionType {
	case CompressionNone:
		compressedData = data
	case CompressionGzip:
		compressedData, err = compressGzip(data, int(level))
	case CompressionZlib:
		compressedData, err = compressZlib(data, int(level))
	default:
		return nil, fmt.Errorf("tipo de compresión no soportado: %s", compressionType)
	}

	if err != nil {
		return nil, err
	}

	// Crear reader con los datos comprimidos
	return ioutil.NopCloser(bytes.NewReader(compressedData)), nil
}

// DecompressReader descomprime datos de un reader
func DecompressReader(reader io.Reader, compressionType CompressionType) (io.ReadCloser, error) {
	switch compressionType {
	case CompressionNone:
		return ioutil.NopCloser(reader), nil
	case CompressionGzip:
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return gzReader, nil
	case CompressionZlib:
		zlibReader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return zlibReader, nil
	default:
		return nil, fmt.Errorf("tipo de compresión no soportado: %s", compressionType)
	}
}
