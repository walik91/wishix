//go:build integration

package services_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationIndependentWishCopy(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "copy-owner@example.test", "copyowner")
	other := f.account(t, "copy-other@example.test", "copyother")
	list := f.newList(t, owner, "Copy test", "public")
	listID := list["id"].(string)
	source := f.listCall(t, "POST", "/api/wishes", map[string]any{"title": "Gift", "description": "Notes", "price": "12.50", "currency": "EUR", "priority": "high", "shop_links": []string{"https://first.test/", "https://second.test/"}, "list_ids": []string{listID}}, owner, 201)["wish"].(map[string]any)
	id := source["id"].(string)
	image := models.WishImage{WishID: id, Version: security.UUID(), Data: []byte("test image bytes")}
	if err := f.database.Create(&image).Error; err != nil {
		t.Fatal(err)
	}
	executor := db.NewExecutor(f.database)
	config := &security.Config{Origin: listOrigin, Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, &reservationMessages{}, config)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reserve := func(wishID, email string) {
		t.Helper()
		input := services.ReservationInput{WishID: wishID, ListID: listID, Email: email, Locale: "en"}
		challenge, err := bookings.Start(ctx, input, nil)
		if err != nil {
			t.Fatal("start reservation", err)
		}
		input.Challenge, input.Code = challenge, f.mail.code(email, "reservation")
		if err := bookings.Confirm(ctx, input, nil); err != nil {
			t.Fatal("confirm reservation", err)
		}
	}
	reserve(id, "copy-guest1@example.test")
	path := "/api/wishes/" + id + "/copy"
	for _, tc := range []struct {
		cookie       *http.Cookie
		origin, body string
		want         int
	}{
		{nil, listOrigin, "{}", 401}, {other, listOrigin, "{}", 404}, {owner, "https://attacker.test", "{}", 403}, {owner, listOrigin, `{"owner_id":"x"}`, 400}, {owner, listOrigin, `null`, 400},
	} {
		if status, _ := f.wishRequest(t, "POST", path, tc.body, tc.cookie, tc.origin); status != tc.want {
			t.Fatalf("copy access: got %d want %d", status, tc.want)
		}
	}
	// Force the second link insertion to fail after the new wish was inserted.
	if err := f.database.Exec("ALTER TABLE wish_shop_links ADD CONSTRAINT reject_copied_link CHECK (wish_id = '" + id + "'::UUID OR url <> 'https://second.test/')").Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := f.wishRequest(t, "POST", path, "{}", owner, listOrigin); status != 503 {
		t.Fatalf("copy should fail when a link cannot be stored: %d", status)
	}
	var wishCount int64
	if err := f.database.Model(&models.Wish{}).Count(&wishCount).Error; err != nil || wishCount != 1 {
		t.Fatalf("failed copy left a partial wish: %d %v", wishCount, err)
	}
	if err := f.database.Exec("ALTER TABLE wish_shop_links DROP CONSTRAINT reject_copied_link").Error; err != nil {
		t.Fatal(err)
	}
	copy := f.listCall(t, "POST", path, map[string]any{}, owner, 201)["wish"].(map[string]any)
	copyID := copy["id"].(string)
	if copyID == id || copy["title"] != "Gift" || copy["description"] != "Notes" || copy["price"] != "12.50" || copy["priority"] != "high" || len(copy["shop_links"].([]any)) != 2 || copy["image_url"] == nil {
		t.Fatalf("incorrect copy: %v", copy)
	}
	var copiedImage models.WishImage
	if err := f.database.Where("wish_id = ?", copyID).Take(&copiedImage).Error; err != nil || copiedImage.Version == image.Version || !bytes.Equal(copiedImage.Data, image.Data) {
		t.Fatalf("image copy: %v", err)
	}
	if len(f.currentList(t, owner, listID)["wishes"].([]any)) != 1 {
		t.Fatal("copy was automatically added to list")
	}
	f.listCall(t, "POST", "/api/lists/"+listID+"/wishes", map[string]any{"wish_ids": []string{copyID}}, owner, 200)
	reserve(copyID, "copy-guest2@example.test")
	var count int64
	if err := f.database.Model(&models.Reservation{}).Where("wish_id IN ?", []string{id, copyID}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("independent reservations: %d %v", count, err)
	}
	f.listCall(t, "PUT", "/api/wishes/"+copyID, map[string]any{"title": "Copy changed", "edit_version": copy["edit_version"]}, owner, 200)
	if got := f.listCall(t, "GET", "/api/wishes/"+id, map[string]any{}, owner, 200)["wish"].(map[string]any)["title"]; got != "Gift" {
		t.Fatalf("source changed: %v", got)
	}
	f.listCall(t, "DELETE", "/api/wishes/"+id, map[string]any{}, owner, 200)
	if err := f.database.Where("wish_id = ?", copyID).Take(&copiedImage).Error; err != nil {
		t.Fatal("deleting source deleted copy image", err)
	}
}
