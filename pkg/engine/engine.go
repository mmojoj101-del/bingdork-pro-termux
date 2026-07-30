// Package engine orchestrates search providers and query execution.
package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Engine is the core search orchestrator.
type Engine struct {
	providers map[core.ProviderID]core.SearchProvider
	defaultID core.ProviderID
	log       *logger.Logger
	mu        sync.RWMutex
	registry  *ProviderRegistry
}

// New creates a new Engine with the given providers.
func New(log *logger.Logger, providers ...core.SearchProvider) *Engine {
	e := &Engine{
		providers: make(map[core.ProviderID]core.SearchProvider),
		log:       log,
		registry:  NewProviderRegistry(log),
	}

	for _, p := range providers {
		if p != nil {
			e.providers[p.ID()] = p
			e.registry.Register(p)
		}
	}

	if len(e.providers) > 0 {
		// Find the first provider to set as default
		for id := range e.providers {
			e.defaultID = id
			break
		}
	}

	return e
}

// Search executes a search using the specified provider or the default.
func (e *Engine) Search(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	provider, err := e.getProvider(query.Provider)
	if err != nil {
		return nil, fmt.Errorf("engine search: %w", err)
	}

	start := time.Now()
	log := e.log.FromContext(ctx).With(logger.LogFields{
		"provider": provider.ID(),
		"query":    query.Query,
		"page":     query.Page,
	})

	log.Debug("executing search")

	resultSet, err := provider.Search(ctx, query)
	if err != nil {
		log.Error("search failed", err)
		return nil, fmt.Errorf("provider %s search: %w", provider.ID(), err)
	}

	resultSet.Duration = time.Since(start)
	resultSet.Query = query.Query
	resultSet.Provider = provider.ID()
	resultSet.Timestamp = time.Now()

	log.Info("search completed", logger.LogFields{
		"results":  len(resultSet.Results),
		"duration": resultSet.Duration.String(),
	})

	return resultSet, nil
}

// SearchAll executes a search across all enabled providers.
func (e *Engine) SearchAll(ctx context.Context, query *core.SearchQuery) ([]*core.ResultSet, error) {
	e.mu.RLock()
	providers := make([]core.SearchProvider, 0, len(e.providers))
	for _, p := range e.providers {
		providers = append(providers, p)
	}
	e.mu.RUnlock()

	type result struct {
		set *core.ResultSet
		err error
		pid core.ProviderID
	}

	results := make(chan result, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		p := p
		go func() {
			defer wg.Done()
			set, err := p.Search(ctx, query)
			results <- result{set: set, err: err, pid: p.ID()}
		}()
	}

	wg.Wait()
	close(results)

	var sets []*core.ResultSet
	var errors []error

	for r := range results {
		if r.err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", r.pid, r.err))
			continue
		}
		if r.set != nil {
			sets = append(sets, r.set)
		}
	}

	if len(sets) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all providers failed: %v", errors)
	}

	return sets, nil
}

// NextPage fetches the next page of results.
func (e *Engine) NextPage(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	provider, err := e.getProvider(query.Provider)
	if err != nil {
		return nil, fmt.Errorf("engine next page: %w", err)
	}

	query.Page++
	return provider.NextPage(ctx, query)
}

// Health checks the health of a specific provider or all providers.
func (e *Engine) Health(ctx context.Context, providerID ...core.ProviderID) []*core.ProviderHealth {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*core.ProviderHealth
	if len(providerID) > 0 {
		for _, id := range providerID {
			if p, ok := e.providers[id]; ok {
				results = append(results, p.Health(ctx))
			}
		}
	} else {
		for _, p := range e.providers {
			results = append(results, p.Health(ctx))
		}
	}
	return results
}

// Capabilities returns the capabilities of a provider.
func (e *Engine) Capabilities(providerID core.ProviderID) (*core.ProviderCapabilities, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, ok := e.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}
	return p.Capabilities(), nil
}

// Providers returns a list of registered provider IDs.
func (e *Engine) Providers() []core.ProviderID {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]core.ProviderID, 0, len(e.providers))
	for id := range e.providers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

// Provider returns a specific provider by ID.
func (e *Engine) Provider(id core.ProviderID) (core.SearchProvider, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.providers[id]
	return p, ok
}

// RegisterProvider adds a new provider to the engine.
func (e *Engine) RegisterProvider(p core.SearchProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providers[p.ID()] = p
	e.registry.Register(p)
	if len(e.providers) == 1 {
		e.defaultID = p.ID()
	}
	e.log.Info("provider registered", logger.LogFields{
		"provider": p.ID(),
	})
}

// SetDefault sets the default provider.
func (e *Engine) SetDefault(id core.ProviderID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.providers[id]; !ok {
		return fmt.Errorf("provider not found: %s", id)
	}
	e.defaultID = id
	return nil
}

// Registry returns the provider registry for plugin management.
func (e *Engine) Registry() *ProviderRegistry {
	return e.registry
}

// Close gracefully shuts down all providers.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, p := range e.providers {
		e.log.Debug("closing provider", logger.LogFields{"provider": id})
		_ = p // In a full implementation, providers would implement io.Closer
	}
}

// getProvider retrieves a provider by ID, falling back to default.
func (e *Engine) getProvider(id core.ProviderID) (core.SearchProvider, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if id == "" {
		id = e.defaultID
	}

	p, ok := e.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %q not found (default: %q)", id, e.defaultID)
	}
	return p, nil
}

// ProviderRegistry manages provider lifecycle and discovery.
type ProviderRegistry struct {
	providers map[core.ProviderID]core.SearchProvider
	mu        sync.RWMutex
	log       *logger.Logger
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry(log *logger.Logger) *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[core.ProviderID]core.SearchProvider),
		log:       log,
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(p core.SearchProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
	r.log.Info("provider registered in registry", logger.LogFields{
		"provider": p.ID(),
	})
}

// Unregister removes a provider from the registry.
func (r *ProviderRegistry) Unregister(id core.ProviderID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
	r.log.Info("provider unregistered", logger.LogFields{
		"provider": id,
	})
}

// Get retrieves a provider by ID.
func (r *ProviderRegistry) Get(id core.ProviderID) (core.SearchProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// List returns all registered provider IDs.
func (r *ProviderRegistry) List() []core.ProviderID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]core.ProviderID, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// Len returns the number of registered providers.
func (r *ProviderRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}
