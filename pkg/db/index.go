package db

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// IndexType define el tipo de índice
type IndexType string

const (
	// IndexTypeUnique índice único
	IndexTypeUnique IndexType = "unique"
	// IndexTypeNonUnique índice no único
	IndexTypeNonUnique IndexType = "non-unique"
	// IndexTypeText índice de texto
	IndexTypeText IndexType = "text"
)

// Index representa un índice en la base de datos
type Index struct {
	Name       string            // Nombre del índice
	Collection string            // Colección a la que pertenece
	Fields     []string          // Campos indexados
	Type       IndexType         // Tipo de índice
	Unique     bool              // Si el índice es único
	CreatedAt  time.Time         // Fecha de creación
	UpdatedAt  time.Time         // Fecha de actualización
	Data       map[string][]string // Datos del índice: valor -> IDs de documentos
	mutex      sync.RWMutex      // Mutex para concurrencia
}

// NewIndex crea un nuevo índice
func NewIndex(name string, collection string, fields []string, indexType IndexType) *Index {
	now := time.Now()
	return &Index{
		Name:       name,
		Collection: collection,
		Fields:     fields,
		Type:       indexType,
		Unique:     indexType == IndexTypeUnique,
		CreatedAt:  now,
		UpdatedAt:  now,
		Data:       make(map[string][]string),
	}
}

// AddDocument añade un documento al índice
func (idx *Index) AddDocument(doc *Document) error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Obtener valor del campo indexado
	value, err := idx.getIndexValue(doc)
	if err != nil {
		return err
	}

	// Verificar unicidad si es necesario
	if idx.Unique && len(idx.Data[value]) > 0 {
		return fmt.Errorf("violación de índice único: %s", value)
	}

	// Añadir documento al índice
	if _, exists := idx.Data[value]; !exists {
		idx.Data[value] = []string{}
	}

	// Verificar si el documento ya está en el índice
	for _, id := range idx.Data[value] {
		if id == doc.ID {
			return nil // Ya está indexado
		}
	}

	// Añadir ID del documento
	idx.Data[value] = append(idx.Data[value], doc.ID)
	idx.UpdatedAt = time.Now()

	return nil
}

// RemoveDocument elimina un documento del índice
func (idx *Index) RemoveDocument(docID string) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Buscar y eliminar el documento de todos los valores
	for value, ids := range idx.Data {
		newIDs := []string{}
		for _, id := range ids {
			if id != docID {
				newIDs = append(newIDs, id)
			}
		}

		if len(newIDs) == 0 {
			delete(idx.Data, value)
		} else {
			idx.Data[value] = newIDs
		}
	}

	idx.UpdatedAt = time.Now()
}

// UpdateDocument actualiza un documento en el índice
func (idx *Index) UpdateDocument(doc *Document) error {
	// Primero eliminar el documento
	idx.RemoveDocument(doc.ID)

	// Luego añadirlo de nuevo
	return idx.AddDocument(doc)
}

// Search busca documentos por valor indexado
func (idx *Index) Search(value string) []string {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	// Búsqueda exacta
	if ids, exists := idx.Data[value]; exists {
		return ids
	}

	// Si es un índice de texto, realizar búsqueda parcial
	if idx.Type == IndexTypeText {
		var results []string
		for indexValue, ids := range idx.Data {
			if strings.Contains(indexValue, value) {
				results = append(results, ids...)
			}
		}
		return results
	}

	return []string{}
}

// getIndexValue obtiene el valor indexado de un documento
func (idx *Index) getIndexValue(doc *Document) (string, error) {
	if len(idx.Fields) == 0 {
		return "", fmt.Errorf("no hay campos definidos para el índice")
	}

	// Para índices de un solo campo
	if len(idx.Fields) == 1 {
		field := idx.Fields[0]
		
		// Si el campo es "_id", usar el ID del documento
		if field == "_id" {
			return doc.ID, nil
		}

		// Obtener el valor del campo
		value, err := getFieldValue(doc.Data, field)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%v", value), nil
	}

	// Para índices compuestos
	var values []string
	for _, field := range idx.Fields {
		if field == "_id" {
			values = append(values, doc.ID)
			continue
		}

		value, err := getFieldValue(doc.Data, field)
		if err != nil {
			return "", err
		}

		values = append(values, fmt.Sprintf("%v", value))
	}

	return strings.Join(values, "|"), nil
}

// getFieldValue obtiene el valor de un campo en un documento
func getFieldValue(data interface{}, field string) (interface{}, error) {
	// Si el campo contiene puntos, es un campo anidado
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		current := parts[0]
		rest := parts[1]

		// Obtener el valor del campo actual
		m, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("no se puede acceder al campo anidado %s", field)
		}

		value, exists := m[current]
		if !exists {
			return nil, fmt.Errorf("campo %s no encontrado", current)
		}

		// Recursivamente obtener el resto del campo
		return getFieldValue(value, rest)
	}

	// Campo simple
	switch v := data.(type) {
	case map[string]interface{}:
		value, exists := v[field]
		if !exists {
			return nil, fmt.Errorf("campo %s no encontrado", field)
		}
		return value, nil
	default:
		// Intentar acceder al campo usando reflection
		val := reflect.ValueOf(data)
		if val.Kind() == reflect.Struct {
			field = strings.Title(field) // Convertir a mayúscula inicial para campos exportados
			fieldVal := val.FieldByName(field)
			if fieldVal.IsValid() {
				return fieldVal.Interface(), nil
			}
		}
		return nil, fmt.Errorf("tipo de datos no soportado para acceder al campo %s", field)
	}
}

// IndexManager gestiona los índices de la base de datos
type IndexManager struct {
	Indexes map[string]*Index // Nombre del índice -> Índice
	mutex   sync.RWMutex      // Mutex para concurrencia
}

// NewIndexManager crea un nuevo gestor de índices
func NewIndexManager() *IndexManager {
	return &IndexManager{
		Indexes: make(map[string]*Index),
	}
}

// CreateIndex crea un nuevo índice
func (im *IndexManager) CreateIndex(name string, collection string, fields []string, indexType IndexType) (*Index, error) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// Verificar si ya existe un índice con ese nombre
	if _, exists := im.Indexes[name]; exists {
		return nil, fmt.Errorf("ya existe un índice con el nombre %s", name)
	}

	// Crear índice
	index := NewIndex(name, collection, fields, indexType)
	im.Indexes[name] = index

	return index, nil
}

// GetIndex obtiene un índice por su nombre
func (im *IndexManager) GetIndex(name string) (*Index, bool) {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	index, exists := im.Indexes[name]
	return index, exists
}

// GetIndexesForCollection obtiene todos los índices de una colección
func (im *IndexManager) GetIndexesForCollection(collection string) []*Index {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	var indexes []*Index
	for _, index := range im.Indexes {
		if index.Collection == collection {
			indexes = append(indexes, index)
		}
	}

	return indexes
}

// DropIndex elimina un índice
func (im *IndexManager) DropIndex(name string) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	if _, exists := im.Indexes[name]; !exists {
		return fmt.Errorf("índice %s no encontrado", name)
	}

	delete(im.Indexes, name)
	return nil
}

// AddDocument añade un documento a todos los índices de su colección
func (im *IndexManager) AddDocument(doc *Document) error {
	indexes := im.GetIndexesForCollection(doc.Collection)
	for _, index := range indexes {
		if err := index.AddDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

// RemoveDocument elimina un documento de todos los índices de su colección
func (im *IndexManager) RemoveDocument(doc *Document) {
	indexes := im.GetIndexesForCollection(doc.Collection)
	for _, index := range indexes {
		index.RemoveDocument(doc.ID)
	}
}

// UpdateDocument actualiza un documento en todos los índices de su colección
func (im *IndexManager) UpdateDocument(doc *Document) error {
	indexes := im.GetIndexesForCollection(doc.Collection)
	for _, index := range indexes {
		if err := index.UpdateDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

// FindDocumentsByIndex busca documentos usando un índice específico
func (im *IndexManager) FindDocumentsByIndex(indexName string, value string) ([]string, error) {
	index, exists := im.GetIndex(indexName)
	if !exists {
		return nil, fmt.Errorf("índice %s no encontrado", indexName)
	}

	return index.Search(value), nil
}

// FindDocumentsByField busca documentos por un campo específico
func (im *IndexManager) FindDocumentsByField(collection string, field string, value string) ([]string, error) {
	// Buscar un índice que contenga solo este campo
	var index *Index
	for _, idx := range im.GetIndexesForCollection(collection) {
		if len(idx.Fields) == 1 && idx.Fields[0] == field {
			index = idx
			break
		}
	}

	if index == nil {
		return nil, fmt.Errorf("no hay índice para el campo %s en la colección %s", field, collection)
	}

	return index.Search(value), nil
}

// RebuildIndex reconstruye un índice con todos los documentos de la colección
func (im *IndexManager) RebuildIndex(indexName string, documents []*Document) error {
	index, exists := im.GetIndex(indexName)
	if !exists {
		return fmt.Errorf("índice %s no encontrado", indexName)
	}

	// Limpiar índice
	index.mutex.Lock()
	index.Data = make(map[string][]string)
	index.mutex.Unlock()

	// Añadir documentos
	for _, doc := range documents {
		if doc.Collection == index.Collection {
			if err := index.AddDocument(doc); err != nil {
				return err
			}
		}
	}

	return nil
}

// RebuildAllIndexes reconstruye todos los índices
func (im *IndexManager) RebuildAllIndexes(documents []*Document) error {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	for name := range im.Indexes {
		if err := im.RebuildIndex(name, documents); err != nil {
			return err
		}
	}

	return nil
}
