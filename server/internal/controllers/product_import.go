package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"wishix/server/internal/productimport"
	"wishix/server/internal/services"
)

type ProductImport struct {
	service *services.ProductImport
	auth    *Auth
}

func decodeLogCheck(s string) string {
	var decoded strings.Builder
	decoded.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if b, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				decoded.WriteByte(byte(b))
				i += 2
				continue
			}
		}
		decoded.WriteByte(s[i])
	}
	return decoded.String()
}

// Preserve shop URLs as entered, but do not log our reservation capabilities,
// including links nested inside query parameters or encoded in other URLs.
func importLogURL(raw, origin string) string {
	u, err := productimport.ValidateURL(raw)
	if err != nil {
		return ""
	}
	configured, err := url.Parse(origin)
	ownHost := ""
	if err == nil {
		ownHost = strings.ToLower(configured.Hostname())
	}
	decoded := strings.ToLower(raw)
	sensitive := ownHost != "" && strings.Contains(decoded, ownHost)
	for i := 0; i <= 4 && !sensitive; i++ {
		if strings.Contains(decoded, "reserv") || strings.Contains(decoded, "booking") || ownHost != "" && strings.Contains(decoded, ownHost) {
			sensitive = true
			break
		}
		next := decodeLogCheck(decoded)
		if next == decoded {
			break
		}
		decoded = strings.ToLower(next)
	}
	if sensitive {
		u.Path, u.RawPath = "", ""
		u.RawQuery, u.ForceQuery, u.Fragment = "", false, ""
		return u.String()
	}
	return raw
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
