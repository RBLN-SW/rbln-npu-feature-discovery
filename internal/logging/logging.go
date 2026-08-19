// Package logging configures the process-wide slog logger: level-gated JSON
// (default) or text on stdout, with normalized output keys — "ts"
// (RFC3339Nano), lowercase "level", and a short "caller" at debug.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// New builds a slog logger writing to w.
// level: "error"|"warning"|"info"|"debug" ("" = info).
// format: "json"|"text" ("" = json).
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{
		Level: lvl,
		// The caller attr's cost and noise are only worth it at debug.
		AddSource:   lvl <= slog.LevelDebug,
		ReplaceAttr: replaceAttr,
	}
	f, err := parseFormat(format)
	if err != nil {
		return nil, err
	}
	var h slog.Handler
	switch f {
	case "json":
		h = slog.NewJSONHandler(w, opts)
	case "text":
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h), nil
}

// SetupFromEnv reads LOG_LEVEL / LOG_FORMAT and installs the process-wide
// default logger (stdout).
// Empty values default to info/json (the production defaults). Invalid
// values do not kill the process: only the offending variable falls back
// to its default, and a Warn carrying a "fallback" key is emitted through
// the installed logger.
func SetupFromEnv() {
	level, format := os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT")
	var levelErr, formatErr error
	if _, err := parseLevel(level); err != nil {
		levelErr, level = err, "info"
	}
	if _, err := parseFormat(format); err != nil {
		formatErr, format = err, "json"
	}
	logger, err := New(os.Stdout, level, format)
	if err != nil {
		// Unreachable: both values were validated or replaced above.
		slog.Error("Failed to install logger", "err", err)
		return
	}
	slog.SetDefault(logger)
	// With LOG_LEVEL=error an invalid LOG_FORMAT's warn is suppressed by the
	// gate — accepted, since the error gate was chosen explicitly.
	if levelErr != nil {
		slog.Warn("Invalid LOG_LEVEL, using default", "err", levelErr, "fallback", "info")
	}
	if formatErr != nil {
		slog.Warn("Invalid LOG_FORMAT, using default", "err", formatErr, "fallback", "json")
	}
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "error":
		return slog.LevelError, nil
	case "warning":
		return slog.LevelWarn, nil
	case "debug":
		return slog.LevelDebug, nil
	}
	return 0, fmt.Errorf("unknown log level %q (error|warning|info|debug)", s)
}

func parseFormat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "json":
		return "json", nil
	case "text":
		return "text", nil
	}
	return "", fmt.Errorf("unknown log format %q (json|text)", s)
}

// replaceAttr normalizes slog output: key "ts" with RFC3339Nano, lowercase
// "level", and a zap-style "caller" ("file:line") instead of the verbose
// source group.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		// User attrs may use the "time" key — convert only the record timestamp.
		if a.Value.Kind() != slog.KindTime {
			return a
		}
		a.Key = "ts"
		a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
	case slog.LevelKey:
		lvl, ok := a.Value.Any().(slog.Level)
		if !ok {
			return a
		}
		a.Value = slog.StringValue(strings.ToLower(lvl.String()))
	case slog.SourceKey:
		src, ok := a.Value.Any().(*slog.Source)
		if !ok {
			return a
		}
		a.Key = "caller"
		a.Value = slog.StringValue(fmt.Sprintf("%s:%d", trimPath(src.File), src.Line))
	}
	return a
}

// trimPath keeps at most the last two path segments for a zap-style short caller.
func trimPath(file string) string {
	idx := strings.LastIndexByte(file, '/')
	if idx == -1 {
		return file
	}
	if idx2 := strings.LastIndexByte(file[:idx], '/'); idx2 != -1 {
		return file[idx2+1:]
	}
	return file
}
