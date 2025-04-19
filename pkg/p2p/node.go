package p2p

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aratan/dbp2p/pkg/config"
	"github.com/aratan/dbp2p/pkg/db"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
)

// Node representa un nodo P2P completo
type Node struct {
	Host        host.Host
	MDNSService *MDNSService
	DHTService  *DHTService
	PubSub      *PubSubService
	ctx         context.Context
	cancel      context.CancelFunc
	Database    *db.Database
	Sync        *db.DBSync
}

// NewNode crea un nuevo nodo P2P con mDNS y DHT
func NewNode(ctx context.Context) (*Node, error) {
	// Crear contexto cancelable
	ctx, cancel := context.WithCancel(ctx)

	// Cargar configuración
	cfg := config.GetConfig()

	// Crear opciones de libp2p
	opts := []libp2p.Option{
		libp2p.Security(noise.ID, noise.New),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	}

	// Añadir direcciones de escucha
	for _, addr := range cfg.Network.LibP2P.ListenAddresses {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			log.Printf("Error al parsear dirección de escucha %s: %v", addr, err)
			continue
		}
		opts = append(opts, libp2p.ListenAddrs(ma))
	}

	// Crear host libp2p
	host, err := libp2p.New(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error al crear host libp2p: %v", err)
	}

	// Crear nodo
	node := &Node{
		Host:   host,
		ctx:    ctx,
		cancel: cancel,
	}

	// Inicializar mDNS si está habilitado
	if cfg.Network.MDNS.Enabled {
		mdnsService, err := NewMDNSService(
			ctx,
			host,
			WithServiceName(cfg.Network.MDNS.ServiceName),
			WithInterval(cfg.Network.MDNS.Interval),
		)
		if err != nil {
			node.Close()
			return nil, fmt.Errorf("error al crear servicio mDNS: %v", err)
		}

		if err := mdnsService.Start(); err != nil {
			node.Close()
			return nil, fmt.Errorf("error al iniciar servicio mDNS: %v", err)
		}

		node.MDNSService = mdnsService
	}

	// Inicializar DHT si está habilitado
	if cfg.Network.DHT.Enabled {
		dhtService, err := NewDHTService(
			ctx,
			host,
			WithDHTMode(cfg.Network.DHT.Mode),
			WithBootstrapPeers(cfg.Network.LibP2P.BootstrapPeers),
			WithBootstrapInterval(cfg.Network.DHT.BootstrapInterval),
		)
		if err != nil {
			node.Close()
			return nil, fmt.Errorf("error al crear servicio DHT: %v", err)
		}

		if err := dhtService.Start(); err != nil {
			node.Close()
			return nil, fmt.Errorf("error al iniciar servicio DHT: %v", err)
		}

		node.DHTService = dhtService
	}

	// Mostrar información del nodo
	addrs := host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	log.Printf("Nodo P2P inicializado con ID: %s", host.ID().String())
	log.Printf("Direcciones: %v", addrStrings)

	return node, nil
}

// Close cierra el nodo P2P y todos sus servicios
func (n *Node) Close() error {
	// Detener servicios en orden inverso
	if n.PubSub != nil {
		n.PubSub.Stop()
	}

	if n.DHTService != nil {
		n.DHTService.Stop()
	}

	if n.MDNSService != nil {
		n.MDNSService.Stop()
	}

	// Cerrar host
	if err := n.Host.Close(); err != nil {
		return fmt.Errorf("error al cerrar host: %v", err)
	}

	// Cancelar contexto
	n.cancel()

	return nil
}

// ID devuelve el ID del nodo
func (n *Node) ID() host.Host {
	return n.Host
}

// GetPeers devuelve los peers conectados
func (n *Node) GetPeers() []string {
	peers := n.Host.Network().Peers()
	peerStrings := make([]string, len(peers))
	for i, peer := range peers {
		peerStrings[i] = peer.String()
	}
	return peerStrings
}

// SetDatabase establece la base de datos para el nodo
func (n *Node) SetDatabase(database *db.Database) error {
	n.Database = database

	// Inicializar sincronización
	sync, err := db.NewDBSync(n.ctx, database, n.PubSub.GetPubSub(), n.Host.ID().String())
	if err != nil {
		return fmt.Errorf("error al inicializar sincronización: %v", err)
	}

	n.Sync = sync

	// Configurar la base de datos para usar la sincronización
	database.SetSync(sync)

	// Sincronizar todos los documentos
	go func() {
		// Esperar un poco para que otros nodos se conecten
		time.Sleep(5 * time.Second)

		// Sincronizar todos los documentos
		if err := database.SyncAllDocuments(); err != nil {
			log.Printf("Error al sincronizar todos los documentos: %v", err)
		}
	}()

	return nil
}

// GetConnectedPeers devuelve la lista de peers conectados
func (n *Node) GetConnectedPeers() []peer.ID {
	return n.Host.Network().Peers()
}

// GetPeerCount devuelve el número de peers conectados
func (n *Node) GetPeerCount() int {
	return len(n.Host.Network().Peers())
}
