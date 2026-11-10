package controllers

import "testing"

func TestImportLogURL(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{
		{"https://shop.example.com/product?token=secret&variant=2#secret", "https://shop.example.com/product"},
		{"https://wishix.example.com/r/secret-token?manage=secret", "https://wishix.example.com"},
		{"https://other.example.com/reservations/secret", "https://other.example.com"},
		{"https://user:password@shop.example.com/item", ""},
		{"http://127.0.0.1/private", ""},
	} {
		if got := importLogURL(tt.raw, "https://wishix.example.com"); got != tt.want {
			t.Errorf("%q: got %q want %q", tt.raw, got, tt.want)
		}
	}
}
