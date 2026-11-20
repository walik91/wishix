package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Facebook struct{ Base }

func NewFacebook(executor *db.Executor) *Facebook { return &Facebook{Base: NewBase(executor)} }

func (r *Facebook) CreateFlow(ctx context.Context, flow *models.AuthFacebookFlow) error {
	return r.Ctx(ctx).Create(flow).Error
}

// Delete and return the state in one statement: an OAuth callback cannot be replayed.
func (r *Facebook) ConsumeFlow(ctx context.Context, hash string, now time.Time) (*models.AuthFacebookFlow, error) {
	var flows []models.AuthFacebookFlow
	err := r.Ctx(ctx).Raw(`DELETE FROM auth_facebook_flows WHERE state_hash = ? AND expires_at > ? RETURNING *`, hash, now).Scan(&flows).Error
	if err != nil || len(flows) == 0 {
		return nil, err
	}
	return &flows[0], nil
}

func (r *Facebook) UserBySubject(ctx context.Context, subject string) (*models.User, error) {
	var user models.User
	err := r.Ctx(ctx).Model(&models.User{}).Select("users.*").Joins("JOIN auth_facebook_identities ON auth_facebook_identities.user_id = users.id AND auth_facebook_identities.deleted_at IS NULL").Where("auth_facebook_identities.subject = ?", subject).Take(&user).Error
	return userResult(&user, err)
}

func (r *Facebook) IdentityByUser(ctx context.Context, userID string) (*models.AuthFacebookIdentity, error) {
	var identity models.AuthFacebookIdentity
	err := r.Ctx(ctx).Where("user_id = ?", userID).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &identity, err
}

func (r *Facebook) CreateIdentity(ctx context.Context, identity *models.AuthFacebookIdentity) error {
	err := r.Ctx(ctx).Create(identity).Error
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return services.ErrRegistrationUnavailable
	}
	return err
}

func (r *Facebook) CreatePending(ctx context.Context, pending *models.AuthFacebookPending) error {
	return r.Ctx(ctx).Create(pending).Error
}

func (r *Facebook) PendingForUpdate(ctx context.Context, hash string, now time.Time) (*models.AuthFacebookPending, error) {
	var pending models.AuthFacebookPending
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND expires_at > ?", hash, now).Take(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pending, err
}

func (r *Facebook) DeletePending(ctx context.Context, hash string) error {
	return r.Ctx(ctx).Where("token_hash = ?", hash).Delete(&models.AuthFacebookPending{}).Error
}

func (r *Facebook) Cleanup(ctx context.Context, now time.Time) error {
	if err := r.Ctx(ctx).Where("expires_at <= ?", now).Delete(&models.AuthFacebookFlow{}).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Where("expires_at <= ?", now).Delete(&models.AuthFacebookPending{}).Error
}
