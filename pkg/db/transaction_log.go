package db

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TransactionType representa el tipo de transacción
type TransactionType string

const (
	// Tipos de transacciones
	TransactionCreate TransactionType = "create"
	TransactionUpdate TransactionType = "update"
	TransactionDelete TransactionType = "delete"
)

// Transaction representa una transacción en el log
type Transaction struct {
	ID        string          `json:"id"`
	Type      TransactionType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	DocID     string          `json:"doc_id"`
	Document  *Document       `json:"document,omitempty"`
}

// TransactionLogger maneja el registro de transacciones
type TransactionLogger struct {
	file   *os.File
	mutex  sync.Mutex
	closed bool
}

// NewTransactionLogger crea una nueva instancia del logger de transacciones
func NewTransactionLogger(filePath string) (*TransactionLogger, error) {
	// Abrir el archivo en modo append (o crearlo si no existe)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo de transacciones: %v", err)
	}

	return &TransactionLogger{
		file:   file,
		closed: false,
	}, nil
}

// LogOperation registra una operación en el log de transacciones
func (tl *TransactionLogger) LogOperation(op Operation, docID string, doc *Document) error {
	if tl.closed {
		return fmt.Errorf("logger de transacciones cerrado")
	}

	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Crear la transacción
	transaction := Transaction{
		ID:        generateUUID(),
		Timestamp: time.Now(),
		DocID:     docID,
		Document:  doc,
	}

	// Mapear la operación al tipo de transacción
	switch op {
	case OperationCreate:
		transaction.Type = TransactionCreate
	case OperationUpdate:
		transaction.Type = TransactionUpdate
	case OperationDelete:
		transaction.Type = TransactionDelete
	default:
		return fmt.Errorf("tipo de operación desconocido: %s", op)
	}

	// Serializar la transacción a JSON
	data, err := json.Marshal(transaction)
	if err != nil {
		return fmt.Errorf("error al serializar transacción: %v", err)
	}

	// Añadir un salto de línea al final
	data = append(data, '\n')

	// Escribir en el archivo
	if _, err := tl.file.Write(data); err != nil {
		return fmt.Errorf("error al escribir transacción: %v", err)
	}

	// Forzar la escritura en disco
	if err := tl.file.Sync(); err != nil {
		return fmt.Errorf("error al sincronizar archivo: %v", err)
	}

	return nil
}

// ReadTransactions lee todas las transacciones del log
func (tl *TransactionLogger) ReadTransactions() ([]Transaction, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Cerrar el archivo actual
	currentFile := tl.file
	currentFile.Close()

	// Reabrir el archivo para lectura
	file, err := os.Open(currentFile.Name())
	if err != nil {
		// Reabrir el archivo original en modo append
		tl.file, _ = os.OpenFile(currentFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		return nil, fmt.Errorf("error al abrir archivo para lectura: %v", err)
	}
	defer file.Close()

	// Leer el archivo línea por línea
	var transactions []Transaction
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var transaction Transaction
		if err := decoder.Decode(&transaction); err != nil {
			// Reabrir el archivo original en modo append
			tl.file, _ = os.OpenFile(currentFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			return nil, fmt.Errorf("error al decodificar transacción: %v", err)
		}
		transactions = append(transactions, transaction)
	}

	// Reabrir el archivo original en modo append
	tl.file, _ = os.OpenFile(currentFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	return transactions, nil
}

// Close cierra el logger de transacciones
func (tl *TransactionLogger) Close() error {
	if tl.closed {
		return nil
	}

	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	tl.closed = true
	return tl.file.Close()
}

// ReplayTransactions reproduce las transacciones en la base de datos
func ReplayTransactions(db *Database, transactions []Transaction) error {
	for _, transaction := range transactions {
		switch transaction.Type {
		case TransactionCreate:
			if transaction.Document != nil {
				// Añadir directamente el documento al almacén local
				db.mutex.Lock()
				db.documents[transaction.Document.ID] = transaction.Document
				db.mutex.Unlock()
			}

		case TransactionUpdate:
			if transaction.Document != nil {
				// Actualizar el documento en el almacén local
				db.mutex.Lock()
				db.documents[transaction.Document.ID] = transaction.Document
				db.mutex.Unlock()
			}

		case TransactionDelete:
			// Eliminar el documento del almacén local
			db.mutex.Lock()
			delete(db.documents, transaction.DocID)
			db.mutex.Unlock()

		default:
			return fmt.Errorf("tipo de transacción desconocido: %s", transaction.Type)
		}
	}

	return nil
}

// generateUUID genera un UUID para las transacciones
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
