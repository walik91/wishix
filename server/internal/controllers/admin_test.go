package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	security "wishix/server/internal/auth"
	"wishix/server/internal/services"
)

type adminLimitRepo struct {
	services.AuthRepository
	deny bool
}

func (r *adminLimitRepo) Allow(context.Context, string, int, time.Time, time.Time) (bool, error) {
	return !r.deny, nil
}
func TestAdminAuthenticationAndOverview(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	config := &security.Config{Origin: "https://wishix.test", Secret: "test-secret", SecureCookie: true, AdminUsername: "admin", AdminPasswordHash: base64.RawURLEncoding.EncodeToString([]byte(security.HashPassword("12345")))}
	repo := &adminLimitRepo{}
	limiter := services.NewAuth(repo, nil, nil, config)
	admin := NewAdmin(config, &Auth{config: config}, limiter, database)
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux)
	call := func(method, path, body string, cookie *http.Cookie, origin bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		if origin {
			r.Header.Set("Origin", config.Origin)
			r.Header.Set("X-Wishix-Request", "1")
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/admin/overview", "", nil, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/admin/login", `{"username":"admin","password":"12345"}`, nil, false); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/admin/login", `{"username":"admin","password":"wrong"}`, nil, true); w.Code != 401 {
		t.Fatal(w.Code)
	}
	repo.deny = true
	if w := call("POST", "/api/admin/login", `{"username":"admin","password":"12345"}`, nil, true); w.Code != 429 {
		t.Fatal(w.Code)
	}
	repo.deny = false
	w := call("POST", "/api/admin/login", `{"username":"admin","password":"12345"}`, nil, true)
	if w.Code != 204 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/admin" {
		t.Fatal("unsafe cookie", cookie)
	}
	mock.ExpectQuery("SELECT ").WillReturnRows(sqlmock.NewRows([]string{"users", "wishes", "lists", "reservations"}).AddRow(1, 2, 3, 4))
	w = call("GET", "/api/admin/overview", "", cookie, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"reservations":4`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("overview %d %s", w.Code, w.Body.String())
	}
	if w = call("POST", "/api/admin/logout", "{}", cookie, false); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w = call("POST", "/api/admin/logout", "{}", cookie, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = call("GET", "/api/admin/overview", "", cookie, false); w.Code != 401 {
		t.Fatal("logout did not revoke session")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"", strings.Repeat("a", 600)} {
		config.AdminPasswordHash = base64.RawURLEncoding.EncodeToString([]byte(security.HashPassword(password)))
		body, err := json.Marshal(map[string]string{"username": "admin", "password": password})
		if err != nil {
			t.Fatal(err)
		}
		if w := call("POST", "/api/admin/login", string(body), nil, true); w.Code != http.StatusNoContent {
			t.Fatalf("login with password length %d: %d", len(password), w.Code)
		}
	}

}

func TestAccountDeletionWaitsForInflightRequests(t *testing.T) {
	admin := &Admin{}
	entered := make(chan struct{})
	release := make(chan struct{})
	deleted := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ordinary", func(w http.ResponseWriter, r *http.Request) { close(entered); <-release })
	mux.HandleFunc("POST /api/admin/users/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		admin.activity.Lock()
		defer admin.activity.Unlock()
		close(deleted)
	})
	handler := admin.Protect(mux)
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ordinary", nil))
		close(done)
	}()
	<-entered
	go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/admin/users/00000000-0000-4000-8000-000000000000/delete", nil))
	select {
	case <-deleted:
		t.Fatal("deletion did not wait for the active request")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-done
	select {
	case <-deleted:
	case <-time.After(time.Second):
		t.Fatal("deletion did not resume")
	}
}
