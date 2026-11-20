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

type Google struct{ Base }

func NewGoogle(executor *db.Executor) *Google { return &Google{Base: NewBase(executor)} }

func (r *Google) CreateFlow(ctx context.Context, flow *models.AuthGoogleFlow) error {
	return r.Ctx(ctx).Create(flow).Error
}

// Delete and return the state in one statement: an OAuth callback cannot be replayed.
func (r *Google) ConsumeFlow(ctx context.Context, hash string, now time.Time) (*models.AuthGoogleFlow, error) {
	var flows []models.AuthGoogleFlow
	err := r.Ctx(ctx).Raw(`DELETE FROM auth_google_flows WHERE state_hash = ? AND expires_at > ? RETURNING *`, hash, now).Scan(&flows).Error
	if err != nil || len(flows) == 0 {
		return nil, err
	}
	return &flows[0], nil
}

func (r *Google) UserBySubject(ctx context.Context, subject string) (*models.User, error) {
	var user models.User
	err := r.Ctx(ctx).Model(&models.User{}).Select("users.*").Joins("JOIN auth_google_identities ON auth_google_identities.user_id = users.id AND auth_google_identities.deleted_at IS NULL").Where("auth_google_identities.subject = ?", subject).Take(&user).Error
	return userResult(&user, err)
}

func (r *Google) IdentityByUser(ctx context.Context, userID string) (*models.AuthGoogleIdentity, error) {
	var identity models.AuthGoogleIdentity
	err := r.Ctx(ctx).Where("user_id = ?", userID).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &identity, err
}

func (r *Google) CreateIdentity(ctx context.Context, identity *models.AuthGoogleIdentity) error {
	err := r.Ctx(ctx).Create(identity).Error
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return services.ErrRegistrationUnavailable
	}
	return err
}

func (r *Google) CreatePending(ctx context.Context, pending *models.AuthGooglePending) error {
	return r.Ctx(ctx).Create(pending).Error
}

func (r *Google) Pending(ctx context.Context, hash string, now time.Time) (*models.AuthGooglePending, error) {
	var pending models.AuthGooglePending
	err := r.Ctx(ctx).Where("token_hash = ? AND expires_at > ?", hash, now).Take(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pending, err
}

func (r *Google) PendingForUpdate(ctx context.Context, hash string, now time.Time) (*models.AuthGooglePending, error) {
	var pending models.AuthGooglePending
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND expires_at > ?", hash, now).Take(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pending, err
}

func (r *Google) DeletePending(ctx context.Context, hash string) error {
	return r.Ctx(ctx).Where("token_hash = ?", hash).Delete(&models.AuthGooglePending{}).Error
}

func (r *Google) Cleanup(ctx context.Context, now time.Time) error {
	if err := r.Ctx(ctx).Where("expires_at <= ?", now).Delete(&models.AuthGoogleFlow{}).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Where("expires_at <= ?", now).Delete(&models.AuthGooglePending{}).Error
}
