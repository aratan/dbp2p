package db

import (
	"container/list"
	"sync"
	"time"
)

// CacheItem representa un elemento en la caché
type CacheItem struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired verifica si el elemento ha expirado
func (ci *CacheItem) IsExpired() bool {
	return !ci.ExpiresAt.IsZero() && time.Now().After(ci.ExpiresAt)
}

// CacheEvictionPolicy define la política de expulsión de la caché
type CacheEvictionPolicy string

const (
	// EvictionPolicyLRU expulsa el elemento menos recientemente usado
	EvictionPolicyLRU CacheEvictionPolicy = "lru"
	// EvictionPolicyLFU expulsa el elemento menos frecuentemente usado
	EvictionPolicyLFU CacheEvictionPolicy = "lfu"
	// EvictionPolicyFIFO expulsa el primer elemento que entró
	EvictionPolicyFIFO CacheEvictionPolicy = "fifo"
)

// Cache implementa un sistema de caché con diferentes políticas de expulsión
type Cache struct {
	items            map[string]*list.Element
	itemsList        *list.List
	accessCount      map[string]int
	maxSize          int
	defaultTTL       time.Duration
	evictionPolicy   CacheEvictionPolicy
	mutex            sync.RWMutex
	cleanupInterval  time.Duration
	stopCleanup      chan struct{}
	stats            CacheStats
}

// CacheStats contiene estadísticas de la caché
type CacheStats struct {
	Hits             int
	Misses           int
	Evictions        int
	Expirations      int
	TotalItems       int
	TotalOperations  int
}

// CacheOption es una función que configura la caché
type CacheOption func(*Cache)

// WithMaxSize establece el tamaño máximo de la caché
func WithMaxSize(size int) CacheOption {
	return func(c *Cache) {
		c.maxSize = size
	}
}

// WithDefaultTTL establece el tiempo de vida por defecto de los elementos
func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *Cache) {
		c.defaultTTL = ttl
	}
}

// WithEvictionPolicy establece la política de expulsión
func WithEvictionPolicy(policy CacheEvictionPolicy) CacheOption {
	return func(c *Cache) {
		c.evictionPolicy = policy
	}
}

// WithCleanupInterval establece el intervalo de limpieza de elementos expirados
func WithCleanupInterval(interval time.Duration) CacheOption {
	return func(c *Cache) {
		c.cleanupInterval = interval
	}
}

// NewCache crea una nueva caché
func NewCache(options ...CacheOption) *Cache {
	cache := &Cache{
		items:           make(map[string]*list.Element),
		itemsList:       list.New(),
		accessCount:     make(map[string]int),
		maxSize:         1000,                // Por defecto 1000 elementos
		defaultTTL:      time.Minute * 10,    // Por defecto 10 minutos
		evictionPolicy:  EvictionPolicyLRU,   // Por defecto LRU
		cleanupInterval: time.Minute,         // Por defecto 1 minuto
		stopCleanup:     make(chan struct{}),
		stats:           CacheStats{},
	}

	// Aplicar opciones
	for _, option := range options {
		option(cache)
	}

	// Iniciar limpieza periódica
	go cache.startCleanup()

	return cache
}

// Set añade o actualiza un elemento en la caché
func (c *Cache) Set(key string, value interface{}, ttl ...time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Determinar TTL
	var expiration time.Time
	if len(ttl) > 0 && ttl[0] > 0 {
		expiration = time.Now().Add(ttl[0])
	} else if c.defaultTTL > 0 {
		expiration = time.Now().Add(c.defaultTTL)
	}

	// Crear elemento
	item := &CacheItem{
		Key:       key,
		Value:     value,
		ExpiresAt: expiration,
	}

	// Verificar si ya existe
	if element, exists := c.items[key]; exists {
		// Actualizar elemento existente
		c.itemsList.MoveToFront(element)
		element.Value = item
		c.accessCount[key]++
	} else {
		// Añadir nuevo elemento
		element := c.itemsList.PushFront(item)
		c.items[key] = element
		c.accessCount[key] = 1
		c.stats.TotalItems++

		// Verificar tamaño máximo
		if c.maxSize > 0 && len(c.items) > c.maxSize {
			c.evict()
		}
	}

	c.stats.TotalOperations++
}

// Get obtiene un elemento de la caché
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	element, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		c.stats.TotalOperations++
		return nil, false
	}

	item := element.Value.(*CacheItem)
	if item.IsExpired() {
		c.removeElement(element)
		c.stats.Expirations++
		c.stats.Misses++
		c.stats.TotalOperations++
		return nil, false
	}

	// Actualizar estadísticas y posición según política
	c.stats.Hits++
	c.stats.TotalOperations++
	c.accessCount[key]++

	if c.evictionPolicy == EvictionPolicyLRU {
		c.itemsList.MoveToFront(element)
	}

	return item.Value, true
}

// Delete elimina un elemento de la caché
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.items[key]; exists {
		c.removeElement(element)
	}

	c.stats.TotalOperations++
}

// Clear elimina todos los elementos de la caché
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*list.Element)
	c.itemsList = list.New()
	c.accessCount = make(map[string]int)
	c.stats = CacheStats{}
}

// Size devuelve el número de elementos en la caché
func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.items)
}

// Keys devuelve todas las claves en la caché
func (c *Cache) Keys() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// GetStats devuelve las estadísticas de la caché
func (c *Cache) GetStats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.stats
}

// Close detiene la limpieza periódica
func (c *Cache) Close() {
	close(c.stopCleanup)
}

// removeElement elimina un elemento de la caché
func (c *Cache) removeElement(element *list.Element) {
	item := element.Value.(*CacheItem)
	delete(c.items, item.Key)
	delete(c.accessCount, item.Key)
	c.itemsList.Remove(element)
	c.stats.TotalItems--
}

// evict expulsa un elemento según la política configurada
func (c *Cache) evict() {
	var element *list.Element

	switch c.evictionPolicy {
	case EvictionPolicyLRU:
		// El menos recientemente usado está al final de la lista
		element = c.itemsList.Back()
	case EvictionPolicyFIFO:
		// El primero que entró está al final de la lista
		element = c.itemsList.Back()
	case EvictionPolicyLFU:
		// El menos frecuentemente usado
		var minKey string
		minCount := -1

		for key, count := range c.accessCount {
			if minCount == -1 || count < minCount {
				minCount = count
				minKey = key
			}
		}

		element = c.items[minKey]
	}

	if element != nil {
		c.removeElement(element)
		c.stats.Evictions++
	}
}

// startCleanup inicia la limpieza periódica de elementos expirados
func (c *Cache) startCleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanup elimina los elementos expirados
func (c *Cache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, element := range c.items {
		item := element.Value.(*CacheItem)
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			c.removeElement(element)
			delete(c.items, key)
			c.stats.Expirations++
		}
	}
}
