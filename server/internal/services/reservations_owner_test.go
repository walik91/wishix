package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type ownerTestAuthRepository struct{ AuthRepository }

func (ownerTestAuthRepository) Allow(context.Context, string, int, time.Time, time.Time) (bool, error) {
	return true, nil
}

type ownerTestReservationRepository struct {
	ReservationRepository
	challenge *models.ReservationChallenge
	inserted  bool
}

func (r *ownerTestReservationRepository) Context(context.Context, string, string, string, string) (*ReservationContext, error) {
	return &ReservationContext{List: &models.Wishlist{ID: "00000000-0000-4000-8000-000000000001", OwnerID: "owner"}, Wish: models.Wish{ID: "00000000-0000-4000-8000-000000000002", OwnerID: "owner"}}, nil
}
func (r *ownerTestReservationRepository) InsertChallenge(_ context.Context, c *models.ReservationChallenge) error {
	r.challenge = c
	return nil
}
func (r *ownerTestReservationRepository) Challenge(context.Context, string) (*models.ReservationChallenge, error) {
	return r.challenge, nil
}
func (r *ownerTestReservationRepository) ByWish(context.Context, string) (*models.Reservation, error) {
	return nil, nil
}
func (r *ownerTestReservationRepository) LockLimit(context.Context, string) error { return nil }
func (r *ownerTestReservationRepository) CountActive(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *ownerTestReservationRepository) Insert(context.Context, *models.Reservation) error {
	r.inserted = true
	return nil
}
func (r *ownerTestReservationRepository) Queue(context.Context, *models.ReservationMailOutbox) error {
	return nil
}
func (r *ownerTestReservationRepository) DeleteChallenge(context.Context, string) error { return nil }

type ownerTestMailer struct{ code string }

func (m *ownerTestMailer) SendCode(_ context.Context, _, _, _, code string) error {
	m.code = code
	return nil
}

func TestOwnerCannotReserveOwnWish(t *testing.T) {
	ctx := context.Background()
	config := &security.Config{Secret: strings.Repeat("secret", 8)}
	repo, mail := &ownerTestReservationRepository{}, &ownerTestMailer{}
	tx := editWishTransaction{}
	auth := &Auth{repo: ownerTestAuthRepository{}, config: config, now: time.Now}
	svc := NewReservations(repo, tx, auth, mail, nil, config)
	input := ReservationInput{WishID: "00000000-0000-4000-8000-000000000002", ListID: "00000000-0000-4000-8000-000000000001", Email: "owner@example.test"}
	owner := &models.User{ID: "owner", Email: input.Email}
	if _, err := svc.Start(ctx, input, owner); !errors.Is(err, ErrReservationUnavailable) || repo.challenge != nil || mail.code != "" {
		t.Fatalf("owner start: %v, challenge=%v", err, repo.challenge != nil)
	}
	// A challenge issued before the restriction was deployed must also be rejected.
	token, code := strings.Repeat("a", 43), "123456"
	id := security.Digest(token)
	repo.challenge = &models.ReservationChallenge{ID: id, WishID: input.WishID, UserID: &owner.ID, Email: input.Email, CodeHash: security.MAC(config.Secret, "reservation-code:"+id+":"+code), AccessHash: security.MAC(config.Secret, "reservation-access:"+accessKey(input)), ExpiresAt: time.Now().Add(time.Minute)}
	input.Challenge, input.Code = token, code
	if err := svc.Confirm(ctx, input, owner); !errors.Is(err, ErrReservationUnavailable) || repo.inserted {
		t.Fatalf("owner confirmation: %v, inserted=%v", err, repo.inserted)
	}
	visitor := &models.User{ID: "visitor", Email: "visitor@example.test"}
	input.Email, input.Challenge, input.Code = visitor.Email, "", ""
	challenge, err := svc.Start(ctx, input, visitor)
	if err != nil {
		t.Fatalf("visitor start: %v", err)
	}
	input.Challenge, input.Code = challenge, mail.code
	if err := svc.Confirm(ctx, input, visitor); err != nil || !repo.inserted {
		t.Fatalf("visitor confirmation: %v, inserted=%v", err, repo.inserted)
	}
}
