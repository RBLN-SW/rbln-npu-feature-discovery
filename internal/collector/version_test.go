package collector

import "testing"

func TestParseDriverVersion(t *testing.T) {
	for _, tc := range []struct {
		name, raw, full, major, minor, patch string
		revision                             *string
		wantErr                              bool
	}{
		{name: "release", raw: "3.4.0", full: "3.4.0", major: "3", minor: "4", patch: "0"},
		{name: "whitespace", raw: " \t3.4.0\n", full: "3.4.0", major: "3", minor: "4", patch: "0"},
		{name: "revision", raw: "1.2.3-rebel1", full: "1.2.3", major: "1", minor: "2", patch: "3", revision: ptr("rebel1")},
		{name: "release candidate", raw: "3.5.0-rc3", full: "3.5.0", major: "3", minor: "5", patch: "0", revision: ptr("rc3")},
		{name: "build metadata", raw: "1.20.300+abc123", full: "1.20.300", major: "1", minor: "20", patch: "300", revision: ptr("abc123")},
		{name: "tilde revision", raw: "1.2.3~rc1", full: "1.2.3", major: "1", minor: "2", patch: "3", revision: ptr("rc1")},
		{name: "revision with separators", raw: "1.2.3-rc1-build2", full: "1.2.3", major: "1", minor: "2", patch: "3", revision: ptr("rc1-build2")},
		{name: "empty", wantErr: true},
		{name: "blank", raw: " \n", wantErr: true},
		{name: "major only", raw: "3", wantErr: true},
		{name: "patch missing", raw: "3.4", wantErr: true},
		{name: "patch missing with revision", raw: "3.4-rc1", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			full, revision, major, minor, patch, err := parseDriverVersion(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("accepted malformed version %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if full != tc.full || major != tc.major || minor != tc.minor || patch != tc.patch {
				t.Fatalf("parsed %q as %q / %q.%q.%q", tc.raw, full, major, minor, patch)
			}
			if (revision == nil) != (tc.revision == nil) || (revision != nil && *revision != *tc.revision) {
				t.Fatalf("revision=%v, want %v", revision, tc.revision)
			}
		})
	}
}
