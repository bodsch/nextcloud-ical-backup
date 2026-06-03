package repository

import (
	"database/sql"
	"strconv"
	"strings"

	"bodsch.me/nextcloud-ical-backup/internal/domain"
	"bodsch.me/nextcloud-ical-backup/internal/ncconfig"
	"bodsch.me/nextcloud-ical-backup/internal/util"
)

// systemPrincipalPrefix marks server-internal principals (system addressbook,
// rooms, resources) that are never user-restorable.
const systemPrincipalPrefix = "principals/system/"

// sqlRepository implements Repository on top of database/sql. Both supported
// drivers (modernc.org/sqlite and go-sql-driver/mysql) use "?" placeholders,
// so a single implementation serves both backends.
type sqlRepository struct {
	db     *sql.DB
	prefix string
	schema ncconfig.SchemaProfile
}

// newSQLRepository wraps an open database/sql connection as a Repository.
func newSQLRepository(db *sql.DB, prefix string, schema ncconfig.SchemaProfile) *sqlRepository {
	return &sqlRepository{db: db, prefix: prefix, schema: schema}
}

// table returns the prefixed table name (e.g. "oc_" + "calendars").
func (r *sqlRepository) table(name string) string { return r.prefix + name }

// ListCalendars returns all user-owned calendars, skipping server-internal
// (system) principals.
func (r *sqlRepository) ListCalendars() ([]domain.CalendarItem, error) {
	rows, err := r.db.Query("SELECT id, principaluri, uri, displayname, calendarcolor FROM " +
		r.table("calendars") + " ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calendars []domain.CalendarItem
	for rows.Next() {
		var (
			id                          int64
			principal, uri, name, color sql.NullString
		)
		if err := rows.Scan(&id, &principal, &uri, &name, &color); err != nil {
			return nil, err
		}
		if strings.HasPrefix(principal.String, systemPrincipalPrefix) {
			continue
		}
		calendars = append(calendars, domain.CalendarItem{
			ID:          id,
			Username:    util.NormalizePrincipalURI(principal.String),
			URI:         uri.String,
			DisplayName: displayName(name.String, uri.String, "calendar", id),
			Color:       cleanColor(color),
		})
	}
	return calendars, rows.Err()
}

// ListAddressbooks returns all user-owned addressbooks, skipping
// server-internal (system) principals.
func (r *sqlRepository) ListAddressbooks() ([]domain.AddressbookItem, error) {
	rows, err := r.db.Query("SELECT id, principaluri, uri, displayname FROM " +
		r.table("addressbooks") + " ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []domain.AddressbookItem
	for rows.Next() {
		var (
			id                   int64
			principal, uri, name sql.NullString
		)
		if err := rows.Scan(&id, &principal, &uri, &name); err != nil {
			return nil, err
		}
		if strings.HasPrefix(principal.String, systemPrincipalPrefix) {
			continue
		}
		books = append(books, domain.AddressbookItem{
			ID:          id,
			Username:    util.NormalizePrincipalURI(principal.String),
			URI:         uri.String,
			DisplayName: displayName(name.String, uri.String, "addressbook", id),
		})
	}
	return books, rows.Err()
}

// CalendarObjects returns the raw calendardata of a calendar, excluding cached
// webcal subscription objects (Nextcloud >= 15) and trashed components
// (Nextcloud >= 22) according to the schema profile.
func (r *sqlRepository) CalendarObjects(calendarID int64) ([]string, error) {
	conditions := []string{"calendarid = ?"}
	if r.schema.FilterSubscriptionCache {
		conditions = append(conditions, "calendartype = 0")
	}
	if r.schema.HasTrashbin {
		conditions = append(conditions, "deleted_at IS NULL")
	}
	query := "SELECT calendardata FROM " + r.table("calendarobjects") +
		" WHERE " + strings.Join(conditions, " AND ") + " ORDER BY id"
	return r.queryBlobs(query, calendarID)
}

// Cards returns the raw carddata of an addressbook.
func (r *sqlRepository) Cards(addressbookID int64) ([]string, error) {
	query := "SELECT carddata FROM " + r.table("cards") + " WHERE addressbookid = ? ORDER BY id"
	return r.queryBlobs(query, addressbookID)
}

// Close releases the underlying database connection.
func (r *sqlRepository) Close() error { return r.db.Close() }

// queryBlobs runs a single-column query and decodes each row to a string,
// transparently handling both text and BLOB columns.
func (r *sqlRepository) queryBlobs(query string, arg int64) ([]string, error) {
	rows, err := r.db.Query(query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, string(data))
	}
	return out, rows.Err()
}

// displayName picks the best available human readable name, falling back to
// the URI and finally to "<kind>-<id>".
func displayName(name, uri, kind string, id int64) string {
	switch {
	case name != "":
		return name
	case uri != "":
		return uri
	default:
		return kind + "-" + strconv.FormatInt(id, 10)
	}
}

// cleanColor normalizes a calendar color, treating NULL/empty as "no color".
func cleanColor(color sql.NullString) string {
	if !color.Valid {
		return ""
	}
	text := strings.TrimSpace(color.String)
	if text == "" || strings.EqualFold(text, "NULL") {
		return ""
	}
	return text
}
