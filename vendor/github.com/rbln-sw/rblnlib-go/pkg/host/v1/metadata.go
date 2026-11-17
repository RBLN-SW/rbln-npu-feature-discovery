package host

import (
	"context"
	"errors"
	system "github.com/rbln-sw/rblnlib-go/pkg/system/v1"
	"os"
	"path/filepath"
	"strings"
)

// Native metadata can be read with just the driver class mounted, without a
// complete PCI inventory. No function number (including rbln0) is required.
func (c *Client) nativeAttribute(ctx context.Context, attribute string, skipUnreadable bool) (string, error) {
	for _, class := range []string{"accel", "rebellions"} {
		root := c.classRoot(class)
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", failure("native metadata", root, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if entry.Name() == "rsd" || strings.HasPrefix(entry.Name(), "rblnfs") {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			if class == "accel" {
				vendor, err := readHex(filepath.Join(dir, "device/vendor"), 16)
				if err != nil || vendor != uint64(system.VendorID) {
					continue
				}
			}
			value, err := readText(filepath.Join(dir, attribute))
			if errors.Is(err, system.ErrNotFound) || (skipUnreadable && err != nil) {
				continue
			}
			if err != nil {
				return "", err
			}
			if value != "" {
				return value, nil
			}
		}
	}
	return "", nil
}

func (c *Client) ReadCardName(ctx context.Context) (string, error) {
	return c.nativeAttribute(ctx, "card_name", true)
}
func (c *Client) ReadDriverVersion(ctx context.Context) (string, error) {
	return c.nativeAttribute(ctx, "kernel_version", false)
}
