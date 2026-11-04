package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"wishix/server/internal/productimport"
	"wishix/server/internal/services"
)

type ProductImport struct {
	service *services.ProductImport
	auth    *Auth
}

func NewProductImport(service *services.ProductImport, auth *Auth) *ProductImport {
	return &ProductImport{service: service, auth: auth}
}

func (c *ProductImport) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/wishes/import-preview", c.preview)
}

func (c *ProductImport) preview(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != c.auth.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		c.auth.write(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		c.auth.write(w, http.StatusUnsupportedMediaType, map[string]string{"error": "invalid_input"})
		return
	}
	user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
	if err != nil {
		c.auth.failure(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		URL    string `json:"url"`
		Locale string `json:"locale"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
		c.auth.failure(w, services.ErrInvalidInput)
		return
	}
	result, err := c.service.Preview(r.Context(), user.ID, input.URL, input.Locale)
	if err != nil {
		if !errors.Is(err, productimport.ErrURL) && !errors.Is(err, productimport.ErrFetch) && !errors.Is(err, productimport.ErrBusy) {
			c.auth.failure(w, err)
			return
		}
		status, code := http.StatusUnprocessableEntity, "import_unavailable"
		if errors.Is(err, productimport.ErrURL) {
			status, code = http.StatusBadRequest, "import_url_invalid"
		}
		if errors.Is(err, productimport.ErrBusy) {
			status, code = http.StatusTooManyRequests, "import_busy"
			w.Header().Set("Retry-After", "2")
		}
		c.auth.write(w, status, map[string]string{"error": code})
		return
	}
	c.auth.write(w, http.StatusOK, result)
}
