// Package cache provides caching for search results.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Cache is the interface for caching search results.
type Cache interface {
	// Get retrieves a cached result set.
	Get(ctx context.Context, key string) (*core.ResultSet, error)

	// Set stores a result set in cache.
	Set(ctx context.Context, key string, resultSet *core.ResultSet) error

	// Delete removes a cached entry.
	Delete(ctx context.Context, key string) error

	// Clear removes all cached entries.
	Clear(ctx context.Context) error

	// Stats returns cache statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Close cleans up cache resources.
	Close() error
}

// Stats holds cache statistics.
type Stats struct {
	Size      int   `json:"size"`
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Items     int   `json:"items"`
	MemoryKB  int64 `json:"memory_kb"`
}

// entry represents a cached item with TTL.
type entry struct {
	Key       string           `json:"key"`
	ResultSet *core.ResultSet  `json:"result_set"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt time.Time        `json:"expires_at"`
}

// MemoryCache implements Cache using in-memory storage.
type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*entry
	maxSize  int
	ttl      time.Duration
	cleanup  time.Duration
	hits     int64
	misses   int64
	log      *logger.Logger
	stopCh   chan struct{}
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(cfg *core.CacheConfig, log *logger.Logger) (*MemoryCache, error) {
	c := &MemoryCache{
		items:   make(map[string]*entry),
		maxSize: cfg.MemorySize,
		ttl:     cfg.TTL,
		cleanup: cfg.CleanupInterval,
		log:     log,
		stopCh:  make(chan struct{}),
	}

	if c.maxSize <= 0 {
		c.maxSize = 10000
	}
	if c.ttl <= 0 {
		c.ttl = 5 * time.Minute
	}
	if c.cleanup <= 0 {
		c.cleanup = time.Minute
	}

	// Start cleanup goroutine
	go c.cleanupLoop()

	return c, nil
}

// Get retrieves a cached entry.
func (c *MemoryCache) Get(ctx context.Context, key string) (*core.ResultSet, error) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, nil
	}

	// Check expiration
	if time.Now().After(e.ExpiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.misses++
		c.mu.Unlock()
		return nil, nil
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return e.ResultSet, nil
}

// Set stores an entry in cache.
func (c *MemoryCache) Set(ctx context.Context, key string, resultSet *core.ResultSet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &entry{
		Key:       key,
		ResultSet: resultSet,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}
	return nil
}

// Delete removes an entry.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

// Clear removes all entries.
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*entry)
	c.hits = 0
	c.misses = 0
	return nil
}

// Stats returns cache statistics.
func (c *MemoryCache) Stats(ctx context.Context) (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &Stats{
		Size:     len(c.items),
		Hits:     c.hits,
		Misses:   c.misses,
		Items:    len(c.items),
	}, nil
}

// Close stops the cleanup goroutine.
func (c *MemoryCache) Close() error {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
	return nil
}

// cleanupLoop periodically removes expired entries.
func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCh:
			return
		}
	}
}

// removeExpired deletes all expired entries.
func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, e := range c.items {
		if now.After(e.ExpiresAt) {
			delete(c.items, key)
		}
	}
}

// evictOldest removes the oldest entry (simple FIFO eviction).
func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, e := range c.items {
		if first || e.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = e.CreatedAt
			first = false
		}
	}
	delete(c.items, oldestKey)
	c.log.Debug("evicted oldest cache entry", logger.LogFields{
		"key": oldestKey,
		"age": time.Since(oldestTime).String(),
	})
}

// DiskCache implements Cache using a file-based store.
type DiskCache struct {
	path   string
	mu     sync.RWMutex
	ttl    time.Duration
	log    *logger.Logger
	hits   int64
	misses int64
}

// NewDiskCache creates a new disk-based cache.
func NewDiskCache(cfg *core.CacheConfig, log *logger.Logger) (*DiskCache, error) {
	path := cfg.DiskPath
	if path == "" {
		path = filepath.Join(os.TempDir(), "bingdork-cache")
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	return &DiskCache{
		path: path,
		ttl:  cfg.TTL,
		log:  log,
	}, nil
}

// Get retrieves a cached entry from disk.
func (c *DiskCache) Get(ctx context.Context, key string) (*core.ResultSet, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheFile := filepath.Join(c.path, sanitizeKey(key)+".json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		c.misses++
		return nil, nil
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		c.misses++
		return nil, nil
	}

	if time.Now().After(e.ExpiresAt) {
		os.Remove(cacheFile)
		c.misses++
		return nil, nil
	}

	c.hits++
	return e.ResultSet, nil
}

// Set stores an entry on disk.
func (c *DiskCache) Set(ctx context.Context, key string, resultSet *core.ResultSet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	e := entry{
		Key:       key,
		ResultSet: resultSet,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}

	cacheFile := filepath.Join(c.path, sanitizeKey(key)+".json")
	return os.WriteFile(cacheFile, data, 0644)
}

// Delete removes a cached entry from disk.
func (c *DiskCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cacheFile := filepath.Join(c.path, sanitizeKey(key)+".json")
	return os.Remove(cacheFile)
}

// Clear removes all cached entries.
func (c *DiskCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.RemoveAll(c.path)
}

// Stats returns disk cache statistics.
func (c *DiskCache) Stats(ctx context.Context) (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &Stats{
		Hits:   c.hits,
		Misses: c.misses,
	}, nil
}

// Close cleans up disk cache.
func (c *DiskCache) Close() error {
	return nil
}

// sanitizeKey makes a key safe for filesystem use.
func sanitizeKey(key string) string {
	safe := make([]byte, 0, len(key))
	for _, b := range []byte(key) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' {
			safe = append(safe, b)
		} else {
			safe = append(safe, '_')
		}
	}
	return string(safe)
}

// Manager oversees multiple cache backends.
type Manager struct {
	caches map[string]Cache
	mu     sync.RWMutex
	log    *logger.Logger
}

// NewManager creates a new cache manager.
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		caches: make(map[string]Cache),
		log:    log,
	}
}

// Register adds a named cache.
func (m *Manager) Register(name string, cache Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caches[name] = cache
}

// Get retrieves a cache by name.
func (m *Manager) Get(name string) (Cache, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.caches[name]
	return c, ok
}

// CloseAll closes all registered caches.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for name, cache := range m.caches {
		if err := cache.Close(); err != nil {
			m.log.Error("failed to close cache", err, logger.LogFields{"name": name})
			lastErr = err
		}
	}
	return lastErr
}

// NewCacheFromConfig creates a cache based on configuration.
func NewCacheFromConfig(cfg *core.CacheConfig, log *logger.Logger) (Cache, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	switch cfg.Type {
	case "memory":
		return NewMemoryCache(cfg, log)
	case "disk":
		return NewDiskCache(cfg, log)
	default:
		return NewMemoryCache(cfg, log)
	}
}
