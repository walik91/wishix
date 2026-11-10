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
	"wishix/server/internal/reservation"
	"wishix/server/internal/services"
)

type reservationMessages struct {
	mu     sync.Mutex
	bodies []string
	fail   bool
}

func (m *reservationMessages) SendMessage(_ context.Context, _, _, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return security.ErrDelivery
	}
	m.bodies = append(m.bodies, body)
	return nil
}

func TestIntegrationReservationsAndListChanges(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "booking-owner@example.test", "bookingowner")
	list := f.newList(t, owner, "Occasion", "link")
	listID := list["id"].(string)
	link := strings.Split(list["share_path"].(string), "/")
	listToken := link[len(link)-1]
	wish := f.newWish(t, owner, "Present", listID)
	wishID := wish["id"].(string)
	other := f.newList(t, owner, "Other", "public")
	otherID := other["id"].(string)
	f.listCall(t, "POST", "/api/lists/"+otherID+"/wishes", map[string]any{"wish_ids": []string{wishID}}, owner, 200)
	executor := db.NewExecutor(f.database)
	messages := &reservationMessages{}
	config := &security.Config{Origin: "https://wishix.test", Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, messages, config)
	ctx, stop := context.WithTimeout(context.Background(), 90*time.Second)
	defer stop()
	input := services.ReservationInput{WishID: wishID, ListID: listID, ListToken: listToken, Email: "guest@example.test", Locale: "en"}
	challenge, err := bookings.Start(ctx, input, nil)
	if err != nil {
		t.Fatal("start", err)
	}
	input.Challenge, input.Code = challenge, f.mail.code(input.Email, "reservation")
	bad := input
	bad.Code = "000000"
	if bad.Code == input.Code {
		bad.Code = "111111"
	}
	for i := 0; i < 5; i++ {
		if err := bookings.Confirm(ctx, bad, nil); err != services.ErrCode {
			t.Fatalf("bad code accepted: %v", err)
		}
	}
	if err := bookings.Confirm(ctx, input, nil); err != services.ErrCode {
		t.Fatalf("attempt limit did not lock the code: %v", err)
	}
	challenge, err = bookings.Start(ctx, input, nil)
	if err != nil {
		t.Fatal("restart", err)
	}
	input.Challenge, input.Code = challenge, f.mail.code(input.Email, "reservation")
	if err := bookings.Confirm(ctx, input, nil); err != nil {
		t.Fatal("confirm", err)
	}
	if err := bookings.Confirm(ctx, input, nil); err != services.ErrCode {
		t.Fatalf("challenge reused: %v", err)
	}
	guest := f.listCall(t, "POST", "/api/public/lists/guest", map[string]string{"id": otherID, "token": ""}, nil, 200)
	publicWish := guest["wishes"].([]any)[0].(map[string]any)
	if publicWish["reserved"] != true || publicWish["guest_recoverable"] != true {
		t.Fatal("reservation not visible through other list", publicWish)
	}
	private := f.currentList(t, owner, listID)["wishes"].([]any)[0].(map[string]any)
	if _, exists := private["reserved"]; exists {
		t.Fatal("reservation disclosed to owner")
	}
	var booking models.Reservation
	if err := f.database.Where("wish_id = ?", wishID).Take(&booking).Error; err != nil {
		t.Fatal(err)
	}
	token := reservation.ManagementToken(config.Secret, booking.ID)
	messages.fail = true
	if err := bookings.ProcessNotifications(ctx); err == nil {
		t.Fatal("delivery error was hidden")
	}
	var pending int64
	if err := f.database.Model(&models.ReservationMailOutbox{}).Where("sent_at IS NULL").Count(&pending).Error; err != nil || pending != 1 {
		t.Fatalf("failed delivery was not retained: %d %v", pending, err)
	}
	if err := f.database.Model(&models.ReservationMailOutbox{}).Where("sent_at IS NULL").Update("available_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	messages.fail = false
	if err := bookings.ProcessNotifications(ctx); err != nil {
		t.Fatal("send", err)
	}
	if len(messages.bodies) != 1 || !strings.Contains(messages.bodies[0], "/reservation#"+token) {
		var queued []models.ReservationMailOutbox
		if err := f.database.Find(&queued).Error; err != nil {
			t.Fatal(err)
		}
		var attempts int
		var sent, leased, available bool
		if len(queued) > 0 {
			attempts = queued[0].Attempts
			sent = queued[0].SentAt != nil
			leased = queued[0].LeaseUntil != nil
			available = !queued[0].AvailableAt.After(time.Now())
		}
		match := len(messages.bodies) > 0 && strings.Contains(messages.bodies[0], "/reservation#"+token)
		t.Fatalf("original management link not emailed: count=%d match=%v queue=%d attempts=%d sent=%v leased=%v available=%v", len(messages.bodies), match, len(queued), attempts, sent, leased, available)
	}
	view, err := bookings.Manage(ctx, token, "purchase")
	if err != nil || view.Status != "purchased" {
		t.Fatal("purchase", view, err)
	}
	view, err = bookings.Manage(ctx, token, "undo_purchase")
	if err != nil || view.Status != "reserved" {
		t.Fatal("undo", view, err)
	}
	if err := bookings.Recover(ctx, services.ReservationInput{WishID: wishID, ListID: listID, ListToken: listToken, Email: "wrong@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := bookings.Recover(ctx, services.ReservationInput{WishID: wishID, ListID: listID, ListToken: listToken, Email: input.Email}); err != nil {
		t.Fatal(err)
	}
	var recoveryCount int64
	if err := f.database.Model(&models.ReservationMailOutbox{}).Where("kind = ?", "recovery").Count(&recoveryCount).Error; err != nil {
		t.Fatal(err)
	}
	if recoveryCount != 1 {
		var current models.Reservation
		if err := f.database.Where("wish_id = ?", wishID).Take(&current).Error; err != nil {
			t.Fatal(err)
		}
		t.Fatalf("recovery not queued: count=%d guest=%v email_matches=%v status=%s expiry_in_future=%v", recoveryCount, current.UserID == nil, current.Email == input.Email, current.Status, current.ExpiresAt != nil && current.ExpiresAt.After(time.Now()))
	}
	if err := bookings.ProcessNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	if len(messages.bodies) != 2 || !strings.Contains(messages.bodies[len(messages.bodies)-1], "/reservation#"+token) {
		t.Fatalf("recovery did not resend same link: count %d", len(messages.bodies))
	}
	current := f.currentList(t, owner, listID)
	newDate := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02")
	f.listCall(t, "PUT", "/api/lists/"+listID, map[string]any{"title": "Occasion", "visibility": "link", "event_date": newDate, "revision": rev(current)}, owner, 200)
	if err := f.database.Where("wish_id = ?", wishID).Take(&booking).Error; err != nil {
		t.Fatal(err)
	}
	end, _ := reservation.EventEnd(newDate, "UTC")
	if booking.ExpiresAt == nil || !booking.ExpiresAt.Equal(end) {
		t.Fatalf("date did not shorten booking: %+v", booking.ExpiresAt)
	}
	if err := bookings.ProcessNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := bookings.Manage(ctx, token, "purchase"); err != nil {
		t.Fatal(err)
	}
	current = f.currentList(t, owner, listID)
	f.listCall(t, "PUT", "/api/lists/"+listID, map[string]any{"title": "Occasion", "visibility": "link", "event_date": nil, "revision": rev(current)}, owner, 200)
	if err := f.database.Where("wish_id = ?", wishID).Take(&booking).Error; err != nil || booking.Status != "purchased" {
		t.Fatal("removing date changed a purchase", err)
	}
	var removedDateCount int64
	if err := f.database.Model(&models.ReservationMailOutbox{}).Where("kind = ? AND event_date = ?", "date_changed", "").Count(&removedDateCount).Error; err != nil || removedDateCount != 1 {
		t.Fatalf("date removal was not queued for purchased gift: %d %v", removedDateCount, err)
	}
	if err := bookings.ProcessNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	current = f.currentList(t, owner, listID)
	f.listCall(t, "DELETE", "/api/lists/"+listID, map[string]any{"revision": rev(current), "message": "Thank you for thinking of me"}, owner, 200)
	if _, err := bookings.Manage(ctx, token, ""); err != services.ErrReservationExpired {
		t.Fatalf("deleted list retained reservation: %v", err)
	}
	if err := bookings.ProcessNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	remaining := f.currentList(t, owner, otherID)
	if len(remaining["wishes"].([]any)) != 1 {
		t.Fatal("list deletion removed shared wish")
	}
	if err := f.database.Exec("UPDATE wishlists SET event_date = '2000-01-01' WHERE id = ?", otherID).Error; err != nil {
		t.Fatal(err)
	}
	closed := f.listCall(t, "POST", "/api/public/lists/guest", map[string]string{"id": otherID}, nil, 200)
	if closed["list"].(map[string]any)["expired"] != true || len(closed["wishes"].([]any)) != 0 {
		t.Fatal("expired list disclosed wishes", closed)
	}
	if f.currentList(t, owner, otherID)["list"].(map[string]any)["expired"] != true {
		t.Fatal("owner did not see closed status")
	}
	if len(messages.bodies) < 4 {
		t.Fatalf("expected date and deletion notices, got %d", len(messages.bodies))
	}
	if !strings.Contains(messages.bodies[len(messages.bodies)-1], "Thank you for thinking of me") {
		t.Fatal("deletion message was not delivered")
	}
}

func TestIntegrationConcurrentReservationOfSameWish(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "race-owner@example.test", "raceowner")
	list := f.newList(t, owner, "Race", "public")
	listID := list["id"].(string)
	wishID := f.newWish(t, owner, "One gift", listID)["id"].(string)
	executor := db.NewExecutor(f.database)
	config := &security.Config{Origin: "https://wishix.test", Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, &reservationMessages{}, config)
	ctx, stop := context.WithTimeout(context.Background(), 120*time.Second)
	defer stop()
	inputs := []services.ReservationInput{{WishID: wishID, ListID: listID, Email: "race-a@example.test"}, {WishID: wishID, ListID: listID, Email: "race-b@example.test"}}
	for i := range inputs {
		challenge, err := bookings.Start(ctx, inputs[i], nil)
		if err != nil {
			t.Fatal(err)
		}
		inputs[i].Challenge, inputs[i].Code = challenge, f.mail.code(inputs[i].Email, "reservation")
	}
	results := make(chan error, len(inputs))
	var wg sync.WaitGroup
	for _, input := range inputs {
		wg.Add(1)
		go func(value services.ReservationInput) { defer wg.Done(); results <- bookings.Confirm(ctx, value, nil) }(input)
	}
	wg.Wait()
	close(results)
	successes, rejected := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if err == services.ErrReservationUnavailable {
			rejected++
		} else {
			t.Errorf("unexpected race result: %v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("race produced %d successes and %d rejections", successes, rejected)
	}
	var count int64
	if err := f.database.Model(&models.Reservation{}).Where("wish_id = ?", wishID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("reservation count = %d: %v", count, err)
	}
}

func TestIntegrationReservationLimitUsesOriginList(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "limit-owner@example.test", "limitowner")
	a := f.newList(t, owner, "First", "public")["id"].(string)
	b := f.newList(t, owner, "Second", "public")["id"].(string)
	executor := db.NewExecutor(f.database)
	config := &security.Config{Origin: "https://wishix.test", Secret: strings.Repeat("integration-secret", 3)}
	bookings := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, &reservationMessages{}, config)
	ctx, stop := context.WithTimeout(context.Background(), 120*time.Second)
	defer stop()
	create := func(wishID, listID string) error {
		input := services.ReservationInput{WishID: wishID, ListID: listID, Email: "one-guest@example.test"}
		var err error
		input.Challenge, err = bookings.Start(ctx, input, nil)
		if err != nil {
			return err
		}
		input.Code = f.mail.code(input.Email, "reservation")
		return bookings.Confirm(ctx, input, nil)
	}
	var ids []string
	for i := 0; i < 5; i++ {
		wish := f.newWish(t, owner, "Gift", a, b)
		ids = append(ids, wish["id"].(string))
	}
	for _, id := range ids[:4] {
		if err := create(id, a); err != nil {
			t.Fatal("first four:", err)
		}
	}
	var purchased models.Reservation
	if err := f.database.Where("wish_id = ?", ids[0]).Take(&purchased).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := bookings.Manage(ctx, reservation.ManagementToken(config.Secret, purchased.ID), "purchase"); err != nil {
		t.Fatal(err)
	}
	if err := create(ids[4], a); err != services.ErrReservationLimit {
		t.Fatalf("purchased must occupy slot: %v", err)
	}
	if err := create(ids[4], b); err != nil {
		t.Fatalf("different list should have separate limit: %v", err)
	}
	guestA := f.listCall(t, "POST", "/api/public/lists/guest", map[string]string{"id": a}, nil, 200)
	for _, item := range guestA["wishes"].([]any) {
		if item.(map[string]any)["reserved"] != true {
			t.Fatal("cross-list booking must mark same wish reserved")
		}
	}
}
