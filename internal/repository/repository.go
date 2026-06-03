// Package repository provides read-only access to a Nextcloud installation's
// calendar and contact data.
package repository

import "bodsch.me/nextcloud-ical-backup/internal/domain"

// Repository is the read API consumed by the backup service.
type Repository interface {
	// ListCalendars returns all user-owned calendars.
	ListCalendars() ([]domain.CalendarItem, error)
	// ListAddressbooks returns all user-owned addressbooks.
	ListAddressbooks() ([]domain.AddressbookItem, error)
	// CalendarObjects returns the raw calendardata blobs of a calendar.
	CalendarObjects(calendarID int64) ([]string, error)
	// Cards returns the raw carddata blobs of an addressbook.
	Cards(addressbookID int64) ([]string, error)
	// Close releases the underlying database connection.
	Close() error
}
