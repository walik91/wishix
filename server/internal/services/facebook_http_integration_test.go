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
	"testing"

	security "wishix/server/internal/auth"
	"wishix/server/internal/controllers"
	"wishix/server/internal/db"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationFacebookHTTPRegistration(t *testing.T) {
	f := newAuthFixture(t)
	provider := &fakeFacebook{identity: security.FacebookIdentity{Subject: "http-facebook", Email: "http-facebook@example.test"}}
	facebook := services.NewFacebook(f.auth, repositories.NewFacebook(db.NewExecutor(f.database)), provider)
	config := &security.Config{Origin: "https://wishix.test", SecureCookie: true}
	mux := http.NewServeMux()
	controllers.NewFacebook(facebook, controllers.NewAuth(f.auth, config, slog.New(slog.NewTextHandler(io.Discard, nil)))).RegisterRoutes(mux)
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
	start := post("/api/auth/facebook/start", `{"mode":"login"}`, nil)
	var startBody struct {
		URL string `json:"url"`
	}
	if start.Code != 200 || json.Unmarshal(start.Body.Bytes(), &startBody) != nil {
		t.Fatal("Facebook start failed")
	}
	authorization, err := url.Parse(startBody.URL)
	if err != nil || len(start.Result().Cookies()) != 1 {
		t.Fatal("no state cookie")
	}
	stateCookie := start.Result().Cookies()[0]
	if !stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe state cookie")
	}
	callback := httptest.NewRequest(http.MethodGet, "/api/auth/facebook/callback?state="+url.QueryEscape(authorization.Query().Get("state"))+"&code=secret-code", nil)
	callback.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, callback)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/facebook/finish" || strings.Contains(rec.Header().Get("Location"), "secret") {
		t.Fatal("callback failed or exposed its code")
	}
	var pending *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "wishix_facebook_pending" {
			pending = cookie
		}
	}
	if pending == nil || !pending.HttpOnly || !pending.Secure {
		t.Fatal("missing secure registration cookie")
	}
	finish := post("/api/auth/facebook/complete", `{"username":"http_facebook"}`, pending)
	if finish.Code != http.StatusOK {
		t.Fatalf("Facebook signup failed: %d %s", finish.Code, finish.Body.String())
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
	if user, err := f.auth.Current(context.Background(), session.Value); err != nil || user.Email != "http-facebook@example.test" {
		t.Fatal("registration session is not usable")
	}
}
