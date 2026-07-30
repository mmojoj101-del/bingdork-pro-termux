package output

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResultSet() *core.ResultSet {
	return &core.ResultSet{
		Query:    "test query",
		Provider: "bing",
		Results: []*core.Result{
			{
				Title:       "Test Result",
				URL:         "https://example.com",
				Host:        "example.com",
				RootDomain:  "example.com",
				Description: "A test result description",
				SearchPos:   1,
				Page:        0,
				Engine:      "bing",
			},
			{
				Title:       "Another Result",
				URL:         "https://test.org/page",
				Host:        "test.org",
				RootDomain:  "test.org",
				Description: "Another description",
				SearchPos:   2,
				Page:        0,
				Engine:      "bing",
			},
		},
	}
}

func TestJSONExporter(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: "", PrettyPrint: true}
	exporter, err := NewJSONExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
}

func TestJSONExporter_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.json")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path, PrettyPrint: false}
	exporter, err := NewJSONExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test query")
	assert.Contains(t, string(data), "example.com")
}

func TestCSVExporter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.csv")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path}
	exporter, err := NewCSVExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Position,Title,URL,Host")
	assert.Contains(t, content, "Test Result")
	assert.Contains(t, content, "https://example.com")
}

func TestTXTExporter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path}
	exporter, err := NewTXTExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Test Result")
	assert.Contains(t, content, "example.com")
	assert.Contains(t, content, "test query")
}

func TestMarkdownExporter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path}
	exporter, err := NewMarkdownExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# Search Results")
	assert.Contains(t, content, "Test Result")
}

func TestJSONLExporter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.jsonl")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path}
	exporter, err := NewJSONLExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "example.com")
	assert.Contains(t, lines[1], "test.org")
}

func TestYAMLExporter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.yaml")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path}
	exporter, err := NewYAMLExporter(cfg, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "query: test query")
	assert.Contains(t, content, "engine: bing")
}

func TestManager(t *testing.T) {
	log := logger.NewNopLogger()
	mgr := NewManager(log)

	// Register exporters
	var buf bytes.Buffer
	_ = buf

	jsonCfg := &core.OutputConfig{PrettyPrint: false}
	jsonExporter, err := NewJSONExporter(jsonCfg, log)
	require.NoError(t, err)
	mgr.Register(jsonExporter)

	txtCfg := &core.OutputConfig{}
	txtExporter, err := NewTXTExporter(txtCfg, log)
	require.NoError(t, err)
	mgr.Register(txtExporter)

	ctx := context.Background()
	err = mgr.ExportAll(ctx, testResultSet())
	require.NoError(t, err)

	err = mgr.CloseAll()
	require.NoError(t, err)
}

func TestNewExporterFromConfig(t *testing.T) {
	log := logger.NewNopLogger()

	tests := []struct {
		format string
		name   string
	}{
		{"json", "json"},
		{"csv", "csv"},
		{"txt", "txt"},
		{"text", "txt"},
		{"md", "markdown"},
		{"markdown", "markdown"},
		{"yaml", "yaml"},
		{"jsonl", "jsonl"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cfg := &core.OutputConfig{Format: tt.format, File: filepath.Join(t.TempDir(), "output."+tt.format)}
			exporter, err := NewExporterFromConfig(cfg, log)
			require.NoError(t, err)
			assert.Equal(t, tt.name, exporter.Name())
			exporter.Close()
		})
	}
}

func TestExporter_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "append.csv")
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{File: path, Append: true}

	// First export
	exporter, err := NewCSVExporter(cfg, log)
	require.NoError(t, err)
	ctx := context.Background()
	err = exporter.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter.Close()

	// Append
	exporter2, err := NewCSVExporter(cfg, log)
	require.NoError(t, err)
	err = exporter2.Export(ctx, testResultSet())
	require.NoError(t, err)
	exporter2.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Greater(t, len(lines), 2) // header + 2 + 2 = 5
}
