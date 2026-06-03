// Package service contains the application logic: building iCalendar/vCard
// content and the backup and restore workflows on top of a repository.
package service

import (
	"fmt"
	"path/filepath"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
	"bodsch.me/nextcloud-ical-backup/internal/repository"
	"bodsch.me/nextcloud-ical-backup/internal/util"
)

// excludedCalendarURIs lists automatically generated calendars that are not
// real user data.
var excludedCalendarURIs = map[string]struct{}{"contact_birthdays": {}}

// ExportOptions controls how a backup is written.
type ExportOptions struct {
	DryRun bool
	// Aggregate writes one combined file per item instead of one per entry.
	Aggregate bool
}

// BackupService exports calendars and addressbooks from a repository.
type BackupService struct {
	repo repository.Repository
}

// NewBackupService creates a BackupService for the given repository.
func NewBackupService(repo repository.Repository) *BackupService {
	return &BackupService{repo: repo}
}

// SelectedCalendars returns calendars matching the filter, excluding generated ones.
func (s *BackupService) SelectedCalendars(f domain.BackupFilter) ([]domain.CalendarItem, error) {
	if !f.IncludeCalendars {
		return nil, nil
	}
	all, err := s.repo.ListCalendars()
	if err != nil {
		return nil, err
	}
	var out []domain.CalendarItem
	for _, c := range all {
		if _, excluded := excludedCalendarURIs[c.URI]; excluded {
			continue
		}
		if f.MatchesUser(c.Username) && f.MatchesCalendar(c.DisplayName) {
			out = append(out, c)
		}
	}
	return out, nil
}

// SelectedAddressbooks returns addressbooks matching the filter.
func (s *BackupService) SelectedAddressbooks(f domain.BackupFilter) ([]domain.AddressbookItem, error) {
	if !f.IncludeAddressbooks {
		return nil, nil
	}
	all, err := s.repo.ListAddressbooks()
	if err != nil {
		return nil, err
	}
	var out []domain.AddressbookItem
	for _, b := range all {
		if f.MatchesUser(b.Username) && f.MatchesAddressbook(b.DisplayName) {
			out = append(out, b)
		}
	}
	return out, nil
}

// Export writes all selected calendars and addressbooks to backupRoot.
func (s *BackupService) Export(backupRoot string, f domain.BackupFilter, opt ExportOptions) (domain.BackupReport, error) {
	var report domain.BackupReport

	calendars, err := s.SelectedCalendars(f)
	if err != nil {
		return report, err
	}
	for _, c := range calendars {
		target, err := s.exportCalendar(c, backupRoot, opt)
		if err != nil {
			return report, err
		}
		report.Calendars = append(report.Calendars, fmt.Sprintf("%s/%s -> %s", c.Username, c.DisplayName, target))
	}

	books, err := s.SelectedAddressbooks(f)
	if err != nil {
		return report, err
	}
	for _, b := range books {
		target, err := s.exportAddressbook(b, backupRoot, opt)
		if err != nil {
			return report, err
		}
		report.Addressbooks = append(report.Addressbooks, fmt.Sprintf("%s/%s -> %s", b.Username, b.DisplayName, target))
	}
	return report, nil
}

// ListItems returns a human readable listing of selectable items.
func (s *BackupService) ListItems(f domain.BackupFilter) ([]string, error) {
	calendars, err := s.SelectedCalendars(f)
	if err != nil {
		return nil, err
	}
	books, err := s.SelectedAddressbooks(f)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(calendars)+len(books))
	for _, c := range calendars {
		items = append(items, "calendar: "+c.Username+"/"+c.DisplayName)
	}
	for _, b := range books {
		items = append(items, "addressbook: "+b.Username+"/"+b.DisplayName)
	}
	return items, nil
}

// exportCalendar writes a single calendar, either as one combined .ics file
// (aggregate) or one .ics per event in a per-calendar directory. It returns the
// path written to.
func (s *BackupService) exportCalendar(item domain.CalendarItem, backupRoot string, opt ExportOptions) (string, error) {
	icsDir := filepath.Join(backupRoot, item.Username, "ics")
	name := util.SanitizeFilename(item.DisplayName)

	if opt.Aggregate {
		output := filepath.Join(icsDir, name+".ics")
		if opt.DryRun {
			return output, nil
		}
		objects, err := s.repo.CalendarObjects(item.ID)
		if err != nil {
			return "", err
		}
		return output, writeFile(output, BuildICal(item.DisplayName, item.Color, objects))
	}

	targetDir := filepath.Join(icsDir, name)
	objects, err := s.repo.CalendarObjects(item.ID)
	if err != nil {
		return "", err
	}
	return targetDir, writeEntries(objects, targetDir, ".ics", opt.DryRun)
}

// exportAddressbook writes a single addressbook, either as one combined .vcf
// file (aggregate) or one .vcf per contact in a per-addressbook directory. It
// returns the path written to.
func (s *BackupService) exportAddressbook(item domain.AddressbookItem, backupRoot string, opt ExportOptions) (string, error) {
	vcfDir := filepath.Join(backupRoot, item.Username, "vcf")
	name := util.SanitizeFilename(item.DisplayName)

	if opt.Aggregate {
		output := filepath.Join(vcfDir, name+".vcf")
		if opt.DryRun {
			return output, nil
		}
		cards, err := s.repo.Cards(item.ID)
		if err != nil {
			return "", err
		}
		return output, writeFile(output, BuildVCard(cards))
	}

	targetDir := filepath.Join(vcfDir, name)
	cards, err := s.repo.Cards(item.ID)
	if err != nil {
		return "", err
	}
	return targetDir, writeEntries(cards, targetDir, ".vcf", opt.DryRun)
}

// writeEntries writes one raw object per file into targetDir. Filenames are
// derived from each object's UID and de-duplicated. The directory is only
// created when at least one entry exists, so empty items leave no stray dir.
func writeEntries(rawItems []string, targetDir, suffix string, dryRun bool) error {
	used := map[string]struct{}{}
	for i, raw := range rawItems {
		base := util.ExtractUID(raw)
		if base == "" {
			base = fmt.Sprintf("item-%d", i+1)
		}
		name := uniqueName(util.SanitizeFilename(base), used)
		used[name] = struct{}{}
		if dryRun {
			continue
		}
		if err := util.EnsureDir(targetDir); err != nil {
			return err
		}
		if err := util.WriteCRLFLines(filepath.Join(targetDir, name+suffix), util.NormalizeLines(raw)); err != nil {
			return err
		}
	}
	return nil
}

// writeFile writes lines (CRLF separated) to path, creating parent dirs.
func writeFile(path string, lines []string) error {
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return util.WriteCRLFLines(path, lines)
}

// uniqueName returns base, or base with a numeric suffix, so that no name in
// used is reused within the same directory.
func uniqueName(base string, used map[string]struct{}) string {
	if _, ok := used[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d", base, n)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}
