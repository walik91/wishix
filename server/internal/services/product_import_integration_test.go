//go:build integration

package services_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"wishix/server/internal/models"
)

func TestIntegrationProductImportAccessAndBudget(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "importer@example.test", "importer")
	const path = "/api/wishes/import-preview"
	const origin = "https://wishix.test"
	if status, _ := f.wishRequest(t, http.MethodPost, path, `{"url":"https://shop.example.com/item"}`, nil, origin); status != 401 {
		t.Fatal("anonymous import accepted", status)
	}
	if status, _ := f.wishRequest(t, http.MethodPost, path, `{"url":"https://shop.example.com/item"}`, owner, "https://attacker.test"); status != 403 {
		t.Fatal("cross-origin import accepted", status)
	}
	for _, body := range []string{`{"url":"http://127.0.0.1/"}`, `{"url":"file:///etc/passwd"}`, `{"url":"https://shop.example.com/","extra":true}`, `{} {}`, `{"url":"` + strings.Repeat("a", 5000) + `"}`} {
		if status, _ := f.wishRequest(t, http.MethodPost, path, body, owner, origin); status != 400 {
			t.Fatal("invalid import accepted", status)
		}
	}
	var user models.User
	if err := f.database.Where("username = ?", "importer").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 20; n++ {
		if err := f.auth.LimitProductImport(context.Background(), user.ID); err != nil {
			t.Fatal(err)
		}
	}
	// Exhaust the budget without making any external network request.
	status, body := f.wishRequest(t, http.MethodPost, path, `{"url":"https://shop.example.com/item"}`, owner, origin)
	if status != 429 || body["error"] != "rate_limited" {
		t.Fatalf("import budget not enforced: %d %v", status, body)
	}
	for _, model := range []any{&models.Wish{}, &models.WishImage{}} {
		var count int64
		if err := f.database.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("preview created wish/image records", err, count)
		}
	}
}
