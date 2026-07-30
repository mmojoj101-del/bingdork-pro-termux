// Package cli provides the command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/bingdork/bingdork/internal/cache"
	"github.com/bingdork/bingdork/internal/config"
	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/internal/metrics"
	"github.com/bingdork/bingdork/internal/plugin"
	"github.com/bingdork/bingdork/internal/scheduler"
	"github.com/bingdork/bingdork/pkg/engine"
	"github.com/bingdork/bingdork/pkg/extractor"
	"github.com/bingdork/bingdork/pkg/output"
	"github.com/bingdork/bingdork/pkg/providers/bing"
	"github.com/bingdork/bingdork/pkg/storage"
)

// App holds all application components.
type App struct {
	Config    *core.Config
	Log       *logger.Logger
	Engine    *engine.Engine
	Scheduler *scheduler.Scheduler
	Metrics   *metrics.Collector
	Cache     *cache.Manager
	Storage   *storage.Manager
	Output    *output.Manager
	Extractor *extractor.Extractor
	Plugin    *plugin.Loader
	ConfigMgr *config.Manager

	ctx    context.Context
	cancel context.CancelFunc
}

// NewApp creates a new application instance.
func NewApp() *App {
	return &App{}
}

// Initialize sets up all components.
func (a *App) Initialize(cfgPath string) error {
	// Context with cancellation
	a.ctx, a.cancel = context.WithCancel(context.Background())

	// Config
	a.ConfigMgr = config.New()
	var cfg *core.Config
	var err error
	if cfgPath != "" {
		cfg, err = a.ConfigMgr.LoadFile(cfgPath)
	} else {
		cfg, err = a.ConfigMgr.Load()
	}
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	a.Config = cfg

	// Logger
	log, err := logger.New(&cfg.Logging)
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	a.Log = log

	// Metrics
	a.Metrics = metrics.NewCollector(&cfg.Metrics, log)

	// Cache
	cacheMgr := cache.NewManager(log)
	if cfg.Cache.Enabled {
		c, err := cache.NewCacheFromConfig(&cfg.Cache, log)
		if err != nil {
			log.Warn("failed to initialize cache", logger.LogFields{"error": err})
		} else if c != nil {
			cacheMgr.Register("default", c)
		}
	}
	a.Cache = cacheMgr

	// Storage
	storageMgr := storage.NewManager(log)
	store, err := storage.NewStoreFromConfig(&cfg.Storage, log)
	if err != nil {
		log.Warn("failed to initialize storage", logger.LogFields{"error": err})
	} else {
		storageMgr.Register("default", store)
	}
	a.Storage = storageMgr

	// Providers
	var providers []core.SearchProvider
	if cfg.Providers.Bing.Enabled {
		bingProvider, err := bing.New(&cfg.Providers.Bing, &cfg.Network, log)
		if err != nil {
			log.Warn("failed to initialize Bing provider", logger.LogFields{"error": err})
		} else {
			providers = append(providers, bingProvider)
		}
	}

	// Engine
	a.Engine = engine.New(log, providers...)

	// Extractor
	a.Extractor = extractor.New(log)

	// Output
	outputMgr := output.NewManager(log)
	exporter, err := output.NewExporterFromConfig(&cfg.Output, log)
	if err != nil {
		log.Warn("failed to initialize output exporter", logger.LogFields{"error": err})
	} else {
		outputMgr.Register(exporter)
	}
	a.Output = outputMgr

	// Scheduler
	sched, err := scheduler.New(&cfg.Scheduler, log)
	if err != nil {
		return fmt.Errorf("initializing scheduler: %w", err)
	}
	a.Scheduler = sched

	// Register search handler in scheduler
	sched.RegisterHandler("search", func(ctx context.Context, task *scheduler.Task) error {
		resultSet, err := a.Engine.Search(ctx, task.Query)
		if err != nil {
			return err
		}

		// Extract/enrich
		resultSet, err = a.Extractor.Extract(ctx, resultSet)
		if err != nil {
			return err
		}

		// Export
		if err := a.Output.ExportAll(ctx, resultSet); err != nil {
			return err
		}

		// Store
		if store, ok := a.Storage.Get("default"); ok {
			if err := store.Save(ctx, resultSet); err != nil {
				log.Warn("failed to save results", logger.LogFields{"error": err})
			}
		}

		// Metrics
		a.Metrics.RecordQuery(string(task.Query.Provider), true, resultSet.Duration, len(resultSet.Results))

		return nil
	})

	// Plugin system
	a.Plugin = plugin.NewLoader(&cfg.Plugins, log)
	if cfg.Plugins.Enabled {
		plugins, err := a.Plugin.LoadAll(a.ctx)
		if err != nil {
			log.Warn("failed to load plugins", logger.LogFields{"error": err})
		}
		_ = plugins
	}

	return nil
}

// Start begins the application.
func (a *App) Start() {
	a.Log.Info("starting BingDork Pro")

	// Start scheduler
	a.Scheduler.Start(a.ctx)

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		a.Log.Info("received signal", logger.LogFields{"signal": sig.String()})
	case <-a.ctx.Done():
	}
}

// Stop gracefully shuts down.
func (a *App) Stop() {
	a.Log.Info("shutting down BingDork Pro")

	a.Scheduler.Stop()
	a.Engine.Close()
	a.Cache.CloseAll()
	a.Plugin.CloseAll()

	if a.cancel != nil {
		a.cancel()
	}

	a.Log.Info("shutdown complete")
}

// ExecuteSearch performs a single search and displays results.
func (a *App) ExecuteSearch(query *core.SearchQuery) error {
	// Check cache first
	if cache, ok := a.Cache.Get("default"); ok {
		cacheKey := fmt.Sprintf("%s:%s:%d", query.Provider, query.Query, query.Page)
		if cached, err := cache.Get(a.ctx, cacheKey); err == nil && cached != nil {
			a.Metrics.RecordCacheHit()
			return a.Output.ExportAll(a.ctx, cached)
		}
		a.Metrics.RecordCacheMiss()
	}

	// Execute search
	resultSet, err := a.Engine.Search(a.ctx, query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Extract
	resultSet, err = a.Extractor.Extract(a.ctx, resultSet)
	if err != nil {
		return err
	}

	// Export
	if err := a.Output.ExportAll(a.ctx, resultSet); err != nil {
		return err
	}

	// Cache
	if cache, ok := a.Cache.Get("default"); ok {
		cacheKey := fmt.Sprintf("%s:%s:%d", query.Provider, query.Query, query.Page)
		if err := cache.Set(a.ctx, cacheKey, resultSet); err != nil {
			a.Log.Warn("failed to cache results", logger.LogFields{"error": err})
		}
	}

	// Store
	if store, ok := a.Storage.Get("default"); ok {
		if err := store.Save(a.ctx, resultSet); err != nil {
			a.Log.Warn("failed to save results", logger.LogFields{"error": err})
		}
	}

	// Metrics
	a.Metrics.RecordQuery(string(query.Provider), true, resultSet.Duration, len(resultSet.Results))

	return nil
}

// ExecuteBatch runs multiple queries.
func (a *App) ExecuteBatch(queries []string, provider core.ProviderID, delay string) error {
	var delayDur int
	if delay != "" {
		fmt.Sscanf(delay, "%d", &delayDur)
	}

	for i, q := range queries {
		query := &core.SearchQuery{
			Query:    q,
			Provider: provider,
		}

		a.Log.Info("executing batch query", logger.LogFields{
			"index": i + 1,
			"total": len(queries),
			"query": q,
		})

		if err := a.ExecuteSearch(query); err != nil {
			a.Log.Error("batch query failed", err, logger.LogFields{"query": q})
		}
	}

	return nil
}

// ExecuteDoctor runs diagnostics.
func (a *App) ExecuteDoctor() error {
	a.Log.Info("running diagnostics")

	// Check all providers
	health := a.Engine.Health(a.ctx)
	for _, h := range health {
		status := "healthy"
		if !h.Healthy {
			status = "unhealthy"
		}
		a.Log.Info("provider health", logger.LogFields{
			"provider": h.Provider,
			"status":   status,
			"latency":  h.Latency.String(),
		})
	}

	// Print config
	cfgYAML := config.DefaultConfigYAML()
	fmt.Println("\nDefault Configuration:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(cfgYAML)

	return nil
}

// RootCommand returns the root cobra command.
func RootCommand() *cobra.Command {
	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "bingdork",
		Short: "BingDork Pro - Advanced Search Automation Framework",
		Long: `BingDork Pro is a professional search automation framework for OSINT,
asset discovery, defensive security, and authorized bug bounty research.

It provides advanced search capabilities across multiple search engines
with built-in anti-bot evasion, CAPTCHA bypass, and result processing.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize app
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Path to configuration file")

	// Subcommands
	rootCmd.AddCommand(NewSearchCmd())
	rootCmd.AddCommand(NewBatchCmd())
	rootCmd.AddCommand(NewExportCmd())
	rootCmd.AddCommand(NewStatsCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewDoctorCmd())
	rootCmd.AddCommand(NewCacheCmd())
	rootCmd.AddCommand(NewVersionCmd())

	return rootCmd
}

// NewSearchCmd creates the search command.
func NewSearchCmd() *cobra.Command {
	var (
		provider string
		page     int
		output   string
		format   string
		maxResults int
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Execute a search query",
		Long: `Execute a single search query across configured providers.
Supports Bing advanced operators: site:, intitle:, inurl:, filetype:, etc.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := NewApp()
			if err := app.Initialize(""); err != nil {
				return err
			}
			defer app.Stop()

			query := strings.Join(args, " ")
			searchQuery := &core.SearchQuery{
				Query:      query,
				Page:       page,
				MaxResults: maxResults,
				Options:    make(map[string]string),
			}

			if provider != "" {
				searchQuery.Provider = core.ProviderID(provider)
			}
			if output != "" {
				app.Config.Output.File = output
			}
			if format != "" {
				app.Config.Output.Format = format
			}

			return app.ExecuteSearch(searchQuery)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Search provider (bing, google, etc.)")
	cmd.Flags().IntVarP(&page, "page", "n", 0, "Page number")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format (json, csv, txt, md)")
	cmd.Flags().IntVarP(&maxResults, "max-results", "m", 0, "Maximum results")

	return cmd
}

// NewBatchCmd creates the batch command.
func NewBatchCmd() *cobra.Command {
	var (
		provider string
		file     string
		delay    string
		output   string
		format   string
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Execute multiple queries from a file",
		Long: `Execute multiple search queries from a file (one query per line).
Supports TXT, JSON, and CSV input formats.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := NewApp()
			if err := app.Initialize(""); err != nil {
				return err
			}
			defer app.Stop()

			if output != "" {
				app.Config.Output.File = output
			}
			if format != "" {
				app.Config.Output.Format = format
			}

			// Read queries from file or stdin
			var queries []string
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading query file: %w", err)
				}
				queries = strings.Split(strings.TrimSpace(string(data)), "\n")
			} else if len(args) > 0 {
				queries = args
			} else {
				return fmt.Errorf("no queries provided; use --file or pass queries as arguments")
			}

			prov := core.ProviderID(provider)
			if prov == "" {
				prov = core.ProviderBing
			}

			return app.ExecuteBatch(queries, prov, delay)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "bing", "Search provider")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Query file path")
	cmd.Flags().StringVarP(&delay, "delay", "d", "0", "Delay between queries (seconds)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "t", "", "Output format")

	return cmd
}

// NewExportCmd creates the export command.
func NewExportCmd() *cobra.Command {
	var (
		format string
		input  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export search results between formats",
		Long:  `Convert stored search results from one format to another.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("export command requires a storage backend with data")
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "json", "Export format")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Input file")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file")

	return cmd
}

// NewStatsCmd creates the stats command.
func NewStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show execution statistics",
		Long:  `Display search statistics, unique domains, and performance metrics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := NewApp()
			if err := app.Initialize(""); err != nil {
				return err
			}
			defer app.Stop()

			snapshot := app.Metrics.Snapshot()
			fmt.Printf("\nBingDork Pro Statistics\n")
			fmt.Printf("%s\n", strings.Repeat("=", 50))
			fmt.Printf("Queries Total:     %d\n", snapshot.QueriesTotal)
			fmt.Printf("Queries Success:   %d\n", snapshot.QueriesSuccess)
			fmt.Printf("Queries Failed:    %d\n", snapshot.QueriesFailed)
			fmt.Printf("Results Total:     %d\n", snapshot.ResultsTotal)
			fmt.Printf("Results Filtered:  %d\n", snapshot.ResultsFiltered)
			fmt.Printf("Unique Domains:    %d\n", snapshot.UniqueDomains)
			fmt.Printf("Cache Hits:        %d\n", snapshot.CacheHits)
			fmt.Printf("Cache Misses:      %d\n", snapshot.CacheMisses)
			fmt.Printf("Avg Response:      %s\n", snapshot.AvgResponseTime)
			fmt.Printf("Uptime:            %s\n", snapshot.Uptime)
			fmt.Printf("CAPTCHA Detected:  %d\n", snapshot.CAPTCHADetected)
			fmt.Printf("CAPTCHA Solved:    %d\n", snapshot.CAPTCHASolved)
			fmt.Println()

			return nil
		},
	}

	return cmd
}

// NewConfigCmd creates the config command.
func NewConfigCmd() *cobra.Command {
	var init bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View, initialize, or modify BingDork Pro configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if init {
				path := "bingdork.yaml"
				if len(args) > 0 {
					path = args[0]
				}
				if err := config.WriteDefaultConfig(path); err != nil {
					return fmt.Errorf("writing default config: %w", err)
				}
				fmt.Printf("Default configuration written to %s\n", path)
				return nil
			}

			// Print current config
			fmt.Println(config.DefaultConfigYAML())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&init, "init", "i", false, "Initialize default config file")

	return cmd
}

// NewDoctorCmd creates the doctor command.
func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run system diagnostics",
		Long:  `Check configuration, provider connectivity, and system health.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := NewApp()
			if err := app.Initialize(""); err != nil {
				return err
			}
			defer app.Stop()

			return app.ExecuteDoctor()
		},
	}

	return cmd
}

// NewCacheCmd creates the cache command.
func NewCacheCmd() *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage cache",
		Long:  `View, clear, or inspect the search cache.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := NewApp()
			if err := app.Initialize(""); err != nil {
				return err
			}
			defer app.Stop()

			if clear {
				if cache, ok := app.Cache.Get("default"); ok {
					if err := cache.Clear(app.ctx); err != nil {
						return fmt.Errorf("clearing cache: %w", err)
					}
					app.Log.Info("cache cleared")
				}
				return nil
			}

			// Show cache stats
			if cache, ok := app.Cache.Get("default"); ok {
				stats, err := cache.Stats(app.ctx)
				if err != nil {
					return err
				}
				fmt.Printf("\nCache Statistics\n")
				fmt.Printf("%s\n", strings.Repeat("=", 40))
				fmt.Printf("Items:  %d\n", stats.Items)
				fmt.Printf("Hits:   %d\n", stats.Hits)
				fmt.Printf("Misses: %d\n", stats.Misses)
				fmt.Println()
			} else {
				fmt.Println("Cache is disabled")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&clear, "clear", "c", false, "Clear the cache")

	return cmd
}

// NewVersionCmd creates the version command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("BingDork Pro v%s\n", Version)
			fmt.Printf("Commit: %s\n", Commit)
			fmt.Printf("Date:   %s\n", Date)
			fmt.Printf("Go:     %s\n", GoVersion)
		},
	}
}

// Version information set at build time.
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = "unknown"
)

// Execute runs the CLI.
func Execute() {
	rootCmd := RootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Main function template for cmd/bingdork
func Main() {
	Execute()
}

// HomeDir returns the user's home directory.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// ConfigPath returns the default config path.
func ConfigPath() string {
	return filepath.Join(homeDir(), ".bingdork", "bingdork.yaml")
}
