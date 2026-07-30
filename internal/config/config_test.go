package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	mgr := New()
	require.NotNil(t, mgr)
	assert.NotNil(t, mgr.Viper())
}

func TestLoad_DefaultConfig(t *testing.T) {
	mgr := New()
	cfg, err := mgr.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "bing", string(cfg.Providers.Default))
	assert.True(t, cfg.Providers.Bing.Enabled)
	assert.Equal(t, 10.0, cfg.Providers.Bing.RateLimit)
	assert.True(t, cfg.Network.Evasion.Enabled)
}

func TestLoadFile(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test_config.yaml")
	configContent := `
logging:
  level: debug
network:
  timeout: 15s
  retry_count: 5
providers:
  default: bing
  bing:
    enabled: true
    rate_limit: 20
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	mgr := New()
	cfg, err := mgr.LoadFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 5, cfg.Network.RetryCount)
	assert.Equal(t, 20.0, cfg.Providers.Bing.RateLimit)
}

func TestLoadFile_NotFound(t *testing.T) {
	mgr := New()
	_, err := mgr.LoadFile("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bingdork.yaml")

	err := WriteDefaultConfig(path)
	require.NoError(t, err)

	// Verify file was written
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "BingDork Pro Configuration")
	assert.Contains(t, string(data), "bing:")
}

func TestDefaultConfigYAML(t *testing.T) {
	yaml := DefaultConfigYAML()
	assert.NotEmpty(t, yaml)
	assert.True(t, strings.Contains(yaml, "logging:"))
	assert.True(t, strings.Contains(yaml, "bing:"))
	assert.True(t, strings.Contains(yaml, "evasion:"))
	assert.True(t, strings.Contains(yaml, "captcha:"))
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("BINGDORK_LOGGING_LEVEL", "debug")
	defer os.Unsetenv("BINGDORK_LOGGING_LEVEL")

	os.Setenv("BINGDORK_NETWORK_TIMEOUT", "60s")
	defer os.Unsetenv("BINGDORK_NETWORK_TIMEOUT")

	mgr := New()
	cfg, err := mgr.Load()
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "1m0s", cfg.Network.Timeout.String()) // viper parses "60s" as 1m0s
}

func TestEnvironmentVariableMapping(t *testing.T) {
	os.Setenv("BINGDORK_PROVIDERS_BING_RATE_LIMIT", "30")
	defer os.Unsetenv("BINGDORK_PROVIDERS_BING_RATE_LIMIT")

	os.Setenv("BINGDORK_NETWORK_EVASION_ENABLED", "false")
	defer os.Unsetenv("BINGDORK_NETWORK_EVASION_ENABLED")

	mgr := New()
	cfg, err := mgr.Load()
	require.NoError(t, err)

	assert.Equal(t, 30.0, cfg.Providers.Bing.RateLimit)
	assert.False(t, cfg.Network.Evasion.Enabled)
}

func TestProviderConfigs(t *testing.T) {
	yaml := DefaultConfigYAML()
	assert.True(t, strings.Contains(yaml, "google:"))
	assert.True(t, strings.Contains(yaml, "duckduckgo:"))
	assert.True(t, strings.Contains(yaml, "brave:"))
	assert.True(t, strings.Contains(yaml, "yahoo:"))
	assert.True(t, strings.Contains(yaml, "mojeek:"))
	assert.True(t, strings.Contains(yaml, "searxng:"))
}
