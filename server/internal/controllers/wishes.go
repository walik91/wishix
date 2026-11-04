package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"wishix/server/internal/models"
	"wishix/server/internal/services"
	"wishix/server/internal/wishimage"
)

type Wishes struct {
	service *services.Wishes
	auth    *Auth
	uploads chan struct{}
}

func NewWishes(service *services.Wishes, auth *Auth) *Wishes {
	return &Wishes{service: service, auth: auth, uploads: make(chan struct{}, 2)}
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
	ImageURL    *string   `json:"image_url"`
}

func privateWish(wish *models.Wish) wishResponse {
	links := make([]string, 0, len(wish.ShopLinks))
	for _, link := range wish.ShopLinks {
		links = append(links, link.URL)
	}
	return wishResponse{ID: wish.ID, Title: wish.Title, Description: wish.Description, Price: wish.Price, Currency: wish.Currency, Priority: wish.Priority, CreatedAt: wish.CreatedAt, ShopLinks: links, ImageURL: wishImageURL(wish.ID, wish.ImageVersion)}
}

func wishImageURL(id, version string) *string {
	if version == "" {
		return nil
	}
	path := "/api/wishes/" + id + "/image?v=" + version
	return &path
}

func (c *Wishes) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/wishes", c.authorized(c.list))
	mux.HandleFunc("POST /api/wishes", c.authorized(c.create))
	mux.HandleFunc("DELETE /api/wishes/{id}", c.authorized(c.delete))
	mux.HandleFunc("GET /api/wishes/{id}/image", c.authorized(c.image))
	mux.HandleFunc("PUT /api/wishes/{id}/image", c.authorized(c.saveImage))
	mux.HandleFunc("DELETE /api/wishes/{id}/image", c.authorized(c.deleteImage))
}

func (c *Wishes) authorized(handler func(http.ResponseWriter, *http.Request, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		multipart := false
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if r.Header.Get("Origin") != c.auth.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				c.auth.write(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			multipart = mediaType == "multipart/form-data" && (r.Method == http.MethodPost || r.Method == http.MethodPut)
			if (r.Method == http.MethodPut && !multipart) || (mediaType != "application/json" && !multipart) {
				c.auth.write(w, http.StatusUnsupportedMediaType, map[string]string{"error": "invalid_input"})
				return
			}
			limit := int64(48 * 1024)
			if multipart {
				limit = wishimage.MaxUploadBytes + 128*1024
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
		if err != nil {
			c.auth.failure(w, err)
			return
		}
		if multipart {
			// Bound buffering as well as decoding, without temporary disk files.
			select {
			case c.uploads <- struct{}{}:
				defer func() { <-c.uploads }()
			default:
				w.Header().Set("Retry-After", "2")
				c.auth.write(w, http.StatusTooManyRequests, map[string]string{"error": "wish_image_busy"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
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
	var input services.WishInput
	var err error
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		input, err = readWishUpload(r, true)
	} else {
		input, err = decodeWish(r.Body)
	}
	if err != nil {
		return err
	}
	wish, err := c.service.Create(r.Context(), ownerID, input)
	if err != nil {
		return err
	}
	c.auth.write(w, http.StatusCreated, map[string]any{"wish": privateWish(wish)})
	return nil
}

func decodeWish(reader io.Reader) (services.WishInput, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var input services.WishInput
	if err := decoder.Decode(&input); err != nil {
		return input, services.ErrInvalidInput
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return input, services.ErrInvalidInput
	}
	return input, nil
}

func readWishUpload(r *http.Request, withWish bool) (services.WishInput, error) {
	var input services.WishInput
	var imageData []byte
	reader, err := r.MultipartReader()
	if err != nil {
		return input, services.ErrInvalidInput
	}
	metadata, image := false, false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var size *http.MaxBytesError
			if errors.As(err, &size) {
				return input, wishimage.ErrTooLarge
			}
			return input, services.ErrInvalidInput
		}
		switch part.FormName() {
		case "wish":
			if !withWish || metadata || part.FileName() != "" {
				return input, services.ErrInvalidInput
			}
			data, err := io.ReadAll(io.LimitReader(part, 48*1024+1))
			if err != nil || len(data) > 48*1024 {
				return input, services.ErrInvalidInput
			}
			input, err = decodeWish(bytes.NewReader(data))
			if err != nil {
				return input, err
			}
			metadata = true
		case "image":
			if image || part.FileName() == "" {
				return input, services.ErrInvalidInput
			}
			data, err := io.ReadAll(io.LimitReader(part, wishimage.MaxUploadBytes+1))
			var size *http.MaxBytesError
			if len(data) > wishimage.MaxUploadBytes || errors.As(err, &size) {
				return input, wishimage.ErrTooLarge
			}
			if err != nil || len(data) == 0 {
				return input, wishimage.ErrInvalid
			}
			imageData = data
			image = true
		default:
			return input, services.ErrInvalidInput
		}
		_ = part.Close()
	}
	if !image || (withWish && !metadata) {
		return input, services.ErrInvalidInput
	}
	input.Image = imageData
	return input, nil
}

func (c *Wishes) image(w http.ResponseWriter, r *http.Request, ownerID string) error {
	image, err := c.service.Image(r.Context(), ownerID, r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `inline; filename="wish.jpg"`)
	if r.Method != http.MethodHead {
		_, _ = w.Write(image.Data)
	}
	return nil
}

func (c *Wishes) saveImage(w http.ResponseWriter, r *http.Request, ownerID string) error {
	input, err := readWishUpload(r, false)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	version, err := c.service.SaveImage(r.Context(), ownerID, id, input.Image)
	if err != nil {
		return err
	}
	c.auth.write(w, http.StatusOK, map[string]any{"image_url": wishImageURL(id, version)})
	return nil
}

func (c *Wishes) deleteImage(w http.ResponseWriter, r *http.Request, ownerID string) error {
	if err := c.service.DeleteImage(r.Context(), ownerID, r.PathValue("id")); err != nil {
		return err
	}
	c.auth.write(w, http.StatusOK, map[string]any{"image_url": nil})
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
	} else if code == "wish_image_too_large" {
		status = http.StatusRequestEntityTooLarge
	} else if code == "unavailable" {
		status = http.StatusServiceUnavailable
		c.auth.logger.Error("wish operation unavailable")
	}
	c.auth.write(w, status, map[string]string{"error": code})
}
