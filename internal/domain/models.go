// Package domain holds the I/O-free core models shared across the services and
// adapters of the Nextcloud backup utility.
package domain

// ItemType is the type of a backupable Nextcloud item.
type ItemType string

const (
	// Calendar identifies an iCalendar (.ics) item.
	Calendar ItemType = "calendar"
	// Addressbook identifies a vCard (.vcf) item.
	Addressbook ItemType = "addressbook"
)

// CalendarItem is a single calendar owned by a Nextcloud user.
type CalendarItem struct {
	ID          int64
	Username    string
	URI         string
	DisplayName string
	Color       string // empty string means "no color"
}

// AddressbookItem is a single addressbook owned by a Nextcloud user.
type AddressbookItem struct {
	ID          int64
	Username    string
	URI         string
	DisplayName string
}

// StringSet is a set of strings. A nil set means "no restriction".
type StringSet map[string]struct{}

// NewStringSet builds a StringSet from a slice; an empty slice yields nil
// (i.e. "match everything").
func NewStringSet(values []string) StringSet {
	if len(values) == 0 {
		return nil
	}
	set := make(StringSet, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

// BackupFilter restricts which users/calendars/addressbooks are processed.
// A nil StringSet means the corresponding dimension is unrestricted.
type BackupFilter struct {
	Users               StringSet
	Calendars           StringSet
	Addressbooks        StringSet
	IncludeCalendars    bool
	IncludeAddressbooks bool
}

// DefaultFilter returns a filter that selects everything.
func DefaultFilter() BackupFilter {
	return BackupFilter{IncludeCalendars: true, IncludeAddressbooks: true}
}

// MatchesUser reports whether username passes the user filter.
func (f BackupFilter) MatchesUser(username string) bool { return matches(f.Users, username) }

// MatchesCalendar reports whether a calendar display name passes the filter.
func (f BackupFilter) MatchesCalendar(name string) bool { return matches(f.Calendars, name) }

// MatchesAddressbook reports whether an addressbook display name passes the filter.
func (f BackupFilter) MatchesAddressbook(name string) bool { return matches(f.Addressbooks, name) }

// matches reports whether value passes a set filter; a nil set matches all.
func matches(set StringSet, value string) bool {
	if set == nil {
		return true
	}
	_, ok := set[value]
	return ok
}

// BackupReport summarizes a backup or restore run.
type BackupReport struct {
	Calendars    []string
	Addressbooks []string
	Skipped      []string
}

// Total returns the number of processed (calendar + addressbook) items.
func (r BackupReport) Total() int { return len(r.Calendars) + len(r.Addressbooks) }
