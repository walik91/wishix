package services

import (
	"errors"
	"strings"
	"testing"
)

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
