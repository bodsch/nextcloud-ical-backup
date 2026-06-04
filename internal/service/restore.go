package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
)

// OccRunner abstracts an occ invocation; injectable for testing.
type OccRunner func(args []string, stdin []byte) (string, error)

// RestoreTarget is a restorable calendar/addressbook discovered in the backup
// tree (one file in aggregate mode, one per entry in per-entry mode).
type RestoreTarget struct {
	Username string
	ItemType domain.ItemType
	Name     string
	Files    []string
}

// RestoreService imports backup files into Nextcloud through occ.
type RestoreService struct {
	occ []string
	run OccRunner
}

// NewRestoreService builds a RestoreService. occCommand is split on whitespace
// (e.g. "php /var/www/nextcloud/occ"). A nil runner uses a real subprocess.
func NewRestoreService(occCommand string, runner OccRunner) (*RestoreService, error) {
	if strings.TrimSpace(occCommand) == "" {
		return nil, fmt.Errorf("an occ command is required for restore operations")
	}
	s := &RestoreService{occ: strings.Fields(occCommand), run: runner}
	if s.run == nil {
		s.run = s.runSubprocess
	}
	return s, nil
}

// Restore imports the selected calendars from a backup tree. Addressbooks are
// reported as skipped, because stock Nextcloud occ has no contact import.
func (s *RestoreService) Restore(backupRoot string, f domain.BackupFilter, dryRun bool) (domain.BackupReport, error) {
	var report domain.BackupReport
	targets, err := s.SelectedTargets(backupRoot, f)
	if err != nil {
		return report, err
	}
	for _, t := range targets {
		label := fmt.Sprintf("%s/%s (%d file(s))", t.Username, t.Name, len(t.Files))
		if t.ItemType == domain.Addressbook {
			report.Skipped = append(report.Skipped, "addressbook: "+label)
			continue
		}
		if err := s.restoreCalendar(t, dryRun); err != nil {
			return report, err
		}
		report.Calendars = append(report.Calendars, "calendar: "+label)
	}
	return report, nil
}

// ListItems returns a human readable listing of restorable targets.
func (s *RestoreService) ListItems(backupRoot string, f domain.BackupFilter) ([]string, error) {
	targets, err := s.SelectedTargets(backupRoot, f)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(targets))
	for _, t := range targets {
		items = append(items, fmt.Sprintf("%s: %s/%s (%d file(s))", t.ItemType, t.Username, t.Name, len(t.Files)))
	}
	return items, nil
}

// SelectedTargets discovers and filters restorable targets under backupRoot.
func (s *RestoreService) SelectedTargets(backupRoot string, f domain.BackupFilter) ([]RestoreTarget, error) {
	all, err := discover(backupRoot)
	if err != nil {
		return nil, err
	}
	var selected []RestoreTarget
	for _, t := range all {
		if !f.MatchesUser(t.Username) {
			continue
		}
		if t.ItemType == domain.Calendar {
			if !f.IncludeCalendars || !f.MatchesCalendar(t.Name) {
				continue
			}
		} else if !f.IncludeAddressbooks || !f.MatchesAddressbook(t.Name) {
			continue
		}
		selected = append(selected, t)
	}
	return selected, nil
}

// layout describes one backup subtree (ics/vcf) and the item type it holds.
type layout struct {
	sub      string
	itemType domain.ItemType
	suffix   string
}

// discover walks the backup tree and returns every restorable target, detecting
// both the aggregate (single file) and per-entry (subdirectory) layouts.
func discover(backupRoot string) ([]RestoreTarget, error) {
	info, err := os.Stat(backupRoot)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("backup root not found: %s", backupRoot)
	}
	userDirs, err := os.ReadDir(backupRoot)
	if err != nil {
		return nil, err
	}

	var targets []RestoreTarget
	layouts := []layout{
		{"ics", domain.Calendar, ".ics"},
		{"vcf", domain.Addressbook, ".vcf"},
	}
	for _, user := range userDirs {
		if !user.IsDir() {
			continue
		}
		for _, l := range layouts {
			itemDir := filepath.Join(backupRoot, user.Name(), l.sub)
			entries, err := os.ReadDir(itemDir)
			if err != nil {
				continue
			}
			// Aggregate layout: a single file directly under ics/ or vcf/.
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), l.suffix) {
					targets = append(targets, RestoreTarget{
						Username: user.Name(),
						ItemType: l.itemType,
						Name:     strings.TrimSuffix(e.Name(), l.suffix),
						Files:    []string{filepath.Join(itemDir, e.Name())},
					})
				}
			}
			// Per-entry layout: one subdirectory per calendar/addressbook.
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				files := filesWithSuffix(filepath.Join(itemDir, e.Name()), l.suffix)
				if len(files) > 0 {
					targets = append(targets, RestoreTarget{
						Username: user.Name(),
						ItemType: l.itemType,
						Name:     e.Name(),
						Files:    files,
					})
				}
			}
		}
	}
	return targets, nil
}

// filesWithSuffix returns the sorted regular files in dir ending with suffix.
func filesWithSuffix(dir, suffix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files
}

// restoreCalendar ensures the target calendar exists and imports each of its
// backup files via occ. It is a no-op in dry-run mode.
func (s *RestoreService) restoreCalendar(t RestoreTarget, dryRun bool) error {
	if dryRun {
		return nil
	}
	calendarURI, err := s.ensureCalendar(t.Username, t.Name)
	if err != nil {
		return err
	}
	for _, file := range t.Files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := s.run([]string{"calendar:import", "--format", "ical", t.Username, calendarURI}, data); err != nil {
			return err
		}
	}
	return nil
}

// calendarRow is one entry parsed from `occ dav:list-calendars` output.
type calendarRow struct {
	uri         string
	displayName string
}

// ensureCalendar resolves the calendar URI to import into, creating the
// calendar first if no calendar with a matching URI or display name exists
// yet. It returns the URI (not the display name): calendar:import expects the
// URI, and an existing calendar's URI often differs from its display name
// (e.g. display name "Personal" has URI "personal").
func (s *RestoreService) ensureCalendar(username, wanted string) (string, error) {
	existing, err := s.run([]string{"dav:list-calendars", username, "--no-ansi"}, nil)
	if err != nil {
		return "", err
	}
	for _, c := range parseCalendars(existing) {
		if c.uri == wanted || c.displayName == wanted {
			return c.uri, nil
		}
	}
	// Not present yet: create it. dav:create-calendar uses the given name as
	// the calendar URI, so that same name is the URI to import into.
	if _, err := s.run([]string{"dav:create-calendar", username, wanted}, nil); err != nil {
		return "", err
	}
	return wanted, nil
}

// parseCalendars extracts calendars from the table printed by
// `occ dav:list-calendars`. The columns are, in order: URI, Displayname,
// Owner principal, Owner displayname, Writable. Header, separator (+---+) and
// informational lines are ignored.
func parseCalendars(output string) []calendarRow {
	var rows []calendarRow
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		// Split on "|"; cols[0] is the empty text before the first border,
		// so the real columns start at index 1. Need at least URI + name.
		cols := strings.Split(line, "|")
		if len(cols) < 3 {
			continue
		}
		uri := strings.TrimSpace(cols[1])
		if uri == "" || uri == "URI" { // skip empty rows and the header
			continue
		}
		rows = append(rows, calendarRow{uri: uri, displayName: strings.TrimSpace(cols[2])})
	}
	return rows
}

// runSubprocess is the default OccRunner: it executes the configured occ
// command with the given arguments and optional stdin, returning its stdout.
func (s *RestoreService) runSubprocess(args []string, stdin []byte) (string, error) {
	full := append(append([]string{}, s.occ[1:]...), args...)
	cmd := exec.Command(s.occ[0], full...)
	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return "", fmt.Errorf("occ command failed (%s %s):\n%s", s.occ[0], strings.Join(full, " "), stderr)
	}
	return string(out), nil
}
