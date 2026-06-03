package service

import (
	"slices"
	"testing"
)

const eventRaw = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Nextcloud//EN\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\nEND:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\nUID:event-1\r\nSUMMARY:Meeting\r\n" +
	"BEGIN:VALARM\r\nACTION:DISPLAY\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

const secondEventRaw = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\nEND:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\nUID:event-2\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func count(lines []string, value string) int {
	n := 0
	for _, l := range lines {
		if l == value {
			n++
		}
	}
	return n
}

func TestBuildICalDropsCalendarLevelProperties(t *testing.T) {
	lines := BuildICal("Personal", "", []string{eventRaw})
	if count(lines, "VERSION:2.0") != 1 {
		t.Errorf("VERSION must appear exactly once, got %d", count(lines, "VERSION:2.0"))
	}
	prods := 0
	for _, l := range lines {
		if len(l) >= 7 && l[:7] == "PRODID:" {
			prods++
		}
	}
	if prods != 1 {
		t.Errorf("embedded PRODID leaked, got %d PRODID lines", prods)
	}
}

func TestBuildICalKeepsNestedComponents(t *testing.T) {
	lines := BuildICal("Personal", "", []string{eventRaw})
	if !slices.Contains(lines, "BEGIN:VALARM") || !slices.Contains(lines, "END:VALARM") {
		t.Error("nested VALARM not preserved")
	}
	if slices.Index(lines, "END:VEVENT") > slices.Index(lines, "BEGIN:VTIMEZONE") {
		t.Error("VEVENT must close before the timezone block")
	}
}

func TestBuildICalDeduplicatesVTimezone(t *testing.T) {
	lines := BuildICal("Personal", "", []string{eventRaw, secondEventRaw})
	if count(lines, "BEGIN:VTIMEZONE") != 1 || count(lines, "TZID:Europe/Berlin") != 1 {
		t.Error("VTIMEZONE was not deduplicated")
	}
	if !slices.Contains(lines, "UID:event-1") || !slices.Contains(lines, "UID:event-2") {
		t.Error("both events must survive")
	}
	if lines[len(lines)-1] != "END:VCALENDAR" {
		t.Error("calendar must end with END:VCALENDAR")
	}
}

func TestBuildICalEmitsColor(t *testing.T) {
	lines := BuildICal("Personal", "#FF0000", []string{eventRaw})
	if !slices.Contains(lines, "X-APPLE-CALENDAR-COLOR:#FF0000") {
		t.Error("color not emitted")
	}
}
