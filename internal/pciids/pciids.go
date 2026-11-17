// Package pciids delegates PCI database interpretation to rblnlib-go.
package pciids

import (
	host "github.com/rbln-sw/rblnlib-go/pkg/host/v1"
	"io"
	"strings"
)

var defaultPCIIDsPaths []string

type PCIIDsLookup struct {
	source *host.PCIIDs
	Vendor string
}

func (p *PCIIDsLookup) Lookup(raw string) string {
	if p == nil {
		return ""
	}
	return p.source.LookupHex(raw)
}
func (p *PCIIDsLookup) Len() int {
	if p == nil {
		return 0
	}
	return p.source.Len()
}
func LoadPCIIDsForVendor(path, vendor string) (*PCIIDsLookup, error) {
	p, err := host.LoadVendorPCIIDs(path, vendor)
	if err != nil {
		return nil, err
	}
	return &PCIIDsLookup{source: p, Vendor: strings.ToLower(vendor)}, nil
}
func LoadRebellionsPCIIDs(path string) (*PCIIDsLookup, error) {
	return LoadPCIIDsForVendor(path, "1eff")
}
func FindPCIIDsPath() (string, error) {
	if defaultPCIIDsPaths != nil {
		return host.FindPCIIDsIn(defaultPCIIDsPaths)
	}
	return host.FindPCIIDsPath()
}
func parsePCIIDsForVendor(r io.Reader, vendor string) (*PCIIDsLookup, error) {
	p, err := host.ParseVendorPCIIDs(r, vendor)
	if err != nil {
		return nil, err
	}
	return &PCIIDsLookup{source: p, Vendor: strings.ToLower(vendor)}, nil
}
func normalizeDeviceID(raw string) string { return strings.TrimPrefix(strings.ToLower(raw), "0x") }
