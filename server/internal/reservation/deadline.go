// Package reservation contains the time rules for gift reservations.
package reservation

import (
	"fmt"
	"time"
	_ "time/tzdata" // The production image may not contain a zoneinfo database.
)

const period = 14 * 24 * time.Hour

// EventEnd is the exclusive end of the calendar date in the author's zone.
// An explicit IANA zone is required so server and guest timezones cannot alter it.
func EventEnd(date, zone string) (time.Time, error) {
	if zone == "" || zone == "Local" {
		return time.Time{}, fmt.Errorf("invalid event timezone")
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid event timezone")
	}
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Year() < 1 || parsed.Year() > 9999 {
		return time.Time{}, fmt.Errorf("invalid event date")
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day()+1, 0, 0, 0, 0, location), nil
}

// InitialDeadline returns the first reservation's expiry. event is the instant
// at which the event day ends; its location is used for calendar-day arithmetic.
// A caller must reject an event that has already ended.
func InitialDeadline(now time.Time, event *time.Time) time.Time {
	deadline := now.Add(period)
	if event == nil {
		return deadline
	}
	fiveDaysBefore := event.AddDate(0, 0, -5)
	if now.Before(fiveDaysBefore) {
		if fiveDaysBefore.Before(deadline) {
			return fiveDaysBefore
		}
		return deadline
	}
	if event.Before(deadline) {
		return *event
	}
	return deadline
}

// ExtendedDeadline returns a 14-day renewal capped at the event, if any.
// The caller checks that the reservation is active and the result extends it.
func ExtendedDeadline(now time.Time, event *time.Time) time.Time {
	deadline := now.Add(period)
	if event != nil && event.Before(deadline) {
		return *event
	}
	return deadline
}

// ShortenedDeadline preserves an existing deadline unless an event is earlier.
// Removing or moving an event later never silently extends a reservation.
func ShortenedDeadline(deadline time.Time, event *time.Time) time.Time {
	if event != nil && event.Before(deadline) {
		return *event
	}
	return deadline
}
