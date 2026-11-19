//go:build integration

package services_test

import (
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

func TestIntegrationSocialPrivacyAndProfile(t *testing.T) {
	f := newAuthFixture(t)
	alice := f.account(t, "social-alice@example.test", "socialalice")
	bob := f.account(t, "social-bob@example.test", "socialbob")
	carol := f.account(t, "social-carol@example.test", "socialcarol")
	details := map[string]any{"biography": "Public description", "birth_date": "2000-02-29", "show_age": false, "show_followers": false, "show_following": false}
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	private := f.listCall(t, "GET", "/api/profile/details", nil, alice, 200)
	if private["birth_date"] != "2000-02-29" || private["biography"] != "Public description" {
		t.Fatal("private settings missing", private)
	}
	public := f.listCall(t, "GET", "/api/public/profiles/socialalice/details", nil, nil, 200)
	if public["birth_date"] != nil || public["age"] != nil || public["biography"] != "Public description" || public["has_avatar"] != false {
		t.Fatal("public description or private age incorrect", public)
	}
	image := testImage(t, color.RGBA{R: 180, G: 60, B: 80, A: 255})
	if res := f.imageRequest(t, "PUT", "/api/profile/avatar", image, "image/png", alice, listOrigin); res.Code != 200 {
		t.Fatalf("upload: %d %s", res.Code, res.Body.String())
	}
	if res := f.imageRequest(t, "GET", "/api/public/profiles/socialalice/avatar", nil, "", nil, listOrigin); res.Code != 200 || res.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("uploaded avatar unavailable", res.Code)
	}
	searchItem := f.listCall(t, "GET", "/api/search/people?q=socialalice", nil, bob, 200)["people"].([]any)[0].(map[string]any)
	if searchItem["has_avatar"] != true {
		t.Fatal("public avatar missing in search", searchItem)
	}
	details["show_age"] = true
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	public = f.listCall(t, "GET", "/api/public/profiles/socialalice/details", nil, nil, 200)
	if public["age"] == nil || public["birth_date"] != nil || public["biography"] != "Public description" || public["has_avatar"] != true {
		t.Fatal("public details incorrect", public)
	}
	if res := f.imageRequest(t, "GET", "/api/public/profiles/socialalice/avatar", nil, "", nil, listOrigin); res.Code != 200 || res.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("avatar unavailable", res.Code)
	}
	searchItem = f.listCall(t, "GET", "/api/search/people?q=socialalice", nil, bob, 200)["people"].([]any)[0].(map[string]any)
	if searchItem["has_avatar"] != true {
		t.Fatal("public avatar missing in search", searchItem)
	}
	details["show_age"] = false
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	public = f.listCall(t, "GET", "/api/public/profiles/socialalice/details", nil, nil, 200)
	if public["age"] != nil || public["birth_date"] != nil || public["has_avatar"] != true {
		t.Fatal("age privacy not restored", public)
	}
	if res := f.imageRequest(t, "PUT", "/api/profile/avatar", image, "image/png", alice, "https://attacker.test"); res.Code != 403 {
		t.Fatal("cross origin upload", res.Code)
	}
	if res := f.imageRequest(t, "PUT", "/api/profile/avatar", []byte("not an image"), "image/png", alice, listOrigin); res.Code != 400 {
		t.Fatal("invalid image accepted", res.Code)
	}
	f.listCall(t, "PUT", "/api/social/socialbob/follow", map[string]any{"follow": true}, alice, 200)
	f.listCall(t, "PUT", "/api/social/socialalice/follow", map[string]any{"follow": true}, bob, 200)
	f.listCall(t, "PUT", "/api/social/socialalice/follow", map[string]any{"follow": true}, alice, 400)
	f.listCall(t, "PUT", "/api/social/socialalice/follow", map[string]any{"follow": true}, nil, 401)
	f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, carol, 404)
	f.listCall(t, "GET", "/api/social/socialalice/people/following", nil, carol, 404)
	if items := f.listCall(t, "GET", "/api/social/socialbob/people/followers", nil, bob, 200)["people"].([]any); len(items) != 1 || items[0].(map[string]any)["has_avatar"] != true {
		t.Fatal("public avatar missing in social list", items)
	}
	f.listCall(t, "DELETE", "/api/profile/avatar", nil, alice, 200)
	if res := f.imageRequest(t, "GET", "/api/public/profiles/socialalice/avatar", nil, "", nil, listOrigin); res.Code != 404 {
		t.Fatal("deleted avatar remained public", res.Code)
	}
	details["biography"] = ""
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	public = f.listCall(t, "GET", "/api/public/profiles/socialalice/details", nil, nil, 200)
	if public["has_avatar"] != false || public["biography"] != nil {
		t.Fatal("removed public details still visible", public)
	}
	details["show_following"] = true
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	if items := f.listCall(t, "GET", "/api/social/socialalice/people/following", nil, nil, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("public following", items)
	}
	f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, nil, 404)
	details["show_following"] = false
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	f.listCall(t, "GET", "/api/social/socialalice/people/following", nil, nil, 404)
	if items := f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, alice, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("owner followers", items)
	}
	details["show_followers"] = true
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	if items := f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, nil, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("public followers", items)
	}
	details["show_followers"] = false
	f.listCall(t, "PUT", "/api/profile/details", details, alice, 200)
	f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, nil, 404)
	if items := f.listCall(t, "GET", "/api/social/socialalice/people/followers", nil, alice, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("owner lost access after hiding followers", items)
	}
	privateList := f.newList(t, alice, "Secret", "private")["id"].(string)
	f.listCall(t, "GET", "/api/lists/"+privateList, nil, bob, 404)
	f.listCall(t, "POST", "/api/public/lists/guest", map[string]any{"id": privateList, "token": ""}, nil, 404)
	f.listCall(t, "POST", "/api/social/socialalice/report", map[string]any{"message": "Issue"}, bob, 200)
	var count int64
	if err := f.database.Table("user_reports").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("report not saved", count, err)
	}
	for i := 0; i < 9; i++ {
		f.listCall(t, "POST", "/api/social/socialalice/report", map[string]any{"message": "Issue"}, bob, 200)
	}
	f.listCall(t, "POST", "/api/social/socialalice/report", map[string]any{"message": "Issue"}, bob, 429)
	f.listCall(t, "PUT", "/api/social/socialbob/block", map[string]any{"block": true}, alice, 200)
	if err := f.database.Table("user_follows").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("block did not remove follows", count, err)
	}
	f.listCall(t, "PUT", "/api/social/socialalice/follow", map[string]any{"follow": true}, bob, 409)
	if items := f.listCall(t, "GET", "/api/social/socialalice/people/blocked", nil, alice, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("blocked list", items)
	}
	f.listCall(t, "GET", "/api/social/socialalice/people/blocked", nil, bob, 404)
	f.listCall(t, "PUT", "/api/social/socialbob/block", map[string]any{"block": false}, alice, 200)
	f.listCall(t, "PUT", "/api/social/socialalice/follow", map[string]any{"follow": true}, bob, 200)
	f.listCall(t, "PUT", "/api/profile/username", map[string]any{"username": "socialnew"}, alice, 200)
	var redirectExpiry struct{ ExpiresAt time.Time }
	if err := f.database.Table("username_registry").Select("expires_at").Where("username = ?", "socialalice").Scan(&redirectExpiry).Error; err != nil {
		t.Fatal(err)
	}
	if remaining := time.Until(redirectExpiry.ExpiresAt); remaining < 60*24*time.Hour-time.Minute || remaining > 60*24*time.Hour {
		t.Fatalf("unexpected redirect duration: %v", remaining)
	}
	oldURL := httptest.NewRecorder()
	f.handler.ServeHTTP(oldURL, httptest.NewRequest("GET", "/u/socialalice", nil))
	if oldURL.Code != http.StatusFound || oldURL.Header().Get("Location") != "/u/socialnew" || oldURL.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("old profile did not redirect safely", oldURL.Code, oldURL.Header())
	}
	if p := f.listCall(t, "GET", "/api/public/profiles/socialalice", nil, nil, 200); p["username"] != "socialnew" {
		t.Fatal("old profile redirect not resolved", p)
	}
	f.listCall(t, "PUT", "/api/profile/username", map[string]any{"username": "anothername", "redirect": false}, alice, 409)
	if status, _, _ := f.request(t, "register", map[string]string{"username": "socialalice", "email": "reserved@example.test", "password": initialPassword}, nil); status != http.StatusConflict {
		t.Fatalf("redirect name not reserved: %d", status)
	}
	if items := f.listCall(t, "GET", "/api/social/socialnew/people/followers", nil, alice, 200)["people"].([]any); len(items) != 1 {
		t.Fatal("rename lost follows", items)
	}
	if err := f.database.Exec("UPDATE username_registry SET expires_at = ? WHERE username = ?", time.Now().Add(-time.Hour), "socialalice").Error; err != nil {
		t.Fatal(err)
	}
	f.listCall(t, "GET", "/api/public/profiles/socialalice", nil, nil, 404)
	if status, _, _ := f.request(t, "register", map[string]string{"username": "socialalice", "email": "expired-reserved@example.test", "password": initialPassword}, nil); status != http.StatusConflict {
		t.Fatalf("expired address can point to another account: %d", status)
	}
	expiredURL := httptest.NewRecorder()
	f.handler.ServeHTTP(expiredURL, httptest.NewRequest("GET", "/u/socialalice", nil))
	if expiredURL.Header().Get("Location") != "" {
		t.Fatal("expired username still redirects", expiredURL.Header())
	}
	pending := f.registration(t, "pending-claim@example.test", "pendingclaim")
	f.listCall(t, "PUT", "/api/profile/username", map[string]any{"username": "pendingclaim", "redirect": false}, carol, 200)
	if err := f.database.Exec("UPDATE users SET username_changed_at = ? WHERE username = ?", time.Now().Add(-31*24*time.Hour), "pendingclaim").Error; err != nil {
		t.Fatal(err)
	}
	f.listCall(t, "PUT", "/api/profile/username", map[string]any{"username": "carolfinal", "redirect": true}, carol, 200)
	status, _, _ := f.request(t, "verify-email", map[string]string{"challenge": pending, "code": f.mail.code("pending-claim@example.test", "register")}, nil)
	if status != http.StatusConflict {
		t.Fatalf("stale registration captured renamed account: %d", status)
	}
	if p := f.listCall(t, "GET", "/api/public/profiles/pendingclaim", nil, nil, 200); p["username"] != "carolfinal" {
		t.Fatal("stale signup changed redirect target", p)
	}
	if p := f.listCall(t, "GET", "/api/public/profiles/socialcarol", nil, nil, 200); p["username"] != "carolfinal" {
		t.Fatal("legacy redirect=false disabled automatic redirect", p)
	}
	if err := f.database.Exec("UPDATE users SET username_changed_at = ? WHERE username = ?", time.Now().Add(-31*24*time.Hour), "socialnew").Error; err != nil {
		t.Fatal(err)
	}
	f.listCall(t, "PUT", "/api/profile/username", map[string]any{"username": "socialalice", "redirect": false}, alice, 200)
	if p := f.listCall(t, "GET", "/api/public/profiles/socialnew", nil, nil, 200); p["username"] != "socialalice" {
		t.Fatal("previous username did not redirect after reclaiming an old username", p)
	}
}

func TestIntegrationSocialConcurrentBlockAndUsernameClaim(t *testing.T) {
	f := newAuthFixture(t)
	f.account(t, "race-social-a@example.test", "racesociala")
	f.account(t, "race-social-b@example.test", "racesocialb")
	var a, b models.User
	if err := f.database.Where("username = ?", "racesociala").Take(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Where("username = ?", "racesocialb").Take(&b).Error; err != nil {
		t.Fatal(err)
	}
	executor := db.NewExecutor(f.database)
	social := services.NewSocial(repositories.NewSocial(executor), db.NewTransactionManager(executor))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var followErr, blockErr error
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; followErr = social.Follow(ctx, b.ID, a.Username, true) }()
	go func() { defer wg.Done(); <-start; blockErr = social.Block(ctx, a.ID, b.Username, true) }()
	close(start)
	wg.Wait()
	if blockErr != nil {
		t.Fatal("block", blockErr)
	}
	if followErr != nil && followErr != services.ErrSocialUnavailable {
		t.Fatal("follow", followErr)
	}
	var count int64
	if err := f.database.Table("user_follows").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("follow survived block", count, err)
	}
	var first, second error
	start = make(chan struct{})
	wg.Add(2)
	go func() { defer wg.Done(); <-start; first = social.Rename(ctx, a.ID, "shared_target") }()
	go func() { defer wg.Done(); <-start; second = social.Rename(ctx, b.ID, "shared_target") }()
	close(start)
	wg.Wait()
	if !((first == nil && second == services.ErrUsernameTaken) || (second == nil && first == services.ErrUsernameTaken)) {
		t.Fatalf("username race: %v / %v", first, second)
	}
	f.account(t, "race-social-c@example.test", "racesocialc")
	var thirdUser models.User
	if err := f.database.Where("username = ?", "racesocialc").Take(&thirdUser).Error; err != nil {
		t.Fatal(err)
	}
	pending := f.registration(t, "race-pending@example.test", "race_signup")
	code := f.mail.code("race-pending@example.test", "register")
	var renameErr, signupErr error
	start = make(chan struct{})
	wg.Add(2)
	go func() { defer wg.Done(); <-start; renameErr = social.Rename(ctx, thirdUser.ID, "race_signup") }()
	go func() { defer wg.Done(); <-start; _, signupErr = f.auth.Verify(ctx, pending, code) }()
	close(start)
	wg.Wait()
	if !((renameErr == nil && signupErr == services.ErrRegistrationUnavailable) || (renameErr == services.ErrUsernameTaken && signupErr == nil)) {
		t.Fatalf("rename/signup race: %v / %v", renameErr, signupErr)
	}
}
