package main

import (
	"log/slog"
	"os"

	appcmd "github.com/rebellions-sw/rbln-npu-feature-discovery/internal/cmd"
	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/logging"
)

func main() {
	logging.SetupFromEnv()
	app := appcmd.NewApp()
	if err := app.Execute(); err != nil {
		slog.Error("Command execution failed", "err", err)
		os.Exit(1)
	}
}
