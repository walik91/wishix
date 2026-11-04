package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

const sessionCookie = "wishix_session"

type Auth struct {
	service *services.Auth
	config  *security.Config
	logger  *slog.Logger
}

func NewAuth(service *services.Auth, config *security.Config, logger *slog.Logger) *Auth {
	return &Auth{service: service, config: config, logger: logger}
}

type authRequest struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Locale    string `json:"locale"`
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

// Explicit private account DTO: never serialize persistence models or hashes.
type accountResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func privateAccount(user *models.User) accountResponse {
	return accountResponse{ID: user.ID, Username: user.Username, Email: user.Email}
}

func (c *Auth) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", c.mutation("register", c.register))
	mux.HandleFunc("POST /api/auth/verify-email", c.mutation("verify-email", c.verify))
	mux.HandleFunc("POST /api/auth/resend", c.mutation("resend", c.resend))
	mux.HandleFunc("POST /api/auth/login", c.mutation("login", c.login))
	mux.HandleFunc("POST /api/auth/logout", c.mutation("logout", c.logout))
	mux.HandleFunc("POST /api/auth/forgot-password", c.mutation("forgot-password", c.forgot))
	mux.HandleFunc("POST /api/auth/reset-password", c.mutation("reset-password", c.reset))
	mux.HandleFunc("GET /api/auth/me", c.me)
}

func (c *Auth) mutation(action string, handler func(http.ResponseWriter, *http.Request, authRequest) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		// Fixed configured origin, not Host or client-supplied proxy headers.
		// Custom header + JSON block cross-origin forms, including login CSRF.
		if r.Header.Get("Origin") != c.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			c.write(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if mediaType != "application/json" {
			c.write(w, http.StatusUnsupportedMediaType, map[string]string{"error": "invalid_input"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input authRequest
		if err := decoder.Decode(&input); err != nil {
			c.failure(w, services.ErrInvalidInput)
			return
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			c.failure(w, services.ErrInvalidInput)
			return
		}
		if err := c.service.LimitIP(r.Context(), action, c.clientIP(r)); err != nil {
			c.failure(w, err)
			return
		}
		if err := handler(w, r, input); err != nil {
			c.failure(w, err)
		}
	}
}

func (c *Auth) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "unknown"
	}
	for _, network := range c.config.TrustedProxies {
		if network.Contains(ip) {
			// Both supported proxies overwrite this with the socket peer IP.
			if real := net.ParseIP(r.Header.Get("X-Real-IP")); real != nil {
				ip = real
			}
			break
		}
	}
	if ip.To4() == nil {
		ip = ip.Mask(net.CIDRMask(64, 128))
	}
	return ip.String()
}

func (c *Auth) register(w http.ResponseWriter, r *http.Request, input authRequest) error {
	challenge, err := c.service.Register(r.Context(), input.Email, input.Username, input.Password, input.Locale)
	if err != nil {
		return err
	}
	c.write(w, http.StatusAccepted, map[string]any{"challenge": challenge, "resend_after": 60})
	return nil
}
func (c *Auth) verify(w http.ResponseWriter, r *http.Request, input authRequest) error {
	result, err := c.service.Verify(r.Context(), input.Challenge, input.Code)
	if err != nil {
		return err
	}
	c.authenticated(w, result)
	return nil
}
func (c *Auth) resend(w http.ResponseWriter, r *http.Request, input authRequest) error {
	if err := c.service.Resend(r.Context(), input.Challenge); err != nil {
		return err
	}
	c.write(w, http.StatusOK, map[string]any{"status": "ok", "resend_after": 60})
	return nil
}
func (c *Auth) login(w http.ResponseWriter, r *http.Request, input authRequest) error {
	result, err := c.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		return err
	}
	if err := c.service.Logout(r.Context(), c.token(r)); err != nil {
		return err
	}
	c.authenticated(w, result)
	return nil
}
func (c *Auth) logout(w http.ResponseWriter, r *http.Request, _ authRequest) error {
	if err := c.service.Logout(r.Context(), c.token(r)); err != nil {
		return err
	}
	c.clearCookie(w)
	c.write(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
func (c *Auth) forgot(w http.ResponseWriter, r *http.Request, input authRequest) error {
	challenge, err := c.service.ForgotPassword(r.Context(), input.Email, input.Locale)
	if err != nil {
		return err
	}
	c.write(w, http.StatusAccepted, map[string]any{"challenge": challenge, "resend_after": 60})
	return nil
}
func (c *Auth) reset(w http.ResponseWriter, r *http.Request, input authRequest) error {
	if err := c.service.ResetPassword(r.Context(), input.Challenge, input.Code, input.Password); err != nil {
		return err
	}
	c.clearCookie(w)
	c.write(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
func (c *Auth) me(w http.ResponseWriter, r *http.Request) {
	user, err := c.service.Current(r.Context(), c.token(r))
	if err != nil {
		c.failure(w, err)
		return
	}
	c.write(w, http.StatusOK, map[string]any{"user": privateAccount(user)})
}
func (c *Auth) token(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func (c *Auth) authenticated(w http.ResponseWriter, result services.Authenticated) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: result.Token, Path: "/", HttpOnly: true, Secure: c.config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60, Expires: result.ExpiresAt})
	c.write(w, http.StatusOK, map[string]any{"user": privateAccount(result.User)})
}
func (c *Auth) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: c.config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func (c *Auth) failure(w http.ResponseWriter, err error) {
	code := services.ErrorCode(err)
	status := http.StatusBadRequest
	switch code {
	case "invalid_credentials", "unauthorized":
		status = http.StatusUnauthorized
	case "registration_unavailable":
		status = http.StatusConflict
	case "rate_limited":
		status = http.StatusTooManyRequests
		w.Header().Set("Retry-After", "60")
	case "email_unavailable", "unavailable":
		status = http.StatusServiceUnavailable
	}
	if code == "unavailable" {
		c.logger.Error("authentication operation unavailable")
	}
	c.write(w, status, map[string]string{"error": code})
}
func (c *Auth) write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
