package services

import (
	"testing"
	"time"

	"wishix/server/internal/models"
)

func TestListExpiryUsesStoredTimezoneAndEndOfDay(t *testing.T) {
	date := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	list := &models.Wishlist{EventDate: &date, EventTimezone: "Europe/Chisinau"}
	before := time.Date(2026, 12, 20, 21, 59, 59, 0, time.UTC)
	if ListExpired(list, before) || !ListExpired(list, before.Add(time.Second)) {
		t.Fatal("list must close at midnight in the author's timezone")
	}
	if ListExpired(&models.Wishlist{}, before) {
		t.Fatal("undated list must not expire")
	}
	list.EventTimezone = "invalid/zone"
	if !ListExpired(list, before) {
		t.Fatal("invalid stored timezone must fail closed")
	}
}
