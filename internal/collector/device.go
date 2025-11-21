package collector

import (
	"fmt"
	"strings"
)

type DeviceProduct string

const (
	DeviceFamilyATOM  = "ATOM"
	DeviceFamilyREBEL = "REBEL"
)

var deviceProductMap = map[string]DeviceProduct{
	"1020": "CA02",
	"1021": "CA02",
	"1120": "CA12",
	"1121": "CA02",
	"1150": "CA15",
	"1220": "CA22",
	"1221": "CA22",
	"1250": "CA25",
}

func productFromDeviceID(id string) (DeviceProduct, error) {
	p, ok := deviceProductMap[id]
	if !ok {
		return "", fmt.Errorf("unknown device id: %s", id)
	}
	return p, nil
}

func (p DeviceProduct) FeatureString() string {
	return fmt.Sprintf("RBLN-%s", string(p))
}

func (p DeviceProduct) Family() (string, error) {
	name := string(p)
	switch {
	case strings.HasPrefix(name, "CA"):
		return DeviceFamilyATOM, nil
	case strings.HasPrefix(name, "CR"):
		return DeviceFamilyREBEL, nil
	default:
		return "", fmt.Errorf("unknown product name: %s", name)
	}
}
