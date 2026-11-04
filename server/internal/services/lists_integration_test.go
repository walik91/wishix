//go:build integration

package services_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"wishix/server/internal/models"
)

const listOrigin = "https://wishix.test"

func listBody(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func (f *authFixture) listCall(t *testing.T, method, path string, value any, cookie *http.Cookie, want int) map[string]any {
	t.Helper()
	status, body := f.wishRequest(t, method, path, listBody(t, value), cookie, listOrigin)
	if status != want {
		t.Fatalf("%s %s: %d, wanted %d: %v", method, path, status, want, body)
	}
	return body
}
func (f *authFixture) newList(t *testing.T, cookie *http.Cookie, title, visibility string) map[string]any {
	t.Helper()
	return f.listCall(t, "POST", "/api/lists", map[string]any{"title": title, "visibility": visibility, "event_date": "2027-06-14", "description": "A real list"}, cookie, 201)["list"].(map[string]any)
}
func (f *authFixture) newWish(t *testing.T, cookie *http.Cookie, title string, ids ...string) map[string]any {
	t.Helper()
	return f.listCall(t, "POST", "/api/wishes", map[string]any{"title": title, "list_ids": ids}, cookie, 201)["wish"].(map[string]any)
}
func (f *authFixture) currentList(t *testing.T, cookie *http.Cookie, id string) map[string]any {
	t.Helper()
	return f.listCall(t, "GET", "/api/lists/"+id, map[string]any{}, cookie, 200)
}
func rev(body map[string]any) any { return body["list"].(map[string]any)["revision"] }

func TestIntegrationListLifecycleAndSharedWishIdentity(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "listowner@example.test", "listowner")
	other := f.account(t, "listother@example.test", "listother")
	a := f.newList(t, owner, "Birthday", "")
	aID := a["id"].(string)
	b := f.newList(t, owner, "Ideas", "private")
	bID := b["id"].(string)
	if a["visibility"] != "link" || !strings.HasPrefix(a["share_path"].(string), "?token=") || b["share_path"] != "" {
		t.Fatal("incorrect default sharing")
	}
	one := f.newWish(t, owner, "Shared gift", aID, bID)
	oneID := one["id"].(string)
	two := f.newWish(t, owner, "Another gift")
	twoID := two["id"].(string)
	foreign := f.newWish(t, other, "Foreign gift")["id"].(string)
	for _, id := range []string{aID, bID} {
		body := f.currentList(t, owner, id)
		if len(body["wishes"].([]any)) != 1 || body["wishes"].([]any)[0].(map[string]any)["id"] != oneID {
			t.Fatal("wish identity was duplicated")
		}
	}
	f.listCall(t, "POST", "/api/lists/"+aID+"/wishes", map[string]any{"wish_ids": []string{twoID, twoID}}, owner, 200)
	f.listCall(t, "POST", "/api/lists/"+aID+"/wishes", map[string]any{"wish_ids": []string{twoID}}, owner, 200)
	f.listCall(t, "POST", "/api/lists/"+aID+"/wishes", map[string]any{"wish_ids": []string{oneID, foreign}}, owner, 400)
	f.listCall(t, "POST", "/api/lists/"+aID+"/wishes", map[string]any{"wish_ids": []string{foreign}}, other, 404)
	current := f.currentList(t, owner, aID)
	if len(current["wishes"].([]any)) != 2 {
		t.Fatal("non-idempotent add or partial unauthorized add")
	}
	updated := f.listCall(t, "PUT", "/api/lists/"+aID, map[string]any{"title": "Renamed", "description": "Updated", "event_date": "2027-07-15", "visibility": "public", "revision": rev(current)}, owner, 200)["list"].(map[string]any)
	if updated["wish_count"] != float64(2) || updated["event_date"] != "2027-07-15" {
		t.Fatal("metadata edit lost contents", updated)
	}
	f.listCall(t, "PUT", "/api/lists/"+aID, map[string]any{"title": "Stale", "revision": rev(current)}, owner, 409)
	copy := f.listCall(t, "POST", "/api/lists/"+aID+"/copy", map[string]any{"title": "Next occasion"}, owner, 201)["list"].(map[string]any)
	copyID := copy["id"].(string)
	if copy["event_date"] != nil || copy["visibility"] != "link" || copy["share_path"] == updated["share_path"] || copy["wish_count"] != float64(2) {
		t.Fatal("copy retained date/link or lost wishes", copy)
	}
	copyBody := f.currentList(t, owner, copyID)
	for _, wish := range copyBody["wishes"].([]any) {
		id := wish.(map[string]any)["id"]
		if id != oneID && id != twoID {
			t.Fatal("copy created independent wishes")
		}
	}
	var count int64
	if err := f.database.Model(&models.Wish{}).Where("owner_id = (SELECT id FROM users WHERE username = ?)", "listowner").Count(&count).Error; err != nil || count != 2 {
		t.Fatal("copy duplicated collection", err, count)
	}
	f.listCall(t, "DELETE", "/api/lists/"+aID+"/wishes/"+oneID, map[string]any{}, owner, 200)
	if len(f.currentList(t, owner, bID)["wishes"].([]any)) != 1 || len(f.currentList(t, owner, copyID)["wishes"].([]any)) != 2 {
		t.Fatal("remove affected another list")
	}
	f.listCall(t, "DELETE", "/api/lists/"+copyID+"/wishes", map[string]any{"revision": rev(copyBody)}, owner, 200)
	if len(f.currentList(t, owner, copyID)["wishes"].([]any)) != 0 {
		t.Fatal("clear failed")
	}
	currentB := f.currentList(t, owner, bID)
	f.listCall(t, "DELETE", "/api/lists/"+bID, map[string]any{"revision": rev(currentB)}, owner, 200)
	f.listCall(t, "GET", "/api/lists/"+bID, map[string]any{}, owner, 404)
	all := f.listCall(t, "GET", "/api/wishes", map[string]any{}, owner, 200)
	if len(all["wishes"].([]any)) != 2 {
		t.Fatal("clear/delete removed collection wishes")
	}
	// A global wish deletion cascades memberships and invalidates stale destructive dialogs.
	before := f.currentList(t, owner, aID)
	f.listCall(t, "DELETE", "/api/wishes/"+twoID, map[string]any{}, owner, 200)
	f.listCall(t, "DELETE", "/api/lists/"+aID+"/wishes", map[string]any{"revision": rev(before)}, owner, 409)
	if len(f.currentList(t, owner, aID)["wishes"].([]any)) != 0 {
		t.Fatal("wish deletion left dangling membership")
	}
}

func TestIntegrationListMembershipAtomicityAndConcurrentEdits(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "membership@example.test", "membership")
	other := f.account(t, "stranger@example.test", "stranger")
	a := f.newList(t, owner, "One", "link")
	b := f.newList(t, owner, "Two", "private")
	foreign := f.newList(t, other, "Foreign", "private")
	aID, bID := a["id"].(string), b["id"].(string)
	f.listCall(t, "POST", "/api/wishes", map[string]any{"title": "Must roll back", "list_ids": []any{aID, foreign["id"]}}, owner, 400)
	if all := f.listCall(t, "GET", "/api/wishes", map[string]any{}, owner, 200); len(all["wishes"].([]any)) != 0 {
		t.Fatal("invalid selected list left an orphan wish")
	}
	wish := f.newWish(t, owner, "Gift", aID)
	wishID := wish["id"].(string)
	membership := f.listCall(t, "GET", "/api/wishes/"+wishID+"/lists", nil, owner, 200)
	f.listCall(t, "PUT", "/api/wishes/"+wishID+"/lists", map[string]any{"list_ids": []any{bID, foreign["id"]}, "version": membership["version"]}, owner, 400)
	f.listCall(t, "PUT", "/api/wishes/"+wishID+"/lists", map[string]any{"list_ids": []string{aID, bID}, "version": membership["version"]}, owner, 200)
	f.listCall(t, "PUT", "/api/wishes/"+wishID+"/lists", map[string]any{"list_ids": []string{}, "version": membership["version"]}, owner, 409)
	if len(f.currentList(t, owner, aID)["wishes"].([]any)) != 1 || len(f.currentList(t, owner, bID)["wishes"].([]any)) != 1 {
		t.Fatal("membership replacement was partial")
	}
	membership = f.listCall(t, "GET", "/api/wishes/"+wishID+"/lists", nil, owner, 200)
	before := f.currentList(t, owner, aID)
	f.listCall(t, "DELETE", "/api/lists/"+aID+"/wishes", map[string]any{"revision": rev(before)}, owner, 200)
	f.listCall(t, "PUT", "/api/wishes/"+wishID+"/lists", map[string]any{"list_ids": []string{aID, bID}, "version": membership["version"]}, owner, 409)
	f.listCall(t, "GET", "/api/wishes/"+wishID+"/lists", nil, other, 404)
	f.listCall(t, "PUT", "/api/wishes/"+wishID+"/lists", map[string]any{"list_ids": []string{}, "version": 1}, other, 404)
	// Concurrent edits against one revision must not silently overwrite each other.
	before = f.currentList(t, owner, bID)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, title := range []string{"First edit", "Second edit"} {
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			status, _ := f.wishRequest(t, "PUT", "/api/lists/"+bID, listBody(t, map[string]any{"title": title, "visibility": "private", "revision": rev(before)}), owner, listOrigin)
			statuses <- status
		}(title)
	}
	wg.Wait()
	close(statuses)
	seen := map[int]int{}
	for status := range statuses {
		seen[status]++
	}
	if seen[200] != 1 || seen[409] != 1 {
		t.Fatal("concurrent update lost data", seen)
	}
	// Database constraints also prohibit cross-owner links independently of API checks.
	var otherUser models.User
	if err := f.database.Where("username = ?", "stranger").First(&otherUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Create(&models.WishlistWish{ListID: aID, WishID: wishID, OwnerID: otherUser.ID}).Error; err == nil {
		t.Fatal("database accepted cross-owner link")
	}
}

func TestIntegrationListSharingAndPrivateData(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "sharing@example.test", "sharing")
	link := f.newList(t, owner, "Hidden link list", "link")
	public := f.newList(t, owner, "Public occasion", "public")
	private := f.newList(t, owner, "Private occasion", "private")
	linkID, publicID := link["id"].(string), public["id"].(string)
	visible := f.newWish(t, owner, "Visible gift", linkID, publicID)
	visibleID := visible["id"].(string)
	hidden := f.newWish(t, owner, "Private gift", private["id"].(string))
	hiddenID := hidden["id"].(string)
	// Image storage is already normalized by the tested upload processor; this checks authorization.
	for _, id := range []string{visibleID, hiddenID} {
		if err := f.database.Create(&models.WishImage{WishID: id, Version: "00000000-0000-4000-8000-000000000001", Data: []byte{255, 216, 255, 217}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := strings.TrimPrefix(link["share_path"].(string), "?token=")
	view := f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": token}, nil, 200)
	if view["username"] != "sharing" || len(view["wishes"].([]any)) != 1 || view["wishes"].([]any)[0].(map[string]any)["id"] != visibleID {
		t.Fatal("shared list exposed unrelated wishes", view)
	}
	for _, object := range []map[string]any{view, view["list"].(map[string]any), view["wishes"].([]any)[0].(map[string]any)} {
		for _, key := range []string{"owner_id", "email", "revision", "share_token", "image_url", "reservations", "purchased", "lists_version"} {
			if _, ok := object[key]; ok {
				t.Fatal("private public field", key)
			}
		}
	}
	profile := f.listCall(t, "GET", "/api/public/profiles/sharing", nil, nil, 200)
	if len(profile["lists"].([]any)) != 1 || profile["lists"].([]any)[0].(map[string]any)["title"] != "Public occasion" {
		t.Fatal("profile exposed private/link lists", profile)
	}
	imageStatus := func(token, id string) int {
		r := httptest.NewRequest("POST", "/api/public/lists/image", strings.NewReader(listBody(t, map[string]string{"token": token, "wish_id": id})))
		r.Header.Set("Origin", listOrigin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Wishix-Request", "1")
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("public image can be cached")
		}
		return w.Code
	}
	if imageStatus(token, visibleID) != 200 || imageStatus(token, hiddenID) != 404 {
		t.Fatal("public image membership not checked")
	}
	current := f.currentList(t, owner, linkID)
	f.listCall(t, "PUT", "/api/lists/"+linkID, map[string]any{"title": "Now private", "visibility": "private", "revision": rev(current)}, owner, 200)
	f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": token}, nil, 404)
	if imageStatus(token, visibleID) != 404 {
		t.Fatal("old image link bypasses private visibility")
	}
	current = f.currentList(t, owner, linkID)
	f.listCall(t, "PUT", "/api/lists/"+linkID, map[string]any{"title": "Link again", "visibility": "link", "revision": rev(current)}, owner, 200)
	current = f.currentList(t, owner, linkID)
	rotated := f.listCall(t, "POST", "/api/lists/"+linkID+"/share-link", map[string]any{"revision": rev(current)}, owner, 200)["list"].(map[string]any)
	newToken := strings.TrimPrefix(rotated["share_path"].(string), "?token=")
	f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": token}, nil, 404)
	f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": newToken}, nil, 200)
	var stored models.Wishlist
	if err := f.database.Where("id = ?", private["id"]).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	f.listCall(t, "POST", "/api/public/lists/view", map[string]string{"token": stored.ShareToken}, nil, 404)
	for _, operation := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/lists", nil}, {"GET", "/api/lists/" + linkID, nil}, {"POST", "/api/lists", map[string]string{"title": "Guest"}}, {"DELETE", "/api/lists/" + linkID, map[string]int{"revision": 1}}, {"POST", "/api/lists/" + linkID + "/copy", map[string]string{"title": "Copy"}}, {"PUT", "/api/wishes/" + visibleID + "/lists", map[string]any{"version": 1, "list_ids": []any{}}},
	} {
		f.listCall(t, operation.method, operation.path, operation.body, nil, 401)
	}
	current = f.currentList(t, owner, publicID)
	f.listCall(t, "DELETE", "/api/lists/"+publicID, map[string]any{"revision": rev(current)}, owner, 200)
	if len(f.listCall(t, "GET", "/api/public/profiles/sharing", nil, nil, 200)["lists"].([]any)) != 0 {
		t.Fatal("deleted list remains public")
	}
}

func TestIntegrationSharePageRedirectAndPrivacy(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "canonical@example.test", "canonical")
	link := f.newList(t, owner, "Link list", "link")
	public := f.newList(t, owner, "Public list", "public")
	private := f.newList(t, owner, "Private list", "private")
	linkToken := strings.TrimPrefix(link["share_path"].(string), "?token=")
	publicToken := strings.TrimPrefix(public["share_path"].(string), "?token=")
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	old := request("/outdated/wishlist/shared?token=" + linkToken)
	if old.Code != http.StatusMovedPermanently || old.Header().Get("Location") != "/canonical/wishlist/shared?token="+linkToken || !strings.Contains(old.Header().Get("X-Robots-Tag"), "noindex") {
		t.Fatal("link-only canonical redirect is incorrect")
	}
	current := request("/canonical/wishlist/shared?token=" + linkToken)
	if current.Code != 200 || current.Header().Get("X-Accel-Redirect") != "/_shared_index" || !strings.Contains(current.Header().Get("X-Robots-Tag"), "noindex") || current.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("link-only response lacks required headers")
	}
	publicRedirect := request("/outdated/wishlist/shared?token=" + publicToken)
	if publicRedirect.Code != http.StatusMovedPermanently || publicRedirect.Header().Get("X-Robots-Tag") != "" {
		t.Fatal("public redirect should be indexable")
	}
	var stored models.Wishlist
	if err := f.database.Where("id = ?", private["id"]).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{stored.ShareToken, "not-a-token", strings.Repeat("x", 43)} {
		w := request("/outdated/wishlist/shared?token=" + invalid)
		if w.Code != 200 || w.Header().Get("Location") != "" || !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") {
			t.Fatal("private or invalid token exposed a redirect")
		}
	}
	if err := f.database.Model(&models.User{}).Where("username = ?", "canonical").Update("username", "renamed").Error; err != nil {
		t.Fatal(err)
	}
	changed := request("/canonical/wishlist/shared?token=" + linkToken)
	if changed.Code != http.StatusMovedPermanently || changed.Header().Get("Location") != "/renamed/wishlist/shared?token="+linkToken {
		t.Fatal("username change did not produce a permanent redirect")
	}
}

func TestIntegrationListInputAndCSRF(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.account(t, "listinput@example.test", "listinput")
	for _, input := range []any{map[string]string{"title": ""}, map[string]string{"title": strings.Repeat("a", 121)}, map[string]string{"title": "Name", "description": strings.Repeat("a", 2001)}, map[string]string{"title": "Name", "event_date": "2027-02-29"}, map[string]string{"title": "Name", "visibility": "archived"}, map[string]string{"title": "Name", "owner_id": "attacker"}} {
		f.listCall(t, "POST", "/api/lists", input, owner, 400)
	}
	if status, _ := f.wishRequest(t, "POST", "/api/lists", `{"title":"Cross site"}`, owner, "https://attacker.test"); status != 403 {
		t.Fatal("cross-site list mutation accepted")
	}
	if status, _ := f.wishRequest(t, "POST", "/api/lists", `{"title":"Name"} {}`, owner, listOrigin); status != 400 {
		t.Fatal("trailing JSON accepted")
	}
}
