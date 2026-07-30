// Package storage provides persistent storage for search results.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/mattn/go-sqlite3"
	bolt "go.etcd.io/bbolt"
)

// Store is the interface for result storage.
type Store interface {
	// Save persists a result set.
	Save(ctx context.Context, resultSet *core.ResultSet) error

	// Query retrieves results matching the given query.
	Query(ctx context.Context, q *Query) ([]*core.Result, error)

	// Stats returns storage statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Close closes the store.
	Close() error
}

// Query represents a storage query.
type Query struct {
	Query       string `json:"query,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Host        string `json:"host,omitempty"`
	RootDomain  string `json:"root_domain,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	FromDate    string `json:"from_date,omitempty"`
	ToDate      string `json:"to_date,omitempty"`
}

// Stats holds storage statistics.
type Stats struct {
	TotalResults  int   `json:"total_results"`
	TotalQueries  int   `json:"total_queries"`
	UniqueDomains int   `json:"unique_domains"`
	StorageSize   int64 `json:"storage_size"`
}

// Manager oversees multiple storage backends.
type Manager struct {
	stores map[string]Store
	mu     sync.RWMutex
	log    *logger.Logger
}

// NewManager creates a new storage manager.
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		stores: make(map[string]Store),
		log:    log,
	}
}

// Register adds a named store.
func (m *Manager) Register(name string, store Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stores[name] = store
	m.log.Info("storage registered", logger.LogFields{"name": name})
}

// Get retrieves a store by name.
func (m *Manager) Get(name string) (Store, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.stores[name]
	return s, ok
}

// CloseAll closes all registered stores.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for name, store := range m.stores {
		if err := store.Close(); err != nil {
			m.log.Error("failed to close store", err, logger.LogFields{"name": name})
			lastErr = err
		}
	}
	return lastErr
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	path string
	db   *sqlite3.SQLiteConn
	log  *logger.Logger
	mu   sync.Mutex
}

// NewSQLiteStore creates a new SQLite-backed store.
func NewSQLiteStore(path string, log *logger.Logger) (*SQLiteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	// For simplicity, we use a file-based approach
	// In production, use database/sql with SQLite driver
	s := &SQLiteStore{
		path: path,
		log:  log,
	}

	log.Info("sqlite store initialized", logger.LogFields{"path": path})
	return s, nil
}

// Save persists a result set to SQLite.
func (s *SQLiteStore) Save(ctx context.Context, resultSet *core.ResultSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Errorf("sqlite store: not fully implemented, use JSON/CSV fallback")
}

// Query retrieves results from SQLite.
func (s *SQLiteStore) Query(ctx context.Context, q *Query) ([]*core.Result, error) {
	return nil, fmt.Errorf("sqlite query not fully implemented")
}

// Stats returns SQLite storage statistics.
func (s *SQLiteStore) Stats(ctx context.Context) (*Stats, error) {
	return &Stats{}, nil
}

// Close closes the SQLite store.
func (s *SQLiteStore) Close() error {
	return nil
}

// BoltStore implements Store using BoltDB.
type BoltStore struct {
	db  *bolt.DB
	mu  sync.Mutex
	log *logger.Logger
}

// NewBoltStore creates a new BoltDB-backed store.
func NewBoltStore(path string, log *logger.Logger) (*BoltStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating bolt directory: %w", err)
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening bolt db: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("results"))
		if err != nil {
			return fmt.Errorf("creating results bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("metadata"))
		return err
	}); err != nil {
		return nil, err
	}

	s := &BoltStore{
		db:  db,
		log: log,
	}

	log.Info("boltdb store initialized", logger.LogFields{"path": path})
	return s, nil
}

// Save persists a result set to BoltDB.
func (s *BoltStore) Save(ctx context.Context, resultSet *core.ResultSet) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("results"))
		for _, result := range resultSet.Results {
			data, err := json.Marshal(result)
			if err != nil {
				return err
			}
			key := []byte(fmt.Sprintf("%s:%s:%d", resultSet.Query, result.URL, result.SearchPos))
			if err := b.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// Query retrieves results from BoltDB.
func (s *BoltStore) Query(ctx context.Context, q *Query) ([]*core.Result, error) {
	var results []*core.Result
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("results"))
		return b.ForEach(func(k, v []byte) error {
			var r core.Result
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			results = append(results, &r)
			return nil
		})
	})
	return results, err
}

// Stats returns BoltDB storage statistics.
func (s *BoltStore) Stats(ctx context.Context) (*Stats, error) {
	var stats Stats
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("results"))
		stats.TotalResults = b.Stats().KeyN
		return nil
	})
	return &stats, err
}

// Close closes the BoltDB store.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// JSONLineStore implements a simple JSON-lines file store.
type JSONLineStore struct {
	path string
	f    *os.File
	mu   sync.Mutex
	log  *logger.Logger
}

// NewJSONLineStore creates a new JSON-lines file store.
func NewJSONLineStore(path string, log *logger.Logger) (*JSONLineStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	return &JSONLineStore{
		path: path,
		f:    f,
		log:  log,
	}, nil
}

// Save appends results as JSON lines.
func (s *JSONLineStore) Save(ctx context.Context, resultSet *core.ResultSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encoder := json.NewEncoder(s.f)
	for _, result := range resultSet.Results {
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return s.f.Sync()
}

// Query is not supported for JSON lines (linear scan required).
func (s *JSONLineStore) Query(ctx context.Context, q *Query) ([]*core.Result, error) {
	return nil, fmt.Errorf("json-line store does not support queries")
}

// Stats returns file size.
func (s *JSONLineStore) Stats(ctx context.Context) (*Stats, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return &Stats{}, nil
	}
	return &Stats{StorageSize: info.Size()}, nil
}

// Close closes the file.
func (s *JSONLineStore) Close() error {
	return s.f.Close()
}

// NewStoreFromConfig creates a store based on configuration.
func NewStoreFromConfig(cfg *core.StorageConfig, log *logger.Logger) (Store, error) {
	switch cfg.Type {
	case "sqlite":
		return NewSQLiteStore(cfg.Path, log)
	case "boltdb":
		return NewBoltStore(cfg.BoltDB, log)
	case "json", "jsonl", "jsonline":
		return NewJSONLineStore(cfg.Path, log)
	default:
		return NewJSONLineStore(cfg.Path, log)
	}
}
