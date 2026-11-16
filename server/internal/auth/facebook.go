package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const facebookAuthorization = "https://www.facebook.com/v27.0/dialog/oauth"
const facebookToken = "https://graph.facebook.com/v27.0/oauth/access_token"
const facebookUserInfo = "https://graph.facebook.com/v27.0/me"

var ErrFacebook = errors.New("facebook authentication failed")

type FacebookIdentity struct {
	Subject string `json:"id"`
	Email   string `json:"email"`
}

type FacebookClient struct {
	config *Config
	client *http.Client
}

func NewFacebookClient(config *Config) *FacebookClient {
	return &FacebookClient{config: config, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (f *FacebookClient) Enabled() bool {
	return strings.TrimSpace(f.config.FacebookAppID) != "" && strings.TrimSpace(f.config.FacebookAppSecret) != ""
}

func (f *FacebookClient) AuthorizationURL(state, _ string) string {
	values := url.Values{"client_id": {f.config.FacebookAppID}, "redirect_uri": {f.config.Origin + "/api/auth/facebook/callback"}, "response_type": {"code"}, "scope": {"public_profile,email"}, "state": {state}}
	return facebookAuthorization + "?" + values.Encode()
}

func (f *FacebookClient) Exchange(ctx context.Context, code, _ string) (FacebookIdentity, error) {
	var identity FacebookIdentity
	if !f.Enabled() || code == "" || len(code) > 4096 {
		return identity, ErrFacebook
	}
	values := url.Values{"code": {code}, "client_id": {f.config.FacebookAppID}, "client_secret": {f.config.FacebookAppSecret}, "redirect_uri": {f.config.Origin + "/api/auth/facebook/callback"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, facebookToken+"?"+values.Encode(), nil)
	if err != nil {
		return identity, ErrFacebook
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if f.json(req, &tokens) != nil || tokens.AccessToken == "" {
		return identity, ErrFacebook
	}
	proof := hmac.New(sha256.New, []byte(f.config.FacebookAppSecret))
	_, _ = proof.Write([]byte(tokens.AccessToken))
	query := url.Values{"fields": {"id,email"}, "appsecret_proof": {hex.EncodeToString(proof.Sum(nil))}}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, facebookUserInfo+"?"+query.Encode(), nil)
	if err != nil {
		return identity, ErrFacebook
	}
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	if f.json(req, &identity) != nil || identity.Subject == "" || len(identity.Subject) > 255 {
		return FacebookIdentity{}, ErrFacebook
	}
	return identity, nil
}

func (f *FacebookClient) json(req *http.Request, target any) error {
	response, err := f.client.Do(req)
	if err != nil {
		return ErrFacebook
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrFacebook
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(target) != nil {
		return ErrFacebook
	}
	return nil
}
