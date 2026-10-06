package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	useCollector(t, collectorFunc(func() error { return nil }))
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

type collectorFunc func() error

func (f collectorFunc) CollectOnce() error { return f() }

// This helper replaces a package-global factory; callers must not run in parallel.
func useCollector(t *testing.T, c onceCollector) {
	t.Helper()
	previous := newCollector
	newCollector = func(string, bool) onceCollector { return c }
	t.Cleanup(func() { newCollector = previous })
}

func TestStartOneshot(t *testing.T) {
	collectionErr := errors.New("collection failed")
	for _, wantErr := range []error{nil, collectionErr} {
		name := "success"
		if wantErr != nil {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			useCollector(t, collectorFunc(func() error {
				calls++
				return wantErr
			}))
			err := Start(context.Background(), Config{Oneshot: true})
			if !errors.Is(err, wantErr) || calls != 1 {
				t.Fatalf("oneshot error=%v calls=%d, want %v and exactly one collection", err, calls, wantErr)
			}
		})
	}
}

func TestCommandPassesOutputOptionsToCollector(t *testing.T) {
	// Clear configuration inherited from the developer's shell.
	for _, key := range []string{"OUTPUT_FILE", "SLEEP_INTERVAL", "ONESHOT", "NO_TIMESTAMP"} {
		t.Setenv("RBLN_NPU_FEATURE_DISCOVERY_"+key, "")
	}
	output := filepath.Join(t.TempDir(), "custom-features")
	previous := newCollector
	var gotOutput string
	var gotNoTimestamp bool
	calls := 0
	newCollector = func(path string, noTimestamp bool) onceCollector {
		gotOutput, gotNoTimestamp = path, noTimestamp
		return collectorFunc(func() error { calls++; return nil })
	}
	t.Cleanup(func() { newCollector = previous })
	app := NewApp()
	app.SetArgs([]string{"--oneshot", "--no-timestamp", "--output-file", output})
	if err := app.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotOutput != output || !gotNoTimestamp || calls != 1 {
		t.Fatalf("collector received path=%q noTimestamp=%v calls=%d", gotOutput, gotNoTimestamp, calls)
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

// flakyCollector fails its first `failures` collections, then succeeds.
type flakyCollector struct {
	failures int
	calls    int
}

func (f *flakyCollector) CollectOnce() error {
	f.calls++
	if f.calls <= f.failures {
		return fmt.Errorf("simulated failure %d", f.calls)
	}
	return nil
}

// A successful collection after failed cycles must leave explicit log
// evidence — with unchanged labels the snapshot stays silent, so without a
// recovery log the error stream just stops and recovery is only inferable.
func TestStartLogsRecoveryAfterFailedCycles(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	useCollector(t, &flakyCollector{failures: 2})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := Config{
		OutputFile:    filepath.Join(t.TempDir(), "labels"),
		SleepInterval: 20 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- Start(ctx, cfg) }()

	deadline := time.After(10 * time.Second)
	for !strings.Contains(buf.String(), "Collection recovered") {
		select {
		case err := <-done:
			t.Fatalf("Start returned early: %v, logs: %s", err, buf.String())
		case <-deadline:
			t.Fatalf("recovery never logged: %s", buf.String())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	if !strings.Contains(buf.String(), `"failedCycles":2`) {
		t.Fatalf("recovery must count the failed cycles, got: %s", buf.String())
	}
}

// The real signal path: a delivered SIGTERM must both return nil and leave a
// shutdown log naming the signal.
func TestStartLogsSignalNameOnSIGTERM(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	collected := make(chan struct{})
	var collectedOnce sync.Once
	useCollector(t, collectorFunc(func() error {
		collectedOnce.Do(func() { close(collected) })
		return nil
	}))

	cfg := Config{
		OutputFile:    filepath.Join(t.TempDir(), "labels"),
		SleepInterval: time.Hour,
	}

	done := make(chan error, 1)
	go func() { done <- Start(context.Background(), cfg) }()

	// Start registers the signal handler before the first collection. The
	// channel lets this test exercise signals without reading host sysfs.
	select {
	case <-collected:
	case err := <-done:
		t.Fatalf("Start returned before signal: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("initial collection never ran")
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
