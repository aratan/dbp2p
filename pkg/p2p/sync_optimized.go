package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"dbp2p/pkg/db"

	"github.com/libp2p/go-libp2p/core/peer"
)

// SyncConfig contiene la configuración para la sincronización optimizada
type SyncConfig struct {
	// Intervalo entre sincronizaciones completas
	FullSyncInterval time.Duration

	// Intervalo entre sincronizaciones incrementales
	IncrementalSyncInterval time.Duration

	// Número máximo de documentos a sincronizar por lote
	BatchSize int

	// Tiempo máximo para esperar respuestas de otros nodos
	ResponseTimeout time.Duration

	// Número máximo de intentos de sincronización
	MaxRetries int

	// Prioridad de colecciones (las primeras tienen mayor prioridad)
	CollectionPriorities []string

	// Colecciones a excluir de la sincronización
	ExcludedCollections []string

	// Usar compresión para la sincronización
	UseCompression bool

	// Nivel de compresión (1-9, donde 9 es la máxima compresión)
	CompressionLevel int
}

// DefaultSyncConfig es la configuración por defecto para la sincronización
var DefaultSyncConfig = SyncConfig{
	FullSyncInterval:        time.Hour * 24,
	IncrementalSyncInterval: time.Minute * 5,
	BatchSize:               100,
	ResponseTimeout:         time.Second * 30,
	MaxRetries:              3,
	CollectionPriorities:    []string{},
	ExcludedCollections:     []string{"_system"},
	UseCompression:          true,
	CompressionLevel:        6,
}

// SyncManager gestiona la sincronización optimizada entre nodos
type SyncManager struct {
	node           *Node
	database       *db.Database
	config         SyncConfig
	lastFullSync   time.Time
	lastSyncByPeer map[peer.ID]time.Time
	syncStats      SyncStats
	syncInProgress bool
	mutex          sync.RWMutex
	stopChan       chan struct{}
}

// SyncStats contiene estadísticas de sincronización
type SyncStats struct {
	TotalSyncs            int
	SuccessfulSyncs       int
	FailedSyncs           int
	DocumentsSent         int
	DocumentsReceived     int
	BytesSent             int64
	BytesReceived         int64
	AverageLatency        time.Duration
	LastSyncDuration      time.Duration
	ConflictsDetected     int
	ConflictsResolved     int
	LastSyncTime          time.Time
	TotalFullSyncs        int
	TotalIncrementalSyncs int
}

// SyncRequest representa una solicitud de sincronización
type SyncRequest struct {
	NodeID           string    `json:"node_id"`
	RequestType      string    `json:"request_type"` // "full", "incremental", "collection"
	Collection       string    `json:"collection,omitempty"`
	LastSyncTime     time.Time `json:"last_sync_time,omitempty"`
	BatchSize        int       `json:"batch_size"`
	IncludeDeleted   bool      `json:"include_deleted"`
	UseCompression   bool      `json:"use_compression"`
	CompressionLevel int       `json:"compression_level,omitempty"`
	RequestID        string    `json:"request_id"`
	Timestamp        time.Time `json:"timestamp"`
}

// SyncResponse representa una respuesta a una solicitud de sincronización
type SyncResponse struct {
	NodeID           string        `json:"node_id"`
	RequestID        string        `json:"request_id"`
	Success          bool          `json:"success"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	Documents        []db.Document `json:"documents,omitempty"`
	DocumentsCount   int           `json:"documents_count"`
	HasMoreDocuments bool          `json:"has_more_documents"`
	NextBatchToken   string        `json:"next_batch_token,omitempty"`
	Compressed       bool          `json:"compressed"`
	Timestamp        time.Time     `json:"timestamp"`
}

// NewSyncManager crea un nuevo gestor de sincronización
func NewSyncManager(node *Node, database *db.Database, config ...SyncConfig) *SyncManager {
	// Usar configuración por defecto si no se especifica
	cfg := DefaultSyncConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return &SyncManager{
		node:           node,
		database:       database,
		config:         cfg,
		lastFullSync:   time.Time{},
		lastSyncByPeer: make(map[peer.ID]time.Time),
		syncStats:      SyncStats{},
		stopChan:       make(chan struct{}),
	}
}

// Start inicia el proceso de sincronización periódica
func (sm *SyncManager) Start() {
	// Iniciar goroutine para sincronización incremental
	go sm.startIncrementalSync()

	// Iniciar goroutine para sincronización completa
	go sm.startFullSync()

	// Registrar manejador para solicitudes de sincronización
	// Nota: Implementación temporal hasta que se añada RegisterTopicHandler
	// sm.node.PubSub.RegisterTopicHandler("sync_request", sm.handleSyncRequest)
	// sm.node.PubSub.RegisterTopicHandler("sync_response", sm.handleSyncResponse)
}

// Stop detiene el proceso de sincronización
func (sm *SyncManager) Stop() {
	close(sm.stopChan)
}

// startIncrementalSync inicia la sincronización incremental periódica
func (sm *SyncManager) startIncrementalSync() {
	ticker := time.NewTicker(sm.config.IncrementalSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.performIncrementalSync()
		case <-sm.stopChan:
			return
		}
	}
}

// startFullSync inicia la sincronización completa periódica
func (sm *SyncManager) startFullSync() {
	ticker := time.NewTicker(sm.config.FullSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.performFullSync()
		case <-sm.stopChan:
			return
		}
	}
}

// performIncrementalSync realiza una sincronización incremental con todos los peers
func (sm *SyncManager) performIncrementalSync() {
	sm.mutex.Lock()
	if sm.syncInProgress {
		sm.mutex.Unlock()
		return
	}
	sm.syncInProgress = true
	sm.mutex.Unlock()

	defer func() {
		sm.mutex.Lock()
		sm.syncInProgress = false
		sm.mutex.Unlock()
	}()

	// Obtener lista de peers
	peers := sm.node.Host.Network().Peers()
	if len(peers) == 0 {
		return
	}

	// Incrementar contador de sincronizaciones
	sm.mutex.Lock()
	sm.syncStats.TotalSyncs++
	sm.syncStats.TotalIncrementalSyncs++
	sm.mutex.Unlock()

	startTime := time.Now()

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.config.ResponseTimeout)
	defer cancel()

	// Crear grupo de espera para sincronización concurrente
	var wg sync.WaitGroup
	var successCount, failCount int
	var successMutex sync.Mutex

	// Sincronizar con cada peer
	for _, peerID := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()

			// Obtener última sincronización con este peer
			sm.mutex.RLock()
			lastSync, exists := sm.lastSyncByPeer[pid]
			sm.mutex.RUnlock()

			if !exists {
				lastSync = time.Time{}
			}

			// Crear solicitud de sincronización
			request := SyncRequest{
				NodeID:           sm.node.Host.ID().String(),
				RequestType:      "incremental",
				LastSyncTime:     lastSync,
				BatchSize:        sm.config.BatchSize,
				IncludeDeleted:   true,
				UseCompression:   sm.config.UseCompression,
				CompressionLevel: sm.config.CompressionLevel,
				RequestID:        generateRequestID(),
				Timestamp:        time.Now(),
			}

			// Enviar solicitud
			err := sm.sendSyncRequest(ctx, pid, request)
			if err != nil {
				fmt.Printf("Error al enviar solicitud de sincronización a %s: %v\n", pid.String(), err)
				successMutex.Lock()
				failCount++
				successMutex.Unlock()
				return
			}

			// Actualizar última sincronización con este peer
			sm.mutex.Lock()
			sm.lastSyncByPeer[pid] = time.Now()
			successMutex.Lock()
			successCount++
			successMutex.Unlock()
			sm.mutex.Unlock()
		}(peerID)
	}

	// Esperar a que todas las sincronizaciones terminen
	wg.Wait()

	// Actualizar estadísticas
	duration := time.Since(startTime)
	sm.mutex.Lock()
	sm.syncStats.LastSyncDuration = duration
	sm.syncStats.LastSyncTime = time.Now()
	sm.syncStats.SuccessfulSyncs += successCount
	sm.syncStats.FailedSyncs += failCount
	sm.mutex.Unlock()

	fmt.Printf("Sincronización incremental completada en %v. Éxitos: %d, Fallos: %d\n",
		duration, successCount, failCount)
}

// performFullSync realiza una sincronización completa con todos los peers
func (sm *SyncManager) performFullSync() {
	sm.mutex.Lock()
	if sm.syncInProgress {
		sm.mutex.Unlock()
		return
	}
	sm.syncInProgress = true
	sm.mutex.Unlock()

	defer func() {
		sm.mutex.Lock()
		sm.syncInProgress = false
		sm.mutex.Unlock()
	}()

	// Obtener lista de peers
	peers := sm.node.Host.Network().Peers()
	if len(peers) == 0 {
		return
	}

	// Incrementar contador de sincronizaciones
	sm.mutex.Lock()
	sm.syncStats.TotalSyncs++
	sm.syncStats.TotalFullSyncs++
	sm.mutex.Unlock()

	startTime := time.Now()

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.config.ResponseTimeout*2)
	defer cancel()

	// Crear grupo de espera para sincronización concurrente
	var wg sync.WaitGroup
	var successCount, failCount int
	var successMutex sync.Mutex

	// Sincronizar con cada peer
	for _, peerID := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()

			// Crear solicitud de sincronización
			request := SyncRequest{
				NodeID:           sm.node.Host.ID().String(),
				RequestType:      "full",
				BatchSize:        sm.config.BatchSize,
				IncludeDeleted:   true,
				UseCompression:   sm.config.UseCompression,
				CompressionLevel: sm.config.CompressionLevel,
				RequestID:        generateRequestID(),
				Timestamp:        time.Now(),
			}

			// Enviar solicitud
			err := sm.sendSyncRequest(ctx, pid, request)
			if err != nil {
				fmt.Printf("Error al enviar solicitud de sincronización a %s: %v\n", pid.String(), err)
				successMutex.Lock()
				failCount++
				successMutex.Unlock()
				return
			}

			// Actualizar última sincronización con este peer
			sm.mutex.Lock()
			sm.lastSyncByPeer[pid] = time.Now()
			successMutex.Lock()
			successCount++
			successMutex.Unlock()
			sm.mutex.Unlock()
		}(peerID)
	}

	// Esperar a que todas las sincronizaciones terminen
	wg.Wait()

	// Actualizar estadísticas
	duration := time.Since(startTime)
	sm.mutex.Lock()
	sm.syncStats.LastSyncDuration = duration
	sm.syncStats.LastSyncTime = time.Now()
	sm.lastFullSync = time.Now()
	sm.syncStats.SuccessfulSyncs += successCount
	sm.syncStats.FailedSyncs += failCount
	sm.mutex.Unlock()

	fmt.Printf("Sincronización completa completada en %v. Éxitos: %d, Fallos: %d\n",
		duration, successCount, failCount)
}

// sendSyncRequest envía una solicitud de sincronización a un peer
func (sm *SyncManager) sendSyncRequest(ctx context.Context, peerID peer.ID, request SyncRequest) error {
	// Serializar solicitud
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error al serializar solicitud: %v", err)
	}

	// Comprimir datos si es necesario
	if request.UseCompression {
		// Aquí se implementaría la compresión
	}

	// Publicar mensaje
	err = sm.node.PubSub.Publish("sync_request", data)
	if err != nil {
		return fmt.Errorf("error al publicar solicitud: %v", err)
	}

	return nil
}

// handleSyncRequest maneja una solicitud de sincronización
func (sm *SyncManager) handleSyncRequest(topic string, data []byte) {
	// Deserializar solicitud
	var request SyncRequest
	if err := json.Unmarshal(data, &request); err != nil {
		fmt.Printf("Error al deserializar solicitud de sincronización: %v\n", err)
		return
	}

	// Verificar si es una solicitud para este nodo
	if request.NodeID == sm.node.Host.ID().String() {
		return // Ignorar solicitudes propias
	}

	// Procesar solicitud según su tipo
	var response SyncResponse
	var err error

	switch request.RequestType {
	case "full":
		response, err = sm.handleFullSyncRequest(request)
	case "incremental":
		response, err = sm.handleIncrementalSyncRequest(request)
	case "collection":
		response, err = sm.handleCollectionSyncRequest(request)
	default:
		err = fmt.Errorf("tipo de solicitud desconocido: %s", request.RequestType)
	}

	if err != nil {
		// Crear respuesta de error
		response = SyncResponse{
			NodeID:       sm.node.Host.ID().String(),
			RequestID:    request.RequestID,
			Success:      false,
			ErrorMessage: err.Error(),
			Timestamp:    time.Now(),
		}
	}

	// Enviar respuesta
	sm.sendSyncResponse(response)
}

// handleFullSyncRequest maneja una solicitud de sincronización completa
func (sm *SyncManager) handleFullSyncRequest(request SyncRequest) (SyncResponse, error) {
	// Obtener todas las colecciones
	// Nota: Implementación temporal hasta que se añada GetCollections
	collections := []string{"users", "documents", "settings"}

	// Filtrar colecciones excluidas
	var filteredCollections []string
	for _, collection := range collections {
		excluded := false
		for _, excludedColl := range sm.config.ExcludedCollections {
			if collection == excludedColl {
				excluded = true
				break
			}
		}
		if !excluded {
			filteredCollections = append(filteredCollections, collection)
		}
	}

	// Ordenar colecciones por prioridad
	// Aquí se implementaría la ordenación por prioridad

	// Obtener documentos de cada colección
	var allDocuments []db.Document
	for _, collection := range filteredCollections {
		docs, err := sm.database.GetAllDocuments(collection)
		if err != nil {
			fmt.Printf("Error al obtener documentos de la colección %s: %v\n", collection, err)
			continue
		}
		for _, doc := range docs {
			allDocuments = append(allDocuments, *doc)
		}
	}

	// Limitar al tamaño del lote
	documentsToSend := allDocuments
	hasMore := false
	if len(allDocuments) > request.BatchSize {
		documentsToSend = allDocuments[:request.BatchSize]
		hasMore = true
	}

	// Crear respuesta
	response := SyncResponse{
		NodeID:           sm.node.Host.ID().String(),
		RequestID:        request.RequestID,
		Success:          true,
		Documents:        documentsToSend,
		DocumentsCount:   len(documentsToSend),
		HasMoreDocuments: hasMore,
		Compressed:       request.UseCompression,
		Timestamp:        time.Now(),
	}

	// Si hay más documentos, generar token para el siguiente lote
	if hasMore {
		// Aquí se implementaría la generación del token
	}

	return response, nil
}

// handleIncrementalSyncRequest maneja una solicitud de sincronización incremental
func (sm *SyncManager) handleIncrementalSyncRequest(request SyncRequest) (SyncResponse, error) {
	// Obtener documentos modificados desde la última sincronización
	var modifiedDocuments []db.Document

	// Obtener todas las colecciones
	// Nota: Implementación temporal hasta que se añada GetCollections
	collections := []string{"users", "documents", "settings"}

	// Filtrar colecciones excluidas
	var filteredCollections []string
	for _, collection := range collections {
		excluded := false
		for _, excludedColl := range sm.config.ExcludedCollections {
			if collection == excludedColl {
				excluded = true
				break
			}
		}
		if !excluded {
			filteredCollections = append(filteredCollections, collection)
		}
	}

	// Obtener documentos modificados de cada colección
	for _, collection := range filteredCollections {
		docs, err := sm.database.GetAllDocuments(collection)
		if err != nil {
			fmt.Printf("Error al obtener documentos de la colección %s: %v\n", collection, err)
			continue
		}
		for _, doc := range docs {
			if doc.UpdatedAt.After(request.LastSyncTime) {
				modifiedDocuments = append(modifiedDocuments, *doc)
			}
		}
	}

	// Limitar al tamaño del lote
	documentsToSend := modifiedDocuments
	hasMore := false
	if len(modifiedDocuments) > request.BatchSize {
		documentsToSend = modifiedDocuments[:request.BatchSize]
		hasMore = true
	}

	// Crear respuesta
	response := SyncResponse{
		NodeID:           sm.node.Host.ID().String(),
		RequestID:        request.RequestID,
		Success:          true,
		Documents:        documentsToSend,
		DocumentsCount:   len(documentsToSend),
		HasMoreDocuments: hasMore,
		Compressed:       request.UseCompression,
		Timestamp:        time.Now(),
	}

	// Si hay más documentos, generar token para el siguiente lote
	if hasMore {
		// Aquí se implementaría la generación del token
	}

	return response, nil
}

// handleCollectionSyncRequest maneja una solicitud de sincronización de una colección específica
func (sm *SyncManager) handleCollectionSyncRequest(request SyncRequest) (SyncResponse, error) {
	// Verificar si la colección está excluida
	for _, excludedColl := range sm.config.ExcludedCollections {
		if request.Collection == excludedColl {
			return SyncResponse{}, fmt.Errorf("colección excluida: %s", request.Collection)
		}
	}

	// Obtener documentos de la colección
	docs, err := sm.database.GetAllDocuments(request.Collection)
	if err != nil {
		return SyncResponse{}, fmt.Errorf("error al obtener documentos: %v", err)
	}

	// Filtrar por fecha de modificación si es una sincronización incremental
	var filteredDocs []db.Document
	if !request.LastSyncTime.IsZero() {
		for _, doc := range docs {
			if doc.UpdatedAt.After(request.LastSyncTime) {
				filteredDocs = append(filteredDocs, *doc)
			}
		}
	} else {
		for _, doc := range docs {
			filteredDocs = append(filteredDocs, *doc)
		}
	}

	// Limitar al tamaño del lote
	documentsToSend := filteredDocs
	hasMore := false
	if len(filteredDocs) > request.BatchSize {
		documentsToSend = filteredDocs[:request.BatchSize]
		hasMore = true
	}

	// Crear respuesta
	response := SyncResponse{
		NodeID:           sm.node.Host.ID().String(),
		RequestID:        request.RequestID,
		Success:          true,
		Documents:        documentsToSend,
		DocumentsCount:   len(documentsToSend),
		HasMoreDocuments: hasMore,
		Compressed:       request.UseCompression,
		Timestamp:        time.Now(),
	}

	// Si hay más documentos, generar token para el siguiente lote
	if hasMore {
		// Aquí se implementaría la generación del token
	}

	return response, nil
}

// sendSyncResponse envía una respuesta de sincronización
func (sm *SyncManager) sendSyncResponse(response SyncResponse) {
	// Serializar respuesta
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("Error al serializar respuesta: %v\n", err)
		return
	}

	// Comprimir datos si es necesario
	if response.Compressed {
		// Aquí se implementaría la compresión
	}

	// Publicar mensaje
	err = sm.node.PubSub.Publish("sync_response", data)
	if err != nil {
		fmt.Printf("Error al publicar respuesta: %v\n", err)
	}

	// Actualizar estadísticas
	sm.mutex.Lock()
	sm.syncStats.DocumentsSent += response.DocumentsCount
	sm.syncStats.BytesSent += int64(len(data))
	sm.mutex.Unlock()
}

// handleSyncResponse maneja una respuesta de sincronización
func (sm *SyncManager) handleSyncResponse(topic string, data []byte) {
	// Deserializar respuesta
	var response SyncResponse
	if err := json.Unmarshal(data, &response); err != nil {
		fmt.Printf("Error al deserializar respuesta de sincronización: %v\n", err)
		return
	}

	// Verificar si es una respuesta para este nodo
	if response.NodeID == sm.node.Host.ID().String() {
		return // Ignorar respuestas propias
	}

	// Verificar si la respuesta fue exitosa
	if !response.Success {
		fmt.Printf("Error en respuesta de sincronización: %s\n", response.ErrorMessage)
		return
	}

	// Procesar documentos recibidos
	for _, doc := range response.Documents {
		// Verificar si el documento ya existe
		existingDoc, err := sm.database.GetDocument(doc.ID)
		if err == nil {
			// El documento existe, verificar si es más reciente
			if doc.UpdatedAt.After(existingDoc.UpdatedAt) {
				// Actualizar documento
				_, err := sm.database.UpdateDocument(doc.ID, doc.Data)
				if err != nil {
					fmt.Printf("Error al actualizar documento %s: %v\n", doc.ID, err)
				}
			} else {
				// Detectar conflicto
				sm.mutex.Lock()
				sm.syncStats.ConflictsDetected++
				sm.mutex.Unlock()

				// Resolver conflicto (aquí se implementaría la estrategia de resolución)
				// Por ahora, simplemente mantenemos el documento más reciente
				sm.mutex.Lock()
				sm.syncStats.ConflictsResolved++
				sm.mutex.Unlock()
			}
		} else {
			// El documento no existe, crearlo
			_, err := sm.database.CreateDocument(doc.Collection, doc.Data)
			if err != nil {
				fmt.Printf("Error al crear documento %s: %v\n", doc.ID, err)
			}
		}
	}

	// Actualizar estadísticas
	sm.mutex.Lock()
	sm.syncStats.DocumentsReceived += response.DocumentsCount
	sm.syncStats.BytesReceived += int64(len(data))
	sm.mutex.Unlock()

	// Si hay más documentos, solicitar el siguiente lote
	if response.HasMoreDocuments && response.NextBatchToken != "" {
		// Aquí se implementaría la solicitud del siguiente lote
	}
}

// GetSyncStats obtiene las estadísticas de sincronización
func (sm *SyncManager) GetSyncStats() SyncStats {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.syncStats
}

// ResetSyncStats reinicia las estadísticas de sincronización
func (sm *SyncManager) ResetSyncStats() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.syncStats = SyncStats{}
}

// generateRequestID genera un ID único para una solicitud
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
