package controllers

import (
	"errors"
	"net/http"

	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Reservations struct {
	service *services.Reservations
	auth    *Auth
	lists   *Lists
}

func NewReservations(service *services.Reservations, auth *Auth, lists *Lists) *Reservations {
	return &Reservations{service: service, auth: auth, lists: lists}
}
func (c *Reservations) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/reservations/start", c.route("reservation-start", c.start))
	mux.HandleFunc("POST /api/reservations/confirm", c.route("reservation-confirm", c.confirm))
	mux.HandleFunc("POST /api/reservations/recover", c.route("reservation-recover", c.recover))
	mux.HandleFunc("POST /api/reservations/manage", c.route("reservation-manage", c.manage))
}
func (c *Reservations) route(action string, handle func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		if !c.lists.mutationHeaders(w, r) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := c.auth.service.LimitIP(r.Context(), action, c.auth.clientIP(r)); err != nil {
			c.failure(w, err)
			return
		}
		if err := handle(w, r); err != nil {
			c.failure(w, err)
		}
	}
}
func (c *Reservations) user(r *http.Request) (*models.User, error) {
	user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
	if errors.Is(err, services.ErrSession) {
		return nil, nil
	}
	return user, err
}
func (c *Reservations) start(w http.ResponseWriter, r *http.Request) error {
	var input services.ReservationInput
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	user, err := c.user(r)
	if err != nil {
		return err
	}
	challenge, err := c.service.Start(r.Context(), input, user)
	if err != nil {
		return err
	}
	c.auth.write(w, 202, map[string]string{"challenge": challenge})
	return nil
}
func (c *Reservations) confirm(w http.ResponseWriter, r *http.Request) error {
	var input services.ReservationInput
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	user, err := c.user(r)
	if err != nil {
		return err
	}
	if err := c.service.Confirm(r.Context(), input, user); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Reservations) recover(w http.ResponseWriter, r *http.Request) error {
	var input services.ReservationInput
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if err := c.service.Recover(r.Context(), input); err != nil {
		return err
	}
	c.auth.write(w, 202, map[string]string{"status": "ok"})
	return nil
}
func (c *Reservations) manage(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Token  string `json:"token"`
		Action string `json:"action"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	view, err := c.service.Manage(r.Context(), input.Token, input.Action)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"reservation": view})
	return nil
}
func (c *Reservations) failure(w http.ResponseWriter, err error) {
	code := services.ReservationErrorCode(err)
	status := 400
	switch code {
	case "reservation_unavailable", "reservation_limit", "reservation_expired", "reservation_action_invalid":
		status = 409
	case "rate_limited":
		status = 429
	case "email_unavailable", "unavailable":
		status = 503
	}
	if code == "unavailable" {
		c.auth.logger.Error("reservation operation unavailable")
	}
	c.auth.write(w, status, map[string]string{"error": code})
}
