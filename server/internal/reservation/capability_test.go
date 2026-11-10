package reservation

import (
	"encoding/base64"
	"testing"
)

func TestManagementToken(t *testing.T) {
	const secret = "an example secret longer than 32 bytes"
	const reservationID = "3a942db3-665b-40fb-926d-3c901c9c8c54"
	token := ManagementToken(secret, reservationID)
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("invalid token encoding or entropy length: %v", err)
	}
	if again := ManagementToken(secret, reservationID); token != again {
		t.Fatal("the original link cannot be restored")
	}
	if other := ManagementToken(secret, "4a942db3-665b-40fb-926d-3c901c9c8c54"); token == other {
		t.Fatal("different reservations share a token")
	}
	if other := ManagementToken("another secret longer than 32 bytes", reservationID); token == other {
		t.Fatal("different secrets share a token")
	}
}
