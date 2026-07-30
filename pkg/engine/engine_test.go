package engine

import (
	"context"
	"testing"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements core.SearchProvider for testing.
type mockProvider struct {
	id           core.ProviderID
	searchFn     func(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error)
	nextPageFn   func(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error)
	healthFn     func(ctx context.Context) *core.ProviderHealth
	capsFn       func() *core.ProviderCapabilities
	rateLimitFn  func(ctx context.Context) *core.RateLimitInfo
	metadataFn   func(ctx context.Context) map[string]interface{}
}

func (m *mockProvider) ID() core.ProviderID { return m.id }
func (m *mockProvider) Search(ctx context.Context, q *core.SearchQuery) (*core.ResultSet, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, q)
	}
	return &core.ResultSet{Results: []*core.Result{{Title: "test", URL: "https://example.com"}}}, nil
}
func (m *mockProvider) NextPage(ctx context.Context, q *core.SearchQuery) (*core.ResultSet, error) {
	if m.nextPageFn != nil {
		return m.nextPageFn(ctx, q)
	}
	return &core.ResultSet{}, nil
}
func (m *mockProvider) Health(ctx context.Context) *core.ProviderHealth {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return &core.ProviderHealth{Provider: m.id, Healthy: true}
}
func (m *mockProvider) Capabilities() *core.ProviderCapabilities {
	if m.capsFn != nil {
		return m.capsFn()
	}
	return &core.ProviderCapabilities{Pagination: true}
}
func (m *mockProvider) RateLimit(ctx context.Context) *core.RateLimitInfo {
	if m.rateLimitFn != nil {
		return m.rateLimitFn(ctx)
	}
	return &core.RateLimitInfo{RequestsPerMinute: 10}
}
func (m *mockProvider) Metadata(ctx context.Context) map[string]interface{} {
	if m.metadataFn != nil {
		return m.metadataFn(ctx)
	}
	return map[string]interface{}{"provider": string(m.id)}
}

func TestNewEngine(t *testing.T) {
	log := logger.NewNopLogger()
	mock := &mockProvider{id: core.ProviderBing}
	e := New(log, mock)
	require.NotNil(t, e)
	assert.Contains(t, e.Providers(), core.ProviderBing)
}

func TestSearch(t *testing.T) {
	log := logger.NewNopLogger()
	mock := &mockProvider{id: core.ProviderBing}
	e := New(log, mock)

	resultSet, err := e.Search(context.Background(), &core.SearchQuery{Query: "test"})
	require.NoError(t, err)
	require.NotNil(t, resultSet)
	assert.Len(t, resultSet.Results, 1)
	assert.Equal(t, "test", resultSet.Query)
}

func TestSearch_ProviderNotFound(t *testing.T) {
	log := logger.NewNopLogger()
	e := New(log) // no providers

	_, err := e.Search(context.Background(), &core.SearchQuery{Query: "test", Provider: "nonexistent"})
	assert.Error(t, err)
}

func TestSearchAll(t *testing.T) {
	log := logger.NewNopLogger()
	mock1 := &mockProvider{id: core.ProviderBing}
	mock2 := &mockProvider{id: core.ProviderGoogle}
	e := New(log, mock1, mock2)

	sets, err := e.SearchAll(context.Background(), &core.SearchQuery{Query: "test"})
	require.NoError(t, err)
	assert.Len(t, sets, 2)
}

func TestHealth(t *testing.T) {
	log := logger.NewNopLogger()
	mock := &mockProvider{id: core.ProviderBing}
	e := New(log, mock)

	health := e.Health(context.Background())
	require.Len(t, health, 1)
	assert.True(t, health[0].Healthy)
	assert.Equal(t, core.ProviderBing, health[0].Provider)
}

func TestCapabilities(t *testing.T) {
	log := logger.NewNopLogger()
	mock := &mockProvider{
		id: core.ProviderBing,
		capsFn: func() *core.ProviderCapabilities {
			return &core.ProviderCapabilities{Pagination: true, MaxPages: 50}
		},
	}
	e := New(log, mock)

	caps, err := e.Capabilities(core.ProviderBing)
	require.NoError(t, err)
	assert.True(t, caps.Pagination)
	assert.Equal(t, 50, caps.MaxPages)
}

func TestSetDefault(t *testing.T) {
	log := logger.NewNopLogger()
	mock1 := &mockProvider{id: "provider1"}
	mock2 := &mockProvider{id: "provider2"}
	e := New(log, mock1, mock2)

	err := e.SetDefault("provider2")
	require.NoError(t, err)
}

func TestSetDefault_NotFound(t *testing.T) {
	log := logger.NewNopLogger()
	e := New(log, &mockProvider{id: core.ProviderBing})

	err := e.SetDefault("nonexistent")
	assert.Error(t, err)
}

func TestRegisterProvider(t *testing.T) {
	log := logger.NewNopLogger()
	e := New(log)
	assert.Len(t, e.Providers(), 0)

	e.RegisterProvider(&mockProvider{id: "new-provider"})
	assert.Contains(t, e.Providers(), core.ProviderID("new-provider"))
}

func TestProviderRegistry(t *testing.T) {
	log := logger.NewNopLogger()
	registry := NewProviderRegistry(log)

	mock := &mockProvider{id: core.ProviderBing}
	registry.Register(mock)

	assert.Equal(t, 1, registry.Len())
	assert.Contains(t, registry.List(), core.ProviderBing)

	p, ok := registry.Get(core.ProviderBing)
	assert.True(t, ok)
	assert.Equal(t, core.ProviderBing, p.ID())

	registry.Unregister(core.ProviderBing)
	assert.Equal(t, 0, registry.Len())
}

func BenchmarkEngineSearch(b *testing.B) {
	log := logger.NewNopLogger()
	mock := &mockProvider{id: core.ProviderBing}
	e := New(log, mock)

	ctx := context.Background()
	query := &core.SearchQuery{Query: "benchmark test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Search(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngineSearchAll(b *testing.B) {
	log := logger.NewNopLogger()
	mock1 := &mockProvider{id: "p1"}
	mock2 := &mockProvider{id: "p2"}
	mock3 := &mockProvider{id: "p3"}
	e := New(log, mock1, mock2, mock3)

	ctx := context.Background()
	query := &core.SearchQuery{Query: "benchmark test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.SearchAll(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
	}
}
