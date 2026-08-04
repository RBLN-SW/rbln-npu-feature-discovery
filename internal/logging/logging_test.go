package logging

import (
	"bytes"
	"context"
	"encoding/json"
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
	case "trace":
		logger.Log(context.Background(), LevelTrace, "Dumped payload", "bytes", 42)
	case "error":
		logger.Error("Request failed", "err", "boom")
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

func TestNewDefaultsToInfoJSONWithContractKeys(t *testing.T) {
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

func TestNewTraceLevelRendersTraceAndCaller(t *testing.T) {
	m := logLine(t, "trace", "json", "trace")
	if m == nil {
		t.Fatal("trace line suppressed at trace level")
	}
	if m["level"] != "trace" {
		t.Fatalf("level = %v, want trace", m["level"])
	}
	if _, ok := m["caller"].(string); !ok {
		t.Fatal("caller must be present at trace level")
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
