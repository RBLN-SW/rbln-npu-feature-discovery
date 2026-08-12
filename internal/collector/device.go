package collector

import (
	"strings"
)

// productLabelFromPCIIDsName strips the " (PF)" / " (VF)" suffix pci.ids
// entries carry — parentheses and spaces are not valid in label values.
func productLabelFromPCIIDsName(name string) string {
	if i := strings.IndexByte(name, ' '); i > 0 {
		return name[:i]
	}
	return name
}
