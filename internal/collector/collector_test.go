package collector

import "testing"

func TestApplyProduct(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "RBLN-CR13")
	if f.NPUProduct == nil || *f.NPUProduct != "RBLN-CR13" {
		t.Fatalf("NPUProduct = %v, want RBLN-CR13", f.NPUProduct)
	}
}

func TestApplyProductEmptyIsNonFatal(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "")
	if f.NPUProduct != nil {
		t.Fatal("empty product must leave the product label unset")
	}
}
