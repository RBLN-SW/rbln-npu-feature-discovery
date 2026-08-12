package cmd

import (
	"testing"

	"github.com/spf13/pflag"
)

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
