// Package applog routes diagnostic logging to a file (or /dev/null).
// File output keeps the TUI clean; stderr would corrupt the rendered frame.
package applog

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Init opens path for writing (truncating) and configures slog at level.
// If path is empty, logs are discarded. Returns a closer the caller should
// invoke at shutdown.
func Init(path, level string) (io.Closer, error) {
	if path == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return noopCloser{}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return noopCloser{}, err
	}
	h := slog.NewTextHandler(f, &slog.HandlerOptions{Level: parseLevel(level)})
	slog.SetDefault(slog.New(h))
	return f, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
