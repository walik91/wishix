//go:build integration

package services_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestIntegrationPeopleSearchAndNamePrivacy(t *testing.T) {
	f := newAuthFixture(t)
	viewer := f.account(t, "search-viewer@example.test", "searchviewer")
	owner := f.account(t, "search-owner@example.test", "searchowner")
	search := func(q, after string) map[string]any {
		return f.listCall(t, "GET", "/api/search/people?q="+url.QueryEscape(q)+"&after="+url.QueryEscape(after), nil, viewer, 200)
	}
	f.listCall(t, "GET", "/api/search/people?q=search", nil, nil, 401)
	f.listCall(t, "GET", "/api/profile/name", nil, nil, 401)
	initial := f.listCall(t, "GET", "/api/profile/name", nil, owner, 200)
	if initial["display_name"] != "" || initial["show_display_name"] != false {
		t.Fatal("name must be private by default", initial)
	}
	if status, _ := f.wishRequest(t, "PUT", "/api/profile/name", `{"display_name":"spoofed","show_display_name":true}`, owner, "https://attacker.test"); status != 403 {
		t.Fatalf("cross-origin edit accepted: %d", status)
	}
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": "Лилия Уникальная", "show_display_name": false}, owner, 200)
	if got := search("уникальная", "")["people"].([]any); len(got) != 0 {
		t.Fatal("private name matched search", got)
	}
	got := search("SEARCHOWNER", "")["people"].([]any)
	if len(got) != 1 {
		t.Fatal("username search failed", got)
	}
	person := got[0].(map[string]any)
	if person["username"] != "searchowner" || person["display_name"] != nil || person["email"] != nil || person["id"] != nil {
		t.Fatal("private data in result", person)
	}
	profile := f.listCall(t, "GET", "/api/public/profiles/searchowner", nil, nil, 200)
	if profile["display_name"] != "" {
		t.Fatal("private name in public profile", profile)
	}
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": "Лилия Уникальная", "show_display_name": true}, owner, 200)
	got = search("уНИКАЛЬНАЯ", "")["people"].([]any)
	if len(got) != 1 || got[0].(map[string]any)["display_name"] != "Лилия Уникальная" {
		t.Fatal("public unicode name not found", got)
	}
	profile = f.listCall(t, "GET", "/api/public/profiles/searchowner", nil, nil, 200)
	if profile["display_name"] != "Лилия Уникальная" || profile["email"] != nil {
		t.Fatal("public profile name", profile)
	}
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": "Лилия Уникальная", "show_display_name": false}, owner, 200)
	if got := search("уникальная", "")["people"].([]any); len(got) != 0 {
		t.Fatal("hidden name still searchable", got)
	}
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": "", "show_display_name": true}, owner, 400)
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": strings.Repeat("a", 101), "show_display_name": false}, owner, 400)
	f.listCall(t, "PUT", "/api/profile/name", map[string]any{"display_name": "hello\nworld", "show_display_name": false}, owner, 400)
	f.listCall(t, "GET", "/api/search/people?q=%20", nil, viewer, 400)
	if got := search("%", "")["people"].([]any); len(got) != 0 {
		t.Fatal("wildcard search interpreted as pattern", got)
	}
	for i := 0; i < 23; i++ {
		username := fmt.Sprintf("batchuser%02d", i)
		if err := f.database.Exec("INSERT INTO users (id, username, email, password_hash, email_verified_at, created_at) VALUES (gen_random_uuid(), ?, ?, 'x', now(), now())", username, username+"@example.test").Error; err != nil {
			t.Fatal(err)
		}
	}
	first := search("batchuser", "")
	if len(first["people"].([]any)) != 20 || first["next_cursor"] == "" {
		t.Fatal("first page", first)
	}
	second := search("batchuser", first["next_cursor"].(string))
	if len(second["people"].([]any)) != 3 || second["next_cursor"] != "" {
		t.Fatal("second page", second)
	}
}
