package db

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Transaction representa una transacción en la base de datos
type Transaction struct {
	Operation  Operation  `json:"operation"`
	DocumentID string     `json:"document_id"`
	Document   *Document  `json:"document,omitempty"`
}

// TransactionLogger registra las transacciones en un archivo
type TransactionLogger struct {
	file   *os.File
	mutex  sync.Mutex
	writer *bufio.Writer
}

// NewTransactionLogger crea un nuevo logger de transacciones
func NewTransactionLogger(filePath string) (*TransactionLogger, error) {
	// Crear directorio si no existe
	dir := filePath[:len(filePath)-len("/transactions.log")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio para log de transacciones: %v", err)
	}

	// Abrir archivo en modo append
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo de log: %v", err)
	}

	return &TransactionLogger{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// LogOperation registra una operación en el log
func (tl *TransactionLogger) LogOperation(op Operation, docID string, doc *Document) error {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Crear transacción
	transaction := Transaction{
		Operation:  op,
		DocumentID: docID,
		Document:   doc,
	}

	// Serializar a JSON
	data, err := json.Marshal(transaction)
	if err != nil {
		return fmt.Errorf("error al serializar transacción: %v", err)
	}

	// Escribir en el archivo
	if _, err := tl.writer.Write(data); err != nil {
		return fmt.Errorf("error al escribir transacción: %v", err)
	}

	// Escribir nueva línea
	if err := tl.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("error al escribir nueva línea: %v", err)
	}

	// Flush para asegurar que se escriba en el archivo
	if err := tl.writer.Flush(); err != nil {
		return fmt.Errorf("error al hacer flush: %v", err)
	}

	return nil
}

// ReadTransactions lee todas las transacciones del log
func (tl *TransactionLogger) ReadTransactions() ([]Transaction, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Cerrar y reabrir el archivo en modo lectura
	tl.writer.Flush()
	tl.file.Close()

	file, err := os.Open(tl.file.Name())
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo para lectura: %v", err)
	}
	defer file.Close()

	// Leer transacciones
	var transactions []Transaction
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var transaction Transaction
		if err := json.Unmarshal([]byte(line), &transaction); err != nil {
			return nil, fmt.Errorf("error al deserializar transacción: %v", err)
		}

		transactions = append(transactions, transaction)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error al leer transacciones: %v", err)
	}

	// Reabrir el archivo en modo append
	tl.file, err = os.OpenFile(tl.file.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error al reabrir archivo: %v", err)
	}
	tl.writer = bufio.NewWriter(tl.file)

	return transactions, nil
}

// Close cierra el logger
func (tl *TransactionLogger) Close() error {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	if err := tl.writer.Flush(); err != nil {
		return err
	}

	return tl.file.Close()
}

// ReplayTransactions aplica las transacciones a la base de datos
func ReplayTransactions(db *Database, transactions []Transaction) error {
	for _, transaction := range transactions {
		switch transaction.Operation {
		case OperationCreate:
			if transaction.Document != nil {
				db.documents[transaction.DocumentID] = transaction.Document
			}
		case OperationUpdate:
			if transaction.Document != nil {
				db.documents[transaction.DocumentID] = transaction.Document
			}
		case OperationDelete:
			delete(db.documents, transaction.DocumentID)
		}
	}

	return nil
}
