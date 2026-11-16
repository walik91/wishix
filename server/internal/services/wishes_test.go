package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wishix/server/internal/models"
)

type editWishRepository struct {
	WishRepository
	wish   *models.Wish
	writes int
}

func (r *editWishRepository) Get(_ context.Context, owner, id string) (*models.Wish, error) {
	if r.wish == nil || r.wish.OwnerID != owner || r.wish.ID != id {
		return nil, nil
	}
	return r.wish, nil
}
func (r *editWishRepository) Update(_ context.Context, wish *models.Wish) (bool, error) {
	r.writes++
	r.wish = wish
	return true, nil
}

type editWishTransaction struct{}

func (editWishTransaction) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestWishEditVersionAndOwnership(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	repo := &editWishRepository{wish: &models.Wish{ID: id, OwnerID: "owner", Title: "Original", Priority: "medium"}}
	service := NewWishes(repo, editWishTransaction{})
	version := WishEditVersion(repo.wish)
	input := WishInput{Title: "Changed", Priority: "high", EditVersion: version}
	if err := service.Update(t.Context(), "other", id, input); !errors.Is(err, ErrWishNotFound) {
		t.Fatalf("other owner: %v", err)
	}
	if err := service.Update(t.Context(), "owner", id, input); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	if repo.writes != 1 || repo.wish.Title != "Changed" {
		t.Fatal("update was not applied")
	}
	if err := service.Update(t.Context(), "owner", id, input); !errors.Is(err, ErrWishConflict) {
		t.Fatalf("stale update: %v", err)
	}
	if repo.writes != 1 {
		t.Fatal("stale update wrote data")
	}
	newVersion := WishEditVersion(repo.wish)
	repo.wish.ImageVersion = "new-image"
	if WishEditVersion(repo.wish) != newVersion {
		t.Fatal("image changes must not conflict with field edits")
	}
}

func TestValidateWish(t *testing.T) {
	text := func(value string) *string { return &value }
	for _, test := range []struct {
		name  string
		input WishInput
		want  error
	}{
		{"manual wish without links or price", WishInput{Title: " A little adventure "}, nil},
		{"Unicode title", WishInput{Title: strings.Repeat("🎁", 200)}, nil},
		{"blank title", WishInput{Title: " \n "}, ErrWishTitle},
		{"long title", WishInput{Title: strings.Repeat("я", 201)}, ErrWishTitle},
		{"long description", WishInput{Title: "Book", Description: strings.Repeat("a", 5001)}, ErrWishDescription},
		{"zero price", WishInput{Title: "Book", Price: text("0"), Currency: "usd"}, nil},
		{"maximum price", WishInput{Title: "Book", Price: text("9999999999.99"), Currency: "EUR"}, nil},
		{"negative price", WishInput{Title: "Book", Price: text("-1"), Currency: "EUR"}, ErrWishPrice},
		{"oversized price", WishInput{Title: "Book", Price: text("10000000000"), Currency: "EUR"}, ErrWishPrice},
		{"fractional cent", WishInput{Title: "Book", Price: text("0.001"), Currency: "EUR"}, ErrWishPrice},
		{"missing currency", WishInput{Title: "Book", Price: text("10")}, ErrWishPrice},
		{"unsupported priority", WishInput{Title: "Book", Priority: "urgent"}, ErrWishPriority},
		{"multiple shop links", WishInput{Title: "Book", ShopLinks: []string{"https://shop.test/book", "http://other.test/book"}}, nil},
		{"script URL", WishInput{Title: "Book", ShopLinks: []string{"javascript:alert(1)"}}, ErrWishLinks},
		{"protocol relative URL", WishInput{Title: "Book", ShopLinks: []string{"//shop.test"}}, ErrWishLinks},
		{"URL credentials", WishInput{Title: "Book", ShopLinks: []string{"https://secret:password@shop.test"}}, ErrWishLinks},
		{"URL whitespace", WishInput{Title: "Book", ShopLinks: []string{"https://shop.test/a b"}}, ErrWishLinks},
		{"too many links", WishInput{Title: "Book", ShopLinks: make([]string, 11)}, ErrWishLinks},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateWish(test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	input, err := validateWish(WishInput{Title: " Book ", Description: " Notes ", Price: text(" 10.1 "), Currency: " eur "})
	if err != nil || input.Title != "Book" || input.Description != "Notes" || *input.Price != "10.10" || input.Currency != "EUR" || input.Priority != "medium" {
		t.Fatalf("incorrect normalization: %+v, %v", input, err)
	}
}
