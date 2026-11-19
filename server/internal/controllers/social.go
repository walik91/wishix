package controllers

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"wishix/server/internal/services"
	"wishix/server/internal/wishimage"
)

type Social struct {
	service *services.Social
	auth    *Auth
	lists   *Lists
	images  *wishimage.Processor
	uploads chan struct{}
}

func NewSocial(service *services.Social, auth *Auth, lists *Lists) *Social {
	return &Social{service: service, auth: auth, lists: lists, images: wishimage.New(), uploads: make(chan struct{}, 2)}
}
func (c *Social) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/profile/details", c.lists.authorized(c.settings))
	mux.HandleFunc("PUT /api/profile/details", c.lists.authorized(c.saveSettings))
	mux.HandleFunc("PUT /api/profile/username", c.authorized(c.rename))
	mux.HandleFunc("GET /api/profile/avatar", c.lists.authorized(c.privateAvatar))
	mux.HandleFunc("DELETE /api/profile/avatar", c.lists.authorized(c.deleteAvatar))
	mux.HandleFunc("PUT /api/profile/avatar", c.uploadAvatar)
	mux.HandleFunc("GET /api/public/profiles/{username}/details", c.publicInfo)
	mux.HandleFunc("GET /api/public/profiles/{username}/avatar", c.publicAvatar)
	mux.HandleFunc("GET /api/social/{username}/state", c.authorized(c.state))
	mux.HandleFunc("PUT /api/social/{username}/follow", c.authorized(c.follow))
	mux.HandleFunc("PUT /api/social/{username}/block", c.authorized(c.block))
	mux.HandleFunc("POST /api/social/{username}/report", c.authorized(c.report))
	mux.HandleFunc("GET /api/social/{username}/people/{kind}", c.people)
}
func (c *Social) authorized(handler func(http.ResponseWriter, *http.Request, string) error) http.HandlerFunc {
	return c.lists.authorized(func(w http.ResponseWriter, r *http.Request, owner string) error {
		err := handler(w, r, owner)
		var social services.SocialError
		if errors.As(err, &social) {
			c.auth.write(w, http.StatusConflict, map[string]string{"error": string(social)})
			return nil
		}
		return err
	})
}
func (c *Social) settings(w http.ResponseWriter, r *http.Request, owner string) error {
	value, err := c.service.Settings(r.Context(), owner)
	if err != nil {
		return err
	}
	if value == nil {
		return services.ErrSession
	}
	c.auth.write(w, 200, value)
	return nil
}
func (c *Social) saveSettings(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Biography     *string `json:"biography"`
		BirthDate     *string `json:"birth_date"`
		ShowAge       *bool   `json:"show_age"`
		ShowFollowers *bool   `json:"show_followers"`
		ShowFollowing *bool   `json:"show_following"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Biography == nil || input.ShowAge == nil || input.ShowFollowers == nil || input.ShowFollowing == nil {
		return services.ErrInvalidInput
	}
	value := services.ProfileSettings{Biography: *input.Biography, BirthDate: input.BirthDate, ShowAge: *input.ShowAge, ShowFollowers: *input.ShowFollowers, ShowFollowing: *input.ShowFollowing}
	if err := c.service.SaveSettings(r.Context(), owner, value); err != nil {
		return err
	}
	return c.settings(w, r, owner)
}
func (c *Social) rename(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Username string `json:"username"`
		Redirect *bool  `json:"redirect"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Redirect == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.Rename(r.Context(), owner, input.Username, *input.Redirect); err != nil {
		return err
	}
	return c.settings(w, r, owner)
}
func (c *Social) state(w http.ResponseWriter, r *http.Request, owner string) error {
	value, err := c.service.State(r.Context(), owner, r.PathValue("username"))
	if err != nil {
		return err
	}
	c.auth.write(w, 200, value)
	return nil
}
func (c *Social) follow(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Follow *bool `json:"follow"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Follow == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.Follow(r.Context(), owner, r.PathValue("username"), *input.Follow); err != nil {
		return err
	}
	return c.state(w, r, owner)
}
func (c *Social) block(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Block *bool `json:"block"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Block == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.Block(r.Context(), owner, r.PathValue("username"), *input.Block); err != nil {
		return err
	}
	return c.state(w, r, owner)
}
func (c *Social) report(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Message string `json:"message"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if err := c.auth.service.LimitSocialReport(r.Context(), owner); err != nil {
		return err
	}
	if err := c.service.Report(r.Context(), owner, r.PathValue("username"), input.Message); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Social) publicInfo(w http.ResponseWriter, r *http.Request) {
	value, err := c.service.PublicInfo(r.Context(), r.PathValue("username"))
	if err != nil {
		c.lists.failure(w, err)
		return
	}
	c.auth.write(w, 200, value)
}
func (c *Social) people(w http.ResponseWriter, r *http.Request) {
	viewer := ""
	if token := c.auth.token(r); token != "" {
		user, err := c.auth.service.Current(r.Context(), token)
		if err == nil {
			viewer = user.ID
		} else if !errors.Is(err, services.ErrSession) {
			c.lists.failure(w, err)
			return
		}
	}
	people, next, err := c.service.People(r.Context(), viewer, r.PathValue("username"), r.PathValue("kind"), r.URL.Query().Get("after"))
	if err != nil {
		c.lists.failure(w, err)
		return
	}
	c.auth.write(w, 200, map[string]any{"people": people, "next_cursor": next})
}
func writeAvatar(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
func (c *Social) privateAvatar(w http.ResponseWriter, r *http.Request, owner string) error {
	data, err := c.service.Avatar(r.Context(), owner, false)
	if err != nil {
		return err
	}
	writeAvatar(w, data)
	return nil
}
func (c *Social) publicAvatar(w http.ResponseWriter, r *http.Request) {
	data, err := c.service.Avatar(r.Context(), r.PathValue("username"), true)
	if err != nil {
		c.lists.failure(w, err)
		return
	}
	writeAvatar(w, data)
}
func (c *Social) deleteAvatar(w http.ResponseWriter, r *http.Request, owner string) error {
	if err := c.service.DeleteAvatar(r.Context(), owner); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Social) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != c.auth.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		c.auth.write(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "image/jpeg" && media != "image/png" && media != "image/webp" {
		c.auth.write(w, 415, map[string]string{"error": "invalid_input"})
		return
	}
	user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
	if err != nil {
		c.auth.failure(w, err)
		return
	}
	select {
	case c.uploads <- struct{}{}:
		defer func() { <-c.uploads }()
	default:
		c.auth.write(w, 429, map[string]string{"error": "wish_image_busy"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, wishimage.MaxUploadBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		c.auth.write(w, 413, map[string]string{"error": "wish_image_too_large"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	data, err := c.images.Process(ctx, raw)
	if err != nil {
		c.lists.failure(w, err)
		return
	}
	if err = c.service.SaveAvatar(ctx, user.ID, data); err != nil {
		c.lists.failure(w, err)
		return
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
}
