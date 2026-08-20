package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
)

func logLine(t *testing.T, level, format, emit string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger, err := New(&buf, level, format)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	switch emit {
	case "info":
		logger.Info("Started component", "port", 8080)
	case "debug":
		logger.Debug("Polled daemon", "count", 3)
	}
	if buf.Len() == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, buf.String())
	}
	return m
}

func TestNewDefaultsToInfoJSONWithNormalizedKeys(t *testing.T) {
	m := logLine(t, "", "", "info")
	if m == nil {
		t.Fatal("info line suppressed at default level")
	}
	if m["level"] != "info" {
		t.Fatalf("level = %v, want info (lowercase)", m["level"])
	}
	if m["msg"] != "Started component" {
		t.Fatalf("msg = %v", m["msg"])
	}
	if _, ok := m["ts"].(string); !ok {
		t.Fatalf("ts missing or not string: %v", m["ts"])
	}
	if !strings.Contains(m["ts"].(string), "T") {
		t.Fatalf("ts not RFC3339: %v", m["ts"])
	}
	if _, ok := m["caller"]; ok {
		t.Fatal("caller must be absent at info level")
	}
}

func TestNewGatesDebugAtInfo(t *testing.T) {
	if m := logLine(t, "info", "json", "debug"); m != nil {
		t.Fatalf("debug line leaked at info level: %v", m)
	}
}

func TestNewRejectsTraceLevel(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "trace", "json"); err == nil {
		t.Fatal("want error: this component logs nothing below debug")
	}
}

func TestNewRejectsUnknownLevelAndFormat(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "loud", "json"); err == nil {
		t.Fatal("want error for unknown level")
	}
	if _, err := New(&bytes.Buffer{}, "info", "yaml"); err == nil {
		t.Fatal("want error for unknown format")
	}
}

func TestNewPassesThroughUserTimeAttr(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("Measured duration", "time", "1.5s")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, buf.String())
	}
	if m["time"] != "1.5s" {
		t.Fatalf(`user "time" attr = %v, want "1.5s"`, m["time"])
	}
}

func TestNewTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "", "text")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("Started component", "port", 8080)
	out := buf.String()
	for _, want := range []string{"level=info", `msg="Started component"`, "ts=", "port=8080"} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q: %s", want, out)
		}
	}
}

// Both "warning" and the output spelling "warn" configure the warn gate;
// output always spells "warn".
func TestNewWarnLevelInputAliasesAndOutputSpelling(t *testing.T) {
	for _, level := range []string{"warning", "warn"} {
		var buf bytes.Buffer
		logger, err := New(&buf, level, "json")
		if err != nil {
			t.Fatalf("New(%q): %v", level, err)
		}
		logger.Warn("Request failed")
		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("not JSON: %v: %s", err, buf.String())
		}
		if m["level"] != "warn" {
			t.Fatalf("New(%q): level = %v, want warn (output spelling)", level, m["level"])
		}
	}
}

func TestNewCallerPresentAtDebugGate(t *testing.T) {
	m := logLine(t, "debug", "json", "info")
	if m == nil {
		t.Fatal("info line suppressed at debug level")
	}
	caller, ok := m["caller"].(string)
	if !ok {
		t.Fatal("caller must be present at debug gate")
	}
	if !regexp.MustCompile(`^[^/]+/[^/]+\.go:\d+$`).MatchString(caller) {
		t.Fatalf("caller = %q, want dir/file.go:N", caller)
	}
}

func TestTrimPath(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"c.go", "c.go"},
		{"b/c.go", "b/c.go"},
		{"a/b/c.go", "b/c.go"},
		{"x/a/b/c.go", "b/c.go"},
	} {
		if got := trimPath(tc.in); got != tc.want {
			t.Errorf("trimPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSetupFromEnvFallsBack(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)
	t.Setenv("LOG_LEVEL", "bogus")
	t.Setenv("LOG_FORMAT", "json")
	SetupFromEnv()
	ctx := context.Background()
	if !slog.Default().Enabled(ctx, slog.LevelInfo) {
		t.Fatal("fallback logger must enable info")
	}
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		t.Fatal("fallback logger must gate debug (info default)")
	}
}

func TestSetupFromEnvInvalidLevelEmitsWarn(t *testing.T) {
	t.Setenv("LOG_LEVEL", "bogus")
	t.Setenv("LOG_FORMAT", "json")
	var buf bytes.Buffer
	setupFromEnv(&buf)
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("warn output not JSON: %v: %s", err, buf.String())
	}
	if m["msg"] != "Invalid LOG_LEVEL, using default" {
		t.Fatalf("msg = %v, want invalid-LOG_LEVEL warn", m["msg"])
	}
	if m["fallback"] != "info" {
		t.Fatalf("fallback = %v, want info", m["fallback"])
	}
}

func TestSetupFromEnvInvalidFormatFallsBackToJSON(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "yaml")
	var buf bytes.Buffer
	logger := setupFromEnv(&buf)
	// The warn itself must already be in the fallback format: JSON.
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("fallback output not JSON: %v: %s", err, buf.String())
	}
	if m["msg"] != "Invalid LOG_FORMAT, using default" {
		t.Fatalf("msg = %v, want invalid-LOG_FORMAT warn", m["msg"])
	}
	if m["fallback"] != "json" {
		t.Fatalf("fallback = %v, want json", m["fallback"])
	}
	buf.Reset()
	logger.Info("Started component")
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("subsequent records not JSON: %v: %s", err, buf.String())
	}
}
