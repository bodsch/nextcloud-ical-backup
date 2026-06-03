// Package settings resolves effective configuration with the precedence:
//
//	built-in defaults  <  TOML configuration file  <  CLI parameters
//
// CLI overrides are applied by the caller after Load.
package settings

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
)

// Settings is the effective configuration for a backup or restore run.
type Settings struct {
	ConfigPHP           string   `toml:"config_php"`
	NextcloudPath       string   `toml:"nextcloud_path"`
	BackupRoot          string   `toml:"backup_root"`
	OccCommand          string   `toml:"occ_command"`
	Users               []string `toml:"users"`
	Calendars           []string `toml:"calendars"`
	Addressbooks        []string `toml:"addressbooks"`
	IncludeCalendars    bool     `toml:"include_calendars"`
	IncludeAddressbooks bool     `toml:"include_addressbooks"`
	Aggregate           bool     `toml:"aggregate"`
	DryRun              bool     `toml:"dry_run"`
	ListOnly            bool     `toml:"list_only"`
}

// Defaults returns the built-in default settings.
func Defaults() Settings {
	cwd, _ := os.Getwd()
	return Settings{
		BackupRoot:          filepath.Join(cwd, "backup"),
		IncludeCalendars:    true,
		IncludeAddressbooks: true,
	}
}

// Load returns the defaults overlaid with a TOML configuration file (if given).
// Both flat top-level keys and a [backup] table are supported; table values
// take precedence over their flat counterparts.
func Load(configFile string) (Settings, error) {
	s := Defaults()
	if configFile == "" {
		return s, nil
	}
	if _, err := os.Stat(configFile); err != nil {
		return s, fmt.Errorf("configuration file not found: %s", configFile)
	}
	// Flat top-level keys.
	if _, err := toml.DecodeFile(configFile, &s); err != nil {
		return s, fmt.Errorf("invalid configuration file %s: %w", configFile, err)
	}
	// Optional [backup] table overlays the flat values.
	wrap := struct {
		Backup Settings `toml:"backup"`
	}{Backup: s}
	if _, err := toml.DecodeFile(configFile, &wrap); err != nil {
		return s, fmt.Errorf("invalid configuration file %s: %w", configFile, err)
	}
	return wrap.Backup, nil
}

// ToFilter builds a domain.BackupFilter from the configured selections.
func (s Settings) ToFilter() domain.BackupFilter {
	return domain.BackupFilter{
		Users:               domain.NewStringSet(s.Users),
		Calendars:           domain.NewStringSet(s.Calendars),
		Addressbooks:        domain.NewStringSet(s.Addressbooks),
		IncludeCalendars:    s.IncludeCalendars,
		IncludeAddressbooks: s.IncludeAddressbooks,
	}
}

// ResolveConfigPHP resolves the path to Nextcloud's config.php.
func (s Settings) ResolveConfigPHP() (string, error) {
	switch {
	case s.ConfigPHP != "":
		return s.ConfigPHP, nil
	case s.NextcloudPath != "":
		return filepath.Join(s.NextcloudPath, "config", "config.php"), nil
	default:
		return "", fmt.Errorf("either config_php or nextcloud_path must be configured")
	}
}

// ResolveOccCommand resolves the occ command used for restore.
func (s Settings) ResolveOccCommand() (string, error) {
	if s.OccCommand != "" {
		return s.OccCommand, nil
	}
	if s.NextcloudPath != "" {
		occ := filepath.Join(s.NextcloudPath, "occ")
		if _, err := os.Stat(occ); err == nil {
			return "php " + occ, nil
		}
		return "nextcloud.occ", nil
	}
	return "", fmt.Errorf("either occ_command or nextcloud_path must be configured for restore")
}
