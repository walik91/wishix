package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Wishes struct {
	service *services.Wishes
	auth    *Auth
}

func NewWishes(service *services.Wishes, auth *Auth) *Wishes {
	return &Wishes{service: service, auth: auth}
}

type wishResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       *string   `json:"price"`
	Currency    string    `json:"currency"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	ShopLinks   []string  `json:"shop_links"`
}

func privateWish(wish *models.Wish) wishResponse {
	links := make([]string, 0, len(wish.ShopLinks))
	for _, link := range wish.ShopLinks {
		links = append(links, link.URL)
	}
	return wishResponse{wish.ID, wish.Title, wish.Description, wish.Price, wish.Currency, wish.Priority, wish.CreatedAt, links}
}

func (c *Wishes) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/wishes", c.authorized(c.list))
	mux.HandleFunc("POST /api/wishes", c.authorized(c.create))
	mux.HandleFunc("DELETE /api/wishes/{id}", c.authorized(c.delete))
}

func (c *Wishes) authorized(handler func(http.ResponseWriter, *http.Request, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if r.Header.Get("Origin") != c.auth.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				c.auth.write(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			if mediaType != "application/json" {
				c.auth.write(w, http.StatusUnsupportedMediaType, map[string]string{"error": "invalid_input"})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 48*1024)
		}
		user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
		if err != nil {
			c.auth.failure(w, err)
			return
		}
		if err := handler(w, r, user.ID); err != nil {
			c.failure(w, err)
		}
	}
}

func (c *Wishes) list(w http.ResponseWriter, r *http.Request, ownerID string) error {
	wishes, err := c.service.List(r.Context(), ownerID)
	if err != nil {
		return err
	}
	result := make([]wishResponse, 0, len(wishes))
	for i := range wishes {
		result = append(result, privateWish(&wishes[i]))
	}
	c.auth.write(w, http.StatusOK, map[string]any{"wishes": result})
	return nil
}

func (c *Wishes) create(w http.ResponseWriter, r *http.Request, ownerID string) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input services.WishInput
	if err := decoder.Decode(&input); err != nil {
		return services.ErrInvalidInput
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return services.ErrInvalidInput
	}
	wish, err := c.service.Create(r.Context(), ownerID, input)
	if err != nil {
		return err
	}
	c.auth.write(w, http.StatusCreated, map[string]any{"wish": privateWish(wish)})
	return nil
}

func (c *Wishes) delete(w http.ResponseWriter, r *http.Request, ownerID string) error {
	if err := c.service.Delete(r.Context(), ownerID, r.PathValue("id")); err != nil {
		return err
	}
	c.auth.write(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

func (c *Wishes) failure(w http.ResponseWriter, err error) {
	code := services.WishErrorCode(err)
	status := http.StatusBadRequest
	if code == "wish_not_found" {
		status = http.StatusNotFound
	} else if code == "unavailable" {
		status = http.StatusServiceUnavailable
		c.auth.logger.Error("wish operation unavailable")
	}
	c.auth.write(w, status, map[string]string{"error": code})
}
