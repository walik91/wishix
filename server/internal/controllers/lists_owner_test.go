package controllers

import (
	"encoding/json"
	"testing"

	"wishix/server/internal/models"
)

func TestGuestListHidesReservationsFromOwner(t *testing.T) {
	list := &models.Wishlist{ID: "list", OwnerID: "owner", Visibility: "public"}
	wishes := []models.Wish{{ID: "wish", Title: "Gift", Reserved: true, GuestRecoverable: true}}
	for _, tt := range []struct {
		name    string
		user    *models.User
		isOwner bool
	}{
		{"owner", &models.User{ID: "owner"}, true},
		{"other user", &models.User{ID: "visitor"}, false},
		{"anonymous", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(guestListResponse(list, "example", wishes, tt.user))
			if err != nil {
				t.Fatal(err)
			}
			var response struct {
				IsOwner bool             `json:"is_owner"`
				Wishes  []map[string]any `json:"wishes"`
			}
			if err := json.Unmarshal(raw, &response); err != nil || response.IsOwner != tt.isOwner || len(response.Wishes) != 1 {
				t.Fatalf("invalid response: %s, %v", raw, err)
			}
			_, reserved := response.Wishes[0]["reserved"]
			_, recoverable := response.Wishes[0]["guest_recoverable"]
			if reserved == tt.isOwner || recoverable == tt.isOwner {
				t.Fatalf("wrong reservation fields: %s", raw)
			}
			if !tt.isOwner && (response.Wishes[0]["reserved"] != true || response.Wishes[0]["guest_recoverable"] != true) {
				t.Fatalf("visitor lost reservation status: %s", raw)
			}
		})
	}
}
