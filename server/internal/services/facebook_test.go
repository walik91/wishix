package services

import (
	"context"
	"strings"
	"testing"

	security "wishix/server/internal/auth"
)

func TestDisabledFacebookRejectsAllFlowOperations(t *testing.T) {
	provider := security.NewFacebookClient(&security.Config{FacebookAppID: "app"})
	facebook := NewFacebook(nil, nil, provider)
	ctx := context.Background()
	token := strings.Repeat("a", 43)
	if _, _, err := facebook.Start(ctx, "login", ""); err != ErrInvalidInput {
		t.Fatal("disabled Facebook started")
	}
	if _, err := facebook.Callback(ctx, token, "", "code"); err != ErrInvalidInput {
		t.Fatal("disabled Facebook accepted callback")
	}
	if _, err := facebook.Complete(ctx, token, "username", ""); err != ErrInvalidInput {
		t.Fatal("disabled Facebook completed registration")
	}
}
