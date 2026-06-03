package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
)

const icsBody = "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"
const vcfBody = "BEGIN:VCARD\r\nEND:VCARD\r\n"

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeAggregateTree(t *testing.T, root string) {
	mkdir(t, filepath.Join(root, "alice", "ics"))
	mkdir(t, filepath.Join(root, "alice", "vcf"))
	mkdir(t, filepath.Join(root, "bob", "ics"))
	writeFileT(t, filepath.Join(root, "alice", "ics", "Personal.ics"), icsBody)
	writeFileT(t, filepath.Join(root, "alice", "vcf", "Contacts.vcf"), vcfBody)
	writeFileT(t, filepath.Join(root, "bob", "ics", "Work.ics"), icsBody)
}

func makePerEntryTree(t *testing.T, root string) {
	personal := filepath.Join(root, "alice", "ics", "Personal")
	mkdir(t, personal)
	writeFileT(t, filepath.Join(personal, "evt-1.ics"), icsBody)
	writeFileT(t, filepath.Join(personal, "evt-2.ics"), icsBody)
}

// recordingRunner captures occ invocations instead of executing them.
type recordingRunner struct {
	calls [][]string
}

func (r *recordingRunner) run(args []string, _ []byte) (string, error) {
	r.calls = append(r.calls, args)
	if args[0] == "dav:list-calendars" {
		return "", nil // pretend no calendars exist yet
	}
	return "ok", nil
}

func (r *recordingRunner) commands() []string {
	cmds := make([]string, len(r.calls))
	for i, c := range r.calls {
		cmds[i] = c[0]
	}
	return cmds
}

func TestSelectedTargetsUserFilter(t *testing.T) {
	root := t.TempDir()
	makeAggregateTree(t, root)
	rr := &recordingRunner{}
	svc, _ := NewRestoreService("php occ", rr.run)
	f := domain.DefaultFilter()
	f.Users = domain.NewStringSet([]string{"alice"})
	targets, err := svc.SelectedTargets(root, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range targets {
		if tt.Username != "alice" {
			t.Errorf("unexpected user %q", tt.Username)
		}
	}
}

func TestRestoreAggregate(t *testing.T) {
	root := t.TempDir()
	makeAggregateTree(t, root)
	rr := &recordingRunner{}
	svc, _ := NewRestoreService("php occ", rr.run)
	report, err := svc.Restore(root, domain.DefaultFilter(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Calendars) != 2 || len(report.Skipped) != 1 {
		t.Errorf("calendars=%d skipped=%d", len(report.Calendars), len(report.Skipped))
	}
	cmds := strings.Join(rr.commands(), ",")
	if !strings.Contains(cmds, "calendar:import") || !strings.Contains(cmds, "dav:create-calendar") {
		t.Errorf("commands = %s", cmds)
	}
}

func TestRestorePerEntryImportsEachFile(t *testing.T) {
	root := t.TempDir()
	makePerEntryTree(t, root)
	rr := &recordingRunner{}
	svc, _ := NewRestoreService("php occ", rr.run)
	report, err := svc.Restore(root, domain.DefaultFilter(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Calendars) != 1 || !strings.Contains(report.Calendars[0], "Personal (2 file(s))") {
		t.Errorf("report = %v", report.Calendars)
	}
	imports, creates := 0, 0
	for _, c := range rr.commands() {
		switch c {
		case "calendar:import":
			imports++
		case "dav:create-calendar":
			creates++
		}
	}
	if imports != 2 || creates != 1 {
		t.Errorf("imports=%d creates=%d", imports, creates)
	}
}

func TestRestoreDryRunNoImport(t *testing.T) {
	root := t.TempDir()
	makeAggregateTree(t, root)
	rr := &recordingRunner{}
	svc, _ := NewRestoreService("php occ", rr.run)
	if _, err := svc.Restore(root, domain.DefaultFilter(), true); err != nil {
		t.Fatal(err)
	}
	for _, c := range rr.commands() {
		if c == "calendar:import" {
			t.Error("dry-run must not import")
		}
	}
}

func TestEmptyOccCommandRejected(t *testing.T) {
	if _, err := NewRestoreService("", nil); err == nil {
		t.Error("expected error for empty occ command")
	}
}

func TestMissingBackupRootErrors(t *testing.T) {
	svc, _ := NewRestoreService("php occ", (&recordingRunner{}).run)
	if _, err := svc.SelectedTargets(filepath.Join(t.TempDir(), "nope"), domain.DefaultFilter()); err == nil {
		t.Error("expected error for missing backup root")
	}
}
