//go:build integration

package services_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/controllers"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationAdminSoftDelete(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "deleted-owner@example.test", "deletedowner")
	other := f.account(t, "other-owner@example.test", "otherowner")
	list := f.newList(t, owner, "Shared list", "link")
	listID := list["id"].(string)
	parts := strings.Split(list["share_path"].(string), "/")
	token := parts[len(parts)-1]
	wish := f.newWish(t, owner, "Hidden gift", listID)
	wishID := wish["id"].(string)
	otherList := f.newList(t, other, "Another list", "public")
	otherListID := otherList["id"].(string)
	otherWish := f.newWish(t, other, "Other gift", otherListID)
	otherWishID := otherWish["id"].(string)
	var deleted models.User
	if err := f.database.Where("username = ?", "deletedowner").Take(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Model(&models.User{}).Where("id = ?", deleted.ID).Update("show_all_wishes", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Exec("INSERT INTO profile_avatars (user_id, version, data) VALUES (?, ?, ?)", deleted.ID, security.UUID(), []byte("avatar")).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Create(&models.WishImage{WishID: wishID, Version: security.UUID(), Data: []byte("image")}).Error; err != nil {
		t.Fatal(err)
	}
	if response := f.imageRequest(t, "GET", "/api/public/profiles/deletedowner/avatar", nil, "", nil, ""); response.Code != 200 {
		t.Fatal("avatar not public before delete", response.Code)
	}
	if err := f.database.Create(&models.AuthGoogleIdentity{Subject: "removed-google-id", UserID: deleted.ID, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Create(&models.AuthFacebookIdentity{Subject: "removed-facebook-id", UserID: deleted.ID, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	bookingID := security.UUID()
	guestBookingID := security.UUID()
	bookings := []models.Reservation{
		{ID: bookingID, WishID: otherWishID, ListID: &otherListID, ContextKey: "list:" + otherListID, Email: deleted.Email, Locale: "en", UserID: &deleted.ID, TokenHash: security.Digest("other-token"), Status: "reserved", ExpiresAt: &expiry, CreatedAt: time.Now()},
		{ID: guestBookingID, WishID: wishID, ListID: &listID, ContextKey: "list:" + listID, Email: "guest@example.test", Locale: "en", TokenHash: security.Digest("guest-token"), Status: "reserved", ExpiresAt: &expiry, CreatedAt: time.Now()},
	}
	for _, booking := range bookings {
		if err := f.database.Create(&booking).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.database.Create(&models.ReservationMailOutbox{ID: security.UUID(), Recipient: "guest@example.test", Locale: "en", Kind: "created", WishTitle: "Hidden gift", ReservationID: &guestBookingID, CreatedAt: time.Now(), AvailableAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	config := &security.Config{Origin: "https://wishix.test", SecureCookie: true, Secret: strings.Repeat("integration-secret", 3), AdminUsername: "admin", AdminPasswordHash: base64.RawURLEncoding.EncodeToString([]byte(security.HashPassword("secret")))}
	admin := controllers.NewAdmin(config, controllers.NewAuth(f.auth, config, slog.New(slog.NewTextHandler(io.Discard, nil))), f.auth, f.database)
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux)
	call := func(method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "192.0.2.2:1234"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Wishix-Request", "1")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/admin/users", "", nil, config.Origin); w.Code != 401 {
		t.Fatal("anonymous users", w.Code)
	}
	if w := call("POST", "/api/admin/users/"+deleted.ID+"/delete", "{}", nil, config.Origin); w.Code != 401 {
		t.Fatal("anonymous delete", w.Code)
	}
	login := call("POST", "/api/admin/login", `{"username":"admin","password":"secret"}`, nil, config.Origin)
	if login.Code != 204 {
		t.Fatal("admin login", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if w := call("POST", "/api/admin/users/"+deleted.ID+"/delete", "{}", cookie, "https://attacker.test"); w.Code != 403 {
		t.Fatal("cross origin", w.Code)
	}
	listed := call("GET", "/api/admin/users", "", cookie, config.Origin)
	var listResult struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listResult); err != nil || len(listResult.Users) != 2 {
		t.Fatal("users listing", listed.Code, err)
	}
	for _, user := range listResult.Users {
		if _, ok := user["password_hash"]; ok {
			t.Fatal("password hash returned")
		}
	}
	// A conflict halfway through the transaction must roll back every change.
	if err := f.database.Exec("INSERT INTO admin_deleted_reservations (reservation_id, deleted_for_user_id, archived_at, reservation) VALUES (?, ?, now(), '{}'::JSONB)", bookingID, deleted.ID).Error; err != nil {
		t.Fatal(err)
	}
	if w := call("POST", "/api/admin/users/"+deleted.ID+"/delete", "{}", cookie, config.Origin); w.Code != 503 {
		t.Fatal("archive conflict did not fail", w.Code)
	}
	if status, _, _ := f.request(t, "me", nil, owner); status != 200 {
		t.Fatal("failed deletion revoked session", status)
	}
	var unsuppressed int64
	if err := f.database.Table("reservation_mail_outbox").Where("suppressed_at IS NULL").Count(&unsuppressed).Error; err != nil || unsuppressed != 1 {
		t.Fatal("failed deletion was not rolled back", err)
	}
	if err := f.database.Exec("DELETE FROM admin_deleted_reservations WHERE reservation_id = ?", bookingID).Error; err != nil {
		t.Fatal(err)
	}
	if w := call("POST", "/api/admin/users/"+deleted.ID+"/delete", "{}", cookie, config.Origin); w.Code != 204 {
		t.Fatal("delete", w.Code, w.Body.String())
	}
	listedAfter := call("GET", "/api/admin/users", "", cookie, config.Origin)
	var activeAfter struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.Unmarshal(listedAfter.Body.Bytes(), &activeAfter); err != nil || len(activeAfter.Users) != 1 || activeAfter.Users[0]["username"] != "otherowner" {
		t.Fatal("deleted user still visible to admin", listedAfter.Code, err)
	}
	if user, err := f.repo.UserByEmail(context.Background(), deleted.Email); err != nil || user != nil {
		t.Fatal("deleted user still visible to GORM", err)
	}
	var activeWishes, activeLists int64
	if err := f.database.Model(&models.Wish{}).Where("id = ?", wishID).Count(&activeWishes).Error; err != nil || activeWishes != 0 {
		t.Fatal("deleted wish still visible to GORM", err)
	}
	if err := f.database.Model(&models.Wishlist{}).Where("id = ?", listID).Count(&activeLists).Error; err != nil || activeLists != 0 {
		t.Fatal("deleted list still visible to GORM", err)
	}
	var deletedWish models.Wish
	var deletedList models.Wishlist
	if err := f.database.Unscoped().Where("id = ?", wishID).Take(&deletedWish).Error; err != nil || !deletedWish.DeletedAt.Valid {
		t.Fatal("wish not soft deleted", err)
	}
	if err := f.database.Unscoped().Where("id = ?", listID).Take(&deletedList).Error; err != nil || !deletedList.DeletedAt.Valid {
		t.Fatal("list not soft deleted", err)
	}
	if w := call("POST", "/api/admin/users/"+deleted.ID+"/delete", "{}", cookie, config.Origin); w.Code != 404 {
		t.Fatal("repeat delete", w.Code)
	}
	if status, _, _ := f.request(t, "me", nil, owner); status != 401 {
		t.Fatal("deleted session remains valid", status)
	}
	if status, _, _ := f.request(t, "login", map[string]string{"email": deleted.Email, "password": initialPassword}, nil); status != 401 {
		t.Fatal("deleted user can login", status)
	}
	f.listCall(t, "GET", "/api/public/profiles/deletedowner", nil, nil, 404)
	f.listCall(t, "GET", "/api/public/profiles/deletedowner/details", nil, nil, 404)
	f.listCall(t, "GET", "/api/public/profiles/deletedowner/wishes", nil, nil, 404)
	if response := f.imageRequest(t, "GET", "/api/public/profiles/deletedowner/avatar", nil, "", nil, ""); response.Code != 404 {
		t.Fatal("deleted avatar exposed", response.Code)
	}
	f.listCall(t, "POST", "/api/public/lists/guest-image", map[string]string{"id": listID, "token": token, "wish_id": wishID}, nil, 404)
	f.listCall(t, "POST", "/api/public/lists/image", map[string]string{"token": token, "wish_id": wishID}, nil, 404)
	if occupied, err := f.repo.EmailTaken(context.Background(), deleted.Email); err != nil || occupied {
		t.Fatal("deleted email is not reusable", err)
	}
	if occupied, err := f.repo.UsernameExists(context.Background(), deleted.Username); err != nil || occupied {
		t.Fatal("deleted username is not reusable", err)
	}

	f.listCall(t, "POST", "/api/public/lists/guest", map[string]string{"id": listID, "token": token}, nil, 404)
	f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": token}, nil, 404)
	search := f.listCall(t, "GET", "/api/search/people?q=deletedowner", nil, other, 200)
	if len(search["people"].([]any)) != 0 {
		t.Fatal("deleted account searchable")
	}
	var archived, remaining, preserved, suppressed int64
	if err := f.database.Table("admin_deleted_reservations").Count(&archived).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Table("reservations").Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Table("wishes").Where("id IN (?,?)", wishID, otherWishID).Count(&preserved).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Table("reservation_mail_outbox").Where("suppressed_at IS NOT NULL").Count(&suppressed).Error; err != nil {
		t.Fatal(err)
	}
	if archived != 2 || remaining != 0 || preserved != 2 || suppressed != 1 {
		t.Fatalf("archive=%d remaining=%d wishes=%d suppressed=%d", archived, remaining, preserved, suppressed)
	}
	var kept models.User
	if err := f.database.Unscoped().Where("id = ?", deleted.ID).Take(&kept).Error; err != nil || !kept.DeletedAt.Valid {
		t.Fatal("user not retained", err)
	}
	// Social sign-in must never recover a deleted account.
	executor := db.NewExecutor(f.database)
	if u, err := repositories.NewGoogle(executor).UserBySubject(context.Background(), "removed-google-id"); err != nil || u != nil {
		t.Fatal("deleted Google identity resolved", err)
	}
	if u, err := repositories.NewFacebook(executor).UserBySubject(context.Background(), "removed-facebook-id"); err != nil || u != nil {
		t.Fatal("deleted Facebook identity resolved", err)
	}
	// A deleted account is no longer treated as an active account when booking as a guest.
	msgs := &reservationMessages{}
	svc := services.NewReservations(repositories.NewReservations(executor), db.NewTransactionManager(executor), f.auth, f.mail, msgs, config)
	if err := svc.ProcessNotifications(context.Background()); err != nil || len(msgs.bodies) != 0 {
		t.Fatal("suppressed mail was delivered", err)
	}
	input := services.ReservationInput{WishID: otherWishID, ListID: otherListID, Email: deleted.Email, Locale: "en"}
	challenge, err := svc.Start(context.Background(), input, nil)
	if err != nil {
		t.Fatal("start new booking", err)
	}
	input.Challenge = challenge
	input.Code = f.mail.code(input.Email, "reservation")
	if err = svc.Confirm(context.Background(), input, nil); err != nil {
		t.Fatal("book released gift", err)
	}
}
