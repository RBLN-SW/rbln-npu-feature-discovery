package main

import (
	"log/slog"
	"os"

	appcmd "github.com/rebellions-sw/rbln-npu-feature-discovery/internal/cmd"
	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/logging"
)

func main() {
	if err := logging.SetupFromEnv(); err != nil {
		slog.Error("Invalid logging configuration", "err", err)
		os.Exit(1)
	}
	app := appcmd.NewApp()
	if err := app.Execute(); err != nil {
		slog.Error("Command execution failed", "err", err)
		os.Exit(1)
	}
}
