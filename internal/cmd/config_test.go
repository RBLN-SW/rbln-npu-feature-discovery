package cmd

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestConfigPrecedenceAndValidation(t *testing.T) {
	defaults := Config{OutputFile: defaultOutput, SleepInterval: time.Minute}
	for _, tc := range []struct {
		name    string
		env     map[string]string
		args    []string
		want    Config
		wantErr bool
	}{
		{name: "defaults", want: defaults},
		{
			name: "environment overrides defaults",
			env: map[string]string{
				"RBLN_NPU_FEATURE_DISCOVERY_OUTPUT_FILE":    "/tmp/from-env",
				"RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL": "120",
				"RBLN_NPU_FEATURE_DISCOVERY_ONESHOT":        "yes",
				"RBLN_NPU_FEATURE_DISCOVERY_NO_TIMESTAMP":   "ON",
			},
			want: Config{OutputFile: "/tmp/from-env", SleepInterval: 2 * time.Minute, Oneshot: true, NoTimestamp: true},
		},
		{
			name: "flags override environment including explicit false",
			env: map[string]string{
				"RBLN_NPU_FEATURE_DISCOVERY_OUTPUT_FILE":    "/tmp/from-env",
				"RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL": "120",
				"RBLN_NPU_FEATURE_DISCOVERY_ONESHOT":        "yes",
				"RBLN_NPU_FEATURE_DISCOVERY_NO_TIMESTAMP":   "true",
			},
			args: []string{"-o", "/tmp/from-flags", "--sleep-interval=10", "--oneshot=false", "--no-timestamp=false"},
			want: Config{OutputFile: "/tmp/from-flags", SleepInterval: 10 * time.Second},
		},
		{
			name: "invalid environment values use defaults",
			env: map[string]string{
				"RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL": "invalid",
				"RBLN_NPU_FEATURE_DISCOVERY_ONESHOT":        "maybe",
				"RBLN_NPU_FEATURE_DISCOVERY_NO_TIMESTAMP":   "maybe",
			},
			want: defaults,
		},
		{name: "minimum interval", args: []string{"--sleep-interval=10"}, want: Config{OutputFile: defaultOutput, SleepInterval: 10 * time.Second}},
		{name: "maximum interval", args: []string{"--sleep-interval=3600"}, want: Config{OutputFile: defaultOutput, SleepInterval: time.Hour}},
		{name: "below minimum", args: []string{"--sleep-interval=9"}, wantErr: true},
		{name: "above maximum", args: []string{"--sleep-interval=3601"}, wantErr: true},
		{name: "zero interval", args: []string{"--sleep-interval=0"}, wantErr: true},
		{name: "negative interval", args: []string{"--sleep-interval=-1"}, wantErr: true},
		{name: "out of range environment", env: map[string]string{"RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL": "9"}, wantErr: true},
		{
			name: "flag repairs out of range environment",
			env:  map[string]string{"RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL": "9"},
			args: []string{"--sleep-interval=60"}, want: defaults,
		},
		{name: "invalid integer flag", args: []string{"--sleep-interval=invalid"}, wantErr: true},
		{name: "invalid bool flag", args: []string{"--oneshot=invalid"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newConfigBuilder(func(key string) string { return tc.env[key] })
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			b.bindFlags(flags)
			err := flags.Parse(tc.args)
			if err == nil {
				err = b.finalize()
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("invalid configuration was accepted")
				}
				return
			}
			if err != nil || b.cfg != tc.want {
				t.Fatalf("config=%+v, error=%v; want %+v", b.cfg, err, tc.want)
			}
		})
	}
}

func TestBooleanEnvironmentAliases(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "Y", "on", "0", "false", "FALSE", "no", "N", "off"} {
		t.Run(value, func(t *testing.T) {
			want := value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "Y" || value == "on"
			got := getenvBoolDefault(func(string) string { return value }, "test", !want)
			if got != want {
				t.Fatalf("%q parsed as %v, want %v", value, got, want)
			}
		})
	}
}

// The rbln-npu-operator passes --rbln-daemon-url unconditionally; the flag
// must stay parseable (as a no-op) until the operator stops sending it.
func TestDeprecatedDaemonURLFlagIsAccepted(t *testing.T) {
	b := newConfigBuilder(func(string) string { return "" })
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	b.bindFlags(fs)

	if err := fs.Parse([]string{"--rbln-daemon-url", "http://10.0.0.1:50051"}); err != nil {
		t.Fatalf("deprecated flag must parse without error: %v", err)
	}
	if err := b.finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
}
