package controllers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"wishix/server/internal/services"
)

const googleStateCookie = "wishix_google_state"
const googlePendingCookie = "wishix_google_pending"

type Google struct {
	service *services.Google
	auth    *Auth
}

func NewGoogle(service *services.Google, auth *Auth) *Google {
	return &Google{service: service, auth: auth}
}

func (c *Google) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/google/status", c.status)
	mux.HandleFunc("POST /api/auth/google/start", c.auth.mutation("google-start", c.start))
	mux.HandleFunc("POST /api/auth/google/complete", c.auth.mutation("google-complete", c.complete))
	mux.HandleFunc("POST /api/auth/google/username-suggestions", c.auth.mutation("google-username-suggestions", c.usernameSuggestions))
	mux.HandleFunc("GET /api/auth/google/callback", c.callback)
}
func (c *Google) usernameSuggestions(w http.ResponseWriter, r *http.Request, _ authRequest) error {
	suggestions, err := c.service.UsernameSuggestions(r.Context(), c.read(r, googlePendingCookie))
	if err != nil {
		return err
	}
	c.auth.write(w, http.StatusOK, map[string]any{"suggestions": suggestions})
	return nil
}

func (c *Google) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: c.auth.config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (c *Google) read(r *http.Request, name string) string {
	value, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return value.Value
}
func (c *Google) status(w http.ResponseWriter, r *http.Request) {
	linked, err := c.service.Linked(r.Context(), c.auth.token(r))
	if err != nil && !errors.Is(err, services.ErrSession) {
		c.auth.failure(w, err)
		return
	}
	c.auth.write(w, http.StatusOK, map[string]any{"enabled": c.service.Enabled(), "linked": linked})
}
func (c *Google) start(w http.ResponseWriter, r *http.Request, input authRequest) error {
	state, destination, err := c.service.Start(r.Context(), input.Mode, c.auth.token(r))
	if err != nil {
		return err
	}
	c.cookie(w, googleStateCookie, state, 600)
	c.auth.write(w, http.StatusOK, map[string]string{"url": destination})
	return nil
}
func (c *Google) complete(w http.ResponseWriter, r *http.Request, input authRequest) error {
	result, err := c.service.Complete(r.Context(), c.read(r, googlePendingCookie), input.Username, c.auth.token(r))
	if err != nil {
		return err
	}
	c.cookie(w, googlePendingCookie, "", -1)
	c.auth.authenticated(w, result)
	return nil
}
func (c *Google) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	state, stored := r.URL.Query().Get("state"), c.read(r, googleStateCookie)
	c.cookie(w, googleStateCookie, "", -1)
	if len(state) != 43 || len(stored) != 43 || subtle.ConstantTimeCompare([]byte(state), []byte(stored)) != 1 {
		http.Redirect(w, r, "/login?google=error", http.StatusSeeOther)
		return
	}
	result, err := c.service.Callback(r.Context(), state, c.auth.token(r), r.URL.Query().Get("code"))
	if err != nil {
		destination := "/login?google=error"
		if errors.Is(err, services.ErrRegistrationUnavailable) {
			destination = "/login?google=conflict"
		}
		if _, sessionErr := c.auth.service.Current(r.Context(), c.auth.token(r)); sessionErr == nil {
			destination = "/my?google=error"
			if errors.Is(err, services.ErrRegistrationUnavailable) {
				destination = "/my?google=conflict"
			}
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}
	switch result.Mode {
	case "link":
		http.Redirect(w, r, "/my?google=linked", http.StatusSeeOther)
	case "register":
		c.cookie(w, googlePendingCookie, result.Pending, int((10 * time.Minute).Seconds()))
		http.Redirect(w, r, "/google/finish", http.StatusSeeOther)
	case "login":
		c.auth.setSessionCookie(w, result.Authenticated)
		http.Redirect(w, r, "/login?google=success", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/login?google=error", http.StatusSeeOther)
	}
}
