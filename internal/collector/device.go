package collector

import (
	"fmt"
	"strings"
)

const (
	DeviceFamilyATOM  = "ATOM"
	DeviceFamilyREBEL = "REBEL"

	productPrefix = "RBLN-"
)

// familyFromProduct derives the device family from a product name such as
// "RBLN-CA25" (the RBLN- prefix is optional): CA* -> ATOM, CR* -> REBEL.
func familyFromProduct(product string) (string, error) {
	name := strings.TrimPrefix(product, productPrefix)
	switch {
	case strings.HasPrefix(name, "CA"):
		return DeviceFamilyATOM, nil
	case strings.HasPrefix(name, "CR"):
		return DeviceFamilyREBEL, nil
	default:
		return "", fmt.Errorf("unknown product name: %s", product)
	}
}

// productLabelFromPCIIDsName strips the " (PF)" / " (VF)" suffix pci.ids
// entries carry — parentheses and spaces are not valid in label values.
func productLabelFromPCIIDsName(name string) string {
	if i := strings.IndexByte(name, ' '); i > 0 {
		return name[:i]
	}
	return name
}
