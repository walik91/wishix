package reservation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// ManagementToken derives a repeatable capability from a secret and a random,
// immutable reservation ID. Domain separation prevents reuse as another token.
// Neither the token nor the secret must be logged or included in public DTOs.
func ManagementToken(secret, reservationID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("wishix:reservation-management:v1\x00"))
	_, _ = mac.Write([]byte(reservationID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
