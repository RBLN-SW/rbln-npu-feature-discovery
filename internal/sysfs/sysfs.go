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
)

type Device struct {
	DeviceID string
}

func DiscoverDevices() ([]Device, error) {
	entries, err := os.ReadDir(pciDevicesPath)
	if err != nil {
		return nil, err
	}

	var devices []Device
	for _, entry := range entries {
		devicePath := filepath.Join(pciDevicesPath, entry.Name())

		vendorBytes, err := os.ReadFile(filepath.Join(devicePath, "vendor"))
		if err != nil {
			return nil, fmt.Errorf("failed to read vendor: %w", err)
		}
		if strings.TrimSpace(string(vendorBytes)) != rblnVendorID {
			continue
		}

		sriovNumvfsPath := filepath.Join(devicePath, "sriov_numvfs")
		if data, err := os.ReadFile(sriovNumvfsPath); err == nil {
			if numvfs, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil && numvfs != 0 {
				// skip PF when SR-IOV is enabled
				continue
			}
		}

		deviceIDBytes, err := os.ReadFile(filepath.Join(devicePath, "device"))
		if err != nil {
			return nil, fmt.Errorf("failed to read device id: %w", err)
		}
		deviceID := strings.TrimSpace(string(deviceIDBytes))
		deviceID = strings.TrimPrefix(deviceID, "0x")

		devices = append(devices, Device{DeviceID: deviceID})
	}

	return devices, nil
}

func ReadDriverVersion() (string, bool, error) {
	sysfsDevice := filepath.Join(rebellionsSysfs, "rbln0")
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

	return strings.TrimSpace(string(versionBytes)), true, nil
}
