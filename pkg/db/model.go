package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Document representa un documento en la base de datos NoSQL
type Document struct {
	ID         string         `json:"id"`
	Collection string         `json:"collection"`
	Data       map[string]any `json:"data"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// EventCallback es una función que se llama cuando ocurre un evento en la base de datos
type EventCallback func(eventType string, collection string, documentID string, document *Document)

// Database representa la base de datos NoSQL
type Database struct {
	documents          map[string]*Document // Mapa de ID a documento
	mutex              sync.RWMutex
	persistence        *PersistenceManager
	dataDir            string
	persistenceEnabled bool
	eventCallbacks     []EventCallback // Callbacks para eventos
	sync               *DBSync         // Gestor de sincronización P2P
	syncEnabled        bool            // Indica si la sincronización está habilitada
}

// NewDatabase crea una nueva instancia de la base de datos
func NewDatabase() *Database {
	return &Database{
		documents:          make(map[string]*Document),
		persistenceEnabled: false,
		eventCallbacks:     []EventCallback{},
	}
}

// NewDatabaseWithPersistence crea una nueva instancia de la base de datos con persistencia
func NewDatabaseWithPersistence(dataDir string) (*Database, error) {
	// Crear el gestor de persistencia
	persistence, err := NewPersistenceManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("error al crear gestor de persistencia: %v", err)
	}

	// Cargar documentos existentes
	documents, err := persistence.LoadAllDocuments()
	if err != nil {
		return nil, fmt.Errorf("error al cargar documentos: %v", err)
	}

	// Crear la base de datos
	db := &Database{
		documents:          documents,
		persistence:        persistence,
		dataDir:            dataDir,
		persistenceEnabled: true,
		eventCallbacks:     []EventCallback{},
	}

	// Reproducir transacciones pendientes
	transactions, err := persistence.transactionLog.ReadTransactions()
	if err != nil {
		return nil, fmt.Errorf("error al leer transacciones: %v", err)
	}

	// Aplicar transacciones
	if err := ReplayTransactions(db, transactions); err != nil {
		return nil, fmt.Errorf("error al reproducir transacciones: %v", err)
	}

	return db, nil
}

// RegisterEventCallback registra un callback para eventos de la base de datos
func (db *Database) RegisterEventCallback(callback EventCallback) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.eventCallbacks = append(db.eventCallbacks, callback)
}

// triggerEvent dispara un evento para todos los callbacks registrados
func (db *Database) triggerEvent(eventType string, collection string, documentID string, document *Document) {
	for _, callback := range db.eventCallbacks {
		go callback(eventType, collection, documentID, document)
	}
}

// CreateDocument crea un nuevo documento en la colección especificada
func (db *Database) CreateDocument(collection string, data map[string]any) (*Document, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Generar un ID único para el documento
	id := uuid.New().String()

	// Crear el documento
	now := time.Now()
	doc := &Document{
		ID:         id,
		Collection: collection,
		Data:       data,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Almacenar el documento
	db.documents[id] = doc

	// Persistir el documento si está habilitada la persistencia
	if db.persistenceEnabled {
		if err := db.persistence.SaveDocument(doc); err != nil {
			return nil, fmt.Errorf("error al persistir documento: %v", err)
		}
	}

	// Sincronizar documento si está habilitada la sincronización
	if db.syncEnabled && db.sync != nil {
		if err := db.sync.PublishCreate(doc); err != nil {
			log.Printf("Error al sincronizar documento: %v", err)
			// No devolvemos error para no bloquear la operación
		}
	}

	// Disparar evento de creación
	db.triggerEvent("create", collection, id, doc)

	return doc, nil
}

// GetDocument obtiene un documento por su ID
func (db *Database) GetDocument(id string) (*Document, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	doc, exists := db.documents[id]
	if !exists {
		return nil, errors.New("documento no encontrado")
	}

	return doc, nil
}

// QueryDocuments busca documentos en una colección que coincidan con los criterios
func (db *Database) QueryDocuments(collection string, query map[string]any) ([]*Document, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	var results []*Document

	// Buscar documentos que coincidan con la colección y los criterios
	for _, doc := range db.documents {
		if doc.Collection != collection {
			continue
		}

		// Verificar si el documento coincide con todos los criterios
		matches := true
		for key, value := range query {
			if docValue, exists := doc.Data[key]; !exists || docValue != value {
				matches = false
				break
			}
		}

		if matches {
			results = append(results, doc)
		}
	}

	return results, nil
}

// UpdateDocument actualiza un documento existente
func (db *Database) UpdateDocument(id string, data map[string]any) (*Document, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	doc, exists := db.documents[id]
	if !exists {
		return nil, errors.New("documento no encontrado")
	}

	// Actualizar los datos
	maps.Copy(doc.Data, data)
	doc.UpdatedAt = time.Now()

	// Persistir el documento si está habilitada la persistencia
	if db.persistenceEnabled {
		if err := db.persistence.UpdateDocument(doc); err != nil {
			return nil, fmt.Errorf("error al persistir actualización: %v", err)
		}
	}

	// Sincronizar documento si está habilitada la sincronización
	if db.syncEnabled && db.sync != nil {
		if err := db.sync.PublishUpdate(doc); err != nil {
			log.Printf("Error al sincronizar actualización: %v", err)
			// No devolvemos error para no bloquear la operación
		}
	}

	// Disparar evento de actualización
	db.triggerEvent("update", doc.Collection, id, doc)

	return doc, nil
}

// DeleteDocument elimina un documento por su ID
func (db *Database) DeleteDocument(id string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	doc, exists := db.documents[id]
	if !exists {
		return errors.New("documento no encontrado")
	}

	// Guardar una copia del documento para el evento
	docCopy := *doc
	collection := doc.Collection

	// Eliminar el documento
	delete(db.documents, id)

	// Persistir la eliminación si está habilitada la persistencia
	if db.persistenceEnabled {
		if err := db.persistence.DeleteDocument(collection, id); err != nil {
			return fmt.Errorf("error al persistir eliminación: %v", err)
		}
	}

	// Sincronizar eliminación si está habilitada la sincronización
	if db.syncEnabled && db.sync != nil {
		if err := db.sync.PublishDelete(id); err != nil {
			log.Printf("Error al sincronizar eliminación: %v", err)
			// No devolvemos error para no bloquear la operación
		}
	}

	// Disparar evento de eliminación
	db.triggerEvent("delete", collection, id, &docCopy)

	return nil
}

// GetAllDocuments devuelve todos los documentos de una colección
func (db *Database) GetAllDocuments(collection string) ([]*Document, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	var results []*Document
	for _, doc := range db.documents {
		if doc.Collection == collection {
			results = append(results, doc)
		}
	}

	return results, nil
}

// SerializeDocument convierte un documento a JSON
func SerializeDocument(doc *Document) ([]byte, error) {
	return json.Marshal(doc)
}

// DeserializeDocument convierte JSON a un documento
func DeserializeDocument(data []byte) (*Document, error) {
	var doc Document
	err := json.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// String devuelve una representación en cadena del documento
func (d *Document) String() string {
	data, _ := json.MarshalIndent(d, "", "  ")
	return fmt.Sprintf("%s", data)
}

// CreateBackup crea una copia de seguridad de la base de datos
func (db *Database) CreateBackup() (string, error) {
	if !db.persistenceEnabled {
		return "", errors.New("persistencia no habilitada")
	}

	return db.persistence.CreateBackup()
}

// RestoreFromBackup restaura la base de datos desde una copia de seguridad
func (db *Database) RestoreFromBackup(backupName string) error {
	if !db.persistenceEnabled {
		return errors.New("persistencia no habilitada")
	}

	// Restaurar desde el backup
	if err := db.persistence.RestoreFromBackup(backupName); err != nil {
		return err
	}

	// Recargar los documentos
	documents, err := db.persistence.LoadAllDocuments()
	if err != nil {
		return fmt.Errorf("error al recargar documentos: %v", err)
	}

	// Actualizar los documentos en memoria
	db.mutex.Lock()
	db.documents = documents
	db.mutex.Unlock()

	return nil
}

// ListBackups lista todas las copias de seguridad disponibles
func (db *Database) ListBackups() ([]string, error) {
	if !db.persistenceEnabled {
		return nil, errors.New("persistencia no habilitada")
	}

	return db.persistence.ListBackups()
}

// DeleteBackup elimina una copia de seguridad
func (db *Database) DeleteBackup(backupName string) error {
	if !db.persistenceEnabled {
		return errors.New("persistencia no habilitada")
	}

	return db.persistence.DeleteBackup(backupName)
}

// SetSync establece el gestor de sincronización
func (db *Database) SetSync(sync *DBSync) {
	db.sync = sync
	db.syncEnabled = true
}

// EnableSync habilita la sincronización
func (db *Database) EnableSync() {
	db.syncEnabled = true
}

// DisableSync deshabilita la sincronización
func (db *Database) DisableSync() {
	db.syncEnabled = false
}

// IsSyncEnabled verifica si la sincronización está habilitada
func (db *Database) IsSyncEnabled() bool {
	return db.syncEnabled
}

// SyncAllDocuments sincroniza todos los documentos con la red
func (db *Database) SyncAllDocuments() error {
	if !db.syncEnabled || db.sync == nil {
		return errors.New("sincronización no habilitada o no configurada")
	}

	return db.sync.SyncAllDocuments()
}

// GetCollections obtiene todas las colecciones
func (db *Database) GetCollections() ([]string, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	// Crear mapa para evitar duplicados
	collectionsMap := make(map[string]bool)

	// Recorrer documentos y obtener colecciones
	for _, doc := range db.documents {
		collectionsMap[doc.Collection] = true
	}

	// Convertir mapa a slice
	collections := make([]string, 0, len(collectionsMap))
	for collection := range collectionsMap {
		collections = append(collections, collection)
	}

	return collections, nil
}
