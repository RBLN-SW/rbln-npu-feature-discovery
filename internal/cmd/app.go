package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/collector"
	"github.com/spf13/cobra"
)

func NewApp() *cobra.Command {
	builder := newConfigBuilder(os.Getenv)

	cmd := &cobra.Command{
		Use:           "rbln-npu-feature-discovery",
		Short:         "Generate NPU labels for node-feature-discovery",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := builder.finalize(); err != nil {
				return err
			}
			return Start(cmd.Context(), builder.cfg)
		},
	}

	builder.bindFlags(cmd.Flags())

	return cmd
}

// version is stamped by the build via -ldflags -X (see Makefile/Dockerfile);
// a plain `go build` yields "dev".
var version = "dev"

type onceCollector interface {
	CollectOnce() error
}

// newCollector is a test seam over the concrete collector constructor.
var newCollector = func(outputFile string, noTimestamp bool) onceCollector {
	return collector.NewFeaturesCollector(outputFile, noTimestamp)
}

func Start(ctx context.Context, cfg Config) error {
	slog.Info("Starting rbln-npu-feature-discovery",
		"version", version,
		"outputFile", cfg.OutputFile,
		"sleepInterval", cfg.SleepInterval.String(),
		"oneshot", cfg.Oneshot,
		"noTimestamp", cfg.NoTimestamp)

	// Cause-aware equivalent of signal.NotifyContext, so the shutdown log
	// can say which signal (or parent cancellation) triggered it.
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case sig := <-sigCh:
			cancel(fmt.Errorf("received signal %s", sig))
		case <-ctx.Done():
		}
	}()

	c := newCollector(cfg.OutputFile, cfg.NoTimestamp)

	if cfg.Oneshot {
		return c.CollectOnce()
	}

	// failedCycles makes the failure→recovery transition explicit in the
	// logs: a recovered cycle with unchanged labels would otherwise leave
	// only an error stream that silently stops.
	failedCycles := 0
	if err := c.CollectOnce(); err != nil {
		slog.Error("Initial collection failed", "err", err)
		failedCycles = 1
	}

	ticker := time.NewTicker(cfg.SleepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down", "reason", context.Cause(ctx))
			return nil
		case <-ticker.C:
			if err := c.CollectOnce(); err != nil {
				failedCycles++
				slog.Error("Periodic collection failed", "err", err)
			} else if failedCycles > 0 {
				slog.Info("Collection recovered", "failedCycles", failedCycles)
				failedCycles = 0
			}
		}
	}
}
