package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	security "wishix/server/internal/auth"
)

func TestFacebookCallbackRejectsMissingOrMismatchedBrowserState(t *testing.T) {
	controller := NewFacebook(nil, NewAuth(nil, &security.Config{SecureCookie: true}, nil))
	for _, stored := range []string{"", strings.Repeat("b", 43)} {
		request := httptest.NewRequest("GET", "/api/auth/facebook/callback?state="+strings.Repeat("a", 43)+"&code=secret", nil)
		if stored != "" {
			request.AddCookie(&http.Cookie{Name: facebookStateCookie, Value: stored})
		}
		response := httptest.NewRecorder()
		controller.callback(response, request)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?facebook=error" {
			t.Fatal("mismatched state accepted")
		}
		if response.Header().Get("Referrer-Policy") != "no-referrer" || strings.Contains(response.Header().Get("Location"), "secret") {
			t.Fatal("callback leaked code")
		}
	}
}
