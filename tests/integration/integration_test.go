// Package integration_test contains integration tests for BingDork Pro.
package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bingdork/bingdork/internal/config"
	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/internal/metrics"
	"github.com/bingdork/bingdork/internal/scheduler"
	"github.com/bingdork/bingdork/pkg/engine"
	"github.com/bingdork/bingdork/pkg/extractor"
	"github.com/bingdork/bingdork/pkg/output"
	"github.com/bingdork/bingdork/pkg/providers/bing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_BingProvider tests the Bing provider with real HTTP requests.
// This test requires network access and is skipped in short mode.
func TestIntegration_BingProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	log := logger.NewNopLogger()
	netCfg := &core.NetworkConfig{
		Timeout:     30,
		RetryCount:  2,
		RateLimit:   2,
		Evasion: core.EvasionConfig{
			Enabled:              true,
			FingerprintRandomize: true,
			HeaderSpoofing:       true,
			BehaviorMimic:        true,
		},
	}
	providerCfg := &core.ProviderConfig{
		Enabled:   true,
		BaseURL:   "https://www.bing.com/search",
		RateLimit: 2,
	}

	provider, err := bing.New(providerCfg, netCfg, log)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Check health
	ctx := context.Background()
	health := provider.Health(ctx)
	t.Logf("Bing health: healthy=%v, latency=%v, error=%q", health.Healthy, health.Latency, health.Error)

	if !health.Healthy {
		t.Skip("Bing is not reachable, skipping")
	}
}

// TestIntegration_EngineWithBing tests the full engine pipeline with Bing.
func TestIntegration_EngineWithBing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	log := logger.NewNopLogger()
	netCfg := &core.NetworkConfig{
		Timeout:    30,
		RetryCount: 2,
		RateLimit:  2,
	}
	providerCfg := &core.ProviderConfig{
		Enabled:   true,
		BaseURL:   "https://www.bing.com/search",
		RateLimit: 2,
	}

	provider, err := bing.New(providerCfg, netCfg, log)
	require.NoError(t, err)

	eng := engine.New(log, provider)
	ctx := context.Background()

	// Test search
	resultSet, err := eng.Search(ctx, &core.SearchQuery{
		Query:    "site:example.com test",
		Provider: "bing",
	})
	if err != nil {
		t.Skipf("Bing search failed (network may be unavailable): %v", err)
	}
	require.NotNil(t, resultSet)
	t.Logf("Got %d results", len(resultSet.Results))
}

// TestIntegration_FullPipeline tests the complete search pipeline.
func TestIntegration_FullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	log := logger.NewNopLogger()
	netCfg := &core.NetworkConfig{Timeout: 30, RateLimit: 2}
	providerCfg := &core.ProviderConfig{Enabled: true, RateLimit: 2}
	outputCfg := &core.OutputConfig{Format: "json"}

	// Set up components
	provider, err := bing.New(providerCfg, netCfg, log)
	require.NoError(t, err)

	eng := engine.New(log, provider)
	extr := extractor.New(log)
	metricsCollector := metrics.NewCollector(&core.MetricsConfig{}, log)

	// Output to temp file
	dir := t.TempDir()
	outputCfg.File = filepath.Join(dir, "results.json")
	exporter, err := output.NewJSONExporter(outputCfg, log)
	require.NoError(t, err)

	outputMgr := output.NewManager(log)
	outputMgr.Register(exporter)

	// Execute search
	ctx := context.Background()
	resultSet, err := eng.Search(ctx, &core.SearchQuery{
		Query:    "site:example.com",
		Provider: "bing",
	})
	if err != nil {
		t.Skipf("Search failed: %v", err)
	}

	// Extract
	resultSet, err = extr.Extract(ctx, resultSet)
	require.NoError(t, err)

	// Export
	err = outputMgr.ExportAll(ctx, resultSet)
	require.NoError(t, err)

	// Metrics
	metricsCollector.RecordQuery("bing", true, resultSet.Duration, len(resultSet.Results))

	// Verify output
	data, err := os.ReadFile(outputCfg.File)
	require.NoError(t, err)
	assert.Contains(t, string(data), "example.com")

	t.Logf("Pipeline complete: %d results exported", len(resultSet.Results))
}

// TestIntegration_SchedulerWithEngine tests the scheduler with the engine.
func TestIntegration_SchedulerWithEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	log := logger.NewNopLogger()
	schedCfg := &core.SchedulerConfig{
		Workers:   2,
		QueueSize: 10,
		JobTimeout: 30,
	}
	sched, err := scheduler.New(schedCfg, log)
	require.NoError(t, err)

	// Set up components
	netCfg := &core.NetworkConfig{Timeout: 30, RateLimit: 2}
	providerCfg := &core.ProviderConfig{Enabled: true, RateLimit: 2}
	provider, err := bing.New(providerCfg, netCfg, log)
	require.NoError(t, err)
	eng := engine.New(log, provider)

	// Register search handler
	sched.RegisterHandler("search", func(ctx context.Context, task *scheduler.Task) error {
		_, err := eng.Search(ctx, task.Query)
		return err
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)

	// Submit a query
	_, err = sched.SubmitQuery(ctx, &core.SearchQuery{
		Query:    "site:example.com",
		Provider: "bing",
	})
	require.NoError(t, err)

	sched.Stop()
	status := sched.Status()
	t.Logf("Scheduler status: %+v", status)
}

// TestIntegration_ConfigLoading tests loading a real config file.
func TestIntegration_ConfigLoading(t *testing.T) {
	mgr := config.New()
	cfg, err := mgr.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "info", cfg.Logging.Level)
	assert.True(t, cfg.Providers.Bing.Enabled)
	assert.True(t, cfg.Network.Evasion.Enabled)

	// Write config to temp file and load it
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.yaml")
	configContent := `
logging:
  level: debug
network:
  timeout: 15s
providers:
  bing:
    enabled: true
    rate_limit: 5
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	mgr2 := config.New()
	cfg2, err := mgr2.LoadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg2.Logging.Level)
	assert.Equal(t, float64(5), cfg2.Providers.Bing.RateLimit)
}
