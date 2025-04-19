package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Operación representa el tipo de operación de base de datos
type Operation string

const (
	// Operaciones CRUD
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationDelete Operation = "delete"
)

// DBMessage representa un mensaje de sincronización de base de datos
type DBMessage struct {
	Operation  Operation `json:"operation"`
	Document   *Document `json:"document,omitempty"`
	DocumentID string    `json:"document_id,omitempty"`
}

// DBSync maneja la sincronización de la base de datos entre nodos
type DBSync struct {
	db      *Database
	topic   *pubsub.Topic
	pubsub  *pubsub.PubSub
	nodeID  string
	enabled bool
	sub     *pubsub.Subscription
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewDBSync crea una nueva instancia de sincronización de base de datos
func NewDBSync(ctx context.Context, db *Database, ps *pubsub.PubSub, nodeID string) (*DBSync, error) {
	log.Printf("Creando nueva instancia de sincronización de base de datos...")

	// Crear contexto cancelable
	syncCtx, cancel := context.WithCancel(ctx)

	// Crear o unirse al tema de sincronización de base de datos
	topic, err := ps.Join("db-sync")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error al unirse al tema de sincronización: %v", err)
	}

	log.Printf("Unido al tema de sincronización 'db-sync'")

	// Suscribirse al tema
	sub, err := topic.Subscribe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error al suscribirse al tema: %v", err)
	}

	log.Printf("Suscrito al tema de sincronización")

	// Crear la instancia de sincronización
	sync := &DBSync{
		db:      db,
		topic:   topic,
		pubsub:  ps,
		nodeID:  nodeID,
		enabled: true,
		sub:     sub,
		ctx:     syncCtx,
		cancel:  cancel,
	}

	// Iniciar la escucha de mensajes
	go sync.listenForUpdates(syncCtx, sub)

	log.Printf("Sincronización de base de datos iniciada correctamente")
	return sync, nil
}

// PublishCreate publica un mensaje de creación de documento
func (s *DBSync) PublishCreate(doc *Document) error {
	msg := DBMessage{
		Operation: OperationCreate,
		Document:  doc,
	}
	return s.publishMessage(msg)
}

// PublishUpdate publica un mensaje de actualización de documento
func (s *DBSync) PublishUpdate(doc *Document) error {
	msg := DBMessage{
		Operation: OperationUpdate,
		Document:  doc,
	}
	return s.publishMessage(msg)
}

// PublishDelete publica un mensaje de eliminación de documento
func (s *DBSync) PublishDelete(docID string) error {
	msg := DBMessage{
		Operation:  OperationDelete,
		DocumentID: docID,
	}
	return s.publishMessage(msg)
}

// publishMessage serializa y publica un mensaje en el tema
func (s *DBSync) publishMessage(msg DBMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Usar un contexto con timeout para evitar bloqueos indefinidos
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.topic.Publish(ctx, data)
	if err != nil {
		log.Printf("Error al publicar mensaje de sincronización: %v", err)
		return err
	}

	log.Printf("Mensaje de sincronización publicado: %s - %s", msg.Operation, getDocumentID(msg))
	return nil
}

// getDocumentID obtiene el ID del documento de un mensaje
func getDocumentID(msg DBMessage) string {
	if msg.Document != nil {
		return msg.Document.ID
	}
	return msg.DocumentID
}

// listenForUpdates escucha mensajes de actualización de la base de datos
func (s *DBSync) listenForUpdates(ctx context.Context, sub *pubsub.Subscription) {
	log.Printf("Iniciando escucha de actualizaciones de sincronización...")
	for {
		// Verificar si el contexto ha sido cancelado
		select {
		case <-ctx.Done():
			log.Printf("Deteniendo escucha de actualizaciones de sincronización")
			return
		default:
			// Continuar
		}

		msg, err := sub.Next(ctx)
		if err != nil {
			log.Printf("Error recibiendo mensaje de sincronización: %v", err)
			continue
		}

		// Ignorar mensajes propios
		if msg.ReceivedFrom.String() == s.nodeID {
			// log.Printf("Ignorando mensaje propio")
			continue
		}

		log.Printf("Mensaje de sincronización recibido de: %s", msg.ReceivedFrom.String())

		// Deserializar el mensaje
		var dbMsg DBMessage
		if err := json.Unmarshal(msg.Data, &dbMsg); err != nil {
			log.Printf("Error deserializando mensaje: %v", err)
			continue
		}

		// Procesar el mensaje según la operación
		switch dbMsg.Operation {
		case OperationCreate:
			if dbMsg.Document != nil {
				// Añadir directamente el documento al almacén local
				s.db.mutex.Lock()
				s.db.documents[dbMsg.Document.ID] = dbMsg.Document
				s.db.mutex.Unlock()

				// Persistir el documento si está habilitada la persistencia
				if s.db.persistenceEnabled {
					if err := s.db.persistence.SaveDocument(dbMsg.Document); err != nil {
						log.Printf("Error al persistir documento sincronizado: %v", err)
					}
				}

				fmt.Printf("Documento sincronizado (creado): %s\n", dbMsg.Document.ID)
			}

		case OperationUpdate:
			if dbMsg.Document != nil {
				// Actualizar el documento en el almacén local
				s.db.mutex.Lock()
				s.db.documents[dbMsg.Document.ID] = dbMsg.Document
				s.db.mutex.Unlock()

				// Persistir el documento si está habilitada la persistencia
				if s.db.persistenceEnabled {
					if err := s.db.persistence.UpdateDocument(dbMsg.Document); err != nil {
						log.Printf("Error al persistir actualización sincronizada: %v", err)
					}
				}

				fmt.Printf("Documento sincronizado (actualizado): %s\n", dbMsg.Document.ID)
			}

		case OperationDelete:
			if dbMsg.DocumentID != "" {
				// Obtener la colección del documento antes de eliminarlo
				var collection string
				s.db.mutex.RLock()
				if doc, exists := s.db.documents[dbMsg.DocumentID]; exists {
					collection = doc.Collection
				}
				s.db.mutex.RUnlock()

				// Eliminar el documento del almacén local
				s.db.mutex.Lock()
				delete(s.db.documents, dbMsg.DocumentID)
				s.db.mutex.Unlock()

				// Persistir la eliminación si está habilitada la persistencia y se conoce la colección
				if s.db.persistenceEnabled && collection != "" {
					if err := s.db.persistence.DeleteDocument(collection, dbMsg.DocumentID); err != nil {
						log.Printf("Error al persistir eliminación sincronizada: %v", err)
					}
				}

				fmt.Printf("Documento sincronizado (eliminado): %s\n", dbMsg.DocumentID)
			}
		}
	}
}

// Close cierra la sincronización de base de datos
func (s *DBSync) Close() error {
	if s.cancel != nil {
		s.cancel()
	}

	if s.sub != nil {
		s.sub.Cancel()
	}

	s.enabled = false
	log.Printf("Sincronización de base de datos cerrada")
	return nil
}

// IsEnabled verifica si la sincronización está habilitada
func (s *DBSync) IsEnabled() bool {
	return s.enabled
}

// SyncAllDocuments sincroniza todos los documentos con la red
func (s *DBSync) SyncAllDocuments() error {
	if !s.enabled {
		return fmt.Errorf("sincronización no habilitada")
	}

	log.Printf("Iniciando sincronización completa de documentos...")

	// Obtener todos los documentos
	s.db.mutex.RLock()
	documents := make([]*Document, 0, len(s.db.documents))
	for _, doc := range s.db.documents {
		documents = append(documents, doc)
	}
	s.db.mutex.RUnlock()

	// Publicar cada documento
	for _, doc := range documents {
		err := s.PublishCreate(doc)
		if err != nil {
			log.Printf("Error al sincronizar documento %s: %v", doc.ID, err)
		}

		// Pequeña pausa para no saturar la red
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Sincronización completa finalizada: %d documentos sincronizados", len(documents))
	return nil
}
