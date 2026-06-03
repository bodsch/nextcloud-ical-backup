// Package util provides small, dependency-free helpers used across the package.
package util

import (
	"os"
	"regexp"
	"strings"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|\s]+`)
	principalPrefixes    = []string{"principals/users/", "principals/system/", "principals/"}
)

// SanitizeFilename converts a display name into a filesystem-safe filename.
// It never returns an empty string (falls back to "unnamed").
func SanitizeFilename(value string) string {
	s := invalidFilenameChars.ReplaceAllString(strings.TrimSpace(value), "_")
	s = strings.Trim(s, "_.-")
	if s == "" {
		return "unnamed"
	}
	return s
}

// NormalizePrincipalURI resolves a Nextcloud principal URI to a bare username.
// Unknown formats are returned unchanged.
func NormalizePrincipalURI(principalURI string) string {
	for _, prefix := range principalPrefixes {
		if strings.HasPrefix(principalURI, prefix) {
			return principalURI[len(prefix):]
		}
	}
	return principalURI
}

// EnsureDir creates dir (and parents) if missing.
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o755) }

// NormalizeLines splits text into lines, stripping trailing carriage returns
// and the final empty element produced by a trailing newline. It mirrors
// Python's str.splitlines() closely enough for iCalendar/vCard data.
func NormalizeLines(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	if last := len(parts) - 1; parts[last] == "" {
		parts = parts[:last]
	}
	lines := make([]string, len(parts))
	for i, p := range parts {
		lines[i] = strings.TrimRight(p, "\r")
	}
	return lines
}

// WriteCRLFLines writes lines to path using RFC 5545/6350 CRLF separators.
func WriteCRLFLines(path string, lines []string) error {
	payload := strings.Join(lines, "\r\n")
	if payload != "" {
		payload += "\r\n"
	}
	return os.WriteFile(path, []byte(payload), 0o644)
}

// ExtractUID returns the component/card UID found outside of any VALARM
// component, unfolding RFC 5545/6350 line folding. It returns "" if no usable
// UID is present.
func ExtractUID(raw string) string {
	lines := NormalizeLines(raw)
	inValarm := false
	for i, line := range lines {
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "BEGIN:VALARM"):
			inValarm = true
			continue
		case strings.HasPrefix(upper, "END:VALARM"):
			inValarm = false
			continue
		}
		if inValarm || !strings.HasPrefix(upper, "UID:") {
			continue
		}
		uid := line[len("UID:"):]
		for _, cont := range lines[i+1:] {
			if strings.HasPrefix(cont, " ") || strings.HasPrefix(cont, "\t") {
				uid += cont[1:]
			} else {
				break
			}
		}
		return strings.TrimSpace(uid)
	}
	return ""
}
