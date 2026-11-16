package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type googleRoundTrip func(*http.Request) (*http.Response, error)

func (f googleRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGoogleCodeExchangeRequiresVerifiedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, userinfo string
		ok             bool
	}{
		{"verified", `{"sub":"google-sub","email":"person@example.com","email_verified":true}`, true},
		{"unverified", `{"sub":"google-sub","email":"person@example.com","email_verified":false}`, false},
		{"missing subject", `{"email":"person@example.com","email_verified":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewGoogleClient(&Config{Origin: "https://wishix.test", GoogleClientID: "client", GoogleClientSecret: "secret"})
			client.client.Transport = googleRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				response := `{"access_token":"access","token_type":"Bearer"}`
				switch r.URL.String() {
				case googleToken:
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if r.Method != "POST" || r.Form.Get("code") != "auth-code" || r.Form.Get("code_verifier") != "verifier" || r.Form.Get("redirect_uri") != "https://wishix.test/api/auth/google/callback" {
						t.Fatal("incorrect token exchange")
					}
				case googleUserInfo:
					if r.Header.Get("Authorization") != "Bearer access" {
						t.Fatal("missing access token")
					}
					response = tc.userinfo
				default:
					t.Fatal("unexpected endpoint")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
			})
			identity, err := client.Exchange(context.Background(), "auth-code", "verifier")
			if (err == nil) != tc.ok || calls != 2 {
				t.Fatalf("identity=%+v err=%v calls=%d", identity, err, calls)
			}
		})
	}
}

func TestGoogleAuthorizationUsesPKCEAndExactCallback(t *testing.T) {
	client := NewGoogleClient(&Config{Origin: "https://wishix.test", GoogleClientID: "client", GoogleClientSecret: "secret"})
	address, err := url.Parse(client.AuthorizationURL("state", "verifier"))
	if err != nil {
		t.Fatal(err)
	}
	q := address.Query()
	if address.Scheme != "https" || address.Host != "accounts.google.com" || q.Get("state") != "state" || q.Get("redirect_uri") != "https://wishix.test/api/auth/google/callback" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != "iMnq5o6zALKXGivsnlom_0F5_WYda32GHkxlV7mq7hQ" {
		t.Fatal("unexpected Google authorization request")
	}
}
