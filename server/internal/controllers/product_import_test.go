package controllers

import "testing"

func TestImportLogURL(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{
		{"https://shop.example.com/product?variant=2&ref=a%2Fb#option", "https://shop.example.com/product?variant=2&ref=a%2Fb#option"},
		{"https://shop.example.com/product?discount=10%25", "https://shop.example.com/product?discount=10%25"},
		{"HTTPS://shop.example.com/product?x=1", "HTTPS://shop.example.com/product?x=1"},
		{"https://wishix.example.com/r/secret-token?manage=secret", "https://wishix.example.com"},
		{"https://wishix.example.com/reservation#secret-token", "https://wishix.example.com"},
		{"https://other.example.com/reservations/secret", "https://other.example.com"},
		{"https://other.example.com/product?redirect=https%3A%2F%2Fwishix.example.com%2Freservation%23secret", "https://other.example.com"},
		{"https://other.example.com/product?redirect=%252Freservation%2523secret", "https://other.example.com"},
		{"https://other.example.com/product?bad=%ZZ&redirect=https%3A%2F%2Fwishix.example.com%2Freservation%23secret", "https://other.example.com"},
		{"https://user:password@shop.example.com/item", ""},
		{"http://127.0.0.1/private", ""},
	} {
		if got := importLogURL(tt.raw, "https://wishix.example.com"); got != tt.want {
			t.Errorf("%q: got %q want %q", tt.raw, got, tt.want)
		}
	}
}
