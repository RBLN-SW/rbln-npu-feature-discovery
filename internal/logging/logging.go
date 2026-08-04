// Package logging configures the process-wide slog logger according to the
// RBLN logging contract (rbln-npu-operator/docs/logging.md).
//
// Canonical copy — 수정 시 모든 repo의 복사본을 함께 갱신할 것:
// rbln-metrics-exporter, rbln-npu-feature-discovery, rbln-k8s-driver-manager,
// sandbox-device-plugin, rbln-npu-operator.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// LevelTrace extends slog's levels downward for the contract's "trace" level.
const LevelTrace = slog.Level(-8)

// New builds a contract-conformant slog logger writing to w.
// level: "error"|"warning"|"info"|"debug"|"trace" ("" = info).
// format: "json"|"text" ("" = json).
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{
		Level: lvl,
		// caller 비용/노이즈는 debug 이상에서만 감수한다.
		AddSource:   lvl <= slog.LevelDebug,
		ReplaceAttr: replaceAttr,
	}
	var h slog.Handler
	switch strings.ToLower(format) {
	case "", "json":
		h = slog.NewJSONHandler(w, opts)
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		return nil, fmt.Errorf("unknown log format %q (json|text)", format)
	}
	return slog.New(h), nil
}

// Setup installs the process-wide default logger (stdout).
func Setup(level, format string) error {
	logger, err := New(os.Stdout, level, format)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	return nil
}

// SetupFromEnv reads LOG_LEVEL / LOG_FORMAT and installs the logger.
// 빈 값이면 info/json (프로덕션 기본).
func SetupFromEnv() error {
	return Setup(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"))
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "", "info":
		return slog.LevelInfo, nil
	case "error":
		return slog.LevelError, nil
	case "warning", "warn":
		return slog.LevelWarn, nil
	case "debug":
		return slog.LevelDebug, nil
	case "trace":
		return LevelTrace, nil
	}
	return 0, fmt.Errorf("unknown log level %q (error|warning|info|debug|trace)", s)
}

// replaceAttr normalizes slog output to the contract: key "ts" with
// RFC3339Nano, lowercase "level" ("trace" for LevelTrace), and a zap-style
// "caller" ("file:line") instead of the verbose source group.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		a.Key = "ts"
		a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
	case slog.LevelKey:
		lvl, ok := a.Value.Any().(slog.Level)
		if !ok {
			return a
		}
		if lvl == LevelTrace {
			a.Value = slog.StringValue("trace")
		} else {
			a.Value = slog.StringValue(strings.ToLower(lvl.String()))
		}
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

// trimPath keeps the last two path segments, matching zap's short caller.
func trimPath(file string) string {
	idx := strings.LastIndexByte(file, '/')
	if idx == -1 {
		return file
	}
	if idx2 := strings.LastIndexByte(file[:idx], '/'); idx2 != -1 {
		return file[idx2+1:]
	}
	return file[idx+1:]
}
