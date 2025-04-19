package p2p

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// MDNSService representa el servicio de descubrimiento mDNS
type MDNSService struct {
	host            host.Host
	mdnsService     mdns.Service
	serviceName     string
	interval        time.Duration
	peerChan        chan peer.AddrInfo
	discoveredPeers map[peer.ID]peer.AddrInfo
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// MDNSOption es una función que configura el servicio mDNS
type MDNSOption func(*MDNSService)

// WithServiceName establece el nombre del servicio mDNS
func WithServiceName(name string) MDNSOption {
	return func(s *MDNSService) {
		s.serviceName = name
	}
}

// WithInterval establece el intervalo de anuncio mDNS
func WithInterval(interval int) MDNSOption {
	return func(s *MDNSService) {
		s.interval = time.Duration(interval) * time.Second
	}
}

// NewMDNSService crea un nuevo servicio de descubrimiento mDNS
func NewMDNSService(ctx context.Context, h host.Host, options ...MDNSOption) (*MDNSService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear servicio con valores predeterminados
	service := &MDNSService{
		host:            h,
		serviceName:     "dbp2p",
		interval:        10 * time.Second,
		peerChan:        make(chan peer.AddrInfo),
		discoveredPeers: make(map[peer.ID]peer.AddrInfo),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Aplicar opciones
	for _, option := range options {
		option(service)
	}

	return service, nil
}

// Start inicia el servicio mDNS
func (s *MDNSService) Start() error {
	// Implementar interfaz de notificación para mDNS
	notifee := &mdnsNotifee{
		peerChan: s.peerChan,
	}

	// Crear servicio mDNS
	mdnsService := mdns.NewMdnsService(s.host, s.serviceName, notifee)

	s.mdnsService = mdnsService

	// Iniciar goroutine para manejar peers descubiertos
	go s.handlePeerDiscovery()

	log.Printf("Servicio mDNS iniciado con nombre '%s' e intervalo %v", s.serviceName, s.interval)
	return nil
}

// Stop detiene el servicio mDNS
func (s *MDNSService) Stop() {
	s.cancel()
	log.Println("Servicio mDNS detenido")
}

// handlePeerDiscovery maneja los peers descubiertos por mDNS
func (s *MDNSService) handlePeerDiscovery() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case peerInfo := <-s.peerChan:
			// Ignorar nuestro propio peer
			if peerInfo.ID == s.host.ID() {
				continue
			}

			s.mutex.Lock()
			// Verificar si ya conocemos este peer
			_, known := s.discoveredPeers[peerInfo.ID]
			if !known {
				// Almacenar el nuevo peer
				s.discoveredPeers[peerInfo.ID] = peerInfo
				log.Printf("Nuevo peer descubierto por mDNS: %s", peerInfo.ID.String())

				// Intentar conectar al peer
				go func(pi peer.AddrInfo) {
					if err := s.host.Connect(s.ctx, pi); err != nil {
						log.Printf("Error al conectar con peer %s: %v", pi.ID.String(), err)
					} else {
						log.Printf("Conectado con peer %s", pi.ID.String())
					}
				}(peerInfo)
			}
			s.mutex.Unlock()
		}
	}
}

// GetDiscoveredPeers devuelve los peers descubiertos por mDNS
func (s *MDNSService) GetDiscoveredPeers() []peer.AddrInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(s.discoveredPeers))
	for _, peerInfo := range s.discoveredPeers {
		peers = append(peers, peerInfo)
	}

	return peers
}

// mdnsNotifee implementa la interfaz mdns.Notifee
type mdnsNotifee struct {
	peerChan chan peer.AddrInfo
}

// HandlePeerFound se llama cuando se encuentra un peer mediante mDNS
func (n *mdnsNotifee) HandlePeerFound(peerInfo peer.AddrInfo) {
	n.peerChan <- peerInfo
}
