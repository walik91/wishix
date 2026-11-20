package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"wishix/server/internal/models"
)

type suggestionGoogleRepository struct {
	GoogleRepository
	pending *models.AuthGooglePending
	err     error
}

func (r suggestionGoogleRepository) Pending(_ context.Context, _ string, _ time.Time) (*models.AuthGooglePending, error) {
	return r.pending, r.err
}

func TestGoogleUsernameSuggestions(t *testing.T) {
	auth := &Auth{repo: suggestionRepository{occupied: map[string]bool{"anya_smith": true}}, now: time.Now}
	google := &Google{auth: auth}
	for _, test := range []struct{ email, first string }{
		{"Anya.Smith+tag@example.test", "anya_smith_tag"},
		{"anya.smith@example.test", "anya_smith1"},
		{"a@example.test", "auser"},
		{"+++@example.test", "user"},
		{strings.Repeat("a", 40) + "@example.test", strings.Repeat("a", 30)},
	} {
		google.repo = suggestionGoogleRepository{pending: &models.AuthGooglePending{Email: test.email}}
		got, err := google.UsernameSuggestions(context.Background(), strings.Repeat("x", 43))
		if err != nil || len(got) != 4 || got[0] != test.first {
			t.Fatalf("email %q: suggestions %v, error %v", test.email, got, err)
		}
	}
	google.repo = suggestionGoogleRepository{}
	if _, err := google.UsernameSuggestions(context.Background(), "short"); err != ErrInvalidInput {
		t.Fatalf("invalid token: %v", err)
	}
	if _, err := google.UsernameSuggestions(context.Background(), strings.Repeat("x", 43)); err != ErrInvalidInput {
		t.Fatalf("missing or expired pending: %v", err)
	}
	google.repo = suggestionGoogleRepository{err: errors.New("database unavailable")}
	if _, err := google.UsernameSuggestions(context.Background(), strings.Repeat("x", 43)); err == nil {
		t.Fatal("expected repository error")
	}
}
