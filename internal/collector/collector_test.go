package collector

import "testing"

func TestApplyProduct(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "RBLN-CR13")
	if f.NPUProduct == nil || *f.NPUProduct != "RBLN-CR13" {
		t.Fatalf("NPUProduct = %v, want RBLN-CR13", f.NPUProduct)
	}
	if f.NPUFamily == nil || *f.NPUFamily != DeviceFamilyREBEL {
		t.Fatalf("NPUFamily = %v, want %s", f.NPUFamily, DeviceFamilyREBEL)
	}
}

func TestApplyProductEmptyIsNonFatal(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "")
	if f.NPUProduct != nil || f.NPUFamily != nil {
		t.Fatal("empty product must leave product/family labels unset")
	}
}

func TestApplyProductUnknownFamily(t *testing.T) {
	f := newFeatures()
	applyProduct(&f, "RBLN-XX99")
	if f.NPUProduct == nil || *f.NPUProduct != "RBLN-XX99" {
		t.Fatalf("NPUProduct = %v, want RBLN-XX99", f.NPUProduct)
	}
	if f.NPUFamily != nil {
		t.Fatal("unknown prefix must leave family label unset")
	}
}
