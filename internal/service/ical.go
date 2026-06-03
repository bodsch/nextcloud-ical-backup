package service

import (
	"strings"

	"bodsch.me/nextcloud-ical-backup/internal/util"
)

const prodID = "PRODID:-//nextcloud-ical-backup//iCal Export//EN"

// BuildICal assembles one RFC 5545 calendar from the raw calendar objects of a
// single calendar. Calendar-level properties of each embedded object are
// dropped, only components are kept, and each distinct VTIMEZONE is collected
// once and appended at the end.
func BuildICal(displayName, color string, rawObjects []string) []string {
	var components, timezones []string
	seen := map[string]struct{}{}
	current := "" // name of the current top-level component, "" = none
	inVTimezone := false
	var tzBuffer []string
	tzDuplicate := false

	consume := func(line string) {
		if line == "BEGIN:VCALENDAR" || line == "END:VCALENDAR" {
			return
		}
		if inVTimezone {
			tzBuffer = append(tzBuffer, line)
			if strings.HasPrefix(line, "TZID:") {
				tzid := line[len("TZID:"):]
				if _, ok := seen[tzid]; ok {
					tzDuplicate = true
				} else {
					seen[tzid] = struct{}{}
				}
			}
			if strings.EqualFold(line, "END:VTIMEZONE") {
				if !tzDuplicate {
					timezones = append(timezones, tzBuffer...)
				}
				inVTimezone = false
				tzBuffer = nil
				tzDuplicate = false
			}
			return
		}
		if current == "" {
			if !strings.HasPrefix(line, "BEGIN:") {
				return // calendar-level property of the embedded object: drop it
			}
			name := line[len("BEGIN:"):]
			if strings.EqualFold(name, "VTIMEZONE") {
				inVTimezone = true
				tzBuffer = []string{line}
				tzDuplicate = false
				return
			}
			current = name
			components = append(components, line)
			return
		}
		// Inside a top-level component: keep every line verbatim (this also
		// preserves nested components such as VALARM / AVAILABLE).
		components = append(components, line)
		if line == "END:"+current {
			current = ""
		}
	}

	for _, raw := range rawObjects {
		for _, line := range util.NormalizeLines(raw) {
			consume(line)
		}
	}

	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		prodID,
		"CALSCALE:GREGORIAN",
		"X-WR-CALNAME:" + displayName,
	}
	if color != "" {
		lines = append(lines, "X-APPLE-CALENDAR-COLOR:"+color)
	}
	lines = append(lines, components...)
	lines = append(lines, timezones...)
	lines = append(lines, "END:VCALENDAR")
	return lines
}
