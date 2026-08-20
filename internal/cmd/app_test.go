package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// syncBuffer guards a bytes.Buffer read from the test goroutine while the
// Start goroutine is still logging into it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The real signal path: a delivered SIGTERM must both return nil and leave a
// shutdown log naming the signal.
func TestStartLogsSignalNameOnSIGTERM(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := Config{
		OutputFile:    filepath.Join(t.TempDir(), "labels"),
		SleepInterval: time.Hour,
	}

	done := make(chan error, 1)
	go func() { done <- Start(context.Background(), cfg) }()

	// Start registers the signal handler before its first collection, so once
	// the collection outcome is logged the SIGTERM is guaranteed to be caught
	// instead of killing the test process.
	collected := func() bool {
		out := buf.String()
		return strings.Contains(out, "Feature labels published") ||
			strings.Contains(out, "Initial collection failed")
	}
	deadline := time.After(10 * time.Second)
	for !collected() {
		select {
		case err := <-done:
			t.Fatalf("Start returned before signal: %v, logs: %s", err, buf.String())
		case <-deadline:
			t.Fatalf("initial collection never logged: %s", buf.String())
		case <-time.After(10 * time.Millisecond):
		}
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() = %v, want nil on SIGTERM", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return after SIGTERM")
	}
	if out := buf.String(); !strings.Contains(out, "received signal terminated") {
		t.Fatalf("shutdown log must name the signal, got: %s", out)
	}
}
