// Package core defines the foundational types and interfaces for BingDork Pro.
// All packages import from this package to avoid circular dependencies.
package core

import (
	"context"
	"net/url"
	"time"
)

// ProviderID uniquely identifies a search engine provider.
type ProviderID string

const (
	ProviderBing       ProviderID = "bing"
	ProviderGoogle     ProviderID = "google"
	ProviderDuckDuckGo ProviderID = "duckduckgo"
	ProviderBrave      ProviderID = "brave"
	ProviderYahoo      ProviderID = "yahoo"
	ProviderMojeek     ProviderID = "mojeek"
	ProviderSearXNG    ProviderID = "searxng"
)

// SearchQuery represents a search query with its metadata.
type SearchQuery struct {
	Query      string            `json:"query" yaml:"query"`
	Provider   ProviderID        `json:"provider,omitempty" yaml:"provider,omitempty"`
	Page       int               `json:"page,omitempty" yaml:"page,omitempty"`
	Options    map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	MaxResults int               `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	Timestamp  time.Time         `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
	ID         string            `json:"id,omitempty" yaml:"id,omitempty"`
}

// Result represents a single search result.
type Result struct {
	Title         string    `json:"title" yaml:"title"`
	URL           string    `json:"url" yaml:"url"`
	Host          string    `json:"host" yaml:"host"`
	RootDomain    string    `json:"root_domain" yaml:"root_domain"`
	Description   string    `json:"description" yaml:"description"`
	SearchPos     int       `json:"search_position" yaml:"search_position"`
	Page          int       `json:"page" yaml:"page"`
	Timestamp     time.Time `json:"timestamp" yaml:"timestamp"`
	Engine        string    `json:"engine" yaml:"engine"`
	RawHTML       string    `json:"raw_html,omitempty" yaml:"raw_html,omitempty"`
	ResponseCode  int       `json:"response_code,omitempty" yaml:"response_code,omitempty"`
	ContentType   string    `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	ContentLength int64     `json:"content_length,omitempty" yaml:"content_length,omitempty"`
}

// NormalizedURL returns a normalized version of the result URL.
func (r *Result) NormalizedURL() string {
	u, err := url.Parse(r.URL)
	if err != nil {
		return r.URL
	}
	u.Fragment = ""
	u.RawQuery = ""
	return u.String()
}

// CanonicalURL returns the canonical form (scheme + host + path).
func (r *Result) CanonicalURL() string {
	u, err := url.Parse(r.URL)
	if err != nil {
		return r.URL
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.RawPath = ""
	u.ForceQuery = false
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u.String()
}

// ResultSet holds a collection of search results with metadata.
type ResultSet struct {
	Results   []*Result  `json:"results" yaml:"results"`
	Total     int        `json:"total" yaml:"total"`
	Query     string     `json:"query" yaml:"query"`
	Provider  ProviderID `json:"provider" yaml:"provider"`
	Duration  time.Duration `json:"duration_ns" yaml:"duration_ns"`
	Timestamp time.Time  `json:"timestamp" yaml:"timestamp"`
	Errors    []error    `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// ProviderCapabilities describes what a search provider supports.
type ProviderCapabilities struct {
	Pagination bool     `json:"pagination" yaml:"pagination"`
	SafeSearch bool     `json:"safe_search" yaml:"safe_search"`
	Language   bool     `json:"language" yaml:"language"`
	Region     bool     `json:"region" yaml:"region"`
	Advanced   []string `json:"advanced_operators" yaml:"advanced_operators"`
	MaxPages   int      `json:"max_pages" yaml:"max_pages"`
	RateLimit  int      `json:"rate_limit_per_minute" yaml:"rate_limit_per_minute"`
}

// RateLimitInfo describes current rate limit state.
type RateLimitInfo struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	Remaining         int           `json:"remaining"`
	ResetAt           time.Time     `json:"reset_at"`
	RetryAfter        time.Duration `json:"retry_after"`
}

// ProviderHealth represents the health status of a provider.
type ProviderHealth struct {
	Provider  ProviderID `json:"provider" yaml:"provider"`
	Healthy   bool       `json:"healthy" yaml:"healthy"`
	Latency   time.Duration `json:"latency" yaml:"latency"`
	LastCheck time.Time  `json:"last_check" yaml:"last_check"`
	Error     string     `json:"error,omitempty" yaml:"error,omitempty"`
}

// SearchProvider is the core interface all search engines must implement.
type SearchProvider interface {
	// ID returns the provider's unique identifier.
	ID() ProviderID

	// Search executes a search query and returns results.
	Search(ctx context.Context, query *SearchQuery) (*ResultSet, error)

	// NextPage fetches the next page of results from a previous search.
	NextPage(ctx context.Context, query *SearchQuery) (*ResultSet, error)

	// Health checks if the provider is reachable and functional.
	Health(ctx context.Context) *ProviderHealth

	// Capabilities returns what this provider supports.
	Capabilities() *ProviderCapabilities

	// RateLimit returns the current rate limit status.
	RateLimit(ctx context.Context) *RateLimitInfo

	// Metadata returns provider-specific metadata.
	Metadata(ctx context.Context) map[string]interface{}
}

// Filter defines a result filter.
type Filter struct {
	Type    FilterType `json:"type" yaml:"type"`
	Pattern string     `json:"pattern" yaml:"pattern"`
	Negate  bool       `json:"negate" yaml:"negate"`
}

// FilterType categorizes the kind of filter.
type FilterType string

const (
	FilterRegexInclude FilterType = "regex_include"
	FilterRegexExclude FilterType = "regex_exclude"
	FilterHostWhitelist FilterType = "host_whitelist"
	FilterHostBlacklist FilterType = "host_blacklist"
	FilterExtension     FilterType = "extension"
	FilterKeyword       FilterType = "keyword"
	FilterDuplicate     FilterType = "duplicate"
)

// Config is the top-level application configuration.
type Config struct {
	Logging  LoggingConfig  `json:"logging" yaml:"logging" mapstructure:"logging"`
	Network  NetworkConfig  `json:"network" yaml:"network" mapstructure:"network"`
	Cache    CacheConfig    `json:"cache" yaml:"cache" mapstructure:"cache"`
	Storage  StorageConfig  `json:"storage" yaml:"storage" mapstructure:"storage"`
	Output   OutputConfig   `json:"output" yaml:"output" mapstructure:"output"`
	Scheduler SchedulerConfig `json:"scheduler" yaml:"scheduler" mapstructure:"scheduler"`
	Providers ProvidersConfig `json:"providers" yaml:"providers" mapstructure:"providers"`
	API      APIConfig      `json:"api" yaml:"api" mapstructure:"api"`
	TUI      TUIConfig      `json:"tui" yaml:"tui" mapstructure:"tui"`
	Metrics  MetricsConfig  `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	Plugins  PluginsConfig  `json:"plugins" yaml:"plugins" mapstructure:"plugins"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level     string `json:"level" yaml:"level" mapstructure:"level"`
	Format    string `json:"format" yaml:"format" mapstructure:"format"`
	Output    string `json:"output" yaml:"output" mapstructure:"output"`
	File      string `json:"file" yaml:"file" mapstructure:"file"`
	NoColor   bool   `json:"no_color" yaml:"no_color" mapstructure:"no_color"`
}

// NetworkConfig holds networking configuration.
type NetworkConfig struct {
	Timeout       time.Duration     `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	RetryCount    int               `json:"retry_count" yaml:"retry_count" mapstructure:"retry_count"`
	RetryWaitMin  time.Duration     `json:"retry_wait_min" yaml:"retry_wait_min" mapstructure:"retry_wait_min"`
	RetryWaitMax  time.Duration     `json:"retry_wait_max" yaml:"retry_wait_max" mapstructure:"retry_wait_max"`
	Proxy         string            `json:"proxy" yaml:"proxy" mapstructure:"proxy"`
	ProxyType     string            `json:"proxy_type" yaml:"proxy_type" mapstructure:"proxy_type"`
	UserAgents    []string          `json:"user_agents" yaml:"user_agents" mapstructure:"user_agents"`
	CustomHeaders map[string]string `json:"custom_headers" yaml:"custom_headers" mapstructure:"custom_headers"`
	RateLimit     float64           `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"`
	RateBurst     int               `json:"rate_burst" yaml:"rate_burst" mapstructure:"rate_burst"`
	HTTP2         bool              `json:"http2" yaml:"http2" mapstructure:"http2"`
	Compress      bool              `json:"compress" yaml:"compress" mapstructure:"compress"`
	KeepAlive     time.Duration     `json:"keep_alive" yaml:"keep_alive" mapstructure:"keep_alive"`
	CAPTCHA       CAPTCHAConfig     `json:"captcha" yaml:"captcha" mapstructure:"captcha"`
	Evasion       EvasionConfig     `json:"evasion" yaml:"evasion" mapstructure:"evasion"`
}

// CAPTCHAConfig holds CAPTCHA bypass configuration.
type CAPTCHAConfig struct {
	Enabled         bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Service         string   `json:"service" yaml:"service" mapstructure:"service"`
	APIKey          string   `json:"api_key" yaml:"api_key" mapstructure:"api_key"`
	AutoSolve       bool     `json:"auto_solve" yaml:"auto_solve" mapstructure:"auto_solve"`
	SessionReuse    bool     `json:"session_reuse" yaml:"session_reuse" mapstructure:"session_reuse"`
	ImageRecognition bool    `json:"image_recognition" yaml:"image_recognition" mapstructure:"image_recognition"`
	AudioRecognition bool    `json:"audio_recognition" yaml:"audio_recognition" mapstructure:"audio_recognition"`
	Timeout         time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	RetryOnFailure  bool     `json:"retry_on_failure" yaml:"retry_on_failure" mapstructure:"retry_on_failure"`
	Providers       []string `json:"providers,omitempty" yaml:"providers,omitempty" mapstructure:"providers,omitempty"`
}

// EvasionConfig holds anti-bot evasion configuration.
type EvasionConfig struct {
	Enabled              bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	FingerprintRandomize bool     `json:"fingerprint_randomize" yaml:"fingerprint_randomize" mapstructure:"fingerprint_randomize"`
	HeaderSpoofing       bool     `json:"header_spoofing" yaml:"header_spoofing" mapstructure:"header_spoofing"`
	IPRotation           bool     `json:"ip_rotation" yaml:"ip_rotation" mapstructure:"ip_rotation"`
	BehaviorMimic        bool     `json:"behavior_mimic" yaml:"behavior_mimic" mapstructure:"behavior_mimic"`
	TLSFingerprint       bool     `json:"tls_fingerprint" yaml:"tls_fingerprint" mapstructure:"tls_fingerprint"`
	ReferrerSpoofing     bool     `json:"referrer_spoofing" yaml:"referrer_spoofing" mapstructure:"referrer_spoofing"`
	RandomDelayMin       time.Duration `json:"random_delay_min" yaml:"random_delay_min" mapstructure:"random_delay_min"`
	RandomDelayMax       time.Duration `json:"random_delay_max" yaml:"random_delay_max" mapstructure:"random_delay_max"`
	ProxyRotation        bool     `json:"proxy_rotation" yaml:"proxy_rotation" mapstructure:"proxy_rotation"`
	ProxyList            []string `json:"proxy_list,omitempty" yaml:"proxy_list,omitempty" mapstructure:"proxy_list,omitempty"`
	RotateEvery          int      `json:"rotate_every" yaml:"rotate_every" mapstructure:"rotate_every"`
	HumanizeMouseMovements bool   `json:"humanize_mouse_movements" yaml:"humanize_mouse_movements" mapstructure:"humanize_mouse_movements"`
}

// CacheConfig holds cache configuration.
type CacheConfig struct {
	Type       string        `json:"type" yaml:"type" mapstructure:"type"`
	MemorySize int           `json:"memory_size" yaml:"memory_size" mapstructure:"memory_size"`
	DiskPath   string        `json:"disk_path" yaml:"disk_path" mapstructure:"disk_path"`
	DiskSize   string        `json:"disk_size" yaml:"disk_size" mapstructure:"disk_size"`
	TTL        time.Duration `json:"ttl" yaml:"ttl" mapstructure:"ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval" mapstructure:"cleanup_interval"`
	Enabled    bool          `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}

// StorageConfig holds storage configuration.
type StorageConfig struct {
	Type   string `json:"type" yaml:"type" mapstructure:"type"`
	Path   string `json:"path" yaml:"path" mapstructure:"path"`
	DSN    string `json:"dsn" yaml:"dsn" mapstructure:"dsn"`
	BoltDB string `json:"boltdb" yaml:"boltdb" mapstructure:"boltdb"`
}

// OutputConfig holds output configuration.
type OutputConfig struct {
	Format      string   `json:"format" yaml:"format" mapstructure:"format"`
	File        string   `json:"file" yaml:"file" mapstructure:"file"`
	PrettyPrint bool     `json:"pretty_print" yaml:"pretty_print" mapstructure:"pretty_print"`
	NoHeader    bool     `json:"no_header" yaml:"no_header" mapstructure:"no_header"`
	RawHTML     bool     `json:"raw_html" yaml:"raw_html" mapstructure:"raw_html"`
	Dir         string   `json:"dir" yaml:"dir" mapstructure:"dir"`
	Append      bool     `json:"append" yaml:"append" mapstructure:"append"`
}

// SchedulerConfig holds scheduler configuration.
type SchedulerConfig struct {
	Workers     int           `json:"workers" yaml:"workers" mapstructure:"workers"`
	QueueSize   int           `json:"queue_size" yaml:"queue_size" mapstructure:"queue_size"`
	RateLimit   float64       `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"`
	RetryCount  int           `json:"retry_count" yaml:"retry_count" mapstructure:"retry_count"`
	Priority    bool          `json:"priority" yaml:"priority" mapstructure:"priority"`
	Resume      bool          `json:"resume" yaml:"resume" mapstructure:"resume"`
	StateFile   string        `json:"state_file" yaml:"state_file" mapstructure:"state_file"`
	JobTimeout  time.Duration `json:"job_timeout" yaml:"job_timeout" mapstructure:"job_timeout"`
}

// ProvidersConfig holds provider-specific configuration.
type ProvidersConfig struct {
	Default    string            `json:"default" yaml:"default" mapstructure:"default"`
	Bing       ProviderConfig    `json:"bing" yaml:"bing" mapstructure:"bing"`
	Google     ProviderConfig    `json:"google" yaml:"google" mapstructure:"google"`
	DuckDuckGo ProviderConfig    `json:"duckduckgo" yaml:"duckduckgo" mapstructure:"duckduckgo"`
	Brave      ProviderConfig    `json:"brave" yaml:"brave" mapstructure:"brave"`
	Yahoo      ProviderConfig    `json:"yahoo" yaml:"yahoo" mapstructure:"yahoo"`
	Mojeek     ProviderConfig    `json:"mojeek" yaml:"mojeek" mapstructure:"mojeek"`
	SearXNG    ProviderConfig    `json:"searxng" yaml:"searxng" mapstructure:"searxng"`
	Custom     map[string]ProviderConfig `json:"custom,omitempty" yaml:"custom,omitempty" mapstructure:"custom,omitempty"`
}

// ProviderConfig holds configuration for a single provider.
type ProviderConfig struct {
	Enabled     bool              `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	BaseURL     string            `json:"base_url" yaml:"base_url" mapstructure:"base_url"`
	APIKey      string            `json:"api_key" yaml:"api_key" mapstructure:"api_key"`
	RateLimit   float64           `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"`
	Timeout     time.Duration     `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	Proxy       string            `json:"proxy" yaml:"proxy" mapstructure:"proxy"`
	Options     map[string]string `json:"options,omitempty" yaml:"options,omitempty" mapstructure:"options,omitempty"`
	UserAgents  []string          `json:"user_agents,omitempty" yaml:"user_agents,omitempty" mapstructure:"user_agents,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty" mapstructure:"headers,omitempty"`
	CAPTCHA     bool              `json:"captcha" yaml:"captcha" mapstructure:"captcha"`
}

// APIConfig holds REST/gRPC API configuration.
type APIConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Type       string `json:"type" yaml:"type" mapstructure:"type"`
	Host       string `json:"host" yaml:"host" mapstructure:"host"`
	Port       int    `json:"port" yaml:"port" mapstructure:"port"`
	GRPCPort   int    `json:"grpc_port" yaml:"grpc_port" mapstructure:"grpc_port"`
	CORS       bool   `json:"cors" yaml:"cors" mapstructure:"cors"`
	AuthToken  string `json:"auth_token" yaml:"auth_token" mapstructure:"auth_token"`
	TLS        bool   `json:"tls" yaml:"tls" mapstructure:"tls"`
	CertFile   string `json:"cert_file" yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile    string `json:"key_file" yaml:"key_file" mapstructure:"key_file"`
}

// TUIConfig holds TUI configuration.
type TUIConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Theme    string `json:"theme" yaml:"theme" mapstructure:"theme"`
	LogLevel string `json:"log_level" yaml:"log_level" mapstructure:"log_level"`
}

// MetricsConfig holds metrics configuration.
type MetricsConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Type       string `json:"type" yaml:"type" mapstructure:"type"`
	Prometheus bool   `json:"prometheus" yaml:"prometheus" mapstructure:"prometheus"`
	Port       int    `json:"port" yaml:"port" mapstructure:"port"`
	Path       string `json:"path" yaml:"path" mapstructure:"path"`
}

// PluginsConfig holds plugin loading configuration.
type PluginsConfig struct {
	Enabled  bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Dir      string   `json:"dir" yaml:"dir" mapstructure:"dir"`
	AllowAll bool     `json:"allow_all" yaml:"allow_all" mapstructure:"allow_all"`
	Allow    []string `json:"allow,omitempty" yaml:"allow,omitempty" mapstructure:"allow,omitempty"`
	Deny     []string `json:"deny,omitempty" yaml:"deny,omitempty" mapstructure:"deny,omitempty"`
}
