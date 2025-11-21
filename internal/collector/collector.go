package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/daemon"
	"github.com/rebellions-sw/rbln-npu-feature-discovery/internal/sysfs"
)

const labelPrefix = "rebellions.ai"

type Features struct {
	NPUPresent            bool
	NPUCount              *int
	NPUFamily             *string
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

func (f Features) toPlainText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/npu.present=%t\n", labelPrefix, f.NPUPresent)
	if f.NPUCount != nil {
		fmt.Fprintf(&b, "%s/npu.count=%d\n", labelPrefix, *f.NPUCount)
	}
	if f.NPUFamily != nil {
		fmt.Fprintf(&b, "%s/npu.family=%s\n", labelPrefix, *f.NPUFamily)
	}
	if f.NPUProduct != nil {
		fmt.Fprintf(&b, "%s/npu.product=%s\n", labelPrefix, *f.NPUProduct)
	}
	if f.DriverVersionFull != nil {
		fmt.Fprintf(&b, "%s/driver-version.full=%s\n", labelPrefix, *f.DriverVersionFull)
	}
	if f.DriverVersionMajor != nil {
		fmt.Fprintf(&b, "%s/driver-version.major=%s\n", labelPrefix, *f.DriverVersionMajor)
	}
	if f.DriverVersionMinor != nil {
		fmt.Fprintf(&b, "%s/driver-version.minor=%s\n", labelPrefix, *f.DriverVersionMinor)
	}
	if f.DriverVersionPatch != nil {
		fmt.Fprintf(&b, "%s/driver-version.patch=%s\n", labelPrefix, *f.DriverVersionPatch)
	}
	if f.DriverVersionRevision != nil {
		fmt.Fprintf(&b, "%s/driver-version.revision=%s\n", labelPrefix, *f.DriverVersionRevision)
	}
	return b.String()
}

type FeaturesCollector struct {
	daemonURL   string
	outputFile  string
	noTimestamp bool
}

func NewFeaturesCollector(daemonURL, outputFile string, noTimestamp bool) *FeaturesCollector {
	return &FeaturesCollector{
		daemonURL:   daemonURL,
		outputFile:  outputFile,
		noTimestamp: noTimestamp,
	}
}

func (c *FeaturesCollector) CollectOnce(ctx context.Context) error {
	features := newFeatures()

	if err := c.collectFromDaemon(ctx, &features); err != nil {
		slog.Debug("failed to collect features from daemon, falling back to sysfs", "err", err)
		if sysfsErr := c.collectFromSysfs(&features); sysfsErr != nil {
			return fmt.Errorf("collecting features from sysfs: %w", sysfsErr)
		}
	}

	if err := c.save(features); err != nil {
		return fmt.Errorf("saving features: %w", err)
	}

	return nil
}

func (c *FeaturesCollector) collectFromDaemon(ctx context.Context, features *Features) error {
	client, err := daemon.NewClient(ctx, c.daemonURL)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			slog.Debug("failed to close daemon client", "err", closeErr)
		}
	}()

	devices, err := client.ServiceableDevices(ctx)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return nil
	}

	features.NPUPresent = true
	features.NPUCount = ptr(len(devices))

	product, err := productFromDeviceID(devices[0].GetDevId())
	if err != nil {
		return err
	}
	features.NPUProduct = ptr(product.FeatureString())

	family, err := product.Family()
	if err != nil {
		return err
	}
	features.NPUFamily = ptr(family)

	versionInfo, err := client.Version(ctx, devices[0])
	if err != nil {
		slog.Debug("failed to fetch driver version from daemon", "err", err)
		return nil
	}

	semver, revision, major, minor, patch, err := parseDriverVersion(versionInfo.GetDrvVersion())
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

func (c *FeaturesCollector) collectFromSysfs(features *Features) error {
	devices, err := sysfs.DiscoverDevices()
	if err != nil {
		return err
	}

	if len(devices) > 0 {
		features.NPUPresent = true
		features.NPUCount = ptr(len(devices))

		product, err := productFromDeviceID(devices[0].DeviceID)
		if err != nil {
			return err
		}
		features.NPUProduct = ptr(product.FeatureString())

		family, err := product.Family()
		if err != nil {
			return err
		}
		features.NPUFamily = ptr(family)
	}

	driverVersion, found, err := sysfs.ReadDriverVersion()
	if err != nil || !found {
		return err
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

func (c *FeaturesCollector) save(features Features) error {
	text := features.toPlainText()

	if !c.noTimestamp {
		expiry := time.Now().Add(time.Hour).Format(time.RFC3339)
		text = fmt.Sprintf("# +expiry-time=%s\n%s", expiry, text)
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

	slog.Debug("features saved", "path", c.outputFile)
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
