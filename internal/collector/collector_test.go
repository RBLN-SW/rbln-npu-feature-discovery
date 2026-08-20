package collector

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// The published label set must be reconstructable from the logs alone:
// an info snapshot on every change, silence while the state is stable.
func TestSaveLogsSnapshotOnChangeOnly(t *testing.T) {
	buf := captureLogs(t)
	c := &FeaturesCollector{
		outputFile:  filepath.Join(t.TempDir(), "labels"),
		noTimestamp: true,
	}

	f := newFeatures()
	f.NPUPresent = true
	f.NPUCount = ptr(2)
	f.NPUProduct = ptr("RBLN-CA22")
	f.DriverVersionFull = ptr("1.2.3")
	f.DriverVersionMajor = ptr("1")
	f.DriverVersionMinor = ptr("2")
	f.DriverVersionPatch = ptr("3")
	f.DriverVersionRevision = ptr("rebel1")

	if err := c.save(f); err != nil {
		t.Fatalf("save: %v", err)
	}
	first := buf.String()
	if !strings.Contains(first, `"msg":"Feature labels published"`) {
		t.Fatalf("first save must log a snapshot, got: %s", first)
	}
	// Every label written by toPlainText must appear in the snapshot.
	for _, want := range []string{
		`"npuPresent":true`, `"npuCount":2`, `"npuProduct":"RBLN-CA22"`,
		`"driverVersionFull":"1.2.3"`, `"driverVersionMajor":"1"`,
		`"driverVersionMinor":"2"`, `"driverVersionPatch":"3"`,
		`"driverVersionRevision":"rebel1"`,
	} {
		if !strings.Contains(first, want) {
			t.Errorf("snapshot missing %s, got: %s", want, first)
		}
	}

	buf.Reset()
	if err := c.save(f); err != nil {
		t.Fatalf("save: %v", err)
	}
	if strings.Contains(buf.String(), "Feature labels published") {
		t.Fatalf("unchanged features must not re-log the snapshot, got: %s", buf.String())
	}

	buf.Reset()
	f.NPUCount = ptr(4)
	if err := c.save(f); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !strings.Contains(buf.String(), `"npuCount":4`) {
		t.Fatalf("changed features must log a new snapshot, got: %s", buf.String())
	}
}

func stubDriverVersion(t *testing.T, version string, found bool, err error) {
	t.Helper()
	prev := readDriverVersion
	readDriverVersion = func() (string, bool, error) { return version, found, err }
	t.Cleanup(func() { readDriverVersion = prev })
}

// A missing kernel_version attribute silently drops all five driver-version
// labels; that degradation must leave log evidence.
func TestCollectDriverVersionWarnsWhenAttributeMissing(t *testing.T) {
	buf := captureLogs(t)
	stubDriverVersion(t, "", false, nil)

	f := newFeatures()
	if err := collectDriverVersion(&f); err != nil {
		t.Fatalf("collectDriverVersion: %v", err)
	}
	if f.DriverVersionFull != nil {
		t.Fatal("driver-version labels must stay omitted when attribute is missing")
	}
	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) || !strings.Contains(out, "Driver version not found") {
		t.Fatalf("missing attribute must warn, got: %s", out)
	}
	if !strings.Contains(out, `"effect":"driver-version labels omitted"`) {
		t.Fatalf("warn must carry the effect key, got: %s", out)
	}
}

func TestCollectDriverVersionSetsLabels(t *testing.T) {
	stubDriverVersion(t, "1.2.3-rebel1", true, nil)

	f := newFeatures()
	if err := collectDriverVersion(&f); err != nil {
		t.Fatalf("collectDriverVersion: %v", err)
	}
	if f.DriverVersionFull == nil || *f.DriverVersionFull != "1.2.3" {
		t.Fatalf("DriverVersionFull = %v, want 1.2.3", f.DriverVersionFull)
	}
	if f.DriverVersionRevision == nil || *f.DriverVersionRevision != "rebel1" {
		t.Fatalf("DriverVersionRevision = %v, want rebel1", f.DriverVersionRevision)
	}
}

func TestApplyProduct(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "RBLN-CR13")
	if f.NPUProduct == nil || *f.NPUProduct != "RBLN-CR13" {
		t.Fatalf("NPUProduct = %v, want RBLN-CR13", f.NPUProduct)
	}
}

func TestApplyProductEmptyIsNonFatal(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "")
	if f.NPUProduct != nil {
		t.Fatal("empty product must leave the product label unset")
	}
}
