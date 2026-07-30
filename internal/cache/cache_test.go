package cache

import (
	"context"
	"testing"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCacheConfig() *core.CacheConfig {
	return &core.CacheConfig{
		Enabled:         true,
		Type:            "memory",
		MemorySize:      100,
		TTL:             5 * time.Minute,
		CleanupInterval: time.Minute,
	}
}

func testResultSet() *core.ResultSet {
	return &core.ResultSet{
		Query:    "test",
		Provider: "bing",
		Results: []*core.Result{
			{Title: "Test", URL: "https://example.com"},
		},
	}
}

func TestMemoryCache(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := testCacheConfig()
	cache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	key := "test-key"

	// Initial miss
	result, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, result)

	// Set
	err = cache.Set(ctx, key, testResultSet())
	require.NoError(t, err)

	// Get hit
	result, err = cache.Get(ctx, key)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test", result.Query)

	// Delete
	err = cache.Delete(ctx, key)
	require.NoError(t, err)

	// Should be gone
	result, err = cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryCache_Expiry(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.CacheConfig{
		Enabled:    true,
		Type:       "memory",
		MemorySize: 100,
		TTL:        1 * time.Nanosecond,
	}
	cache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	key := "expiry-key"

	err = cache.Set(ctx, key, testResultSet())
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	result, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryCache_Clear(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := testCacheConfig()
	cache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	err = cache.Set(ctx, "key1", testResultSet())
	require.NoError(t, err)
	err = cache.Set(ctx, "key2", testResultSet())
	require.NoError(t, err)

	err = cache.Clear(ctx)
	require.NoError(t, err)

	result, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryCache_Eviction(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.CacheConfig{
		Enabled:    true,
		Type:       "memory",
		MemorySize: 2,
	}
	cache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Fill cache to max
	err = cache.Set(ctx, "key1", testResultSet())
	require.NoError(t, err)
	err = cache.Set(ctx, "key2", testResultSet())
	require.NoError(t, err)

	// This should evict one
	err = cache.Set(ctx, "key3", testResultSet())
	require.NoError(t, err)

	// Cache should have 2 items (one was evicted)
	stats, err := cache.Stats(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, stats.Items, 2)
}

func TestMemoryCache_Stats(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := testCacheConfig()
	cache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	stats, err := cache.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)

	// Miss
	cache.Get(ctx, "nonexistent")
	// Hit
	cache.Set(ctx, "exists", testResultSet())
	cache.Get(ctx, "exists")

	stats, err = cache.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses) // the initial miss + the hit counted before
}

func TestNewCacheFromConfig(t *testing.T) {
	log := logger.NewNopLogger()

	// Memory
	cfg := &core.CacheConfig{Enabled: true, Type: "memory", MemorySize: 100}
	cache, err := NewCacheFromConfig(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Disabled
	cfg2 := &core.CacheConfig{Enabled: false}
	cache2, err := NewCacheFromConfig(cfg2, log)
	require.NoError(t, err)
	assert.Nil(t, cache2)
}

func TestManager(t *testing.T) {
	log := logger.NewNopLogger()
	mgr := NewManager(log)

	cfg := testCacheConfig()
	memCache, err := NewMemoryCache(cfg, log)
	require.NoError(t, err)
	defer memCache.Close()

	mgr.Register("primary", memCache)

	c, ok := mgr.Get("primary")
	assert.True(t, ok)
	assert.NotNil(t, c)

	_, ok = mgr.Get("nonexistent")
	assert.False(t, ok)

	err = mgr.CloseAll()
	require.NoError(t, err)
}

func BenchmarkMemoryCache(b *testing.B) {
	log := logger.NewNopLogger()
	cfg := &core.CacheConfig{
		Enabled:    true,
		Type:       "memory",
		MemorySize: 10000,
	}
	cache, err := NewMemoryCache(cfg, log)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "bench-key"
	rs := testResultSet()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, key, rs)
		cache.Get(ctx, key)
	}
}
