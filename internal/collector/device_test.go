package collector

import "testing"

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
