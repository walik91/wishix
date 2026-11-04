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

type Auth struct{ executor *db.Executor }

func NewAuth(executor *db.Executor) *Auth { return &Auth{executor: executor} }

func userResult(user *models.User, err error) (*models.User, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return user, err
}

func (r *Auth) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.executor.DB(ctx).Where("email = ?", email).Take(&user).Error
	return userResult(&user, err)
}

func (r *Auth) UserByID(ctx context.Context, id string, lock bool) (*models.User, error) {
	var user models.User
	query := r.executor.DB(ctx).Where("id = ?", id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return userResult(&user, query.Take(&user).Error)
}

func (r *Auth) UsernameExists(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.executor.DB(ctx).Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (r *Auth) CreateUser(ctx context.Context, user *models.User) error {
	err := r.executor.DB(ctx).Create(user).Error
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return services.ErrRegistrationUnavailable
	}
	return err
}

func (r *Auth) CreateChallenge(ctx context.Context, challenge *models.AuthChallenge) error {
	return r.executor.DB(ctx).Create(challenge).Error
}
func (r *Auth) ChallengeForUpdate(ctx context.Context, id string) (*models.AuthChallenge, error) {
	var challenge models.AuthChallenge
	err := r.executor.DB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &challenge, err
}
func (r *Auth) SaveChallenge(ctx context.Context, c *models.AuthChallenge) error {
	return r.executor.DB(ctx).Model(&models.AuthChallenge{}).Where("id = ?", c.ID).Updates(map[string]any{"code_hash": c.CodeHash, "attempts": c.Attempts, "expires_at": c.ExpiresAt, "sent_at": c.SentAt}).Error
}
func (r *Auth) DeleteChallenge(ctx context.Context, id string) error {
	return r.executor.DB(ctx).Where("id = ?", id).Delete(&models.AuthChallenge{}).Error
}
func (r *Auth) CreateSession(ctx context.Context, s *models.AuthSession) error {
	return r.executor.DB(ctx).Create(s).Error
}
func (r *Auth) SessionUser(ctx context.Context, hash string, now time.Time) (*models.User, error) {
	var user models.User
	err := r.executor.DB(ctx).Model(&models.User{}).Select("users.*").Joins("JOIN auth_sessions ON auth_sessions.user_id = users.id").Where("auth_sessions.token_hash = ? AND auth_sessions.expires_at > ?", hash, now).Take(&user).Error
	return userResult(&user, err)
}
func (r *Auth) DeleteSession(ctx context.Context, hash string) error {
	return r.executor.DB(ctx).Where("token_hash = ?", hash).Delete(&models.AuthSession{}).Error
}
func (r *Auth) DeleteUserSessions(ctx context.Context, id string) error {
	return r.executor.DB(ctx).Where("user_id = ?", id).Delete(&models.AuthSession{}).Error
}
func (r *Auth) DeleteUserChallenges(ctx context.Context, id string) error {
	return r.executor.DB(ctx).Where("user_id = ?", id).Delete(&models.AuthChallenge{}).Error
}
func (r *Auth) UpdatePassword(ctx context.Context, id, hash string) error {
	return r.executor.DB(ctx).Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *Auth) Allow(ctx context.Context, key string, maximum int, now, expiry time.Time) (bool, error) {
	var rows []struct{ Count int }
	err := r.executor.DB(ctx).Raw(`INSERT INTO auth_rate_limits (key, count, expires_at) VALUES (?, 1, ?)
        ON CONFLICT (key) DO UPDATE SET
            count = CASE WHEN auth_rate_limits.expires_at <= ? THEN 1 ELSE auth_rate_limits.count + 1 END,
            expires_at = CASE WHEN auth_rate_limits.expires_at <= ? THEN excluded.expires_at ELSE auth_rate_limits.expires_at END
        WHERE auth_rate_limits.count < ? OR auth_rate_limits.expires_at <= ? RETURNING count`, key, expiry, now, now, maximum, now).Scan(&rows).Error
	return len(rows) == 1, err
}

func (r *Auth) Cleanup(ctx context.Context, now time.Time) error {
	for _, query := range []struct {
		sql    string
		cutoff time.Time
	}{
		{"DELETE FROM auth_sessions WHERE expires_at <= ?", now},
		{"DELETE FROM auth_challenges WHERE expires_at <= ?", now.Add(-24 * time.Hour)},
		{"DELETE FROM auth_rate_limits WHERE expires_at <= ?", now},
	} {
		if err := r.executor.DB(ctx).Exec(query.sql, query.cutoff).Error; err != nil {
			return err
		}
	}
	return nil
}
