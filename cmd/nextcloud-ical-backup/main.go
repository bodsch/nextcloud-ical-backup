// Command nextcloud-ical-backup backs up and restores Nextcloud calendars and
// addressbooks.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bodsch.me/nextcloud-ical-backup/internal/ncconfig"
	"bodsch.me/nextcloud-ical-backup/internal/repository"
	"bodsch.me/nextcloud-ical-backup/internal/service"
	"bodsch.me/nextcloud-ical-backup/internal/settings"
)

// stringList is a repeatable string flag (e.g. --user alice --user bob).
type stringList []string

// String renders the collected values, satisfying flag.Value.
func (s *stringList) String() string { return strings.Join(*s, ",") }

// Set appends a value, satisfying flag.Value so the flag can be repeated.
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// main dispatches to the backup or restore subcommand and maps errors to a
// non-zero exit code.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "backup":
		err = runBackup(os.Args[2:])
	case "restore":
		err = runRestore(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	case "-v", "-version", "--version", "version":
		printVersion(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// usage prints the top-level command synopsis to stderr.
func usage() {
	fmt.Fprint(os.Stderr, `Backup and restore Nextcloud calendars and addressbooks.

Usage:
  nextcloud-ical-backup backup  [options]
  nextcloud-ical-backup restore [options]
  nextcloud-ical-backup --version

Run "nextcloud-ical-backup <command> -h" for command specific options.
`)
}

// runBackup parses the backup flags, resolves the layered settings and exports
// the selected calendars and addressbooks.
func runBackup(argv []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	configFile := fs.String("config", "", "TOML configuration file")
	configPHP := fs.String("config-php", "", "path to Nextcloud config.php")
	nextcloudPath := fs.String("nextcloud-path", "", "Nextcloud installation root")
	backupRoot := fs.String("backup-root", "", "backup tree root directory")
	var users, calendars, addressbooks stringList
	fs.Var(&users, "user", "username to include (repeatable)")
	fs.Var(&calendars, "calendar", "calendar name to include (repeatable)")
	fs.Var(&addressbooks, "addressbook", "addressbook name to include (repeatable)")
	noCalendars := fs.Bool("no-calendars", false, "skip calendars")
	noAddressbooks := fs.Bool("no-addressbooks", false, "skip addressbooks")
	aggregate := fs.Bool("aggregate", false, "write one combined file per item instead of one per entry")
	dryRun := fs.Bool("dry-run", false, "print actions without writing anything")
	listOnly := fs.Bool("list-only", false, "only list selectable items")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	s, err := settings.Load(*configFile)
	if err != nil {
		return err
	}
	set := visited(fs)
	applyIf(set, "config-php", func() { s.ConfigPHP = *configPHP })
	applyIf(set, "nextcloud-path", func() { s.NextcloudPath = *nextcloudPath })
	applyIf(set, "backup-root", func() { s.BackupRoot = *backupRoot })
	applyIf(set, "user", func() { s.Users = users })
	applyIf(set, "calendar", func() { s.Calendars = calendars })
	applyIf(set, "addressbook", func() { s.Addressbooks = addressbooks })
	applyIf(set, "no-calendars", func() { s.IncludeCalendars = !*noCalendars })
	applyIf(set, "no-addressbooks", func() { s.IncludeAddressbooks = !*noAddressbooks })
	applyIf(set, "aggregate", func() { s.Aggregate = *aggregate })
	applyIf(set, "dry-run", func() { s.DryRun = *dryRun })
	applyIf(set, "list-only", func() { s.ListOnly = *listOnly })

	configPath, err := s.ResolveConfigPHP()
	if err != nil {
		return err
	}
	cfg, err := ncconfig.FromPHP(configPath)
	if err != nil {
		return err
	}
	repo, err := repository.Open(cfg)
	if err != nil {
		return err
	}
	defer repo.Close()

	svc := service.NewBackupService(repo)
	filter := s.ToFilter()

	if s.ListOnly {
		items, err := svc.ListItems(filter)
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Println(item)
		}
		return nil
	}

	// Write each run into a dated subdirectory (e.g. backup/2026-06-04/...)
	// so consecutive backups are kept side by side instead of overwriting.
	datedRoot := filepath.Join(s.BackupRoot, time.Now().Format("2006-01-02"))

	report, err := svc.Export(datedRoot, filter, service.ExportOptions{DryRun: s.DryRun, Aggregate: s.Aggregate})
	if err != nil {
		return err
	}
	for _, line := range report.Calendars {
		fmt.Println(line)
	}
	for _, line := range report.Addressbooks {
		fmt.Println(line)
	}
	fmt.Printf("Processed %d item(s).\n", report.Total())
	return nil
}

// runRestore parses the restore flags, resolves the layered settings and
// imports the selected calendars into Nextcloud via occ.
func runRestore(argv []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	configFile := fs.String("config", "", "TOML configuration file")
	nextcloudPath := fs.String("nextcloud-path", "", "Nextcloud installation root")
	backupRoot := fs.String("backup-root", "", "backup tree root directory")
	occCommand := fs.String("occ-command", "", "custom occ invocation")
	var users, calendars, addressbooks stringList
	fs.Var(&users, "user", "username to include (repeatable)")
	fs.Var(&calendars, "calendar", "calendar name to include (repeatable)")
	fs.Var(&addressbooks, "addressbook", "addressbook name to include (repeatable)")
	dryRun := fs.Bool("dry-run", false, "print actions without importing")
	listOnly := fs.Bool("list-only", false, "only list restorable items")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	s, err := settings.Load(*configFile)
	if err != nil {
		return err
	}
	set := visited(fs)
	applyIf(set, "nextcloud-path", func() { s.NextcloudPath = *nextcloudPath })
	applyIf(set, "backup-root", func() { s.BackupRoot = *backupRoot })
	applyIf(set, "occ-command", func() { s.OccCommand = *occCommand })
	applyIf(set, "user", func() { s.Users = users })
	applyIf(set, "calendar", func() { s.Calendars = calendars })
	applyIf(set, "addressbook", func() { s.Addressbooks = addressbooks })
	applyIf(set, "dry-run", func() { s.DryRun = *dryRun })
	applyIf(set, "list-only", func() { s.ListOnly = *listOnly })

	occ, err := s.ResolveOccCommand()
	if err != nil {
		return err
	}
	svc, err := service.NewRestoreService(occ, nil)
	if err != nil {
		return err
	}
	filter := s.ToFilter()

	if s.ListOnly {
		items, err := svc.ListItems(s.BackupRoot, filter)
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Println(item)
		}
		return nil
	}

	report, err := svc.Restore(s.BackupRoot, filter, s.DryRun)
	if err != nil {
		return err
	}
	for _, line := range report.Calendars {
		fmt.Println(line)
	}
	for _, line := range report.Skipped {
		fmt.Println(line)
	}
	fmt.Printf("Restored %d calendar(s), skipped %d item(s).\n", len(report.Calendars), len(report.Skipped))
	return nil
}

// visited returns the set of flag names explicitly provided on the command line.
func visited(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// applyIf runs apply only when the named flag was set on the command line, so
// that unset flags do not override config-file or default values.
func applyIf(set map[string]bool, name string, apply func()) {
	if set[name] {
		apply()
	}
}
