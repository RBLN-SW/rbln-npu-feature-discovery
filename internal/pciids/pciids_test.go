/*
Copyright 2026 Rebellions Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pciids

import (
	"strings"
	"testing"
)

const sample = `# comment
1eff  Rebellions Inc.
	1220  RBLN-CA22 (PF)
	1221  RBLN-CA22 (VF)
	2130  RBLN-CR13 (PF)
		aaaa bbbb  Some subsystem
1234  Other Vendor
	5678  Other Device
`

func TestParseVendorBlock(t *testing.T) {
	lookup, err := parsePCIIDsForVendor(strings.NewReader(sample), "1eff")
	if err != nil {
		t.Fatal(err)
	}
	if got := lookup.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
	cases := map[string]string{
		"1220":   "RBLN-CA22 (PF)",
		"0x1221": "RBLN-CA22 (VF)",
		"0X2130": "RBLN-CR13 (PF)",
		"5678":   "", // different vendor
		"9999":   "", // unknown
	}
	for id, want := range cases {
		if got := lookup.Lookup(id); got != want {
			t.Fatalf("Lookup(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestNilLookupIsEmpty(t *testing.T) {
	var lookup *PCIIDsLookup
	if got := lookup.Lookup("1220"); got != "" {
		t.Fatalf("nil lookup returned %q", got)
	}
}

func TestBundledRebellionsPCIIDs(t *testing.T) {
	lookup, err := LoadRebellionsPCIIDs("../../deps/rebellions-pci.ids")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"1020": "RBLN-CA02 (PF)",
		"1121": "RBLN-CA12 (VF)", // legacy map said CA02 — confirmed typo, VF of 1120 (CA12)
		"1150": "RBLN-CA15 (PF)",
		"2030": "RBLN-CR03 (PF)",
		"2130": "RBLN-CR13 (PF)",
		"2131": "RBLN-CR13 (VF)",
	}
	for id, want := range cases {
		if got := lookup.Lookup(id); got != want {
			t.Fatalf("Lookup(%q) = %q, want %q", id, got, want)
		}
	}
}
