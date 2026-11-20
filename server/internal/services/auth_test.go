package services

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type suggestionRepository struct {
	AuthRepository
	occupied map[string]bool
	err      error
}

func (r suggestionRepository) UsernameExists(_ context.Context, username string) (bool, error) {
	return r.occupied[username], r.err
}

func TestUsernameSuggestions(t *testing.T) {
	a := &Auth{repo: suggestionRepository{occupied: map[string]bool{"anya": true, "anya2": true}}}
	got, err := a.UsernameSuggestions(context.Background(), "Anya")
	if err != nil || strings.Join(got, ",") != "anya1,anya3,anya4,anya5" {
		t.Fatalf("suggestions %v, error %v", got, err)
	}
	got, err = a.UsernameSuggestions(context.Background(), strings.Repeat("x", 30))
	if err != nil || len(got) != 4 || len(got[1]) != 30 || !strings.HasSuffix(got[1], "1") {
		t.Fatalf("long name suggestions %v, error %v", got, err)
	}
	if _, err = a.UsernameSuggestions(context.Background(), "неверно"); err != ErrUsernameInvalid {
		t.Fatalf("invalid username: %v", err)
	}
	a.repo = suggestionRepository{err: errors.New("database unavailable")}
	if _, err = a.UsernameSuggestions(context.Background(), "anya"); err == nil {
		t.Fatal("expected repository error")
	}
}

func TestPasswordLengthBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		password string
		valid    bool
	}{
		{"empty", "", false},
		{"three characters", "abc", false},
		{"four characters", "abcd", true},
		{"three Cyrillic characters", "абв", false},
		{"four Cyrillic characters", "абвг", true},
		{"four supplementary Unicode characters", "😀😁😂😃", true},
		{"maximum length", strings.Repeat("a", 128), true},
		{"over maximum", strings.Repeat("a", 129), false},
		{"invalid UTF-8", "abcd\xff", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if valid := validatePassword(test.password) == nil; valid != test.valid {
				t.Fatalf("validation accepted=%t, want %t", valid, test.valid)
			}
		})
	}
}
