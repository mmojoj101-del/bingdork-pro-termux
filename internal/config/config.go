// Package config manages application configuration via Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/bingdork/bingdork/internal/core"
)

// Manager handles configuration loading, merging, and access.
type Manager struct {
	v *viper.Viper
}

// New creates a new ConfigManager with default settings.
func New() *Manager {
	v := viper.New()
	v.SetConfigName("bingdork")
	v.SetConfigType("yaml")

	// Config search paths
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("$HOME/.bingdork")
	v.AddConfigPath("/etc/bingdork")

	// Environment variable mapping
	v.SetEnvPrefix("BINGDORK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Default values
	setDefaults(v)

	return &Manager{v: v}
}

// Load loads configuration from all sources.
func (m *Manager) Load() (*core.Config, error) {
	// Try to read config file
	if err := m.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg core.Config
	if err := m.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// LoadFile loads configuration from a specific file.
func (m *Manager) LoadFile(path string) (*core.Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving config path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("config file not found: %s: %w", absPath, err)
	}

	m.v.SetConfigFile(absPath)
	if err := m.v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg core.Config
	if err := m.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// Viper returns the underlying viper instance for advanced usage.
func (m *Manager) Viper() *viper.Viper {
	return m.v
}

// WriteDefaultConfig writes a default configuration file to the given path.
func WriteDefaultConfig(path string) error {
	data := DefaultConfigYAML()
	return os.WriteFile(path, []byte(data), 0644)
}

// DefaultConfigYAML returns the default configuration as YAML.
func DefaultConfigYAML() string {
	return `# BingDork Pro Configuration
logging:
  level: info
  format: console
  output: stdout
  file: ""
  no_color: false

network:
  timeout: 30s
  retry_count: 3
  retry_wait_min: 1s
  retry_wait_max: 10s
  proxy: ""
  proxy_type: ""
  rate_limit: 10
  rate_burst: 5
  http2: true
  compress: true
  keep_alive: 30s
  user_agents:
    - "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
    - "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.2 Safari/605.1.15"
    - "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
    - "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0"
  custom_headers: {}
  captcha:
    enabled: false
    service: ""
    api_key: ""
    auto_solve: true
    session_reuse: true
    image_recognition: false
    audio_recognition: false
    timeout: 60s
    retry_on_failure: true
  evasion:
    enabled: true
    fingerprint_randomize: true
    header_spoofing: true
    ip_rotation: false
    behavior_mimic: true
    tls_fingerprint: true
    referrer_spoofing: true
    random_delay_min: 1s
    random_delay_max: 3s
    proxy_rotation: false
    rotate_every: 10

cache:
  type: memory
  memory_size: 10000
  disk_path: ~/.bingdork/cache
  disk_size: 100MB
  ttl: 5m
  cleanup_interval: 1m
  enabled: true

storage:
  type: sqlite
  path: ~/.bingdork/data/bingdork.db
  boltdb: ~/.bingdork/data/bingdork.bolt

output:
  format: json
  pretty_print: true
  raw_html: false

scheduler:
  workers: 5
  queue_size: 100
  rate_limit: 10
  retry_count: 3
  priority: true
  resume: true
  state_file: ~/.bingdork/state.json
  job_timeout: 5m

providers:
  default: bing
  bing:
    enabled: true
    base_url: https://www.bing.com/search
    rate_limit: 10
    timeout: 30s
    captcha: true
  google:
    enabled: false
    base_url: https://www.google.com/search
    rate_limit: 5
    timeout: 30s
  duckduckgo:
    enabled: false
    base_url: https://html.duckduckgo.com/html
    rate_limit: 5
    timeout: 30s
  brave:
    enabled: false
    base_url: https://search.brave.com/search
    rate_limit: 10
    timeout: 30s
  yahoo:
    enabled: false
    base_url: https://search.yahoo.com/search
    rate_limit: 5
    timeout: 30s
  mojeek:
    enabled: false
    base_url: https://www.mojeek.com/search
    rate_limit: 5
    timeout: 30s
  searxng:
    enabled: false
    base_url: http://localhost:8888
    rate_limit: 30
    timeout: 30s

api:
  enabled: false
  type: rest
  host: 127.0.0.1
  port: 8080
  grpc_port: 9090
  cors: true
  tls: false

tui:
  enabled: false
  theme: default
  log_level: info

metrics:
  enabled: false
  type: prometheus
  port: 2112
  path: /metrics

plugins:
  enabled: false
  dir: ~/.bingdork/plugins
  allow_all: false
`
}

// setDefaults configures viper defaults.
func setDefaults(v *viper.Viper) {
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("logging.output", "stdout")

	v.SetDefault("network.timeout", "30s")
	v.SetDefault("network.retry_count", 3)
	v.SetDefault("network.retry_wait_min", "1s")
	v.SetDefault("network.retry_wait_max", "10s")
	v.SetDefault("network.rate_limit", 10)
	v.SetDefault("network.rate_burst", 5)
	v.SetDefault("network.http2", true)
	v.SetDefault("network.compress", true)

	v.SetDefault("network.evasion.enabled", true)
	v.SetDefault("network.evasion.fingerprint_randomize", true)
	v.SetDefault("network.evasion.header_spoofing", true)
	v.SetDefault("network.evasion.behavior_mimic", true)
	v.SetDefault("network.evasion.tls_fingerprint", true)
	v.SetDefault("network.evasion.referrer_spoofing", true)
	v.SetDefault("network.evasion.random_delay_min", "1s")
	v.SetDefault("network.evasion.random_delay_max", "3s")

	v.SetDefault("cache.enabled", true)
	v.SetDefault("cache.type", "memory")
	v.SetDefault("cache.memory_size", 10000)
	v.SetDefault("cache.ttl", "5m")
	v.SetDefault("cache.cleanup_interval", "1m")

	v.SetDefault("storage.type", "sqlite")

	v.SetDefault("output.format", "json")
	v.SetDefault("output.pretty_print", true)

	v.SetDefault("scheduler.workers", 5)
	v.SetDefault("scheduler.queue_size", 100)
	v.SetDefault("scheduler.rate_limit", 10)
	v.SetDefault("scheduler.retry_count", 3)
	v.SetDefault("scheduler.resume", true)
	v.SetDefault("scheduler.job_timeout", "5m")

	v.SetDefault("providers.default", "bing")
	v.SetDefault("providers.bing.enabled", true)
	v.SetDefault("providers.bing.rate_limit", 10)

	v.SetDefault("api.enabled", false)
	v.SetDefault("api.host", "127.0.0.1")
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.grpc_port", 9090)

	v.SetDefault("metrics.enabled", false)

	v.SetDefault("plugins.enabled", false)
	v.SetDefault("plugins.dir", "~/.bingdork/plugins")
}
