package sysfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCardName(t *testing.T) {
	dir := t.TempDir()
	rbln0 := filepath.Join(dir, "rbln0")
	if err := os.MkdirAll(rbln0, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rbln0, "card_name"), []byte("RBLN-CR13\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	name, found, err := readCardName(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found || name != "RBLN-CR13" {
		t.Fatalf("readCardName = (%q, %v), want (RBLN-CR13, true)", name, found)
	}
}

func TestReadCardNameMissingAttr(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rbln0"), 0o755); err != nil {
		t.Fatal(err)
	}
	name, found, err := readCardName(dir)
	if err != nil || found || name != "" {
		t.Fatalf("readCardName = (%q, %v, %v), want empty/false/nil", name, found, err)
	}
}

func TestReadCardNameMissingClassDir(t *testing.T) {
	name, found, err := readCardName(filepath.Join(t.TempDir(), "nope"))
	if err != nil || found || name != "" {
		t.Fatalf("readCardName = (%q, %v, %v), want empty/false/nil", name, found, err)
	}
}
