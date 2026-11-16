//go:build integration

package services_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wishix/server/internal/models"
)

func (f *authFixture) wishRequest(t *testing.T, method, path, body string, cookie *http.Cookie, origin string) (int, map[string]any) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", origin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Wishix-Request", "1")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	f.handler.ServeHTTP(result, request)
	if result.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private wish response can be cached")
	}
	var response map[string]any
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %d %s", result.Code, result.Body.String())
	}
	return result.Code, response
}

func TestIntegrationWishCollectionOwnershipAndDeletion(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "owner@example.test", "owner")
	other := f.account(t, "other@example.test", "other")
	const origin = "https://wishix.test"
	request := func(method, path, body string, cookie *http.Cookie) (int, map[string]any) {
		return f.wishRequest(t, method, path, body, cookie, origin)
	}
	for _, operation := range []struct{ method, path, body string }{
		{"GET", "/api/wishes", ""},
		{"POST", "/api/wishes", `{"title":"Guest wish"}`},
		{"DELETE", "/api/wishes/00000000-0000-4000-8000-000000000001", "{}"},
	} {
		if status, _ := request(operation.method, operation.path, operation.body, nil); status != 401 {
			t.Fatalf("unauthenticated %s: %d", operation.method, status)
		}
	}
	status, body := request("GET", "/api/wishes", "", owner)
	if status != 200 || len(body["wishes"].([]any)) != 0 {
		t.Fatal("new account must have a real empty collection")
	}
	status, body = request("POST", "/api/wishes", `{"title":" Weekend adventure ","description":"With friends","price":"123.40","currency":"eur","priority":"high","shop_links":["https://first.test/item","https://second.test/item"]}`, owner)
	if status != 201 {
		t.Fatalf("create: %d %v", status, body)
	}
	wish := body["wish"].(map[string]any)
	id := wish["id"].(string)
	if wish["title"] != "Weekend adventure" || wish["price"] != "123.40" || wish["currency"] != "EUR" || len(wish["shop_links"].([]any)) != 2 {
		t.Fatal("saved wish differs from the manual input")
	}
	for _, key := range []string{"owner_id", "user_id", "reservations", "purchased", "token"} {
		if _, found := wish[key]; found {
			t.Fatalf("unexpected private field: %s", key)
		}
	}
	// Force different timestamps to verify ordering independently of UUIDs.
	if err := f.database.Model(&models.Wish{}).Where("id = ?", id).Update("created_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	status, body = request("POST", "/api/wishes", `{"title":"A wish without links"}`, owner)
	if status != 201 || body["wish"].(map[string]any)["price"] != nil {
		t.Fatal("a wish requires neither a link nor a price")
	}
	status, body = request("GET", "/api/wishes", "", owner)
	wishes := body["wishes"].([]any)
	if status != 200 || len(wishes) != 2 || wishes[0].(map[string]any)["title"] != "A wish without links" || wishes[1].(map[string]any)["id"] != id {
		t.Fatal("persisted collection is missing or not newest first")
	}
	links := wishes[1].(map[string]any)["shop_links"].([]any)
	if links[0] != "https://first.test/item" || links[1] != "https://second.test/item" {
		t.Fatal("shop link order changed")
	}
	status, body = request("GET", "/api/wishes", "", other)
	if status != 200 || len(body["wishes"].([]any)) != 0 {
		t.Fatal("another account can read the owner's wishes")
	}
	for _, path := range []string{"/api/wishes/" + id, "/api/wishes/00000000-0000-4000-8000-000000000001", "/api/wishes/not-a-uuid"} {
		if status, body := request("DELETE", path, "{}", other); status != 404 || body["error"] != "wish_not_found" {
			t.Fatal("cross-account/absent deletion must return the same response")
		}
	}
	for _, operation := range []struct{ method, path, body string }{
		{"POST", "/api/wishes", `{"title":"Cross-site"}`},
		{"DELETE", "/api/wishes/" + id, "{}"},
	} {
		if status, _ := f.wishRequest(t, operation.method, operation.path, operation.body, owner, "https://attacker.test"); status != 403 {
			t.Fatal("cross-site mutation accepted")
		}
	}
	if status, _ := request("DELETE", "/api/wishes/"+id, "{}", owner); status != 200 {
		t.Fatal("owner cannot delete own wish")
	}
	var count int64
	if err := f.database.Model(&models.WishShopLink{}).Where("wish_id = ?", id).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("deleted wish left shop links behind")
	}
	status, body = request("GET", "/api/wishes", "", owner)
	if status != 200 || len(body["wishes"].([]any)) != 1 {
		t.Fatal("deletion affected the wrong wishes")
	}
	if _, err := f.auth.Current(t.Context(), other.Value); err != nil {
		t.Fatal("deleting a wish affected another account")
	}
}

func TestIntegrationWishValidationAndAtomicCreate(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "validation@example.test", "validation")
	for _, body := range []string{
		`{"title":" "}`,
		`{"title":"Book","owner_id":"00000000-0000-4000-8000-000000000001"}`,
		`{"title":"Book","price":12.5,"currency":"USD"}`,
		`{"title":"Book","price":"12.345","currency":"USD"}`,
		`{"title":"Book","shop_links":["javascript:alert(1)"]}`,
		`{"title":"Book"} {"title":"second"}`,
		`{"title":"` + strings.Repeat("a", 50000) + `"}`,
	} {
		if status, _ := f.wishRequest(t, "POST", "/api/wishes", body, owner, "https://wishix.test"); status != 400 {
			t.Fatalf("invalid input accepted: %d", status)
		}
	}
	// Make association insertion fail at the DB, after inserting the wish.
	if err := f.database.Exec(`ALTER TABLE wish_shop_links ADD CONSTRAINT test_reject_link CHECK (url <> 'https://rejected.test/')`).Error; err != nil {
		t.Fatal(err)
	}
	status, _ := f.wishRequest(t, "POST", "/api/wishes", `{"title":"Must roll back","shop_links":["https://rejected.test/"]}`, owner, "https://wishix.test")
	if status != 503 {
		t.Fatal("association storage failure should be reported")
	}
	var count int64
	if err := f.database.Model(&models.Wish{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed create left a partial wish")
	}
	for _, method := range []string{"POST", "DELETE"} {
		path := "/api/wishes"
		if method == "DELETE" {
			path += "/00000000-0000-4000-8000-000000000001"
		}
		for _, mode := range []string{"missing-header", "wrong-content-type", "cross-site-fetch"} {
			req := httptest.NewRequest(method, path, bytes.NewBufferString(`{"title":"Book"}`))
			req.AddCookie(owner)
			req.Header.Set("Origin", "https://wishix.test")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Wishix-Request", "1")
			if mode == "missing-header" {
				req.Header.Del("X-Wishix-Request")
			}
			if mode == "wrong-content-type" {
				req.Header.Set("Content-Type", "text/plain")
			}
			if mode == "cross-site-fetch" {
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			response := httptest.NewRecorder()
			f.handler.ServeHTTP(response, req)
			if response.Code != 403 && response.Code != 415 {
				t.Fatal("unsafe mutation accepted")
			}
		}
	}
}

func TestIntegrationWishEditOwnershipAndAtomicity(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "edit-owner@example.test", "editowner")
	other := f.account(t, "edit-other@example.test", "editother")
	request := func(method, path, body string, cookie *http.Cookie) (int, map[string]any) {
		return f.wishRequest(t, method, path, body, cookie, "https://wishix.test")
	}
	status, body := request("POST", "/api/wishes", `{"title":"Original","shop_links":["https://first.test/"]}`, owner)
	if status != 201 {
		t.Fatalf("create: %d %v", status, body)
	}
	id := body["wish"].(map[string]any)["id"].(string)
	version := body["wish"].(map[string]any)["edit_version"].(string)
	withVersion := func(value string) string {
		return strings.TrimSuffix(value, "}") + `,"edit_version":"` + version + `"}`
	}
	path := "/api/wishes/" + id
	for _, method := range []string{"GET", "PUT"} {
		if status, _ := request(method, path, `{"title":"Intrusion"}`, nil); status != 401 {
			t.Fatalf("guest %s: %d", method, status)
		}
		if status, response := request(method, path, withVersion(`{"title":"Intrusion"}`), other); status != 404 || response["error"] != "wish_not_found" {
			t.Fatalf("other owner %s: %d %v", method, status, response)
		}
	}
	if status, _ := f.wishRequest(t, "PUT", path, `{"title":"Cross-site"}`, owner, "https://attacker.test"); status != 403 {
		t.Fatal("cross-site update accepted")
	}
	for _, invalid := range []string{`{"title":""}`, `{"title":"Modified","price":"1.001","currency":"USD"}`, `{"title":"Modified","list_ids":[]}`} {
		if status, _ := request("PUT", path, withVersion(invalid), owner); status != 400 {
			t.Fatalf("invalid edit accepted: %d", status)
		}
	}
	if err := f.database.Exec(`ALTER TABLE wish_shop_links ADD CONSTRAINT test_reject_edit_link CHECK (url <> 'https://rejected.test/')`).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := request("PUT", path, withVersion(`{"title":"Must roll back","shop_links":["https://rejected.test/"]}`), owner); status != 503 {
		t.Fatalf("failed update: %d", status)
	}
	status, body = request("GET", path, "", owner)
	wish := body["wish"].(map[string]any)
	if status != 200 || wish["title"] != "Original" || wish["shop_links"].([]any)[0] != "https://first.test/" {
		t.Fatal("partial edit was committed")
	}
	status, body = request("PUT", path, withVersion(`{"title":" Updated ","description":"Notes","price":"12.5","currency":"mdl","priority":"high","shop_links":["https://second.test/","https://third.test/"]}`), owner)
	if status != 200 {
		t.Fatalf("update: %d %v", status, body)
	}
	status, body = request("GET", path, "", owner)
	wish = body["wish"].(map[string]any)
	links := wish["shop_links"].([]any)
	if status != 200 || wish["title"] != "Updated" || wish["price"] != "12.50" || wish["currency"] != "MDL" || wish["priority"] != "high" || len(links) != 2 || links[0] != "https://second.test/" || links[1] != "https://third.test/" {
		t.Fatalf("incorrect persisted update: %v", wish)
	}
	if status, _ := request("PUT", path, withVersion(`{"title":"Stale edit"}`), owner); status != 409 {
		t.Fatalf("stale edit accepted: %d", status)
	}
	version = wish["edit_version"].(string)
	if status, _ := request("PUT", path, withVersion(`{"title":"Without optional fields"}`), owner); status != 200 {
		t.Fatalf("clear fields: %d", status)
	}
	_, body = request("GET", path, "", owner)
	wish = body["wish"].(map[string]any)
	if wish["price"] != nil || wish["currency"] != "" || len(wish["shop_links"].([]any)) != 0 {
		t.Fatalf("optional fields not cleared: %v", wish)
	}
}
