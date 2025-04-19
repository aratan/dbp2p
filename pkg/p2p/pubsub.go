package p2p

import (
	"context"
	"fmt"
	"log"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PubSubService representa el servicio de publicación/suscripción
type PubSubService struct {
	host     host.Host
	pubsub   *pubsub.PubSub
	topics   map[string]*pubsub.Topic
	subs     map[string]*pubsub.Subscription
	handlers map[string]MessageHandler
	ctx      context.Context
	cancel   context.CancelFunc
	mutex    sync.RWMutex
	running  bool
}

// MessageHandler es una función que maneja mensajes recibidos
type MessageHandler func(msg *pubsub.Message) error

// NewPubSubService crea un nuevo servicio de publicación/suscripción
func NewPubSubService(ctx context.Context, h host.Host) (*PubSubService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear GossipSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error al crear GossipSub: %v", err)
	}

	// Crear servicio
	service := &PubSubService{
		host:     h,
		pubsub:   ps,
		topics:   make(map[string]*pubsub.Topic),
		subs:     make(map[string]*pubsub.Subscription),
		handlers: make(map[string]MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
		running:  true,
	}

	log.Println("Servicio PubSub inicializado")
	return service, nil
}

// NewPubSub crea una nueva instancia de Pub/Sub (para compatibilidad con código existente)
func NewPubSub(ctx context.Context, node *Node) (*pubsub.PubSub, error) {
	// Crear servicio PubSub si no existe
	if node.PubSub == nil {
		service, err := NewPubSubService(ctx, node.Host)
		if err != nil {
			return nil, err
		}
		node.PubSub = service
	}

	return node.PubSub.pubsub, nil
}

// Stop detiene el servicio de publicación/suscripción
func (s *PubSubService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	// Cancelar todas las suscripciones
	for topic, sub := range s.subs {
		sub.Cancel()
		delete(s.subs, topic)
		log.Printf("Suscripción cancelada para tema '%s'", topic)
	}

	// Cerrar todos los temas
	s.topics = make(map[string]*pubsub.Topic)

	// Cancelar contexto
	s.cancel()
	s.running = false
	log.Println("Servicio PubSub detenido")
}

// JoinTopic se une a un tema y devuelve el tema
func (s *PubSubService) JoinTopic(topicName string) (*pubsub.Topic, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verificar si ya estamos unidos al tema
	if topic, exists := s.topics[topicName]; exists {
		return topic, nil
	}

	// Unirse al tema
	topic, err := s.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("error al unirse al tema '%s': %v", topicName, err)
	}

	// Almacenar el tema
	s.topics[topicName] = topic
	log.Printf("Unido al tema '%s'", topicName)

	return topic, nil
}

// Subscribe se suscribe a un tema y configura un manejador para los mensajes
func (s *PubSubService) Subscribe(topicName string, handler MessageHandler) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verificar si ya estamos suscritos al tema
	if _, exists := s.subs[topicName]; exists {
		return fmt.Errorf("ya suscrito al tema '%s'", topicName)
	}

	// Obtener o crear el tema
	topic, err := s.JoinTopic(topicName)
	if err != nil {
		return err
	}

	// Suscribirse al tema
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("error al suscribirse al tema '%s': %v", topicName, err)
	}

	// Almacenar la suscripción y el manejador
	s.subs[topicName] = sub
	s.handlers[topicName] = handler

	// Iniciar goroutine para manejar mensajes
	go s.handleMessages(topicName, sub, handler)

	log.Printf("Suscrito al tema '%s'", topicName)
	return nil
}

// Publish publica un mensaje en un tema
func (s *PubSubService) Publish(topicName string, data []byte) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Obtener el tema
	topic, exists := s.topics[topicName]
	if !exists {
		var err error
		topic, err = s.JoinTopic(topicName)
		if err != nil {
			return err
		}
	}

	// Publicar el mensaje
	if err := topic.Publish(s.ctx, data); err != nil {
		return fmt.Errorf("error al publicar en tema '%s': %v", topicName, err)
	}

	log.Printf("Mensaje publicado en tema '%s'", topicName)
	return nil
}

// Unsubscribe cancela la suscripción a un tema
func (s *PubSubService) Unsubscribe(topicName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verificar si estamos suscritos al tema
	sub, exists := s.subs[topicName]
	if !exists {
		return
	}

	// Cancelar la suscripción
	sub.Cancel()
	delete(s.subs, topicName)
	delete(s.handlers, topicName)

	log.Printf("Suscripción cancelada para tema '%s'", topicName)
}

// GetPeers devuelve los peers conectados a un tema
func (s *PubSubService) GetPeers(topicName string) []peer.ID {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Obtener el tema
	topic, exists := s.topics[topicName]
	if !exists {
		return []peer.ID{}
	}

	// Obtener los peers
	return topic.ListPeers()
}

// handleMessages maneja los mensajes recibidos de un tema
func (s *PubSubService) handleMessages(topicName string, sub *pubsub.Subscription, handler MessageHandler) {
	for {
		// Verificar si el contexto ha sido cancelado
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Obtener el siguiente mensaje
		msg, err := sub.Next(s.ctx)
		if err != nil {
			log.Printf("Error al recibir mensaje de tema '%s': %v", topicName, err)
			continue
		}

		// Ignorar mensajes propios
		if msg.ReceivedFrom == s.host.ID() {
			continue
		}

		// Manejar el mensaje
		if err := handler(msg); err != nil {
			log.Printf("Error al manejar mensaje de tema '%s': %v", topicName, err)
		}
	}
}

// GetPubSub devuelve el PubSub subyacente
func (s *PubSubService) GetPubSub() *pubsub.PubSub {
	return s.pubsub
}
