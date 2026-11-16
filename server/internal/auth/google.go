package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleAuthorization = "https://accounts.google.com/o/oauth2/v2/auth"
const googleToken = "https://oauth2.googleapis.com/token"
const googleUserInfo = "https://openidconnect.googleapis.com/v1/userinfo"

var ErrGoogle = errors.New("google authentication failed")

type GoogleIdentity struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

type GoogleClient struct {
	config *Config
	client *http.Client
}

func NewGoogleClient(config *Config) *GoogleClient {
	return &GoogleClient{config: config, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (g *GoogleClient) Enabled() bool {
	return g.config.GoogleClientID != "" && g.config.GoogleClientSecret != ""
}

func (g *GoogleClient) AuthorizationURL(state, verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	values := url.Values{
		"client_id": {g.config.GoogleClientID}, "redirect_uri": {g.config.Origin + "/api/auth/google/callback"},
		"response_type": {"code"}, "scope": {"openid email"}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"},
		"prompt": {"select_account"},
	}
	return googleAuthorization + "?" + values.Encode()
}

func (g *GoogleClient) Exchange(ctx context.Context, code, verifier string) (GoogleIdentity, error) {
	var identity GoogleIdentity
	if !g.Enabled() || code == "" || len(code) > 4096 {
		return identity, ErrGoogle
	}
	form := url.Values{"code": {code}, "code_verifier": {verifier}, "client_id": {g.config.GoogleClientID}, "client_secret": {g.config.GoogleClientSecret}, "redirect_uri": {g.config.Origin + "/api/auth/google/callback"}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleToken, strings.NewReader(form.Encode()))
	if err != nil {
		return identity, ErrGoogle
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tokens struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err = g.json(req, &tokens); err != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") {
		return identity, ErrGoogle
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfo, nil)
	if err != nil {
		return identity, ErrGoogle
	}
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	if err = g.json(req, &identity); err != nil || identity.Subject == "" || len(identity.Subject) > 255 || !identity.EmailVerified {
		return GoogleIdentity{}, ErrGoogle
	}
	return identity, nil
}

func (g *GoogleClient) json(req *http.Request, target any) error {
	response, err := g.client.Do(req)
	if err != nil {
		return ErrGoogle
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrGoogle
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(target); err != nil {
		return ErrGoogle
	}
	return nil
}
