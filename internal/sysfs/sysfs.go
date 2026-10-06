package sysfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	rblnVendorID     = "0x1eff"
	pciDevicesPath   = "/sys/bus/pci/devices"
	rebellionsSysfs  = "/sys/class/rebellions"
	kernelVersionKey = "kernel_version"
	cardNameKey      = "card_name"
)

type Device struct {
	DeviceID string
}

// Reader reads hardware attributes under a filesystem root. Production uses
// "/"; tests can build the same sysfs tree in a temporary directory.
type Reader struct {
	root string
}

func NewReader(root string) *Reader {
	return &Reader{root: root}
}

// DiscoverDevices returns the Rebellions PCI devices plus the PCI addresses
// of physical functions that were skipped because their SR-IOV VFs are
// enabled — the caller logs those so a lowered npu.count stays explainable.
func (r *Reader) DiscoverDevices() ([]Device, []string, error) {
	devicesPath := filepath.Join(r.root, pciDevicesPath)
	entries, err := os.ReadDir(devicesPath)
	if err != nil {
		return nil, nil, err
	}

	var devices []Device
	var skippedPFs []string
	for _, entry := range entries {
		devicePath := filepath.Join(devicesPath, entry.Name())

		vendorBytes, err := os.ReadFile(filepath.Join(devicePath, "vendor"))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read vendor: %w", err)
		}
		if strings.TrimSpace(string(vendorBytes)) != rblnVendorID {
			continue
		}

		sriovNumvfsPath := filepath.Join(devicePath, "sriov_numvfs")
		if data, err := os.ReadFile(sriovNumvfsPath); err == nil {
			if numvfs, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil && numvfs != 0 {
				// skip PF when SR-IOV is enabled
				skippedPFs = append(skippedPFs, entry.Name())
				continue
			}
		}

		deviceIDBytes, err := os.ReadFile(filepath.Join(devicePath, "device"))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read device id: %w", err)
		}
		deviceID := strings.TrimSpace(string(deviceIDBytes))
		deviceID = strings.TrimPrefix(deviceID, "0x")

		devices = append(devices, Device{DeviceID: deviceID})
	}

	return devices, skippedPFs, nil
}

// ReadCardName returns the product name (e.g. "RBLN-CR13") published by the
// driver for the first rbln device exposing card_name. found is false when
// the class dir or a usable attribute is absent (driver not loaded, or too old).
func (r *Reader) ReadCardName() (string, bool, error) {
	classDir := filepath.Join(r.root, rebellionsSysfs)
	entries, err := os.ReadDir(classDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(classDir, entry.Name(), cardNameKey))
		if err != nil {
			continue
		}
		if name := strings.TrimSpace(string(data)); name != "" {
			return name, true, nil
		}
	}
	return "", false, nil
}

// ReadDriverVersion reads rbln0's kernel_version attribute. An absent or blank
// value is unavailable driver metadata, not a failure to discover the NPUs.
// Other read errors are returned so a failed read cannot publish a partial result.
func (r *Reader) ReadDriverVersion() (string, bool, error) {
	sysfsDevice := filepath.Join(r.root, rebellionsSysfs, "rbln0")
	kernelVersionFile := filepath.Join(sysfsDevice, kernelVersionKey)

	if _, err := os.Stat(kernelVersionFile); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	versionBytes, err := os.ReadFile(kernelVersionFile)
	if err != nil {
		return "", false, err
	}

	version := strings.TrimSpace(string(versionBytes))
	return version, version != "", nil
}
