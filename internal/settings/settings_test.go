package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaults(t *testing.T) {
	s, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !s.IncludeCalendars || !s.IncludeAddressbooks || s.DryRun || len(s.Users) != 0 {
		t.Errorf("unexpected defaults: %+v", s)
	}
}

func TestFileOverlaysDefaults(t *testing.T) {
	s, err := Load(writeTOML(t, "backup_root = \"/from/file\"\nusers = [\"alice\"]\ndry_run = true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.BackupRoot != "/from/file" || len(s.Users) != 1 || s.Users[0] != "alice" || !s.DryRun {
		t.Errorf("file not applied: %+v", s)
	}
}

func TestBackupTableSection(t *testing.T) {
	s, err := Load(writeTOML(t, "[backup]\nocc_command = \"php occ\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.OccCommand != "php occ" {
		t.Errorf("[backup] table not honored: %q", s.OccCommand)
	}
}

func TestMissingFileErrors(t *testing.T) {
	if _, err := Load("/nonexistent/tool.toml"); err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestToFilter(t *testing.T) {
	s := Defaults()
	s.Users = []string{"alice"}
	s.Calendars = []string{"Personal"}
	f := s.ToFilter()
	if !f.MatchesUser("alice") || f.MatchesUser("bob") {
		t.Error("user filter wrong")
	}
	if !f.MatchesCalendar("Personal") || f.MatchesCalendar("Other") {
		t.Error("calendar filter wrong")
	}
	if !f.MatchesAddressbook("anything") {
		t.Error("empty addressbook filter must match all")
	}
}

func TestResolveOccCommand(t *testing.T) {
	// An explicit occ command always wins.
	s := Defaults()
	s.OccCommand = "php /custom/occ"
	if got, err := s.ResolveOccCommand(); err != nil || got != "php /custom/occ" {
		t.Errorf("explicit occ = %q, %v", got, err)
	}

	// nextcloud_path with an existing occ file -> "php <path>/occ".
	dir := t.TempDir()
	occ := filepath.Join(dir, "occ")
	if err := os.WriteFile(occ, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s = Defaults()
	s.NextcloudPath = dir
	if got, err := s.ResolveOccCommand(); err != nil || got != "php "+occ {
		t.Errorf("derived occ = %q, %v", got, err)
	}

	// nextcloud_path without an occ file -> snap fallback.
	s = Defaults()
	s.NextcloudPath = t.TempDir()
	if got, err := s.ResolveOccCommand(); err != nil || got != "nextcloud.occ" {
		t.Errorf("snap fallback = %q, %v", got, err)
	}

	// Neither configured -> error.
	if _, err := Defaults().ResolveOccCommand(); err == nil {
		t.Error("expected error when neither occ_command nor nextcloud_path is set")
	}
}

func TestResolveConfigPHP(t *testing.T) {
	s := Defaults()
	s.NextcloudPath = "/var/www/nextcloud"
	got, err := s.ResolveConfigPHP()
	if err != nil || got != filepath.Join("/var/www/nextcloud", "config", "config.php") {
		t.Errorf("resolve = %q, %v", got, err)
	}
	if _, err := Defaults().ResolveConfigPHP(); err == nil {
		t.Error("expected error when no source configured")
	}
}
