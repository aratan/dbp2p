package db

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MemoryManagerConfig contiene la configuración del gestor de memoria
type MemoryManagerConfig struct {
	// Límite de memoria en bytes (0 = sin límite)
	MemoryLimit int64
	
	// Intervalo de comprobación de memoria
	CheckInterval time.Duration
	
	// Umbral de memoria para activar la limpieza (0.0-1.0)
	CleanupThreshold float64
	
	// Número máximo de documentos en memoria (0 = sin límite)
	MaxDocuments int
	
	// Habilitar compresión de documentos en memoria
	EnableCompression bool
	
	// Nivel de compresión (1-9, donde 9 es la máxima compresión)
	CompressionLevel int
}

// DefaultMemoryManagerConfig es la configuración por defecto del gestor de memoria
var DefaultMemoryManagerConfig = MemoryManagerConfig{
	MemoryLimit:       0, // Sin límite
	CheckInterval:     time.Minute,
	CleanupThreshold:  0.8, // 80%
	MaxDocuments:      0, // Sin límite
	EnableCompression: false,
	CompressionLevel:  6,
}

// MemoryStats contiene estadísticas de uso de memoria
type MemoryStats struct {
	// Uso actual de memoria en bytes
	CurrentUsage int64
	
	// Límite de memoria en bytes
	MemoryLimit int64
	
	// Porcentaje de uso de memoria (0.0-1.0)
	UsagePercentage float64
	
	// Número de documentos en memoria
	DocumentCount int
	
	// Número máximo de documentos en memoria
	MaxDocuments int
	
	// Número de limpiezas realizadas
	CleanupCount int
	
	// Última limpieza
	LastCleanup time.Time
	
	// Bytes liberados en la última limpieza
	LastCleanupBytes int64
	
	// Documentos liberados en la última limpieza
	LastCleanupDocs int
	
	// Documentos comprimidos
	CompressedDocs int
	
	// Bytes ahorrados por compresión
	CompressedBytes int64
}

// MemoryManager gestiona el uso de memoria de la base de datos
type MemoryManager struct {
	config         MemoryManagerConfig
	database       *Database
	stats          MemoryStats
	stopChan       chan struct{}
	mutex          sync.RWMutex
	documentSizes  map[string]int64
	accessTimes    map[string]time.Time
	compressedDocs map[string]bool
}

// NewMemoryManager crea un nuevo gestor de memoria
func NewMemoryManager(database *Database, config ...MemoryManagerConfig) *MemoryManager {
	// Usar configuración por defecto si no se especifica
	cfg := DefaultMemoryManagerConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return &MemoryManager{
		config:         cfg,
		database:       database,
		stats:          MemoryStats{},
		stopChan:       make(chan struct{}),
		documentSizes:  make(map[string]int64),
		accessTimes:    make(map[string]time.Time),
		compressedDocs: make(map[string]bool),
	}
}

// Start inicia el gestor de memoria
func (mm *MemoryManager) Start() {
	// Iniciar goroutine para comprobación periódica
	go mm.startMemoryCheck()
}

// Stop detiene el gestor de memoria
func (mm *MemoryManager) Stop() {
	close(mm.stopChan)
}

// startMemoryCheck inicia la comprobación periódica de memoria
func (mm *MemoryManager) startMemoryCheck() {
	ticker := time.NewTicker(mm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.checkMemory()
		case <-mm.stopChan:
			return
		}
	}
}

// checkMemory comprueba el uso de memoria y realiza limpieza si es necesario
func (mm *MemoryManager) checkMemory() {
	// Obtener estadísticas de memoria
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Actualizar estadísticas
	mm.mutex.Lock()
	mm.stats.CurrentUsage = int64(memStats.Alloc)
	mm.stats.MemoryLimit = mm.config.MemoryLimit
	mm.stats.DocumentCount = len(mm.database.documents)
	mm.stats.MaxDocuments = mm.config.MaxDocuments

	// Calcular porcentaje de uso
	if mm.config.MemoryLimit > 0 {
		mm.stats.UsagePercentage = float64(mm.stats.CurrentUsage) / float64(mm.config.MemoryLimit)
	} else {
		mm.stats.UsagePercentage = 0
	}
	mm.mutex.Unlock()

	// Verificar si es necesario realizar limpieza
	needCleanup := false

	// Por límite de memoria
	if mm.config.MemoryLimit > 0 && mm.stats.UsagePercentage >= mm.config.CleanupThreshold {
		needCleanup = true
	}

	// Por número máximo de documentos
	if mm.config.MaxDocuments > 0 && mm.stats.DocumentCount > mm.config.MaxDocuments {
		needCleanup = true
	}

	// Realizar limpieza si es necesario
	if needCleanup {
		mm.cleanup()
	}
}

// cleanup realiza la limpieza de memoria
func (mm *MemoryManager) cleanup() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// Obtener estadísticas antes de la limpieza
	initialUsage := mm.stats.CurrentUsage
	initialDocs := mm.stats.DocumentCount

	// Estrategias de limpieza
	mm.compressDocuments()
	mm.evictLRUDocuments()

	// Actualizar estadísticas después de la limpieza
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	mm.stats.CurrentUsage = int64(memStats.Alloc)
	mm.stats.DocumentCount = len(mm.database.documents)
	mm.stats.CleanupCount++
	mm.stats.LastCleanup = time.Now()
	mm.stats.LastCleanupBytes = initialUsage - mm.stats.CurrentUsage
	mm.stats.LastCleanupDocs = initialDocs - mm.stats.DocumentCount

	fmt.Printf("Limpieza de memoria completada. Liberados %d bytes y %d documentos.\n",
		mm.stats.LastCleanupBytes, mm.stats.LastCleanupDocs)
}

// compressDocuments comprime documentos en memoria
func (mm *MemoryManager) compressDocuments() {
	if !mm.config.EnableCompression {
		return
	}

	// Obtener documentos no comprimidos
	var docsToCompress []string
	for id := range mm.database.documents {
		if !mm.compressedDocs[id] {
			docsToCompress = append(docsToCompress, id)
		}
	}

	// Comprimir documentos
	for _, id := range docsToCompress {
		doc := mm.database.documents[id]
		
		// Obtener tamaño antes de comprimir
		sizeBefore := mm.estimateDocumentSize(doc)
		
		// Comprimir documento
		// Aquí se implementaría la compresión real
		// Por ahora, solo marcamos como comprimido
		
		// Obtener tamaño después de comprimir
		sizeAfter := mm.estimateDocumentSize(doc)
		
		// Actualizar estadísticas
		mm.compressedDocs[id] = true
		mm.stats.CompressedDocs++
		mm.stats.CompressedBytes += sizeBefore - sizeAfter
	}
}

// evictLRUDocuments elimina documentos menos recientemente utilizados
func (mm *MemoryManager) evictLRUDocuments() {
	// Si no hay límite de documentos, no hacer nada
	if mm.config.MaxDocuments <= 0 {
		return
	}

	// Calcular cuántos documentos eliminar
	docsToEvict := mm.stats.DocumentCount - mm.config.MaxDocuments
	if docsToEvict <= 0 {
		return
	}

	// Ordenar documentos por tiempo de acceso
	type docAccess struct {
		id        string
		accessTime time.Time
	}
	
	var accessList []docAccess
	for id, accessTime := range mm.accessTimes {
		accessList = append(accessList, docAccess{id, accessTime})
	}
	
	// Ordenar por tiempo de acceso (más antiguo primero)
	for i := 0; i < len(accessList)-1; i++ {
		for j := i + 1; j < len(accessList); j++ {
			if accessList[i].accessTime.After(accessList[j].accessTime) {
				accessList[i], accessList[j] = accessList[j], accessList[i]
			}
		}
	}

	// Eliminar documentos menos recientemente utilizados
	for i := 0; i < docsToEvict && i < len(accessList); i++ {
		id := accessList[i].id
		
		// Verificar si el documento existe
		if _, exists := mm.database.documents[id]; exists {
			// Guardar documento en disco si es necesario
			if mm.database.persistenceEnabled && mm.database.persistence != nil {
				doc := mm.database.documents[id]
				mm.database.persistence.SaveDocument(doc)
			}
			
			// Eliminar documento de memoria
			delete(mm.database.documents, id)
			delete(mm.documentSizes, id)
			delete(mm.accessTimes, id)
			delete(mm.compressedDocs, id)
		}
	}
}

// TrackDocumentAccess registra el acceso a un documento
func (mm *MemoryManager) TrackDocumentAccess(id string) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	mm.accessTimes[id] = time.Now()
}

// TrackDocumentSize registra el tamaño de un documento
func (mm *MemoryManager) TrackDocumentSize(doc *Document) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	size := mm.estimateDocumentSize(doc)
	mm.documentSizes[doc.ID] = size
	mm.accessTimes[doc.ID] = time.Now()
}

// estimateDocumentSize estima el tamaño en memoria de un documento
func (mm *MemoryManager) estimateDocumentSize(doc *Document) int64 {
	// Tamaño base del documento
	size := int64(100) // Aproximación del tamaño de la estructura
	
	// Tamaño del ID
	size += int64(len(doc.ID))
	
	// Tamaño de la colección
	size += int64(len(doc.Collection))
	
	// Tamaño de los datos
	for key, value := range doc.Data {
		// Tamaño de la clave
		size += int64(len(key))
		
		// Tamaño del valor
		size += mm.estimateValueSize(value)
	}
	
	return size
}

// estimateValueSize estima el tamaño en memoria de un valor
func (mm *MemoryManager) estimateValueSize(value interface{}) int64 {
	if value == nil {
		return 0
	}
	
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case int, int32, float32, bool:
		return 4
	case int64, float64:
		return 8
	case []interface{}:
		size := int64(0)
		for _, item := range v {
			size += mm.estimateValueSize(item)
		}
		return size
	case map[string]interface{}:
		size := int64(0)
		for key, val := range v {
			size += int64(len(key))
			size += mm.estimateValueSize(val)
		}
		return size
	default:
		return 8 // Valor por defecto
	}
}

// GetMemoryStats obtiene las estadísticas de memoria
func (mm *MemoryManager) GetMemoryStats() MemoryStats {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.stats
}

// ForceCleanup fuerza una limpieza de memoria
func (mm *MemoryManager) ForceCleanup() {
	mm.cleanup()
}
