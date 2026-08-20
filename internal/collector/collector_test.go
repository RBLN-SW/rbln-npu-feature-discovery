package collector

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/sysfs"
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

// Every Features field must have a labelFields entry, or a future label
// would reach the file (or the log) without reaching the other.
func TestLabelFieldsCoverAllFeatureFields(t *testing.T) {
	if got, want := len(labelFields), reflect.TypeOf(Features{}).NumField(); got != want {
		t.Fatalf("labelFields has %d entries, Features has %d fields — add the new field to labelFields", got, want)
	}
}

func stubDevices(t *testing.T, devices []sysfs.Device, skippedPFs []string) {
	t.Helper()
	prev := discoverDevices
	discoverDevices = func() ([]sysfs.Device, []string, error) { return devices, skippedPFs, nil }
	t.Cleanup(func() { discoverDevices = prev })
}

// A node without NPUs publishes npu.present=false as a steady state; it must
// not warn about the (expectedly absent) driver version every cycle.
func TestCollectFromSysfsSkipsDriverVersionWithoutDevices(t *testing.T) {
	buf := captureLogs(t)
	stubDevices(t, nil, nil)
	stubDriverVersion(t, "", false, nil) // would warn if consulted

	f := newFeatures()
	c := &FeaturesCollector{}
	if err := c.collectFromSysfs(&f); err != nil {
		t.Fatalf("collectFromSysfs: %v", err)
	}
	if f.NPUPresent {
		t.Fatal("no devices must leave npu.present=false")
	}
	if strings.Contains(buf.String(), "Driver version not found") {
		t.Fatalf("zero-device node must not warn about driver version, got: %s", buf.String())
	}
}

// An SR-IOV PF excluded from npu.count must be discoverable from the logs,
// but a stable exclusion must not repeat every cycle.
func TestCollectFromSysfsLogsSkippedPFsOnChangeOnly(t *testing.T) {
	buf := captureLogs(t)
	c := &FeaturesCollector{}
	stubDevices(t, nil, []string{"0000:17:00.0"})

	f := newFeatures()
	if err := c.collectFromSysfs(&f); err != nil {
		t.Fatalf("collectFromSysfs: %v", err)
	}
	first := buf.String()
	if !strings.Contains(first, "Skipping SR-IOV physical functions") ||
		!strings.Contains(first, "0000:17:00.0") {
		t.Fatalf("skipped PF must be logged with its address, got: %s", first)
	}
	if !strings.Contains(first, `"effect":"excluded from npu.count"`) {
		t.Fatalf("skip log must carry the effect key, got: %s", first)
	}

	buf.Reset()
	if err := c.collectFromSysfs(&f); err != nil {
		t.Fatalf("collectFromSysfs: %v", err)
	}
	if strings.Contains(buf.String(), "Skipping SR-IOV") {
		t.Fatalf("unchanged skip set must not re-log, got: %s", buf.String())
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
