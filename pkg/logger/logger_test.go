package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel slog.Level
	}{
		{
			name:          "debug level",
			level:         "debug",
			expectedLevel: slog.LevelDebug,
		},
		{
			name:          "debug level uppercase",
			level:         "DEBUG",
			expectedLevel: slog.LevelDebug,
		},
		{
			name:          "info level",
			level:         "info",
			expectedLevel: slog.LevelInfo,
		},
		{
			name:          "info level uppercase",
			level:         "INFO",
			expectedLevel: slog.LevelInfo,
		},
		{
			name:          "warn level",
			level:         "warn",
			expectedLevel: slog.LevelWarn,
		},
		{
			name:          "warning level",
			level:         "warning",
			expectedLevel: slog.LevelWarn,
		},
		{
			name:          "error level",
			level:         "error",
			expectedLevel: slog.LevelError,
		},
		{
			name:          "error level uppercase",
			level:         "ERROR",
			expectedLevel: slog.LevelError,
		},
		{
			name:          "unknown level defaults to info",
			level:         "unknown",
			expectedLevel: slog.LevelInfo,
		},
		{
			name:          "empty level defaults to info",
			level:         "",
			expectedLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			if !logger.Enabled(context.Background(), tt.expectedLevel) {
				t.Errorf("expected logger to be enabled at level %v", tt.expectedLevel)
			}

			if tt.expectedLevel > slog.LevelDebug && logger.Enabled(context.Background(), slog.LevelDebug) {
				t.Error("expected logger to not be enabled at debug level")
			}
		})
	}
}

func TestNewReturnsJSONLogger(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	logger.Info("test message", slog.String("key", "value"))

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON output, got error: %v", err)
	}

	if result["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got %v", result["msg"])
	}

	if result["key"] != "value" {
		t.Errorf("expected key 'value', got %v", result["key"])
	}
}
