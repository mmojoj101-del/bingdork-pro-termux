// Package logger provides structured logging for BingDork Pro.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bingdork/bingdork/internal/core"
)

// Logger wraps zerolog with application-specific context.
type Logger struct {
	zerolog.Logger
	level  zerolog.Level
	mu     sync.RWMutex
	attrs  map[string]interface{}
}

// fieldsContextKey is used to store structured fields in context.
type fieldsContextKey struct{}

// LogFields represents structured logging fields.
type LogFields map[string]interface{}

// New creates a new Logger from configuration.
func New(cfg *core.LoggingConfig) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFieldName = "timestamp"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"
	zerolog.CallerFieldName = "caller"

	var output io.Writer = os.Stdout

	if cfg.File != "" {
		dir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating log directory: %w", err)
		}

		output = &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    100, // MB
			MaxBackups: 5,
			MaxAge:     30, // days
			Compress:   true,
		}
	}

	var zlog zerolog.Logger

	switch strings.ToLower(cfg.Format) {
	case "json":
		if cfg.File != "" {
			zlog = zerolog.New(output).Level(level).With().Timestamp().Logger()
		} else {
			zlog = zerolog.New(output).Level(level).With().Timestamp().Logger()
		}
	case "console", "text":
		output = zerolog.ConsoleWriter{
			Out:        output,
			NoColor:    cfg.NoColor,
			TimeFormat: time.RFC3339,
		}
		zlog = zerolog.New(output).Level(level).With().Timestamp().Logger()
	default:
		zlog = zerolog.New(output).Level(level).With().Timestamp().Logger()
	}

	l := &Logger{
		Logger: zlog,
		level:  level,
		attrs:  make(map[string]interface{}),
	}

	// Set as global logger
	log.Logger = zlog

	return l, nil
}

// With returns a new logger with the given fields attached.
func (l *Logger) With(fields LogFields) *Logger {
	ctx := l.Logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{
		Logger: ctx.Logger(),
		level:  l.level,
		attrs:  copyMap(l.attrs),
	}
}

// WithContext returns a new context with logger fields attached.
func WithContext(ctx context.Context, fields LogFields) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	existing, _ := ctx.Value(fieldsContextKey{}).(LogFields)
	if existing == nil {
		existing = make(LogFields)
	}
	for k, v := range fields {
		existing[k] = v
	}
	return context.WithValue(ctx, fieldsContextKey{}, existing)
}

// FromContext extracts a logger from context with attached fields.
func (l *Logger) FromContext(ctx context.Context) *Logger {
	if fields, ok := ctx.Value(fieldsContextKey{}).(LogFields); ok {
		return l.With(fields)
	}
	return l
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...LogFields) {
	evt := l.Logger.Debug()
	for _, f := range fields {
		for k, v := range f {
			evt = evt.Interface(k, v)
		}
	}
	evt.Msg(msg)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...LogFields) {
	evt := l.Logger.Info()
	for _, f := range fields {
		for k, v := range f {
			evt = evt.Interface(k, v)
		}
	}
	evt.Msg(msg)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...LogFields) {
	evt := l.Logger.Warn()
	for _, f := range fields {
		for k, v := range f {
			evt = evt.Interface(k, v)
		}
	}
	evt.Msg(msg)
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error, fields ...LogFields) {
	evt := l.Logger.Error().Err(err)
	for _, f := range fields {
		for k, v := range f {
			evt = evt.Interface(k, v)
		}
	}
	evt.Msg(msg)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string, err error, fields ...LogFields) {
	evt := l.Logger.Fatal().Err(err)
	for _, f := range fields {
		for k, v := range f {
			evt = evt.Interface(k, v)
		}
	}
	evt.Msg(msg)
}

// Trace logs a trace message (zerolog's debug level with trace hint).
func (l *Logger) Trace(msg string, fields ...LogFields) {
	if l.level <= zerolog.DebugLevel {
		evt := l.Logger.Debug().Str("trace", "true")
		for _, f := range fields {
			for k, v := range f {
				evt = evt.Interface(k, v)
			}
		}
		evt.Msg(msg)
	}
}

// ExecutionSummary logs an execution summary.
func (l *Logger) ExecutionSummary(summary map[string]interface{}) {
	l.Info("execution summary", LogFields{
		"summary": summary,
		"type":    "execution_summary",
	})
}

// Level returns the current log level.
func (l *Logger) Level() zerolog.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetLevel dynamically changes the log level.
func (l *Logger) SetLevel(level zerolog.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	l.Logger = l.Logger.Level(level)
}

// SetLevelFromString sets log level from a string.
func (l *Logger) SetLevelFromString(level string) error {
	parsed, err := parseLevel(level)
	if err != nil {
		return err
	}
	l.SetLevel(parsed)
	return nil
}

// NewNopLogger creates a no-op logger for testing.
func NewNopLogger() *Logger {
	return &Logger{
		Logger: zerolog.Nop(),
		level:  zerolog.Disabled,
	}
}

// parseLevel converts a string to zerolog.Level.
func parseLevel(level string) (zerolog.Level, error) {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.DebugLevel, nil // use debug for trace
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "fatal":
		return zerolog.FatalLevel, nil
	case "panic":
		return zerolog.PanicLevel, nil
	case "disabled", "off":
		return zerolog.Disabled, nil
	default:
		return zerolog.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// copyMap deep copies a map.
func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
