package repository

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"bodsch.me/nextcloud-ical-backup/internal/ncconfig"
)

const event = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:1\r\nSUMMARY:Meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
const card = "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:John Doe\r\nEND:VCARD\r\n"

func newTestRepo(t *testing.T) Repository {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nextcloud.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	schema := `
		CREATE TABLE oc_calendars (id INTEGER PRIMARY KEY, principaluri TEXT, uri TEXT, displayname TEXT, calendarcolor TEXT);
		CREATE TABLE oc_calendarobjects (id INTEGER PRIMARY KEY, calendarid INTEGER, calendardata BLOB, calendartype INTEGER DEFAULT 0, deleted_at INTEGER);
		CREATE TABLE oc_addressbooks (id INTEGER PRIMARY KEY, principaluri TEXT, uri TEXT, displayname TEXT);
		CREATE TABLE oc_cards (id INTEGER PRIMARY KEY, addressbookid INTEGER, carddata BLOB);
		INSERT INTO oc_calendars VALUES (1, 'principals/users/alice', 'personal', 'Personal', '#FF0000');
		INSERT INTO oc_calendars VALUES (2, 'principals/system/system', 'sys', 'System', NULL);
		INSERT INTO oc_addressbooks VALUES (1, 'principals/users/alice', 'contacts', 'Contacts');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO oc_calendarobjects VALUES (1, 1, ?, 0, NULL)", event); err != nil {
		t.Fatal(err)
	}
	// Subscription cache (calendartype=1) and trashed (deleted_at) must be ignored.
	db.Exec("INSERT INTO oc_calendarobjects VALUES (2, 1, ?, 1, NULL)", "BEGIN:VEVENT\nUID:cached\nEND:VEVENT\n")
	db.Exec("INSERT INTO oc_calendarobjects VALUES (3, 1, ?, 0, 12345)", "BEGIN:VEVENT\nUID:trashed\nEND:VEVENT\n")
	db.Exec("INSERT INTO oc_cards VALUES (1, 1, ?)", card)
	db.Close()

	cfg := &ncconfig.Config{
		DBType:        ncconfig.SQLite,
		DataDirectory: dir,
		DBName:        "nextcloud",
		DBTablePrefix: "oc_",
		Schema:        ncconfig.SchemaFromVersion("28.0.0"),
	}
	repo, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestListsOnlyUserCalendars(t *testing.T) {
	repo := newTestRepo(t)
	calendars, err := repo.ListCalendars()
	if err != nil {
		t.Fatal(err)
	}
	if len(calendars) != 1 || calendars[0].Username != "alice" {
		t.Fatalf("calendars = %+v (system principal must be excluded)", calendars)
	}
	if calendars[0].Color != "#FF0000" {
		t.Errorf("color = %q", calendars[0].Color)
	}
}

func TestCalendarObjectsFilterSubscriptionAndTrash(t *testing.T) {
	repo := newTestRepo(t)
	objects, err := repo.CalendarObjects(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || !strings.Contains(objects[0], "SUMMARY:Meeting") {
		t.Fatalf("objects = %v", objects)
	}
	if strings.Contains(objects[0], "cached") || strings.Contains(objects[0], "trashed") {
		t.Error("cached/trashed objects must be excluded")
	}
}

func TestCardsReturned(t *testing.T) {
	repo := newTestRepo(t)
	cards, err := repo.Cards(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || !strings.Contains(cards[0], "FN:John Doe") {
		t.Fatalf("cards = %v", cards)
	}
}

func TestMissingDatabaseErrors(t *testing.T) {
	cfg := &ncconfig.Config{
		DBType:        ncconfig.SQLite,
		DataDirectory: t.TempDir(),
		DBName:        "missing",
		DBTablePrefix: "oc_",
		Schema:        ncconfig.SchemaFromVersion("28.0.0"),
	}
	if _, err := Open(cfg); err == nil {
		t.Error("expected error for missing database file")
	}
}
