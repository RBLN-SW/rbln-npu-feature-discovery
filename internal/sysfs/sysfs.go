// Package sysfs adapts rblnlib-go hardware observations to NFD's label source.
package sysfs

import (
	"context"
	"fmt"
	"path/filepath"

	host "github.com/rbln-sw/rblnlib-go/pkg/host/v1"
	system "github.com/rbln-sw/rblnlib-go/pkg/system/v1"
)

type (
	Device struct{ DeviceID string }
	Reader struct {
		root     string
		devices  []system.Device
		observed bool
	}
)

func NewReader(root string) *Reader { return &Reader{root: root} }
func (r *Reader) observe() error {
	r.observed = false
	root, err := filepath.Abs(r.root)
	if err != nil {
		return err
	}
	c, err := host.New(host.Options{SysRoot: filepath.Join(root, "sys"), DevRoot: filepath.Join(root, "dev"), ProcRoot: filepath.Join(root, "proc")})
	if err != nil {
		return err
	}
	snapshot, err := c.ListDevices(context.Background())
	if err != nil {
		return err
	}
	if err := system.IssuesError(snapshot.Issues); err != nil {
		return err
	}
	r.devices = snapshot.Devices
	r.observed = true
	return nil
}

// Counting and the exclusion of hosting PFs are NFD policy.
func (r *Reader) DiscoverDevices() ([]Device, []string, error) {
	if err := r.observe(); err != nil {
		return nil, nil, err
	}
	devices := []Device{}
	skipped := []string{}
	for _, d := range r.devices {
		for _, issue := range d.Issues {
			if issue.Field == "sriov" {
				return nil, nil, issue.Err
			}
		}
		if d.SRIOV != nil && d.SRIOV.Kind == system.PhysicalFunction && d.SRIOV.EnabledVFs > 0 {
			skipped = append(skipped, d.PCIAddress)
			continue
		}
		devices = append(devices, Device{DeviceID: fmt.Sprintf("%04x", d.DeviceID)})
	}
	return devices, skipped, nil
}

func (r *Reader) metadataClient() (*host.Client, error) {
	root, err := filepath.Abs(r.root)
	if err != nil {
		return nil, err
	}
	return host.New(host.Options{SysRoot: filepath.Join(root, "sys")})
}

func (r *Reader) ReadCardName() (string, bool, error) {
	c, err := r.metadataClient()
	if err != nil {
		return "", false, err
	}
	value, err := c.ReadCardName(context.Background())
	return value, value != "", err
}

func (r *Reader) ReadDriverVersion() (string, bool, error) {
	c, err := r.metadataClient()
	if err != nil {
		return "", false, err
	}
	value, err := c.ReadDriverVersion(context.Background())
	return value, value != "", err
}
