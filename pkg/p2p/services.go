package p2p

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// MDNSService representa el servicio de descubrimiento mDNS
type MDNSService struct {
	host     host.Host
	interval time.Duration
	service  string
	ctx      context.Context
	cancel   context.CancelFunc
}

// MDNSOption representa una opción para el servicio mDNS
type MDNSOption func(*MDNSService)

// WithServiceName establece el nombre del servicio mDNS
func WithServiceName(service string) MDNSOption {
	return func(s *MDNSService) {
		s.service = service
	}
}

// WithInterval establece el intervalo de descubrimiento
func WithInterval(seconds int) MDNSOption {
	return func(s *MDNSService) {
		s.interval = time.Duration(seconds) * time.Second
	}
}

// NewMDNSService crea un nuevo servicio mDNS
func NewMDNSService(ctx context.Context, h host.Host, opts ...MDNSOption) (*MDNSService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear servicio con valores predeterminados
	service := &MDNSService{
		host:     h,
		interval: 10 * time.Second,
		service:  "dbp2p",
		ctx:      ctx,
		cancel:   cancel,
	}

	// Aplicar opciones
	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// Start inicia el servicio mDNS
func (s *MDNSService) Start() error {
	log.Printf("Iniciando servicio mDNS con nombre '%s' e intervalo %v", s.service, s.interval)
	return nil
}

// Stop detiene el servicio mDNS
func (s *MDNSService) Stop() error {
	s.cancel()
	log.Printf("Servicio mDNS detenido")
	return nil
}

// DHTService representa el servicio DHT
type DHTService struct {
	host              host.Host
	mode              string
	bootstrapPeers    []string
	bootstrapInterval time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
}

// DHTOption representa una opción para el servicio DHT
type DHTOption func(*DHTService)

// WithDHTMode establece el modo del DHT
func WithDHTMode(mode string) DHTOption {
	return func(s *DHTService) {
		s.mode = mode
	}
}

// WithBootstrapPeers establece los peers de bootstrap
func WithBootstrapPeers(peers []string) DHTOption {
	return func(s *DHTService) {
		s.bootstrapPeers = peers
	}
}

// WithBootstrapInterval establece el intervalo de bootstrap
func WithBootstrapInterval(seconds int) DHTOption {
	return func(s *DHTService) {
		s.bootstrapInterval = time.Duration(seconds) * time.Second
	}
}

// NewDHTService crea un nuevo servicio DHT
func NewDHTService(ctx context.Context, h host.Host, opts ...DHTOption) (*DHTService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear servicio con valores predeterminados
	service := &DHTService{
		host:              h,
		mode:              "client",
		bootstrapInterval: 300 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Aplicar opciones
	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// Start inicia el servicio DHT
func (s *DHTService) Start() error {
	log.Printf("Iniciando servicio DHT en modo '%s'", s.mode)
	return nil
}

// Stop detiene el servicio DHT
func (s *DHTService) Stop() error {
	s.cancel()
	log.Printf("Servicio DHT detenido")
	return nil
}

// PubSubService representa el servicio de publicación/suscripción
type PubSubService struct {
	host   host.Host
	pubsub *pubsub.PubSub
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPubSub crea un nuevo servicio de publicación/suscripción
func NewPubSub(ctx context.Context, node *Node) (*PubSubService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear PubSub
	ps, err := pubsub.NewGossipSub(ctx, node.Host)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error al crear PubSub: %v", err)
	}

	// Crear servicio
	service := &PubSubService{
		host:   node.Host,
		pubsub: ps,
		ctx:    ctx,
		cancel: cancel,
	}

	// Asignar el servicio al nodo
	node.PubSub = service

	return service, nil
}

// GetPubSub devuelve la instancia de PubSub
func (s *PubSubService) GetPubSub() *pubsub.PubSub {
	return s.pubsub
}

// Stop detiene el servicio PubSub
func (s *PubSubService) Stop() error {
	s.cancel()
	log.Printf("Servicio PubSub detenido")
	return nil
}

// GetPeers devuelve los peers conectados
func (s *PubSubService) GetPeers() []peer.ID {
	return s.host.Network().Peers()
}
