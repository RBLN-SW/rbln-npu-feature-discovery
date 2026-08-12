package collector

import "testing"

func TestFamilyFromProduct(t *testing.T) {
	cases := []struct {
		product string
		want    string
	}{
		{"RBLN-CA25", DeviceFamilyATOM},
		{"RBLN-CR13", DeviceFamilyREBEL},
		{"CA02", DeviceFamilyATOM},
	}
	for _, tc := range cases {
		t.Run(tc.product, func(t *testing.T) {
			got, err := familyFromProduct(tc.product)
			if err != nil {
				t.Fatalf("familyFromProduct(%q) error: %v", tc.product, err)
			}
			if got != tc.want {
				t.Fatalf("familyFromProduct(%q) = %q, want %q", tc.product, got, tc.want)
			}
		})
	}
}

func TestFamilyFromProductUnknown(t *testing.T) {
	if _, err := familyFromProduct("RBLN-XX99"); err == nil {
		t.Fatal("expected error for unknown product prefix")
	}
}

func TestProductLabelFromPCIIDsName(t *testing.T) {
	cases := map[string]string{
		"RBLN-CA22 (PF)": "RBLN-CA22",
		"RBLN-CR13 (VF)": "RBLN-CR13",
		"RBLN-CA25":      "RBLN-CA25",
	}
	for in, want := range cases {
		if got := productLabelFromPCIIDsName(in); got != want {
			t.Fatalf("productLabelFromPCIIDsName(%q) = %q, want %q", in, got, want)
		}
	}
}
