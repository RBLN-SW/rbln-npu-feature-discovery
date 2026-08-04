package cmd

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const (
	MinSleepIntervalSeconds = 10
	MaxSleepIntervalSeconds = 3600

	defaultOutput = "/etc/kubernetes/node-feature-discovery/features.d/rbln-features"
)

type Config struct {
	OutputFile    string
	SleepInterval time.Duration
	Oneshot       bool
	NoTimestamp   bool
}

type configBuilder struct {
	cfg              Config
	sleepIntervalSec int
}

func newConfigBuilder(getenv func(string) string) *configBuilder {
	cfg := Config{
		OutputFile:    getenvDefault(getenv, "RBLN_NPU_FEATURE_DISCOVERY_OUTPUT_FILE", defaultOutput),
		SleepInterval: time.Duration(getenvIntDefault(getenv, "RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL", 60)) * time.Second,
		Oneshot:       getenvBoolDefault(getenv, "RBLN_NPU_FEATURE_DISCOVERY_ONESHOT", false),
		NoTimestamp:   getenvBoolDefault(getenv, "RBLN_NPU_FEATURE_DISCOVERY_NO_TIMESTAMP", false),
	}

	return &configBuilder{
		cfg:              cfg,
		sleepIntervalSec: int(cfg.SleepInterval / time.Second),
	}
}

func (b *configBuilder) bindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&b.cfg.Oneshot, "oneshot", b.cfg.Oneshot, "Label once and exit")
	fs.BoolVar(&b.cfg.NoTimestamp, "no-timestamp", b.cfg.NoTimestamp, "Skip writing expiry timestamp to labels")
	fs.IntVar(&b.sleepIntervalSec, "sleep-interval", b.sleepIntervalSec, fmt.Sprintf("Time to sleep between labeling (min: %ds, max: %ds)", MinSleepIntervalSeconds, MaxSleepIntervalSeconds))
	fs.StringVarP(&b.cfg.OutputFile, "output-file", "o", b.cfg.OutputFile, "Path to output file")

	// Deprecated no-op: rbln-npu-operator passes this flag unconditionally
	// (npu_feature_discovery.go WithArgs), so dropping it outright would
	// crash operator-managed pods with "unknown flag". Remove once the
	// operator stops sending it.
	fs.String("rbln-daemon-url", "", "Deprecated: daemon collection was removed; the value is ignored")
	_ = fs.MarkDeprecated("rbln-daemon-url", "daemon collection was removed; the value is ignored")
}

func (b *configBuilder) finalize() error {
	if b.sleepIntervalSec < MinSleepIntervalSeconds || b.sleepIntervalSec > MaxSleepIntervalSeconds {
		return fmt.Errorf("sleep-interval must be %d-%d seconds", MinSleepIntervalSeconds, MaxSleepIntervalSeconds)
	}
	b.cfg.SleepInterval = time.Duration(b.sleepIntervalSec) * time.Second
	return nil
}

func getenvDefault(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func getenvIntDefault(getenv func(string) string, key string, def int) int {
	if v := getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		slog.Warn("Ignoring invalid integer env value", "key", key, "value", v, "fallback", def)
	}
	return def
}

func getenvBoolDefault(getenv func(string) string, key string, def bool) bool {
	if v := getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
		slog.Warn("Ignoring invalid boolean env value", "key", key, "value", v, "fallback", def)
	}
	return def
}
