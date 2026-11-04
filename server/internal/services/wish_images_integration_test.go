//go:build integration

package services_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wishix/server/internal/models"
	"wishix/server/internal/wishimage"
)

func imageUpload(t *testing.T, wish string, data []byte, extra bool) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	// Image before metadata deliberately exercises part-order independence.
	part, err := writer.CreateFormFile("image", "../../private-image.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if wish != "" {
		if err := writer.WriteField("wish", wish); err != nil {
			t.Fatal(err)
		}
	}
	if extra {
		if err := writer.WriteField("owner_id", "another-owner"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func testImage(t *testing.T, colour color.Color) []byte {
	t.Helper()
	image := image.NewRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			image.Set(x, y, colour)
		}
	}
	var body bytes.Buffer
	if err := png.Encode(&body, image); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func (f *authFixture) imageRequest(t *testing.T, method, path string, body []byte, contentType string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Origin", origin)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Wishix-Request", "1")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("image response is cacheable")
	}
	return response
}

func TestIntegrationWishImageLifecycleAndPrivacy(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "image-owner@example.test", "image_owner")
	other := f.account(t, "image-other@example.test", "image_other")
	red := testImage(t, color.RGBA{R: 255, A: 255})
	blue := testImage(t, color.RGBA{B: 255, A: 255})
	body, contentType := imageUpload(t, `{"title":"With a photo","shop_links":["https://shop.test/item"]}`, red, false)
	response := f.imageRequest(t, "POST", "/api/wishes", body, contentType, owner, "https://wishix.test")
	if response.Code != 201 {
		t.Fatalf("create with photo: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Wish struct {
			ID       string `json:"id"`
			ImageURL string `json:"image_url"`
		} `json:"wish"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := created.Wish.ID
	if created.Wish.ImageURL == "" {
		t.Fatal("image-first multipart lost the file")
	}
	path := "/api/wishes/" + id + "/image"
	response = f.imageRequest(t, "GET", created.Wish.ImageURL, nil, "", owner, "")
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("image is not served as a safe JPEG")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
	if err != nil || format != "jpeg" || config.Width != 80 || config.Height != 40 || response.Body.Len() > wishimage.MaxStoredBytes {
		t.Fatal("image was not normalized")
	}
	for _, cookie := range []*http.Cookie{nil, other} {
		response = f.imageRequest(t, "GET", path, nil, "", cookie, "")
		want := 401
		if cookie != nil {
			want = 404
		}
		if response.Code != want {
			t.Fatalf("private image exposed: %d", response.Code)
		}
	}
	for _, method := range []string{"PUT", "DELETE"} {
		data, kind := []byte("{}"), "application/json"
		if method == "PUT" {
			data, kind = imageUpload(t, "", blue, false)
		}
		if response = f.imageRequest(t, method, path, data, kind, other, "https://wishix.test"); response.Code != 404 {
			t.Fatalf("other account can mutate photo: %d", response.Code)
		}
		if response = f.imageRequest(t, method, path, data, kind, owner, "https://attacker.test"); response.Code != 403 {
			t.Fatal("image mutation accepted cross-site")
		}
	}
	body, contentType = imageUpload(t, "", blue, false)
	response = f.imageRequest(t, "PUT", path, body, contentType, owner, "https://wishix.test")
	var updated struct {
		ImageURL string `json:"image_url"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || updated.ImageURL == created.Wish.ImageURL || updated.ImageURL == "" {
		t.Fatalf("replace: %d %s", response.Code, response.Body.String())
	}
	response = f.imageRequest(t, "GET", updated.ImageURL, nil, "", owner, "")
	decoded, _, err := image.Decode(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	r, _, b, _ := decoded.At(10, 10).RGBA()
	if b <= r {
		t.Fatal("replacement still serves original pixels")
	}
	status, list := f.wishRequest(t, "GET", "/api/wishes", "", owner, "https://wishix.test")
	if status != 200 || list["wishes"].([]any)[0].(map[string]any)["image_url"] != updated.ImageURL {
		t.Fatal("list lost persisted image URL")
	}
	if strings.Contains(response.Body.String(), "private-image") {
		t.Fatal("original filename was preserved")
	}
	response = f.imageRequest(t, "DELETE", path, []byte("{}"), "application/json", owner, "https://wishix.test")
	if response.Code != 200 {
		t.Fatal("cannot remove saved image")
	}
	if response = f.imageRequest(t, "GET", path, nil, "", owner, ""); response.Code != 404 {
		t.Fatal("removed image still accessible")
	}
	status, list = f.wishRequest(t, "GET", "/api/wishes", "", owner, "https://wishix.test")
	if status != 200 || list["wishes"].([]any)[0].(map[string]any)["image_url"] != nil {
		t.Fatal("removing image affected wish or left metadata")
	}
	body, contentType = imageUpload(t, "", red, false)
	if response = f.imageRequest(t, "PUT", path, body, contentType, owner, "https://wishix.test"); response.Code != 200 {
		t.Fatal("cannot add image to existing wish")
	}
	if status, _ := f.wishRequest(t, "DELETE", "/api/wishes/"+id, "{}", owner, "https://wishix.test"); status != 200 {
		t.Fatal("cannot delete wish with photo")
	}
	var count int64
	if err := f.database.Model(&models.WishImage{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("wish deletion left an orphan image")
	}
}

func TestIntegrationWishImageValidationAndRollback(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "image-check@example.test", "image_check")
	valid := testImage(t, color.White)
	for _, test := range []struct {
		name, wish string
		data       []byte
		extra      bool
		status     int
	}{
		{"SVG with PNG filename", `{"title":"Bad"}`, []byte(`<svg><script>alert(1)</script></svg>`), false, 400},
		{"broken PNG", `{"title":"Bad"}`, []byte("\x89PNG\r\n\x1a\n"), false, 400},
		{"large file", `{"title":"Bad"}`, make([]byte, wishimage.MaxUploadBytes+1), false, 413},
		{"unknown field", `{"title":"Bad"}`, valid, true, 400},
		{"invalid title", `{"title":" "}`, valid, false, 400},
		{"missing metadata", "", valid, false, 400},
		{"empty file", `{"title":"Bad"}`, []byte{}, false, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, kind := imageUpload(t, test.wish, test.data, test.extra)
			response := f.imageRequest(t, "POST", "/api/wishes", body, kind, owner, "https://wishix.test")
			if response.Code != test.status {
				t.Fatalf("got %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
	if err := f.database.Exec(`ALTER TABLE wish_images ADD CONSTRAINT image_test_failure CHECK (version = '00000000-0000-4000-8000-000000000001')`).Error; err != nil {
		t.Fatal(err)
	}
	body, kind := imageUpload(t, `{"title":"Must roll back","shop_links":["https://shop.test/item"]}`, valid, false)
	response := f.imageRequest(t, "POST", "/api/wishes", body, kind, owner, "https://wishix.test")
	if response.Code != 503 {
		t.Fatalf("storage failure: %d %s", response.Code, response.Body.String())
	}
	for _, model := range []any{&models.Wish{}, &models.WishShopLink{}, &models.WishImage{}} {
		var count int64
		if err := f.database.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("invalid or failed upload left partial data")
		}
	}
}

// Ensure multipart parsing never requires temporary files on disk and rejects
// duplicate file parts even if each file is individually valid.
func TestIntegrationWishImageDuplicateParts(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "image-duplicate@example.test", "image_duplicate")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i := 0; i < 2; i++ {
		part, err := writer.CreateFormFile("image", "photo.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, bytes.NewReader(testImage(t, color.White))); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.WriteField("wish", `{"title":"Duplicate images"}`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := f.imageRequest(t, "POST", "/api/wishes", body.Bytes(), writer.FormDataContentType(), owner, "https://wishix.test")
	if response.Code != 400 {
		t.Fatal("multiple images accepted")
	}
}
