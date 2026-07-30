// Package plugin provides a plugin loading system for extensibility.
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	// Name returns the plugin name.
	Name() string

	// Version returns the plugin version.
	Version() string

	// Init initializes the plugin with configuration.
	Init(ctx context.Context, config map[string]interface{}) error

	// Close cleans up plugin resources.
	Close() error
}

// SearchPlugin extends the search capabilities.
type SearchPlugin interface {
	Plugin
	// Execute performs a plugin-specific search operation.
	Execute(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error)
}

// ProviderPlugin is a plugin that implements a search provider.
type ProviderPlugin interface {
	Plugin
	// Provider returns the search provider implementation.
	Provider(ctx context.Context) (core.SearchProvider, error)
}

// FilterPlugin is a plugin that provides filtering.
type FilterPlugin interface {
	Plugin
	// Filter applies filtering logic.
	Filter(ctx context.Context, results []*core.Result) ([]*core.Result, error)
}

// ExporterPlugin is a plugin that provides export functionality.
type ExporterPlugin interface {
	Plugin
	// Export exports results in a custom format.
	Export(ctx context.Context, resultSet *core.ResultSet) error
}

// Manifest describes a plugin's metadata.
type Manifest struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description" yaml:"description"`
	Author      string `json:"author" yaml:"author"`
	Type        string `json:"type" yaml:"type"`
	APIVersion  string `json:"api_version" yaml:"api_version"`
	MinVersion  string `json:"min_version" yaml:"min_version"`
}

// Loader manages plugin discovery and loading.
type Loader struct {
	dir      string
	log      *logger.Logger
	plugins  map[string]Plugin
	mu       sync.RWMutex
	cfg      *core.PluginsConfig
}

// NewLoader creates a new plugin loader.
func NewLoader(cfg *core.PluginsConfig, log *logger.Logger) *Loader {
	return &Loader{
		dir:     cfg.Dir,
		log:     log,
		plugins: make(map[string]Plugin),
		cfg:     cfg,
	}
}

// LoadAll discovers and loads all plugins from the configured directory.
func (l *Loader) LoadAll(ctx context.Context) ([]Plugin, error) {
	if !l.cfg.Enabled {
		l.log.Debug("plugin system disabled")
		return nil, nil
	}

	dir := l.dir
	if dir == "" {
		dir = "~/.bingdork/plugins"
	}

	// Expand home directory
	if len(dir) > 0 && dir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		dir = filepath.Join(home, dir[1:])
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		l.log.Debug("plugin directory not found", logger.LogFields{"dir": dir})
		return nil, nil
	}

	var loaded []Plugin

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only consider .so files (Go plugins)
		if filepath.Ext(path) != ".so" {
			return nil
		}

		plugin, err := l.loadPlugin(ctx, path)
		if err != nil {
			l.log.Warn("failed to load plugin", logger.LogFields{
				"path":  path,
				"error": err,
			})
			return nil // skip bad plugins
		}

		loaded = append(loaded, plugin)
		return nil
	})

	return loaded, err
}

// Load loads a single plugin from a file path.
func (l *Loader) loadPlugin(ctx context.Context, path string) (Plugin, error) {
	l.log.Info("loading plugin", logger.LogFields{"path": path})

	// Open the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening plugin %s: %w", path, err)
	}

	// Look up Plugin symbol
	sym, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin %s missing Plugin symbol: %w", path, err)
	}

	pluginImpl, ok := sym.(Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement Plugin interface", path)
	}

	// Initialize
	pluginConfig := make(map[string]interface{})
	if err := pluginImpl.Init(ctx, pluginConfig); err != nil {
		return nil, fmt.Errorf("initializing plugin %s: %w", path, err)
	}

	l.mu.Lock()
	l.plugins[pluginImpl.Name()] = pluginImpl
	l.mu.Unlock()

	l.log.Info("plugin loaded", logger.LogFields{
		"name":    pluginImpl.Name(),
		"version": pluginImpl.Version(),
	})

	return pluginImpl, nil
}

// Get retrieves a loaded plugin by name.
func (l *Loader) Get(name string) (Plugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.plugins[name]
	return p, ok
}

// List returns all loaded plugin names.
func (l *Loader) List() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	names := make([]string, 0, len(l.plugins))
	for name := range l.plugins {
		names = append(names, name)
	}
	return names
}

// SearchPlugins returns all plugins that implement SearchPlugin.
func (l *Loader) SearchPlugins() []SearchPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []SearchPlugin
	for _, p := range l.plugins {
		if sp, ok := p.(SearchPlugin); ok {
			result = append(result, sp)
		}
	}
	return result
}

// ProviderPlugins returns all plugins that implement ProviderPlugin.
func (l *Loader) ProviderPlugins() []ProviderPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []ProviderPlugin
	for _, p := range l.plugins {
		if pp, ok := p.(ProviderPlugin); ok {
			result = append(result, pp)
		}
	}
	return result
}

// CloseAll closes all loaded plugins.
func (l *Loader) CloseAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for name, p := range l.plugins {
		if err := p.Close(); err != nil {
			l.log.Error("failed to close plugin", err, logger.LogFields{"name": name})
		}
	}
}

// AllowPlugin checks if a plugin is allowed by the configuration.
func (l *Loader) AllowPlugin(name string) bool {
	if l.cfg.AllowAll {
		return true
	}
	for _, allowed := range l.cfg.Allow {
		if allowed == name {
			return true
		}
	}
	for _, denied := range l.cfg.Deny {
		if denied == name {
			return false
		}
	}
	return len(l.cfg.Allow) == 0 // deny if allow list exists and not empty
}

// BuiltinPlugin provides a base for built-in plugins.
type BuiltinPlugin struct {
	Name    string
	Version string
	Config  map[string]interface{}
	log     *logger.Logger
}

// Init initializes the builtin plugin.
func (bp *BuiltinPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	bp.Config = config
	return nil
}

// Close cleans up.
func (bp *BuiltinPlugin) Close() error {
	return nil
}

// SimpleProviderPlugin provides a template for custom provider plugins.
type SimpleProviderPlugin struct {
	BuiltinPlugin
	provider core.SearchProvider
}

// NewSimpleProviderPlugin creates a new provider plugin.
func NewSimpleProviderPlugin(name, version string, provider core.SearchProvider, log *logger.Logger) *SimpleProviderPlugin {
	return &SimpleProviderPlugin{
		BuiltinPlugin: BuiltinPlugin{
			Name:    name,
			Version: version,
			log:     log,
		},
		provider: provider,
	}
}

// Provider returns the wrapped provider.
func (sp *SimpleProviderPlugin) Provider(ctx context.Context) (core.SearchProvider, error) {
	return sp.provider, nil
}
