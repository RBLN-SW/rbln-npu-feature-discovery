package collector

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/pciids"
	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/sysfs"
)

const labelPrefix = "rebellions.ai"

type Features struct {
	NPUPresent            bool
	NPUCount              *int
	NPUProduct            *string
	DriverVersionFull     *string
	DriverVersionMajor    *string
	DriverVersionMinor    *string
	DriverVersionPatch    *string
	DriverVersionRevision *string
}

func newFeatures() Features {
	return Features{NPUPresent: false}
}

// labelField ties one label to its slog key and value accessor. toPlainText
// and logAttrs both iterate labelFields, so the published file and the log
// snapshot cannot drift apart.
type labelField struct {
	label string
	attr  string
	value func(Features) (any, bool)
}

func strField(label, attr string, get func(Features) *string) labelField {
	return labelField{label, attr, func(f Features) (any, bool) {
		if p := get(f); p != nil {
			return *p, true
		}
		return nil, false
	}}
}

var labelFields = []labelField{
	{"npu.present", "npuPresent", func(f Features) (any, bool) { return f.NPUPresent, true }},
	{"npu.count", "npuCount", func(f Features) (any, bool) {
		if f.NPUCount == nil {
			return nil, false
		}
		return *f.NPUCount, true
	}},
	strField("npu.product", "npuProduct", func(f Features) *string { return f.NPUProduct }),
	strField("driver-version.full", "driverVersionFull", func(f Features) *string { return f.DriverVersionFull }),
	strField("driver-version.major", "driverVersionMajor", func(f Features) *string { return f.DriverVersionMajor }),
	strField("driver-version.minor", "driverVersionMinor", func(f Features) *string { return f.DriverVersionMinor }),
	strField("driver-version.patch", "driverVersionPatch", func(f Features) *string { return f.DriverVersionPatch }),
	strField("driver-version.revision", "driverVersionRevision", func(f Features) *string { return f.DriverVersionRevision }),
}

func (f Features) toPlainText() string {
	var b strings.Builder
	for _, fld := range labelFields {
		if v, ok := fld.value(f); ok {
			fmt.Fprintf(&b, "%s/%s=%v\n", labelPrefix, fld.label, v)
		}
	}
	return b.String()
}

// logAttrs renders the label set as slog key-values so a log reader can
// reconstruct exactly what was published without access to the output file.
func (f Features) logAttrs() []any {
	var attrs []any
	for _, fld := range labelFields {
		if v, ok := fld.value(f); ok {
			attrs = append(attrs, fld.attr, v)
		}
	}
	return attrs
}

type FeaturesCollector struct {
	outputFile    string
	noTimestamp   bool
	pciIDs        *pciids.PCIIDsLookup
	lastPublished string
}

func NewFeaturesCollector(outputFile string, noTimestamp bool) *FeaturesCollector {
	c := &FeaturesCollector{
		outputFile:  outputFile,
		noTimestamp: noTimestamp,
	}
	path, err := pciids.FindPCIIDsPath()
	if err == nil {
		c.pciIDs, err = pciids.LoadRebellionsPCIIDs(path)
	}
	if err != nil {
		slog.Warn("Failed to load pci.ids", "err", err, "effect", "product fallback disabled")
	} else {
		slog.Info("Loaded pci.ids", "path", path, "entries", c.pciIDs.Len())
	}
	return c
}

func (c *FeaturesCollector) CollectOnce() error {
	features := newFeatures()

	if err := c.collectFromSysfs(&features); err != nil {
		return fmt.Errorf("collecting features from sysfs: %w", err)
	}

	if err := c.save(features); err != nil {
		return fmt.Errorf("saving features: %w", err)
	}

	return nil
}

func (c *FeaturesCollector) collectFromSysfs(features *Features) error {
	devices, err := sysfs.DiscoverDevices()
	if err != nil {
		return err
	}

	if len(devices) > 0 {
		features.NPUPresent = true
		features.NPUCount = ptr(len(devices))

		applyProduct(features, c.resolveProduct(devices[0].DeviceID))
	}

	return collectDriverVersion(features)
}

// readDriverVersion is a test seam over the fixed sysfs path.
var readDriverVersion = sysfs.ReadDriverVersion

func collectDriverVersion(features *Features) error {
	driverVersion, found, err := readDriverVersion()
	if err != nil {
		return err
	}
	if !found {
		slog.Warn("Driver version not found in sysfs", "effect", "driver-version labels omitted")
		return nil
	}

	semver, revision, major, minor, patch, err := parseDriverVersion(driverVersion)
	if err != nil {
		return err
	}

	features.DriverVersionFull = ptr(semver)
	features.DriverVersionMajor = ptr(major)
	features.DriverVersionMinor = ptr(minor)
	features.DriverVersionPatch = ptr(patch)
	if revision != nil {
		features.DriverVersionRevision = revision
	}

	return nil
}

// resolveProduct returns the product label ("RBLN-CR13") for a device:
// driver card_name first (authoritative, no per-SKU maintenance), bundled
// pci.ids second (covers old drivers without the card_name attribute).
func (c *FeaturesCollector) resolveProduct(deviceID string) string {
	name, found, err := sysfs.ReadCardName()
	if err != nil {
		slog.Warn("Failed to read card_name", "err", err, "effect", "falling back to pci.ids for product name")
	}
	if found {
		return name
	}
	if name := c.pciIDs.Lookup(deviceID); name != "" {
		return productLabelFromPCIIDsName(name)
	}
	return ""
}

// applyProduct sets NPUProduct. An unresolved product only costs this label —
// never the rest of the feature file — so a new SKU cannot expire
// npu.present/npu.count on the node.
func applyProduct(features *Features, product string) {
	if product == "" {
		slog.Warn("Could not resolve product name", "effect", "npu.product label omitted")
		return
	}
	features.NPUProduct = ptr(product)
}

func (c *FeaturesCollector) save(features Features) error {
	plain := features.toPlainText()
	text := plain

	if !c.noTimestamp {
		expiry := time.Now().Add(time.Hour).Format(time.RFC3339)
		text = fmt.Sprintf("# +expiry-time=%s\n%s", expiry, plain)
	}

	dir := filepath.Dir(c.outputFile)
	if dir == "" {
		return fmt.Errorf("invalid output file path: %s", c.outputFile)
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("output path validation failed: %w", err)
	}

	tempPath := filepath.Join(dir, "."+filepath.Base(c.outputFile))

	if err := os.WriteFile(tempPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("writing temp feature file: %w", err)
	}
	if err := os.Rename(tempPath, c.outputFile); err != nil {
		return fmt.Errorf("publishing feature file: %w", err)
	}

	// Snapshot on change only: keeps steady state quiet while making the
	// published label set reconstructable from the logs.
	if plain != c.lastPublished {
		c.lastPublished = plain
		slog.Info("Feature labels published", features.logAttrs()...)
	}

	slog.Debug("Features saved", "path", c.outputFile)
	return nil
}

func parseDriverVersion(raw string) (semver string, revision *string, major string, minor string, patch string, err error) {
	trimmed := strings.TrimSpace(raw)
	semver = trimmed

	if idx := strings.IndexAny(trimmed, "-+~"); idx != -1 {
		semver = trimmed[:idx]
		rev := trimmed[idx+1:]
		revision = &rev
	}

	parts := strings.Split(semver, ".")
	if len(parts) < 3 {
		err = fmt.Errorf("failed to split semver with dots: %s", semver)
		return
	}

	major, minor, patch = parts[0], parts[1], parts[2]
	return
}

func ptr[T any](v T) *T {
	return &v
}
