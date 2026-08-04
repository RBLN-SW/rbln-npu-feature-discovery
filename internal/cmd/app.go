package cmd

import (
	"context"
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

func Start(ctx context.Context, cfg Config) error {
	slog.Info("Starting rbln-npu-feature-discovery",
		"outputFile", cfg.OutputFile,
		"sleepInterval", cfg.SleepInterval.String(),
		"oneshot", cfg.Oneshot,
		"noTimestamp", cfg.NoTimestamp)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	collector := collector.NewFeaturesCollector(cfg.OutputFile, cfg.NoTimestamp)

	if cfg.Oneshot {
		return collector.CollectOnce()
	}

	if err := collector.CollectOnce(); err != nil {
		slog.Error("Initial collection failed", "err", err)
	}

	ticker := time.NewTicker(cfg.SleepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := collector.CollectOnce(); err != nil {
				slog.Error("Periodic collection failed", "err", err)
			}
		}
	}
}
