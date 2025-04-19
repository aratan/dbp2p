package p2p

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
)

// DHTMode representa el modo de operación del DHT
type DHTMode string

const (
	// DHTModeServer representa el modo servidor del DHT
	DHTModeServer DHTMode = "server"
	// DHTModeClient representa el modo cliente del DHT
	DHTModeClient DHTMode = "client"
)

// DHTService representa el servicio DHT
type DHTService struct {
	host              host.Host
	dht               *dht.IpfsDHT
	mode              DHTMode
	bootstrapPeers    []peer.AddrInfo
	bootstrapInterval time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
	mutex             sync.RWMutex
	running           bool
}

// DHTOption es una función que configura el servicio DHT
type DHTOption func(*DHTService)

// WithDHTMode establece el modo de operación del DHT
func WithDHTMode(mode string) DHTOption {
	return func(s *DHTService) {
		s.mode = DHTMode(mode)
	}
}

// WithBootstrapPeers establece los peers de bootstrap para el DHT
func WithBootstrapPeers(peers []string) DHTOption {
	return func(s *DHTService) {
		addrInfos := make([]peer.AddrInfo, 0, len(peers))
		for _, addrStr := range peers {
			addr, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				log.Printf("Error al parsear dirección de bootstrap %s: %v", addrStr, err)
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
			if err != nil {
				log.Printf("Error al convertir dirección a AddrInfo %s: %v", addrStr, err)
				continue
			}

			addrInfos = append(addrInfos, *peerInfo)
		}
		s.bootstrapPeers = addrInfos
	}
}

// WithBootstrapInterval establece el intervalo de bootstrap para el DHT
func WithBootstrapInterval(interval int) DHTOption {
	return func(s *DHTService) {
		s.bootstrapInterval = time.Duration(interval) * time.Second
	}
}

// NewDHTService crea un nuevo servicio DHT
func NewDHTService(ctx context.Context, h host.Host, options ...DHTOption) (*DHTService, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Crear servicio con valores predeterminados
	service := &DHTService{
		host:              h,
		mode:              DHTModeServer,
		bootstrapPeers:    []peer.AddrInfo{},
		bootstrapInterval: 300 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
		running:           false,
	}

	// Aplicar opciones
	for _, option := range options {
		option(service)
	}

	return service, nil
}

// Start inicia el servicio DHT
func (s *DHTService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("el servicio DHT ya está en ejecución")
	}

	// Determinar el modo DHT
	var mode dht.ModeOpt
	if s.mode == DHTModeServer {
		mode = dht.ModeServer
	} else {
		mode = dht.ModeClient
	}

	// Crear DHT
	kadDHT, err := dht.New(s.ctx, s.host, dht.Mode(mode))
	if err != nil {
		return fmt.Errorf("error al crear DHT: %v", err)
	}

	s.dht = kadDHT

	// Iniciar DHT
	if err := s.dht.Bootstrap(s.ctx); err != nil {
		return fmt.Errorf("error al iniciar bootstrap DHT: %v", err)
	}

	// Conectar a los peers de bootstrap
	if len(s.bootstrapPeers) > 0 {
		go s.connectToBootstrapPeers()
	}

	// Iniciar rutina de bootstrap periódico
	go s.periodicBootstrap()

	s.running = true
	log.Printf("Servicio DHT iniciado en modo %s", s.mode)
	return nil
}

// Stop detiene el servicio DHT
func (s *DHTService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
	log.Println("Servicio DHT detenido")
}

// connectToBootstrapPeers conecta a los peers de bootstrap
func (s *DHTService) connectToBootstrapPeers() {
	for _, peerInfo := range s.bootstrapPeers {
		if err := s.host.Connect(s.ctx, peerInfo); err != nil {
			log.Printf("Error al conectar con peer de bootstrap %s: %v", peerInfo.ID.String(), err)
		} else {
			log.Printf("Conectado con peer de bootstrap %s", peerInfo.ID.String())
		}
	}
}

// periodicBootstrap realiza bootstrap periódico del DHT
func (s *DHTService) periodicBootstrap() {
	ticker := time.NewTicker(s.bootstrapInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			err := s.dht.Bootstrap(s.ctx)
			if err != nil {
				log.Printf("Error en bootstrap periódico: %v", err)
			} else {
				log.Println("Bootstrap periódico completado")
			}
		}
	}
}

// GetDHT devuelve el DHT subyacente
func (s *DHTService) GetDHT() *dht.IpfsDHT {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.dht
}

// GetRoutingTable devuelve la tabla de enrutamiento del DHT
func (s *DHTService) GetRoutingTable() []peer.ID {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return []peer.ID{}
	}

	return s.dht.RoutingTable().ListPeers()
}

// FindPeer busca un peer en la red DHT
func (s *DHTService) FindPeer(id peer.ID) (peer.AddrInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return peer.AddrInfo{}, fmt.Errorf("DHT no inicializado")
	}

	return s.dht.FindPeer(s.ctx, id)
}

// PutValue almacena un valor en la DHT
func (s *DHTService) PutValue(key string, value []byte) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return fmt.Errorf("DHT no inicializado")
	}

	return s.dht.PutValue(s.ctx, key, value)
}

// GetValue recupera un valor de la DHT
func (s *DHTService) GetValue(key string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return nil, fmt.Errorf("DHT no inicializado")
	}

	return s.dht.GetValue(s.ctx, key)
}

// Provide anuncia que este nodo puede proporcionar un valor para la clave dada
func (s *DHTService) Provide(key string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return fmt.Errorf("DHT no inicializado")
	}

	// Crear un CID a partir de la clave
	mh, err := multihash.Sum([]byte(key), multihash.SHA2_256, -1)
	if err != nil {
		return fmt.Errorf("error al crear multihash: %v", err)
	}
	cid := cid.NewCidV1(cid.Raw, mh)

	return s.dht.Provide(s.ctx, cid, true)
}

// FindProviders busca nodos que pueden proporcionar un valor para la clave dada
func (s *DHTService) FindProviders(key string, count int) ([]peer.AddrInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.dht == nil {
		return nil, fmt.Errorf("DHT no inicializado")
	}

	// Crear un CID a partir de la clave
	mh, err := multihash.Sum([]byte(key), multihash.SHA2_256, -1)
	if err != nil {
		return nil, fmt.Errorf("error al crear multihash: %v", err)
	}
	cid := cid.NewCidV1(cid.Raw, mh)

	// Buscar proveedores
	ch := s.dht.FindProvidersAsync(s.ctx, cid, count)

	// Recopilar resultados
	var providers []peer.AddrInfo
	for p := range ch {
		providers = append(providers, p)
		if len(providers) >= count {
			break
		}
	}

	return providers, nil
}
