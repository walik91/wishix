//go:build integration

package services_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationReceivedIsPerListAndCompletesOnlyItsReservation(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "received-owner@example.test", "receivedowner")
	other := f.account(t, "received-other@example.test", "receivedother")
	a := f.newList(t, owner, "Birthday", "public")["id"].(string)
	b := f.newList(t, owner, "New year", "public")["id"].(string)
	wish := f.newWish(t, owner, "Gift certificate", a, b)["id"].(string)
	executor := db.NewExecutor(f.database)
	config := &security.Config{Origin: listOrigin, Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, &reservationMessages{}, config)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	input := services.ReservationInput{WishID: wish, ListID: a, Email: "received-guest@example.test", Locale: "en"}
	challenge, err := bookings.Start(ctx, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	input.Challenge, input.Code = challenge, f.mail.code(input.Email, "reservation")
	if err := bookings.Confirm(ctx, input, nil); err != nil {
		t.Fatal(err)
	}
	mark := func(list string, received bool) {
		current := f.currentList(t, owner, list)
		f.listCall(t, "PUT", "/api/lists/"+list+"/wishes/"+wish+"/received", map[string]any{"received": received, "revision": rev(current)}, owner, 200)
	}
	f.listCall(t, "PUT", "/api/lists/"+a+"/wishes/"+wish+"/received", map[string]any{"received": true, "revision": 1}, other, 404)
	mark(b, true)
	f.listCall(t, "PUT", "/api/lists/"+b+"/wishes/"+wish+"/received", map[string]any{"received": false, "revision": 1}, owner, 409)
	var count int64
	if err := f.database.Model(&models.Reservation{}).Where("wish_id = ?", wish).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("other list changed reservation: %d %v", count, err)
	}
	if _, err := bookings.Start(ctx, services.ReservationInput{WishID: wish, ListID: b, Email: "another@example.test"}, nil); err != services.ErrReservationUnavailable {
		t.Fatalf("received item accepted reservation: %v", err)
	}
	mark(a, true)
	if err := f.database.Model(&models.Reservation{}).Where("wish_id = ?", wish).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("origin reservation not completed: %d %v", count, err)
	}
	mark(b, false)
	if _, err := bookings.Start(ctx, services.ReservationInput{WishID: wish, ListID: b, Email: "another@example.test"}, nil); err != nil {
		t.Fatalf("other list not reservable: %v", err)
	}
	copy := f.listCall(t, "POST", "/api/lists/"+a+"/copy", map[string]any{"title": "Copy"}, owner, 201)["list"].(map[string]any)
	if copy["wish_count"] != float64(0) || len(f.currentList(t, owner, copy["id"].(string))["wishes"].([]any)) != 0 {
		t.Fatal("received wish copied", copy)
	}
	before := f.listCall(t, "GET", "/api/wishes/"+wish+"/lists", nil, owner, 200)
	f.listCall(t, "PUT", "/api/wishes/"+wish+"/lists", map[string]any{"list_ids": []string{a, b}, "version": before["version"]}, owner, 200)
	item := f.currentList(t, owner, a)["wishes"].([]any)[0].(map[string]any)
	if item["received"] != true {
		t.Fatal("membership editing lost received status", item)
	}
	f.listCall(t, "PUT", "/api/profile/collection", map[string]any{"show_all_wishes": true}, owner, 200)
	collection := f.listCall(t, "GET", "/api/public/profiles/receivedowner/wishes", nil, nil, 200)
	item = collection["wishes"].([]any)[0].(map[string]any)
	for _, field := range []string{"received", "reserved", "guest_recoverable"} {
		if _, ok := item[field]; ok {
			t.Fatalf("collection exposed %s", field)
		}
	}
	if _, err := bookings.Start(ctx, services.ReservationInput{WishID: wish, Username: "receivedowner", Email: "guest2@example.test"}, nil); err != services.ErrReservationUnavailable {
		t.Fatalf("collection accepted reservation: %v", err)
	}
}

func TestIntegrationReceivedRacesReservationConfirmation(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "race-received@example.test", "racereceived")
	list := f.newList(t, owner, "Occasion", "public")["id"].(string)
	wish := f.newWish(t, owner, "Gift", list)["id"].(string)
	var member models.WishlistWish
	if err := f.database.Where("list_id = ? AND wish_id = ?", list, wish).Take(&member).Error; err != nil {
		t.Fatal(err)
	}
	var currentList models.Wishlist
	if err := f.database.Where("id = ?", list).Take(&currentList).Error; err != nil {
		t.Fatal(err)
	}
	executor := db.NewExecutor(f.database)
	tx := db.NewTransactionManager(executor)
	config := &security.Config{Origin: listOrigin, Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), tx, f.auth, f.mail, &reservationMessages{}, config)
	lists := services.NewLists(repositories.NewLists(executor), tx)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	input := services.ReservationInput{WishID: wish, ListID: list, Email: "race-guest@example.test", Locale: "en"}
	challenge, err := bookings.Start(ctx, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	input.Challenge, input.Code = challenge, f.mail.code(input.Email, "reservation")
	start := make(chan struct{})
	var markErr, confirmErr error
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		<-start
		markErr = lists.SetReceived(ctx, member.OwnerID, list, wish, currentList.Revision, true)
	}()
	go func() { defer group.Done(); <-start; confirmErr = bookings.Confirm(ctx, input, nil) }()
	close(start)
	group.Wait()
	if markErr != nil {
		t.Fatal("mark received", markErr)
	}
	if confirmErr != nil && confirmErr != services.ErrReservationUnavailable {
		t.Fatal("confirm", confirmErr)
	}
	var count int64
	if err := f.database.Model(&models.Reservation{}).Where("wish_id = ?", wish).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("reservation survived after receipt: %d %v", count, err)
	}
	if err := f.database.Where("list_id = ? AND wish_id = ?", list, wish).Take(&member).Error; err != nil || !member.Received {
		t.Fatalf("received status missing: %v", err)
	}
}
