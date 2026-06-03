package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
)

const fakeEvent = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:1\r\nSUMMARY:Meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
const fakeCard = "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:John Doe\r\nEND:VCARD\r\n"

// fakeRepository implements repository.Repository for service tests.
type fakeRepository struct {
	calendarObjects map[int64][]string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{calendarObjects: map[int64][]string{1: {fakeEvent}, 3: {fakeEvent}}}
}

func (f *fakeRepository) ListCalendars() ([]domain.CalendarItem, error) {
	return []domain.CalendarItem{
		{ID: 1, Username: "alice", URI: "personal", DisplayName: "Personal", Color: "#FF0000"},
		{ID: 2, Username: "alice", URI: "contact_birthdays", DisplayName: "Geburtstage"},
		{ID: 3, Username: "bob", URI: "work", DisplayName: "Work"},
	}, nil
}

func (f *fakeRepository) ListAddressbooks() ([]domain.AddressbookItem, error) {
	return []domain.AddressbookItem{{ID: 1, Username: "alice", URI: "contacts", DisplayName: "Contacts"}}, nil
}

func (f *fakeRepository) CalendarObjects(id int64) ([]string, error) {
	return f.calendarObjects[id], nil
}
func (f *fakeRepository) Cards(id int64) ([]string, error) {
	if id == 1 {
		return []string{fakeCard}, nil
	}
	return nil, nil
}
func (f *fakeRepository) Close() error { return nil }

func names(items []domain.CalendarItem) []string {
	out := make([]string, len(items))
	for i, c := range items {
		out[i] = c.DisplayName
	}
	sort.Strings(out)
	return out
}

func TestContactBirthdaysExcluded(t *testing.T) {
	svc := NewBackupService(newFakeRepository())
	got, _ := svc.SelectedCalendars(domain.DefaultFilter())
	if n := names(got); len(n) != 2 || n[0] != "Personal" || n[1] != "Work" {
		t.Errorf("selected = %v (Geburtstage must be excluded)", n)
	}
}

func TestUserFilterApplies(t *testing.T) {
	svc := NewBackupService(newFakeRepository())
	f := domain.DefaultFilter()
	f.Users = domain.NewStringSet([]string{"bob"})
	got, _ := svc.SelectedCalendars(f)
	if len(got) != 1 || got[0].DisplayName != "Work" {
		t.Errorf("selected = %v", got)
	}
}

func TestPerEntryExport(t *testing.T) {
	root := t.TempDir()
	svc := NewBackupService(newFakeRepository())
	report, err := svc.Export(root, domain.DefaultFilter(), ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Total() != 3 {
		t.Errorf("total = %d", report.Total())
	}
	mustExist(t, filepath.Join(root, "alice", "ics", "Personal", "1.ics"))
	mustExist(t, filepath.Join(root, "bob", "ics", "Work", "1.ics"))
	// Contact has no UID => index fallback.
	mustExist(t, filepath.Join(root, "alice", "vcf", "Contacts", "item-1.vcf"))
	if _, err := os.Stat(filepath.Join(root, "alice", "ics", "Geburtstage")); err == nil {
		t.Error("excluded calendar must not create a directory")
	}
	data, _ := os.ReadFile(filepath.Join(root, "alice", "ics", "Personal", "1.ics"))
	if !strings.HasPrefix(string(data), "BEGIN:VCALENDAR\r\n") || !strings.Contains(string(data), "SUMMARY:Meeting\r\n") {
		t.Errorf("unexpected per-entry content: %q", data)
	}
}

func TestAggregateExport(t *testing.T) {
	root := t.TempDir()
	svc := NewBackupService(newFakeRepository())
	if _, err := svc.Export(root, domain.DefaultFilter(), ExportOptions{Aggregate: true}); err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(root, "alice", "ics", "Personal.ics")
	mustExist(t, personal)
	data, _ := os.ReadFile(personal)
	if !strings.Contains(string(data), "X-APPLE-CALENDAR-COLOR:#FF0000\r\n") {
		t.Error("aggregate calendar must contain the color")
	}
	if _, err := os.Stat(filepath.Join(root, "alice", "ics", "Personal")); err == nil {
		t.Error("aggregate mode must not create a per-entry directory")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	svc := NewBackupService(newFakeRepository())
	report, _ := svc.Export(root, domain.DefaultFilter(), ExportOptions{DryRun: true})
	if report.Total() != 3 {
		t.Errorf("dry-run total = %d", report.Total())
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("dry-run wrote %d entries", len(entries))
	}
}

func TestDuplicateUIDsDeduplicated(t *testing.T) {
	root := t.TempDir()
	repo := newFakeRepository()
	event := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:same\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	repo.calendarObjects[1] = []string{event, event}
	svc := NewBackupService(repo)
	f := domain.DefaultFilter()
	f.Users = domain.NewStringSet([]string{"alice"})
	f.IncludeAddressbooks = false
	if _, err := svc.Export(root, f, ExportOptions{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "alice", "ics", "Personal"))
	got := []string{}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "same.ics" || got[1] != "same_2.ics" {
		t.Errorf("dedup names = %v", got)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file %s: %v", path, err)
	}
}
