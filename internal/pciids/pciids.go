// Package pciids looks up a single PCI vendor's device names from the
// standard pci.ids database (https://pci-ids.ucw.cz/). Ported from
// sandbox-device-plugin's pkg/device_plugin/pciids.go so both projects
// resolve Rebellions device ids the same way. The file is bundled into the
// container image at build time, so production never depends on the host
// having `pciutils` installed.
package pciids

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// rebellionsVendorHex is the Rebellions Inc. PCI vendor id.
const rebellionsVendorHex = "1eff"

// defaultPCIIDsPaths lists the canonical pci.ids locations across distros,
// in the order pciutils-shipping packages place them. The first readable
// path wins. The container image puts the file at the Debian/Ubuntu path.
var defaultPCIIDsPaths = []string{
	"/usr/share/misc/pci.ids",   // Debian/Ubuntu (also the in-image path)
	"/usr/share/hwdata/pci.ids", // RHEL/Fedora
	"/var/lib/pciutils/pci.ids", // update-pciids destination on some distros
}

// PCIIDsLookup is a vendor-scoped device-name lookup populated from a
// pci.ids file. A nil receiver or an empty map means "no entries" — the
// caller decides how to fall back.
type PCIIDsLookup struct {
	Vendor      string            // lowercase 4-hex, e.g. "1eff"
	byDeviceHex map[string]string // "1220" -> "RBLN-CA22 (PF)"
}

// Lookup returns the device name for a raw sysfs device ID such as
// "0x1220", "0X1220", or "1220". Empty means the ID is not in the loaded
// vendor block.
func (p *PCIIDsLookup) Lookup(rawDeviceID string) string {
	if p == nil || len(p.byDeviceHex) == 0 {
		return ""
	}
	return p.byDeviceHex[normalizeDeviceID(rawDeviceID)]
}

// Len exposes the number of device entries loaded — useful for tests and
// for startup logging ("loaded N entries for vendor 1eff").
func (p *PCIIDsLookup) Len() int {
	if p == nil {
		return 0
	}
	return len(p.byDeviceHex)
}

// LoadPCIIDsForVendor reads pci.ids at path and returns a lookup scoped
// to vendor (lowercase 4-hex). Missing vendor in the file is not an error
// — the returned lookup is simply empty.
func LoadPCIIDsForVendor(path, vendor string) (*PCIIDsLookup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// Read-only file: nothing actionable in a Close error.
	defer func() { _ = f.Close() }()
	return parsePCIIDsForVendor(f, vendor)
}

// LoadRebellionsPCIIDs loads the vendor 1eff (Rebellions) block from pci.ids
// at path, so callers need not repeat the vendor constant.
func LoadRebellionsPCIIDs(path string) (*PCIIDsLookup, error) {
	return LoadPCIIDsForVendor(path, rebellionsVendorHex)
}

// FindPCIIDsPath returns the first readable path in defaultPCIIDsPaths.
// Callers that have an explicit path (env override, test fixture) should
// use that instead and bypass discovery.
func FindPCIIDsPath() (string, error) {
	for _, p := range defaultPCIIDsPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("pci.ids not found in any of %v", defaultPCIIDsPaths)
}

// parsePCIIDsForVendor walks pci.ids until the requested vendor section,
// then reads tab-indented device lines until the next non-tabbed line
// (which begins a different vendor) or EOF. See pci.ids(5).
//
// pci.ids structure (relevant excerpt):
//
//	# comment
//	1eff  Rebellions Inc.
//	\t1220  RBLN-CA22 (PF)               ← single-tab device entry
//	\t1221  RBLN-CA22 (VF)
//	\t\t<subsysv> <subsysd>  Subsystem    ← double-tab, ignored
func parsePCIIDsForVendor(r io.Reader, vendor string) (*PCIIDsLookup, error) {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	if len(vendor) != 4 {
		return nil, fmt.Errorf("vendor %q must be 4 hex digits", vendor)
	}

	lookup := &PCIIDsLookup{
		Vendor:      vendor,
		byDeviceHex: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)
	// pci.ids has long subsystem lines; bump the limit comfortably above
	// the largest line we have ever seen (~200B) to be safe.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inVendor := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Non-tabbed line = vendor header. Entering / leaving target block.
		if !strings.HasPrefix(line, "\t") {
			if inVendor {
				return lookup, nil // exited target vendor block
			}
			fields := strings.Fields(line)
			if len(fields) >= 1 && strings.EqualFold(fields[0], vendor) {
				inVendor = true
			}
			continue
		}

		if !inVendor {
			continue
		}

		// Inside the vendor block. Single-tab = device entry. Double-tab =
		// subsystem entry, which we deliberately ignore — we only need the
		// device-level name, and subsystem entries would otherwise overwrite
		// device names when their hex prefix happens to collide.
		if strings.HasPrefix(line, "\t\t") {
			continue
		}

		entry := strings.TrimPrefix(line, "\t")
		// Trailing "# comment" is rare in pci.ids but legal; strip it so it
		// never ends up inside the product name.
		if i := strings.Index(entry, "#"); i >= 0 {
			entry = entry[:i]
		}
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// "1220  RBLN-CA22 (PF)" — first whitespace-separated token is the
		// device hex, the remainder (after one or more spaces) is the name.
		idx := strings.IndexFunc(entry, func(r rune) bool {
			return r == ' ' || r == '\t'
		})
		if idx <= 0 {
			continue // malformed entry, skip silently
		}
		deviceHex := strings.ToLower(entry[:idx])
		name := strings.TrimSpace(entry[idx:])
		if deviceHex != "" && name != "" {
			lookup.byDeviceHex[deviceHex] = name
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pci.ids: %w", err)
	}
	return lookup, nil
}

// normalizeDeviceID strips an optional "0x" / "0X" prefix and lowercases
// the remainder so lookups are tolerant of every sysfs spelling we have
// seen in the wild ("0x1221", "0X1EFF", "1221").
func normalizeDeviceID(raw string) string {
	s := raw
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
