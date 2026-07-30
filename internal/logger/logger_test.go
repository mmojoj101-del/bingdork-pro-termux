package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := &core.LoggingConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}

	log, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)
	assert.Equal(t, "debug", strings.ToLower(cfg.Level))
}

func TestNew_JSONFormat(t *testing.T) {
	cfg := &core.LoggingConfig{
		Level:  "info",
		Format: "json",
	}

	log, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)
}

func TestNew_FileOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	cfg := &core.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "file",
		File:   path,
	}

	log, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)

	// Write some logs
	log.Info("test message")
	log.Debug("debug message")
	log.Warn("warning message")

	// Verify file was written
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "test message")
	assert.Contains(t, content, "debug message")
	assert.Contains(t, content, "warning message")
}

func TestLevels(t *testing.T) {
	cfg := &core.LoggingConfig{Level: "debug"}
	log, err := New(cfg)
	require.NoError(t, err)

	// Test level setting
	err = log.SetLevelFromString("warn")
	require.NoError(t, err)

	err = log.SetLevelFromString("invalid")
	assert.Error(t, err)
}

func TestWithFields(t *testing.T) {
	cfg := &core.LoggingConfig{Level: "debug"}
	log, err := New(cfg)
	require.NoError(t, err)

	fields := LogFields{"component": "test", "id": 123}
	logged := log.With(fields)
	require.NotNil(t, logged)
}

func TestExecutionSummary(t *testing.T) {
	cfg := &core.LoggingConfig{Level: "info"}
	log, err := New(cfg)
	require.NoError(t, err)

	summary := map[string]interface{}{
		"queries":  100,
		"results":  500,
		"duration": "1m30s",
	}
	log.ExecutionSummary(summary)
}

func TestNewNopLogger(t *testing.T) {
	log := NewNopLogger()
	require.NotNil(t, log)
	log.Info("this should not panic")
	log.Error("this should not panic", assert.AnError)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"warning", true},
		{"error", true},
		{"fatal", true},
		{"panic", true},
		{"disabled", true},
		{"invalid", false},
		{"", false},        // unknown level
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseLevel(tt.input)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestContextFields(t *testing.T) {
	cfg := &core.LoggingConfig{Level: "debug"}
	log, err := New(cfg)
	require.NoError(t, err)

	ctx := WithContext(nil, LogFields{"request_id": "abc123"})
	logged := log.FromContext(ctx)
	require.NotNil(t, logged)
}

func TestConcurrentLogging(t *testing.T) {
	cfg := &core.LoggingConfig{Level: "info"}
	log, err := New(cfg)
	require.NoError(t, err)

	// Concurrent logging should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			log.Info("concurrent log")
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkLogging(b *testing.B) {
	cfg := &core.LoggingConfig{Level: "disabled"}
	log, err := New(cfg)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark log message", LogFields{"iteration": i})
	}
}
