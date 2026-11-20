package controllers

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm"
	security "wishix/server/internal/auth"
	"wishix/server/internal/services"
)

const adminCookie = "wishix_admin_session"
const adminSessionLifetime = 8 * time.Hour

type Admin struct {
	config   *security.Config
	auth     *Auth
	limiter  *services.Auth
	db       *gorm.DB
	mu       sync.Mutex
	sessions map[string]time.Time
}

func NewAdmin(config *security.Config, auth *Auth, limiter *services.Auth, db *gorm.DB) *Admin {
	return &Admin{config: config, auth: auth, limiter: limiter, db: db, sessions: make(map[string]time.Time)}
}
func (a *Admin) RegisterRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/admin/login", a.login)
	m.HandleFunc("POST /api/admin/logout", a.logout)
	m.HandleFunc("GET /api/admin/overview", a.overview)
}
func (a *Admin) headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Content-Type", "application/json")
}
func (a *Admin) mutation(w http.ResponseWriter, r *http.Request) bool {
	a.headers(w)
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Header.Get("Origin") != a.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return false
	}
	if media != "application/json" {
		http.Error(w, `{"error":"invalid_input"}`, http.StatusUnsupportedMediaType)
		return false
	}
	return true
}
func (a *Admin) login(w http.ResponseWriter, r *http.Request) {
	if !a.mutation(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF || len(input.Username) > 256 {
		http.Error(w, `{"error":"invalid_input"}`, 400)
		return
	}
	if err := a.limiter.LimitAdminLogin(r.Context(), a.auth.clientIP(r)); err != nil {
		if errors.Is(err, services.ErrRateLimit) {
			http.Error(w, `{"error":"rate_limited"}`, 429)
		} else {
			http.Error(w, `{"error":"unavailable"}`, 503)
		}
		return
	}
	hash, err := base64.RawURLEncoding.DecodeString(a.config.AdminPasswordHash)
	valid := err == nil && a.config.AdminUsername != "" && len(hash) > 0 && security.CheckPassword(input.Password, string(hash))
	if !valid || subtle.ConstantTimeCompare([]byte(input.Username), []byte(a.config.AdminUsername)) != 1 {
		http.Error(w, `{"error":"invalid_credentials"}`, 401)
		return
	}
	token := security.Token()
	now := time.Now()
	a.mu.Lock()
	for key, expiry := range a.sessions {
		if !expiry.After(now) {
			delete(a.sessions, key)
		}
	}
	a.sessions[security.Digest(token)] = now.Add(adminSessionLifetime)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: token, Path: "/api/admin", HttpOnly: true, Secure: a.config.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: int(adminSessionLifetime.Seconds())})
	w.WriteHeader(http.StatusNoContent)
}
func (a *Admin) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookie)
	if err != nil {
		return false
	}
	key := security.Digest(cookie.Value)
	a.mu.Lock()
	defer a.mu.Unlock()
	expiry, ok := a.sessions[key]
	if ok && !expiry.After(time.Now()) {
		delete(a.sessions, key)
		return false
	}
	return ok
}
func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	if !a.mutation(w, r) {
		return
	}
	if cookie, err := r.Cookie(adminCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, security.Digest(cookie.Value))
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Path: "/api/admin", HttpOnly: true, Secure: a.config.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
func (a *Admin) overview(w http.ResponseWriter, r *http.Request) {
	a.headers(w)
	if !a.authenticated(r) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	var counts struct {
		Users        int64 `json:"users"`
		Wishes       int64 `json:"wishes"`
		Lists        int64 `json:"lists"`
		Reservations int64 `json:"reservations"`
	}
	err := a.db.WithContext(r.Context()).Raw(`SELECT (SELECT count(*) FROM users) AS users, (SELECT count(*) FROM wishes) AS wishes, (SELECT count(*) FROM wishlists) AS lists, (SELECT count(*) FROM reservations WHERE status = 'purchased' OR (status = 'reserved' AND expires_at > now())) AS reservations`).Scan(&counts).Error
	if err != nil {
		http.Error(w, `{"error":"unavailable"}`, 503)
		return
	}
	_ = json.NewEncoder(w).Encode(counts)
}
