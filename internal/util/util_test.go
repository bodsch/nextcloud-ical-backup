package util

import "testing"

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"Work Calendar":       "Work_Calendar",
		`a/b\c:d*e?f"g<h>i|j`: "a_b_c_d_e_f_g_h_i_j",
		"   ":                 "unnamed",
		"...":                 "unnamed",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePrincipalURI(t *testing.T) {
	cases := map[string]string{
		"principals/users/alice":   "alice",
		"principals/system/system": "system",
		"principals/bob":           "bob",
		"carol":                    "carol",
	}
	for in, want := range cases {
		if got := NormalizePrincipalURI(in); got != want {
			t.Errorf("NormalizePrincipalURI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeLines(t *testing.T) {
	got := NormalizeLines("a\r\nb\r\n")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("NormalizeLines = %#v", got)
	}
	if NormalizeLines("") != nil {
		t.Fatalf("empty input should yield nil")
	}
}

func TestExtractUID(t *testing.T) {
	if got := ExtractUID("BEGIN:VEVENT\r\nUID:abc-123\r\nEND:VEVENT\r\n"); got != "abc-123" {
		t.Errorf("simple UID = %q", got)
	}
	withAlarm := "BEGIN:VEVENT\r\nBEGIN:VALARM\r\nUID:alarm\r\nEND:VALARM\r\nUID:real\r\nEND:VEVENT\r\n"
	if got := ExtractUID(withAlarm); got != "real" {
		t.Errorf("VALARM UID should be ignored, got %q", got)
	}
	folded := "BEGIN:VEVENT\r\nUID:long-\r\n value-part\r\nEND:VEVENT\r\n"
	if got := ExtractUID(folded); got != "long-value-part" {
		t.Errorf("folded UID = %q", got)
	}
	if got := ExtractUID("BEGIN:VCARD\r\nFN:John\r\nEND:VCARD\r\n"); got != "" {
		t.Errorf("missing UID should be empty, got %q", got)
	}
}
