package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/pciids"
	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/sysfs"
)

// These tests run the real sysfs reader, collector and file publisher together,
// without inspecting the machine running go test or requiring an NPU/NFD.
type hardwareFixture struct {
	root string
	c    *FeaturesCollector
	now  time.Time
}

func newHardwareFixture(t *testing.T) *hardwareFixture {
	t.Helper()
	f := &hardwareFixture{root: t.TempDir(), now: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}
	for _, path := range []string{"sys/bus/pci/devices", "features.d"} {
		if err := os.MkdirAll(filepath.Join(f.root, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.write(t, "pci.ids", "1eff  Rebellions\n\t2130  RBLN-CR13 (PF)\n\t2131  RBLN-CR13 (VF)\n\t1220  RBLN-CA22 (PF)\n")
	ids, err := pciids.LoadRebellionsPCIIDs(filepath.Join(f.root, "pci.ids"))
	if err != nil {
		t.Fatal(err)
	}
	f.c = &FeaturesCollector{
		outputFile: filepath.Join(f.root, "features.d/rbln-features"),
		pciIDs:     ids,
		source:     sysfs.NewReader(f.root),
		now:        func() time.Time { return f.now },
	}
	return f
}

func (f *hardwareFixture) write(t *testing.T, path, value string) {
	t.Helper()
	path = filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *hardwareFixture) remove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(f.root, path)); err != nil {
		t.Fatal(err)
	}
}

func (f *hardwareFixture) device(t *testing.T, addr, id string) {
	t.Helper()
	f.write(t, "sys/bus/pci/devices/"+addr+"/vendor", "0x1eff\n")
	f.write(t, "sys/bus/pci/devices/"+addr+"/device", "0x"+id+"\n")
}

func fileBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Compare the complete contract, including missing/extra/duplicate keys.
// Expiry is deliberately separate from the hardware label values.
func publishedLabels(t *testing.T, c *FeaturesCollector) (map[string]string, time.Time) {
	t.Helper()
	text := fileBytes(t, c.outputFile)
	if !strings.HasSuffix(text, "\n") {
		t.Fatal("feature file must end with a newline")
	}
	labels := make(map[string]string)
	var expiry time.Time
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if strings.HasPrefix(line, "# +expiry-time=") {
			if !expiry.IsZero() {
				t.Fatal("duplicate expiry directive")
			}
			var err error
			expiry, err = time.Parse(time.RFC3339, strings.TrimPrefix(line, "# +expiry-time="))
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(key, "rebellions.ai/") {
			t.Fatalf("invalid feature line %q", line)
		}
		if _, exists := labels[key]; exists {
			t.Fatalf("duplicate label %q", key)
		}
		labels[key] = value
	}
	return labels, expiry
}

func expectLabels(t *testing.T, c *FeaturesCollector, expected map[string]string) {
	t.Helper()
	labels, _ := publishedLabels(t, c)
	if !reflect.DeepEqual(labels, expected) {
		t.Fatalf("published labels:\n%v\nwant:\n%v", labels, expected)
	}
}

func TestCollectOncePublishesCompleteLabels(t *testing.T) {
	f := newHardwareFixture(t)
	f.device(t, "0000:01:00.0", "2130")
	f.device(t, "0000:02:00.0", "2130")
	f.write(t, "sys/class/rebellions/rbln0/card_name", " RBLN-CR13\n")
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "1.2.3-rebel1\n")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{
		"rebellions.ai/npu.present":             "true",
		"rebellions.ai/npu.count":               "2",
		"rebellions.ai/npu.product":             "RBLN-CR13",
		"rebellions.ai/driver-version.full":     "1.2.3",
		"rebellions.ai/driver-version.major":    "1",
		"rebellions.ai/driver-version.minor":    "2",
		"rebellions.ai/driver-version.patch":    "3",
		"rebellions.ai/driver-version.revision": "rebel1",
	})
	_, expiry := publishedLabels(t, f.c)
	if !expiry.Equal(f.now.Add(time.Hour)) {
		t.Fatalf("expiry=%s, want one hour after collection", expiry)
	}
	if _, err := os.Stat(filepath.Join(f.root, "features.d/.rbln-features")); !os.IsNotExist(err) {
		t.Fatalf("successful publication left a temporary file: %v", err)
	}
}

func TestCollectOnceCountsVFsInsteadOfPF(t *testing.T) {
	f := newHardwareFixture(t)
	f.device(t, "0000:01:00.0", "2130")
	f.write(t, "sys/bus/pci/devices/0000:01:00.0/sriov_numvfs", "2\n")
	f.device(t, "0000:01:00.1", "2131")
	f.device(t, "0000:01:00.2", "2131")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{
		"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "2", "rebellions.ai/npu.product": "RBLN-CR13",
	})
}

func TestUnknownProductPreservesDriverLabels(t *testing.T) {
	f := newHardwareFixture(t)
	f.device(t, "0000:01:00.0", "ffff")
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "3.4.0")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{
		"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1",
		"rebellions.ai/driver-version.full": "3.4.0", "rebellions.ai/driver-version.major": "3",
		"rebellions.ai/driver-version.minor": "4", "rebellions.ai/driver-version.patch": "0",
	})
}

func TestNoDevicesDoesNotReadDriverAttributes(t *testing.T) {
	f := newHardwareFixture(t)
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "malformed")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatalf("empty inventory should not consult driver attributes: %v", err)
	}
	expectLabels(t, f.c, map[string]string{"rebellions.ai/npu.present": "false"})
}

func TestCollectOnceProductFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(*testing.T, *hardwareFixture)
		product string
	}{
		{
			name: "card_name wins over pci.ids",
			setup: func(t *testing.T, f *hardwareFixture) {
				f.write(t, "sys/class/rebellions/rbln0/card_name", "RBLN-CA25\n")
			},
			product: "RBLN-CA25",
		},
		{name: "old driver without card_name uses pci.ids", product: "RBLN-CR13"},
		{
			name: "empty card_name uses pci.ids",
			setup: func(t *testing.T, f *hardwareFixture) {
				f.write(t, "sys/class/rebellions/rbln0/card_name", " \n")
			},
			product: "RBLN-CR13",
		},
		{
			name: "card_name works without pci.ids",
			setup: func(t *testing.T, f *hardwareFixture) {
				f.c.pciIDs = nil
				f.write(t, "sys/class/rebellions/rbln0/card_name", "RBLN-CR13")
			},
			product: "RBLN-CR13",
		},
		{
			name: "unknown SKU omits only product",
			setup: func(t *testing.T, f *hardwareFixture) {
				f.write(t, "sys/bus/pci/devices/0000:01:00.0/device", "0xffff")
			},
		},
		{
			name:  "missing database omits only product",
			setup: func(_ *testing.T, f *hardwareFixture) { f.c.pciIDs = nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHardwareFixture(t)
			f.device(t, "0000:01:00.0", "2130")
			if tc.setup != nil {
				tc.setup(t, f)
			}
			if err := f.c.CollectOnce(); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1"}
			if tc.product != "" {
				want["rebellions.ai/npu.product"] = tc.product
			}
			expectLabels(t, f.c, want)
		})
	}
}

func TestProductFallbackAfterClassReadError(t *testing.T) {
	f := newHardwareFixture(t)
	f.write(t, "sys/class/rebellions", "not a directory")
	// Isolate product resolution: a broken class path also prevents the
	// driver-version read later in CollectOnce, which is a separate failure.
	if got := f.c.resolveProduct("2130"); got != "RBLN-CR13" {
		t.Fatalf("product after card_name read error = %q, want pci.ids fallback", got)
	}
}

func TestCollectOnceReplacesStaleLabels(t *testing.T) {
	f := newHardwareFixture(t)
	f.device(t, "0000:01:00.0", "2130")
	f.device(t, "0000:02:00.0", "2130")
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "1.2.3-old")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	// Same collector, new version with no revision: the old revision must go.
	f.remove(t, "sys/bus/pci/devices/0000:02:00.0")
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "3.4.0")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{
		"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1", "rebellions.ai/npu.product": "RBLN-CR13",
		"rebellions.ai/driver-version.full": "3.4.0", "rebellions.ai/driver-version.major": "3",
		"rebellions.ai/driver-version.minor": "4", "rebellions.ai/driver-version.patch": "0",
	})
	f.remove(t, "sys/class/rebellions")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{
		"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1", "rebellions.ai/npu.product": "RBLN-CR13",
	})
	f.write(t, "sys/bus/pci/devices/0000:01:00.0/device", "0xffff")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1"})
	f.remove(t, "sys/bus/pci/devices/0000:01:00.0")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{"rebellions.ai/npu.present": "false"})
}

func TestEmptyDriverVersionPreservesHardwareLabelsAndRecovers(t *testing.T) {
	for _, blank := range []string{"", " \t\n"} {
		t.Run(fmt.Sprintf("blank=%q", blank), func(t *testing.T) {
			f := newHardwareFixture(t)
			f.device(t, "0000:01:00.0", "2130")
			f.write(t, "sys/class/rebellions/rbln0/kernel_version", "1.2.3-old")
			if err := f.c.CollectOnce(); err != nil {
				t.Fatal(err)
			}
			f.now = f.now.Add(time.Minute)
			f.write(t, "sys/class/rebellions/rbln0/kernel_version", blank)
			if err := f.c.CollectOnce(); err != nil {
				t.Fatalf("blank driver version must not prevent hardware label publication: %v", err)
			}
			expectLabels(t, f.c, map[string]string{
				"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1", "rebellions.ai/npu.product": "RBLN-CR13",
			})
			_, expiry := publishedLabels(t, f.c)
			if !expiry.Equal(f.now.Add(time.Hour)) {
				t.Fatalf("hardware labels were not renewed: expiry=%s", expiry)
			}
			f.write(t, "sys/class/rebellions/rbln0/kernel_version", "3.4.0")
			if err := f.c.CollectOnce(); err != nil {
				t.Fatal(err)
			}
			expectLabels(t, f.c, map[string]string{
				"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1", "rebellions.ai/npu.product": "RBLN-CR13",
				"rebellions.ai/driver-version.full": "3.4.0", "rebellions.ai/driver-version.major": "3",
				"rebellions.ai/driver-version.minor": "4", "rebellions.ai/driver-version.patch": "0",
			})
		})
	}
}

func TestEmptyRevisionRemovesPreviouslyPublishedRevision(t *testing.T) {
	f := newHardwareFixture(t)
	f.device(t, "0000:01:00.0", "2130")
	f.write(t, "sys/class/rebellions/rbln0/kernel_version", "1.2.3-old")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.2.3-", "1.2.3+", "1.2.3~"} {
		f.write(t, "sys/class/rebellions/rbln0/kernel_version", version)
		if err := f.c.CollectOnce(); err != nil {
			t.Fatal(err)
		}
		expectLabels(t, f.c, map[string]string{
			"rebellions.ai/npu.present": "true", "rebellions.ai/npu.count": "1", "rebellions.ai/npu.product": "RBLN-CR13",
			"rebellions.ai/driver-version.full": "1.2.3", "rebellions.ai/driver-version.major": "1",
			"rebellions.ai/driver-version.minor": "2", "rebellions.ai/driver-version.patch": "3",
		})
	}
}

func TestExpiryRefreshAndNoTimestamp(t *testing.T) {
	f := newHardwareFixture(t)
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	labels, expiry := publishedLabels(t, f.c)
	f.now = f.now.Add(10 * time.Minute)
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	refreshed, nextExpiry := publishedLabels(t, f.c)
	if !reflect.DeepEqual(labels, refreshed) || !nextExpiry.Equal(expiry.Add(10*time.Minute)) {
		t.Fatalf("unchanged labels must renew expiry: labels=%v expiry=%s", refreshed, nextExpiry)
	}
	f.c.noTimestamp = true
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	withoutTimestamp, noExpiry := publishedLabels(t, f.c)
	if !reflect.DeepEqual(labels, withoutTimestamp) || !noExpiry.IsZero() {
		t.Fatalf("no-timestamp changed labels or retained expiry: %v, %v", withoutTimestamp, noExpiry)
	}
}

func TestCollectOnceFailurePreservesPublishedFileAndRecovers(t *testing.T) {
	for _, stage := range []string{"scan", "driver read", "driver parse", "write"} {
		t.Run(stage, func(t *testing.T) {
			f := newHardwareFixture(t)
			f.device(t, "0000:01:00.0", "2130")
			f.write(t, "sys/class/rebellions/rbln0/kernel_version", "1.2.3")
			if err := f.c.CollectOnce(); err != nil {
				t.Fatal(err)
			}
			previous := fileBytes(t, f.c.outputFile)
			f.now = f.now.Add(time.Minute)
			switch stage {
			case "scan":
				f.remove(t, "sys/bus/pci/devices/0000:01:00.0/vendor")
			case "driver read":
				f.remove(t, "sys/class/rebellions/rbln0/kernel_version")
				if err := os.Mkdir(filepath.Join(f.root, "sys/class/rebellions/rbln0/kernel_version"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "driver parse":
				f.write(t, "sys/class/rebellions/rbln0/kernel_version", "not-a-version")
			case "write":
				// A directory is a portable write failure even when tests run as root.
				if err := os.Mkdir(filepath.Join(f.root, "features.d/.rbln-features"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.c.CollectOnce(); err == nil {
				t.Fatalf("%s failure was swallowed", stage)
			}
			if got := fileBytes(t, f.c.outputFile); got != previous {
				t.Fatalf("%s failure changed the last good publication or its expiry", stage)
			}
			f.device(t, "0000:01:00.0", "2130")
			f.remove(t, "sys/class/rebellions/rbln0/kernel_version")
			f.write(t, "sys/class/rebellions/rbln0/kernel_version", "2.0.0")
			f.remove(t, "features.d/.rbln-features")
			if err := f.c.CollectOnce(); err != nil {
				t.Fatalf("recovery: %v", err)
			}
			labels, expiry := publishedLabels(t, f.c)
			if labels["rebellions.ai/driver-version.full"] != "2.0.0" || !expiry.Equal(f.now.Add(time.Hour)) {
				t.Fatalf("recovery did not publish current values: %v, %s", labels, expiry)
			}
		})
	}
}

func TestPublicationRenameFailureAndRecovery(t *testing.T) {
	f := newHardwareFixture(t)
	f.write(t, "features.d/rbln-features/keep", "unrelated data")
	if err := f.c.CollectOnce(); err == nil || !strings.Contains(err.Error(), "publishing feature file") {
		t.Fatalf("expected rename failure, got %v", err)
	}
	if got := fileBytes(t, filepath.Join(f.c.outputFile, "keep")); got != "unrelated data" {
		t.Fatalf("failed publication modified destination: %q", got)
	}
	f.remove(t, "features.d/rbln-features")
	if err := f.c.CollectOnce(); err != nil {
		t.Fatal(err)
	}
	expectLabels(t, f.c, map[string]string{"rebellions.ai/npu.present": "false"})
}

func TestPublicationRequiresOutputDirectory(t *testing.T) {
	f := newHardwareFixture(t)
	f.remove(t, "features.d")
	if err := f.c.CollectOnce(); err == nil || !strings.Contains(err.Error(), "output path validation") {
		t.Fatalf("expected missing output directory error, got %v", err)
	}
}

// This is a scheduling-dependent stress check, not a deterministic proof of
// rename atomicity: a regression is observable only when a read overlaps a
// partial write. Deterministic publication and failure tests above complement it.
func TestPublicationReadersSeeOnlyCompleteFiles(t *testing.T) {
	f := newHardwareFixture(t)
	f.c.noTimestamp = true
	one, two := Features{NPUPresent: true, NPUCount: ptr(1)}, Features{NPUPresent: true, NPUCount: ptr(2)}
	if err := f.c.save(one); err != nil {
		t.Fatal(err)
	}
	const first = "rebellions.ai/npu.present=true\nrebellions.ai/npu.count=1\n"
	const second = "rebellions.ai/npu.present=true\nrebellions.ai/npu.count=2\n"
	done := make(chan struct{})
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		firstRead := true
		for {
			data, err := os.ReadFile(f.c.outputFile)
			if firstRead {
				close(started)
				firstRead = false
			}
			if err != nil {
				result <- err
				return
			}
			if string(data) != first && string(data) != second {
				result <- fmt.Errorf("reader saw partial/mixed publication: %q", data)
				return
			}
			select {
			case <-done:
				result <- nil
				return
			default:
			}
		}
	}()
	<-started
	// Always join the reader, including if a write fails.
	t.Cleanup(func() {
		close(done)
		if err := <-result; err != nil {
			t.Error(err)
		}
	})
	for i := 0; i < 30; i++ {
		for _, features := range []Features{two, one} {
			if err := f.c.save(features); err != nil {
				t.Fatal(err)
			}
		}
	}
}
