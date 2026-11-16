package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestFacebookAuthorizationAndExchange(t *testing.T) {
	client := NewFacebookClient(&Config{Origin: "https://wishix.test", FacebookAppID: "app", FacebookAppSecret: "secret"})
	address, err := url.Parse(client.AuthorizationURL("state", "unused"))
	if err != nil || address.Host != "www.facebook.com" || address.Query().Get("redirect_uri") != "https://wishix.test/api/auth/facebook/callback" || address.Query().Get("state") != "state" {
		t.Fatal("incorrect authorization request")
	}
	calls := 0
	client.client.Transport = googleRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"access"}`
		if calls == 1 {
			if r.URL.Scheme != "https" || r.URL.Host != "graph.facebook.com" || r.URL.Path != "/v27.0/oauth/access_token" || r.URL.Query().Get("code") != "code" || r.URL.Query().Get("client_secret") != "secret" {
				t.Fatal("invalid exchange")
			}
		} else {
			if r.URL.Scheme != "https" || r.URL.Host != "graph.facebook.com" || r.URL.Path != "/v27.0/me" || r.URL.Query().Get("fields") != "id,email" || r.URL.Query().Get("appsecret_proof") == "" || r.Header.Get("Authorization") != "Bearer access" {
				t.Fatal("invalid user request")
			}
			body = `{"id":"app-scoped-id"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	identity, err := client.Exchange(context.Background(), "code", "")
	if err != nil || identity.Subject != "app-scoped-id" || identity.Email != "" || calls != 2 {
		t.Fatal("identity without email rejected", err)
	}
}
