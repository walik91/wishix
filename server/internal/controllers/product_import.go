package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"wishix/server/internal/productimport"
	"wishix/server/internal/services"
)

type ProductImport struct {
	service *services.ProductImport
	auth    *Auth
}

// Query parameters, fragments and credentials may contain access tokens.
// Never record paths on our own origin or reservation links either.
func importLogURL(raw, origin string) string {
	u, err := productimport.ValidateURL(raw)
	if err != nil {
		return ""
	}
	configured, err := url.Parse(origin)
	path := strings.ToLower(u.Path)
	if err == nil && strings.EqualFold(u.Hostname(), configured.Hostname()) || strings.Contains(path, "reserv") || strings.Contains(path, "booking") {
		u.Path, u.RawPath = "", ""
	}
	u.RawQuery, u.ForceQuery, u.Fragment = "", false, ""
	return u.String()
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
	address := importLogURL(input.URL, c.auth.config.Origin)
	if err != nil {
		reason, upstreamStatus := productimport.FailureDetails(err)
		if errors.Is(err, services.ErrRateLimit) {
			reason = "rate_limited"
		}
		c.auth.logger.Info("product import", "url", address, "status", "failed", "reason", reason, "upstream_status", upstreamStatus)
	} else {
		reason := ""
		switch result.Status {
		case "empty":
			reason = "no_product_metadata"
		case "not_product":
			reason = "not_product"
		case "partial":
			reason = "missing_fields"
		}
		c.auth.logger.Info("product import", "url", address, "status", result.Status, "reason", reason,
			"has_title", result.Title != "", "has_price", result.Price != "", "has_image", result.Image != nil,
			"image_reason", result.ImageReason, "image_upstream_status", result.ImageHTTPStatus)
	}
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
