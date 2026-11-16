package controllers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"wishix/server/internal/services"
)

const facebookStateCookie = "wishix_facebook_state"
const facebookPendingCookie = "wishix_facebook_pending"

type Facebook struct {
	service *services.Facebook
	auth    *Auth
}

func NewFacebook(service *services.Facebook, auth *Auth) *Facebook {
	return &Facebook{service: service, auth: auth}
}

func (c *Facebook) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/facebook/status", c.status)
	mux.HandleFunc("POST /api/auth/facebook/start", c.auth.mutation("facebook-start", c.start))
	mux.HandleFunc("POST /api/auth/facebook/complete", c.auth.mutation("facebook-complete", c.complete))
	mux.HandleFunc("GET /api/auth/facebook/callback", c.callback)
}

func (c *Facebook) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: c.auth.config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (c *Facebook) read(r *http.Request, name string) string {
	value, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return value.Value
}
func (c *Facebook) status(w http.ResponseWriter, r *http.Request) {
	linked, err := c.service.Linked(r.Context(), c.auth.token(r))
	if err != nil && !errors.Is(err, services.ErrSession) {
		c.auth.failure(w, err)
		return
	}
	c.auth.write(w, http.StatusOK, map[string]any{"enabled": c.service.Enabled(), "linked": linked})
}
func (c *Facebook) start(w http.ResponseWriter, r *http.Request, input authRequest) error {
	state, destination, err := c.service.Start(r.Context(), input.Mode, c.auth.token(r))
	if err != nil {
		return err
	}
	c.cookie(w, facebookStateCookie, state, 600)
	c.auth.write(w, http.StatusOK, map[string]string{"url": destination})
	return nil
}
func (c *Facebook) complete(w http.ResponseWriter, r *http.Request, input authRequest) error {
	result, err := c.service.Complete(r.Context(), c.read(r, facebookPendingCookie), input.Username, c.auth.token(r))
	if err != nil {
		return err
	}
	c.cookie(w, facebookPendingCookie, "", -1)
	c.auth.authenticated(w, result)
	return nil
}
func (c *Facebook) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	state, stored := r.URL.Query().Get("state"), c.read(r, facebookStateCookie)
	c.cookie(w, facebookStateCookie, "", -1)
	if len(state) != 43 || len(stored) != 43 || subtle.ConstantTimeCompare([]byte(state), []byte(stored)) != 1 {
		http.Redirect(w, r, "/login?facebook=error", http.StatusSeeOther)
		return
	}
	result, err := c.service.Callback(r.Context(), state, c.auth.token(r), r.URL.Query().Get("code"))
	if err != nil {
		destination := "/login?facebook=error"
		if errors.Is(err, services.ErrRegistrationUnavailable) {
			destination = "/login?facebook=conflict"
		}
		if _, sessionErr := c.auth.service.Current(r.Context(), c.auth.token(r)); sessionErr == nil {
			destination = "/my?facebook=error"
			if errors.Is(err, services.ErrRegistrationUnavailable) {
				destination = "/my?facebook=conflict"
			}
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}
	switch result.Mode {
	case "link":
		http.Redirect(w, r, "/my?facebook=linked", http.StatusSeeOther)
	case "register":
		c.cookie(w, facebookPendingCookie, result.Pending, int((10 * time.Minute).Seconds()))
		http.Redirect(w, r, "/facebook/finish", http.StatusSeeOther)
	case "login":
		c.auth.setSessionCookie(w, result.Authenticated)
		http.Redirect(w, r, "/login?facebook=success", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/login?facebook=error", http.StatusSeeOther)
	}
}
