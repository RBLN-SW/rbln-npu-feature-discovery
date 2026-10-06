package sysfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeAttribute(t *testing.T, root, path, value string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverDevices(t *testing.T) {
	const pci = "sys/bus/pci/devices/"
	tests := []struct {
		name    string
		files   map[string]string
		devices []Device
		skipped []string
		wantErr string
	}{
		{name: "empty bus"},
		{
			name:  "other vendors need no device attributes",
			files: map[string]string{"0000:01:00.0/vendor": "0x1234\n"},
		},
		{
			name: "whitespace and hex prefix",
			files: map[string]string{
				"0000:01:00.0/vendor": " 0x1eff\n", "0000:01:00.0/device": " 0x2130\n",
			},
			devices: []Device{{DeviceID: "2130"}},
		},
		{
			name: "multiple devices including unknown SKU",
			files: map[string]string{
				"0000:01:00.0/vendor": "0x1eff", "0000:01:00.0/device": "0x1220",
				"0000:02:00.0/vendor": "0x1234",
				"0000:03:00.0/vendor": "0x1eff", "0000:03:00.0/device": "0xffff",
			},
			devices: []Device{{DeviceID: "1220"}, {DeviceID: "ffff"}},
		},
		{
			name: "PF with VFs disabled is counted",
			files: map[string]string{
				"0000:01:00.0/vendor": "0x1eff", "0000:01:00.0/device": "0x2130",
				"0000:01:00.0/sriov_numvfs": "0\n",
			},
			devices: []Device{{DeviceID: "2130"}},
		},
		{
			name: "exclude PF while counting its two VFs",
			files: map[string]string{
				"0000:01:00.0/vendor": "0x1eff", "0000:01:00.0/device": "0x2130",
				"0000:01:00.0/sriov_numvfs": " 2\n",
				"0000:01:00.1/vendor":       "0x1eff", "0000:01:00.1/device": "0x2131",
				"0000:01:00.2/vendor": "0x1eff", "0000:01:00.2/device": "0x2131",
			},
			devices: []Device{{DeviceID: "2131"}, {DeviceID: "2131"}},
			skipped: []string{"0000:01:00.0"},
		},
		{
			name: "missing vendor aborts instead of publishing a partial inventory",
			files: map[string]string{
				"0000:01:00.0/vendor": "0x1eff", "0000:01:00.0/device": "0x2130",
				"0000:02:00.0/device": "0x2131",
			},
			wantErr: "read vendor",
		},
		{
			name:    "missing RBLN device attribute",
			files:   map[string]string{"0000:01:00.0/vendor": "0x1eff"},
			wantErr: "read device id",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, pci), 0o755); err != nil {
				t.Fatal(err)
			}
			for path, value := range tc.files {
				writeAttribute(t, root, pci+path, value)
			}
			devices, skipped, err := NewReader(root).DiscoverDevices()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(devices, tc.devices) || !slices.Equal(skipped, tc.skipped) {
				t.Fatalf("devices=%v, skipped=%v; want %v, %v", devices, skipped, tc.devices, tc.skipped)
			}
		})
	}
}

func TestDiscoverDevicesMissingBus(t *testing.T) {
	_, _, err := NewReader(t.TempDir()).DiscoverDevices()
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing sysfs error = %v, want ErrNotExist", err)
	}
}

func TestDiscoverDevicesFollowsSysfsLinks(t *testing.T) {
	root := t.TempDir()
	writeAttribute(t, root, "sys/devices/pci0000:00/0000:01:00.0/vendor", "0x1eff")
	writeAttribute(t, root, "sys/devices/pci0000:00/0000:01:00.0/device", "0x1220")
	bus := filepath.Join(root, "sys/bus/pci/devices")
	if err := os.MkdirAll(bus, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bus, "0000:01:00.0")
	if err := os.Symlink("../../../devices/pci0000:00/0000:01:00.0", link); err != nil {
		t.Fatal(err)
	}
	devices, _, err := NewReader(root).DiscoverDevices()
	if err != nil || !slices.Equal(devices, []Device{{DeviceID: "1220"}}) {
		t.Fatalf("scan through symlink = %v, %v", devices, err)
	}
	// A dangling entry models a device disappearing while the bus is scanned.
	if err := os.RemoveAll(filepath.Join(root, "sys/devices/pci0000:00/0000:01:00.0")); err != nil {
		t.Fatal(err)
	}
	devices, _, err = NewReader(root).DiscoverDevices()
	if !errors.Is(err, fs.ErrNotExist) || len(devices) != 0 {
		t.Fatalf("dangling device = %v, %v; want no partial inventory and ErrNotExist", devices, err)
	}
}

func TestReaderCardName(t *testing.T) {
	const class = "sys/class/rebellions"
	for _, tc := range []struct {
		name    string
		files   map[string]string
		dirs    []string
		product string
		found   bool
		wantErr bool
	}{
		{name: "class directory missing"},
		{name: "empty class directory", dirs: []string{class}},
		{name: "card_name attribute missing", dirs: []string{class + "/rbln0"}},
		{
			name:  "blank card_name",
			files: map[string]string{class + "/rbln0/card_name": " \t\n"},
		},
		{
			name:    "valid card_name with surrounding whitespace",
			files:   map[string]string{class + "/rbln0/card_name": " RBLN-CR13\n"},
			product: "RBLN-CR13", found: true,
		},
		{
			name: "later card after blank attribute",
			files: map[string]string{
				class + "/rbln0/card_name": " \n", class + "/rbln1/card_name": "RBLN-CR13",
			},
			product: "RBLN-CR13", found: true,
		},
		{
			name:    "later card after unreadable attribute",
			dirs:    []string{class + "/rbln0/card_name"},
			files:   map[string]string{class + "/rbln1/card_name": "RBLN-CR13"},
			product: "RBLN-CR13", found: true,
		},
		{
			name:    "invalid class directory",
			files:   map[string]string{class: "not a directory"},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, path := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for path, value := range tc.files {
				writeAttribute(t, root, path, value)
			}
			name, found, err := NewReader(root).ReadCardName()
			if (err != nil) != tc.wantErr || found != tc.found || name != tc.product {
				t.Fatalf("card name = %q, %v, %v; want %q, found=%v, error=%v", name, found, err, tc.product, tc.found, tc.wantErr)
			}
		})
	}
}

func TestReadDriverVersion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "release", value: "3.4.0"},
		{name: "revision with whitespace", value: " \t3.4.0-abc123\n"},
		{name: "empty attribute", value: "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeAttribute(t, root, "sys/class/rebellions/rbln0/kernel_version", tc.value)
			version, found, err := NewReader(root).ReadDriverVersion()
			if err != nil || !found || version != strings.TrimSpace(tc.value) {
				t.Fatalf("driver version = %q, %v, %v", version, found, err)
			}
		})
	}
	t.Run("driver absent", func(t *testing.T) {
		version, found, err := NewReader(t.TempDir()).ReadDriverVersion()
		if err != nil || found || version != "" {
			t.Fatalf("absent driver = %q, %v, %v", version, found, err)
		}
	})
	t.Run("attribute cannot be read", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "sys/class/rebellions/rbln0/kernel_version"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, _, err := NewReader(root).ReadDriverVersion()
		if err == nil {
			t.Fatal("a directory cannot be read as a driver version")
		}
	})
}
