package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// SIGTERM (DaemonSet rollout) cancels the context; that is a graceful
// shutdown and must not surface as an error-level "Command execution
// failed" log + exit 1 in main.
func TestStartReturnsNilOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		OutputFile:    filepath.Join(t.TempDir(), "labels"),
		SleepInterval: time.Hour,
	}

	if err := Start(ctx, cfg); err != nil {
		t.Fatalf("Start() = %v, want nil on graceful shutdown", err)
	}
}
