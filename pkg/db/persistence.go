package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistenceManager maneja la persistencia de la base de datos
type PersistenceManager struct {
	dataDir        string
	transactionLog *TransactionLogger
	mutex          sync.Mutex
}

// NewPersistenceManager crea una nueva instancia del gestor de persistencia
func NewPersistenceManager(dataDir string) (*PersistenceManager, error) {
	// Crear directorio de datos si no existe
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de datos: %v", err)
	}

	// Crear directorio para colecciones
	collectionsDir := filepath.Join(dataDir, "collections")
	if err := os.MkdirAll(collectionsDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de colecciones: %v", err)
	}

	// Inicializar el logger de transacciones
	transactionLog, err := NewTransactionLogger(filepath.Join(dataDir, "transactions.log"))
	if err != nil {
		return nil, fmt.Errorf("error al inicializar el logger de transacciones: %v", err)
	}

	return &PersistenceManager{
		dataDir:        dataDir,
		transactionLog: transactionLog,
	}, nil
}

// SaveDocument guarda un documento en el sistema de archivos
func (pm *PersistenceManager) SaveDocument(doc *Document) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Crear directorio para la colección si no existe
	collectionDir := filepath.Join(pm.dataDir, "collections", doc.Collection)
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de colección: %v", err)
	}

	// Serializar el documento a JSON
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("error al serializar documento: %v", err)
	}

	// Guardar el documento en un archivo
	filePath := filepath.Join(collectionDir, doc.ID+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error al guardar documento: %v", err)
	}

	// Registrar la transacción
	pm.transactionLog.LogOperation(OperationCreate, doc.ID, doc)

	return nil
}

// UpdateDocument actualiza un documento en el sistema de archivos
func (pm *PersistenceManager) UpdateDocument(doc *Document) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Verificar si el documento existe
	filePath := filepath.Join(pm.dataDir, "collections", doc.Collection, doc.ID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("documento no encontrado: %s", doc.ID)
	}

	// Serializar el documento a JSON
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("error al serializar documento: %v", err)
	}

	// Guardar el documento en un archivo
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error al guardar documento: %v", err)
	}

	// Registrar la transacción
	pm.transactionLog.LogOperation(OperationUpdate, doc.ID, doc)

	return nil
}

// DeleteDocument elimina un documento del sistema de archivos
func (pm *PersistenceManager) DeleteDocument(collection, id string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Verificar si el documento existe
	filePath := filepath.Join(pm.dataDir, "collections", collection, id+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("documento no encontrado: %s", id)
	}

	// Eliminar el archivo
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error al eliminar documento: %v", err)
	}

	// Registrar la transacción
	pm.transactionLog.LogOperation(OperationDelete, id, nil)

	return nil
}

// LoadAllDocuments carga todos los documentos del sistema de archivos
func (pm *PersistenceManager) LoadAllDocuments() (map[string]*Document, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	documents := make(map[string]*Document)

	// Obtener todas las colecciones
	collectionsDir := filepath.Join(pm.dataDir, "collections")
	collections, err := os.ReadDir(collectionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Si el directorio no existe, devolver un mapa vacío
			return documents, nil
		}
		return nil, fmt.Errorf("error al leer directorio de colecciones: %v", err)
	}

	// Iterar sobre cada colección
	for _, collectionInfo := range collections {
		if !collectionInfo.IsDir() {
			continue
		}

		collectionName := collectionInfo.Name()
		collectionDir := filepath.Join(collectionsDir, collectionName)

		// Leer todos los archivos de la colección
		files, err := os.ReadDir(collectionDir)
		if err != nil {
			return nil, fmt.Errorf("error al leer directorio de colección %s: %v", collectionName, err)
		}

		// Iterar sobre cada archivo (documento)
		for _, fileInfo := range files {
			if fileInfo.IsDir() || filepath.Ext(fileInfo.Name()) != ".json" {
				continue
			}

			// Leer el archivo
			filePath := filepath.Join(collectionDir, fileInfo.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("error al leer documento %s: %v", fileInfo.Name(), err)
			}

			// Deserializar el documento
			var doc Document
			if err := json.Unmarshal(data, &doc); err != nil {
				return nil, fmt.Errorf("error al deserializar documento %s: %v", fileInfo.Name(), err)
			}

			// Añadir el documento al mapa
			documents[doc.ID] = &doc
		}
	}

	return documents, nil
}

// CreateBackup crea una copia de seguridad de la base de datos
func (pm *PersistenceManager) CreateBackup() (string, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Crear directorio de backups si no existe
	backupsDir := filepath.Join(pm.dataDir, "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return "", fmt.Errorf("error al crear directorio de backups: %v", err)
	}

	// Crear nombre para el backup con timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s", timestamp)
	backupDir := filepath.Join(backupsDir, backupName)

	// Crear directorio para el backup
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("error al crear directorio de backup: %v", err)
	}

	// Copiar directorio de colecciones
	collectionsDir := filepath.Join(pm.dataDir, "collections")
	backupCollectionsDir := filepath.Join(backupDir, "collections")
	if err := copyDir(collectionsDir, backupCollectionsDir); err != nil {
		return "", fmt.Errorf("error al copiar colecciones: %v", err)
	}

	// Copiar archivo de transacciones
	transactionLogPath := filepath.Join(pm.dataDir, "transactions.log")
	backupTransactionLogPath := filepath.Join(backupDir, "transactions.log")
	if err := copyFile(transactionLogPath, backupTransactionLogPath); err != nil {
		return "", fmt.Errorf("error al copiar log de transacciones: %v", err)
	}

	return backupName, nil
}

// RestoreFromBackup restaura la base de datos desde una copia de seguridad
func (pm *PersistenceManager) RestoreFromBackup(backupName string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Verificar si el backup existe
	backupDir := filepath.Join(pm.dataDir, "backups", backupName)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup no encontrado: %s", backupName)
	}

	// Eliminar directorio de colecciones actual
	collectionsDir := filepath.Join(pm.dataDir, "collections")
	if err := os.RemoveAll(collectionsDir); err != nil {
		return fmt.Errorf("error al eliminar directorio de colecciones: %v", err)
	}

	// Copiar directorio de colecciones desde el backup
	backupCollectionsDir := filepath.Join(backupDir, "collections")
	if err := copyDir(backupCollectionsDir, collectionsDir); err != nil {
		return fmt.Errorf("error al restaurar colecciones: %v", err)
	}

	// Cerrar el logger de transacciones actual
	pm.transactionLog.Close()

	// Eliminar archivo de transacciones actual
	transactionLogPath := filepath.Join(pm.dataDir, "transactions.log")
	if err := os.Remove(transactionLogPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al eliminar log de transacciones: %v", err)
	}

	// Copiar archivo de transacciones desde el backup
	backupTransactionLogPath := filepath.Join(backupDir, "transactions.log")
	if err := copyFile(backupTransactionLogPath, transactionLogPath); err != nil {
		return fmt.Errorf("error al restaurar log de transacciones: %v", err)
	}

	// Reiniciar el logger de transacciones
	var err error
	pm.transactionLog, err = NewTransactionLogger(transactionLogPath)
	if err != nil {
		return fmt.Errorf("error al reiniciar el logger de transacciones: %v", err)
	}

	return nil
}

// ListBackups lista todas las copias de seguridad disponibles
func (pm *PersistenceManager) ListBackups() ([]string, error) {
	backupsDir := filepath.Join(pm.dataDir, "backups")
	if _, err := os.Stat(backupsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return nil, fmt.Errorf("error al leer directorio de backups: %v", err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// DeleteBackup elimina una copia de seguridad
func (pm *PersistenceManager) DeleteBackup(backupName string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Verificar si el backup existe
	backupDir := filepath.Join(pm.dataDir, "backups", backupName)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup no encontrado: %s", backupName)
	}

	// Eliminar el directorio de backup
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("error al eliminar backup: %v", err)
	}

	return nil
}

// Funciones auxiliares para copiar archivos y directorios

func copyFile(src, dst string) error {
	// Leer el archivo fuente
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Escribir en el archivo destino
	return os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) error {
	// Crear el directorio destino
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Leer el directorio fuente
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Copiar cada entrada
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Si es un directorio, copiarlo recursivamente
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Si es un archivo, copiarlo
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
