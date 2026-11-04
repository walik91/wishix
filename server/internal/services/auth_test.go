package services

import (
	"strings"
	"testing"
)

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
