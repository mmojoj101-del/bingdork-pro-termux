// Package output provides result exporters in multiple formats.
package output

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"gopkg.in/yaml.v3"
)

// Exporter defines the interface for result exporters.
type Exporter interface {
	// Export writes the result set to the configured output.
	Export(ctx context.Context, resultSet *core.ResultSet) error

	// Name returns the exporter name.
	Name() string

	// Close cleans up exporter resources.
	Close() error
}

// Manager coordinates multiple exporters.
type Manager struct {
	exporters []Exporter
	log       *logger.Logger
	mu        sync.Mutex
}

// NewManager creates a new output manager.
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		exporters: make([]Exporter, 0),
		log:       log,
	}
}

// Register adds an exporter.
func (m *Manager) Register(exporter Exporter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exporters = append(m.exporters, exporter)
	m.log.Info("exporter registered", logger.LogFields{"name": exporter.Name()})
}

// ExportAll writes the result set to all registered exporters.
func (m *Manager) ExportAll(ctx context.Context, resultSet *core.ResultSet) error {
	m.mu.Lock()
	exporters := make([]Exporter, len(m.exporters))
	copy(exporters, m.exporters)
	m.mu.Unlock()

	for _, ex := range exporters {
		if err := ex.Export(ctx, resultSet); err != nil {
			m.log.Error("export failed", err, logger.LogFields{"exporter": ex.Name()})
			return err
		}
	}
	return nil
}

// CloseAll closes all exporters.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for _, ex := range m.exporters {
		if err := ex.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// TXTExporter exports results as plain text.
type TXTExporter struct {
	path  string
	file  *os.File
	mu    sync.Mutex
	log   *logger.Logger
	cfg   *core.OutputConfig
}

// NewTXTExporter creates a new text exporter.
func NewTXTExporter(cfg *core.OutputConfig, log *logger.Logger) (*TXTExporter, error) {
	if cfg.File == "" {
		return &TXTExporter{log: log, cfg: cfg}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening output file: %w", err)
	}

	return &TXTExporter{path: cfg.File, file: f, log: log, cfg: cfg}, nil
}

// Export writes results as plain text.
func (e *TXTExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Query: %s\n", resultSet.Query))
	sb.WriteString(fmt.Sprintf("Provider: %s\n", resultSet.Provider))
	sb.WriteString(fmt.Sprintf("Results: %d\n", len(resultSet.Results)))
	sb.WriteString(strings.Repeat("=", 80) + "\n\n")

	for _, result := range resultSet.Results {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", result.SearchPos, result.Title))
		sb.WriteString(fmt.Sprintf("     URL: %s\n", result.URL))
		sb.WriteString(fmt.Sprintf("     Host: %s\n", result.Host))
		sb.WriteString(fmt.Sprintf("     Domain: %s\n", result.RootDomain))
		if result.Description != "" {
			sb.WriteString(fmt.Sprintf("     Desc: %s\n", result.Description))
		}
		sb.WriteString("\n")
	}

	output := sb.String()
	if e.file != nil {
		_, err := e.file.WriteString(output)
		return err
	}
	fmt.Print(output)
	return nil
}

// Name returns "txt".
func (e *TXTExporter) Name() string { return "txt" }

// Close cleans up.
func (e *TXTExporter) Close() error {
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// CSVExporter exports results as CSV.
type CSVExporter struct {
	path  string
	file  *os.File
	mu    sync.Mutex
	log   *logger.Logger
	cfg   *core.OutputConfig
	writer *csv.Writer
	headerWritten bool
}

// NewCSVExporter creates a new CSV exporter.
func NewCSVExporter(cfg *core.OutputConfig, log *logger.Logger) (*CSVExporter, error) {
	if cfg.File == "" {
		return &CSVExporter{log: log, cfg: cfg}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening csv file: %w", err)
	}

	e := &CSVExporter{
		path:   cfg.File,
		file:   f,
		log:    log,
		cfg:    cfg,
		writer: csv.NewWriter(f),
	}

	// Write header if new file
	if !cfg.Append || !fileExists(cfg.File) {
		e.writer.Write([]string{
			"Position", "Title", "URL", "Host", "RootDomain",
			"Description", "Page", "Engine", "Timestamp",
		})
		e.headerWritten = true
	}

	return e, nil
}

// Export appends results as CSV rows.
func (e *CSVExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, result := range resultSet.Results {
		record := []string{
			fmt.Sprintf("%d", result.SearchPos),
			result.Title,
			result.URL,
			result.Host,
			result.RootDomain,
			result.Description,
			fmt.Sprintf("%d", result.Page),
			result.Engine,
			result.Timestamp.Format(time.RFC3339),
		}
		if err := e.writer.Write(record); err != nil {
			return err
		}
	}
	e.writer.Flush()
	return e.writer.Error()
}

// Name returns "csv".
func (e *CSVExporter) Name() string { return "csv" }

// Close flushes and closes the file.
func (e *CSVExporter) Close() error {
	if e.writer != nil {
		e.writer.Flush()
	}
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// JSONExporter exports results as JSON.
type JSONExporter struct {
	path string
	file *os.File
	mu   sync.Mutex
	log  *logger.Logger
	cfg  *core.OutputConfig
}

// NewJSONExporter creates a new JSON exporter.
func NewJSONExporter(cfg *core.OutputConfig, log *logger.Logger) (*JSONExporter, error) {
	if cfg.File == "" {
		return &JSONExporter{log: log, cfg: cfg}, nil
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening json file: %w", err)
	}

	return &JSONExporter{path: cfg.File, file: f, log: log, cfg: cfg}, nil
}

// Export writes results as JSON.
func (e *JSONExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var data []byte
	var err error

	if e.cfg.PrettyPrint {
		data, err = json.MarshalIndent(resultSet, "", "  ")
	} else {
		data, err = json.Marshal(resultSet)
	}

	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	if e.file != nil {
		_, err = e.file.Write(data)
		if err == nil {
			_, err = e.file.Write([]byte("\n"))
		}
		return err
	}

	fmt.Println(string(data))
	return nil
}

// Name returns "json".
func (e *JSONExporter) Name() string { return "json" }

// Close cleans up.
func (e *JSONExporter) Close() error {
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// JSONLExporter exports results as JSON Lines (one JSON object per line).
type JSONLExporter struct {
	path string
	file *os.File
	mu   sync.Mutex
	log  *logger.Logger
	cfg  *core.OutputConfig
}

// NewJSONLExporter creates a new JSONL exporter.
func NewJSONLExporter(cfg *core.OutputConfig, log *logger.Logger) (*JSONLExporter, error) {
	if cfg.File == "" {
		return &JSONLExporter{log: log, cfg: cfg}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening jsonl file: %w", err)
	}

	return &JSONLExporter{path: cfg.File, file: f, log: log, cfg: cfg}, nil
}

// Export appends results as JSON Lines.
func (e *JSONLExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	encoder := json.NewEncoder(e.file)
	for _, result := range resultSet.Results {
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return nil
}

// Name returns "jsonl".
func (e *JSONLExporter) Name() string { return "jsonl" }

// Close cleans up.
func (e *JSONLExporter) Close() error {
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// MarkdownExporter exports results as Markdown.
type MarkdownExporter struct {
	path string
	file *os.File
	mu   sync.Mutex
	log  *logger.Logger
	cfg  *core.OutputConfig
}

// NewMarkdownExporter creates a new Markdown exporter.
func NewMarkdownExporter(cfg *core.OutputConfig, log *logger.Logger) (*MarkdownExporter, error) {
	if cfg.File == "" {
		return &MarkdownExporter{log: log, cfg: cfg}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening markdown file: %w", err)
	}

	return &MarkdownExporter{path: cfg.File, file: f, log: log, cfg: cfg}, nil
}

// Export writes results as Markdown.
func (e *MarkdownExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Search Results\n\n"))
	sb.WriteString(fmt.Sprintf("**Query:** %s  \n", resultSet.Query))
	sb.WriteString(fmt.Sprintf("**Provider:** %s  \n", resultSet.Provider))
	sb.WriteString(fmt.Sprintf("**Results:** %d  \n", len(resultSet.Results)))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s  \n\n", resultSet.Timestamp.Format(time.RFC3339)))

	sb.WriteString("| # | Title | URL | Host | Description |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, result := range resultSet.Results {
		title := strings.ReplaceAll(result.Title, "|", "\\|")
		desc := strings.ReplaceAll(result.Description, "|", "\\|")
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
			result.SearchPos, title, result.URL, result.Host, desc))
	}

	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("*Generated by BingDork Pro on %s*\n", time.Now().Format(time.RFC3339)))

	output := sb.String()
	if e.file != nil {
		_, err := e.file.WriteString(output)
		return err
	}
	fmt.Print(output)
	return nil
}

// Name returns "markdown".
func (e *MarkdownExporter) Name() string { return "markdown" }

// Close cleans up.
func (e *MarkdownExporter) Close() error {
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// YAMLExporter exports results as YAML.
type YAMLExporter struct {
	path string
	file *os.File
	mu   sync.Mutex
	log  *logger.Logger
	cfg  *core.OutputConfig
}

// NewYAMLExporter creates a new YAML exporter.
func NewYAMLExporter(cfg *core.OutputConfig, log *logger.Logger) (*YAMLExporter, error) {
	if cfg.File == "" {
		return &YAMLExporter{log: log, cfg: cfg}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.OpenFile(cfg.File, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening yaml file: %w", err)
	}

	return &YAMLExporter{path: cfg.File, file: f, log: log, cfg: cfg}, nil
}

// Export writes results as YAML.
func (e *YAMLExporter) Export(ctx context.Context, resultSet *core.ResultSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := yaml.Marshal(resultSet)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	if e.file != nil {
		_, err = e.file.Write(data)
		return err
	}
	fmt.Println(string(data))
	return nil
}

// Name returns "yaml".
func (e *YAMLExporter) Name() string { return "yaml" }

// Close cleans up.
func (e *YAMLExporter) Close() error {
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// NewExporterFromConfig creates the appropriate exporter based on configuration.
func NewExporterFromConfig(cfg *core.OutputConfig, log *logger.Logger) (Exporter, error) {
	switch cfg.Format {
	case "txt", "text":
		return NewTXTExporter(cfg, log)
	case "csv":
		return NewCSVExporter(cfg, log)
	case "json":
		return NewJSONExporter(cfg, log)
	case "jsonl", "json-lines", "ndjson":
		return NewJSONLExporter(cfg, log)
	case "md", "markdown":
		return NewMarkdownExporter(cfg, log)
	case "yaml", "yml":
		return NewYAMLExporter(cfg, log)
	default:
		return NewJSONExporter(cfg, log)
	}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
