package main

import (
	"log/slog"
	"os"
	"time"

	appcmd "github.com/rebellions-sw/rbln-npu-feature-discovery/internal/cmd"
)

func initLogger() {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func main() {
	initLogger()
	app := appcmd.NewApp()
	if err := app.Execute(); err != nil {
		slog.Error("command execution failed", "err", err)
		os.Exit(1)
	}
}
