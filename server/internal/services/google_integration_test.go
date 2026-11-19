//go:build integration

package services_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	security "wishix/server/internal/auth"
	"wishix/server/internal/controllers"
	"wishix/server/internal/db"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationGoogleHTTPRegistration(t *testing.T) {
	f := newAuthFixture(t)
	provider := &fakeGoogle{identity: security.GoogleIdentity{Subject: "http-google", Email: "http-google@example.test", EmailVerified: true}}
	google := services.NewGoogle(f.auth, repositories.NewGoogle(db.NewExecutor(f.database)), provider)
	config := &security.Config{Origin: "https://wishix.test", SecureCookie: true}
	mux := http.NewServeMux()
	controllers.NewGoogle(google, controllers.NewAuth(f.auth, config, slog.New(slog.NewTextHandler(io.Discard, nil)))).RegisterRoutes(mux)
	post := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Origin", config.Origin)
		req.Header.Set("X-Wishix-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.44:1234"
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	start := post("/api/auth/google/start", `{"mode":"login"}`, nil)
	var startBody struct {
		URL string `json:"url"`
	}
	if start.Code != 200 || json.Unmarshal(start.Body.Bytes(), &startBody) != nil {
		t.Fatal("Google start failed")
	}
	authorization, err := url.Parse(startBody.URL)
	if err != nil || len(start.Result().Cookies()) != 1 {
		t.Fatal("no state cookie")
	}
	stateCookie := start.Result().Cookies()[0]
	if !stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe state cookie")
	}
	callback := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state="+url.QueryEscape(authorization.Query().Get("state"))+"&code=secret-code", nil)
	callback.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, callback)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/google/finish" || strings.Contains(rec.Header().Get("Location"), "secret") {
		t.Fatal("callback failed or exposed its code")
	}
	var pending *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "wishix_google_pending" {
			pending = cookie
		}
	}
	if pending == nil || !pending.HttpOnly || !pending.Secure {
		t.Fatal("missing secure registration cookie")
	}
	finish := post("/api/auth/google/complete", `{"username":"http_google"}`, pending)
	if finish.Code != http.StatusOK {
		t.Fatalf("Google signup failed: %d %s", finish.Code, finish.Body.String())
	}
	var session *http.Cookie
	for _, cookie := range finish.Result().Cookies() {
		if cookie.Name == "wishix_session" {
			session = cookie
		}
	}
	if session == nil || !session.HttpOnly {
		t.Fatal("missing session cookie")
	}
	if user, err := f.auth.Current(context.Background(), session.Value); err != nil || user.Email != "http-google@example.test" || user.DisplayName != "http_google" {
		t.Fatal("registration session is not usable")
	}
}

type fakeGoogle struct{ identity security.GoogleIdentity }

func (f *fakeGoogle) Enabled() bool { return true }
func (f *fakeGoogle) AuthorizationURL(state, verifier string) string {
	return "https://accounts.google.com/?state=" + state
}
func (f *fakeGoogle) Exchange(context.Context, string, string) (security.GoogleIdentity, error) {
	return f.identity, nil
}

func TestIntegrationGoogleSignupLinkAndSafety(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	provider := &fakeGoogle{identity: security.GoogleIdentity{Subject: "google-new", Email: "new@example.test", EmailVerified: true}}
	google := services.NewGoogle(f.auth, repositories.NewGoogle(db.NewExecutor(f.database)), provider)
	state, _, err := google.Start(ctx, "login", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := google.Callback(ctx, state, "", "code")
	if err != nil || result.Mode != "register" || len(result.Pending) != 43 {
		t.Fatalf("new account flow: %v", err)
	}
	if _, err := google.Callback(ctx, state, "", "code"); err == nil {
		t.Fatal("OAuth state reused")
	}
	account, err := google.Complete(ctx, result.Pending, "google_user", "")
	if err != nil || account.User.Email != "new@example.test" || account.User.PasswordHash != "" {
		t.Fatalf("Google signup: %v", err)
	}
	if _, err := google.Complete(ctx, result.Pending, "other_user", ""); err == nil {
		t.Fatal("signup token reused")
	}
	state, _, _ = google.Start(ctx, "login", "")
	signedIn, err := google.Callback(ctx, state, "", "code")
	if err != nil || signedIn.Mode != "login" || signedIn.Authenticated.User.ID != account.User.ID {
		t.Fatalf("Google sign in: %v", err)
	}
	if _, err := f.auth.Login(ctx, "new@example.test", "anything"); err != services.ErrCredentials {
		t.Fatal("social account accepted password")
	}
	challenge, err := f.auth.ForgotPassword(ctx, "new@example.test", "en")
	if err != nil || challenge == "" || f.mail.code("new@example.test", "reset") != "" {
		t.Fatal("social account received password reset code")
	}

	existing := f.account(t, "existing@example.test", "existing_user")
	provider.identity = security.GoogleIdentity{Subject: "google-existing", Email: "existing@example.test", EmailVerified: true}
	state, _, _ = google.Start(ctx, "login", "")
	if _, err := google.Callback(ctx, state, "", "code"); err != services.ErrRegistrationUnavailable {
		t.Fatal("email collision merged accounts")
	}
	state, _, err = google.Start(ctx, "link", existing.Value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := google.Callback(ctx, state, account.Token, "code"); err != services.ErrSession {
		t.Fatal("link accepted a different session")
	}
	state, _, _ = google.Start(ctx, "link", existing.Value)
	linked, err := google.Callback(ctx, state, existing.Value, "code")
	if err != nil || linked.Mode != "link" {
		t.Fatalf("link failed: %v", err)
	}
	state, _, _ = google.Start(ctx, "login", "")
	login, err := google.Callback(ctx, state, "", "code")
	current, _ := f.auth.Current(ctx, existing.Value)
	if err != nil || login.Authenticated.User.ID != current.ID {
		t.Fatal("linked Google opened another account")
	}
	state, _, _ = google.Start(ctx, "link", account.Token)
	if _, err := google.Callback(ctx, state, account.Token, "code"); err != services.ErrRegistrationUnavailable {
		t.Fatal("Google identity linked twice")
	}

	provider.identity = security.GoogleIdentity{Subject: "unverified", Email: "unverified@example.test", EmailVerified: false}
	state, _, _ = google.Start(ctx, "login", "")
	if _, err := google.Callback(ctx, state, "", "code"); err == nil {
		t.Fatal("unverified Google email accepted")
	}
}

func TestIntegrationGooglePendingConsumedOnce(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	provider := &fakeGoogle{identity: security.GoogleIdentity{Subject: "race", Email: "race-google@example.test", EmailVerified: true}}
	google := services.NewGoogle(f.auth, repositories.NewGoogle(db.NewExecutor(f.database)), provider)
	state, _, _ := google.Start(ctx, "login", "")
	pending, err := google.Callback(ctx, state, "", "code")
	if err != nil {
		t.Fatal(err)
	}
	var succeeded atomic.Int32
	var wg sync.WaitGroup
	for _, username := range []string{"race_first", "race_second"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if _, err := google.Complete(ctx, pending.Pending, name, ""); err == nil {
				succeeded.Add(1)
			}
		}(username)
	}
	wg.Wait()
	if succeeded.Load() != 1 {
		t.Fatalf("%d accounts created", succeeded.Load())
	}
}
