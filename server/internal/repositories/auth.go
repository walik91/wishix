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

type Auth struct{ Base }

func NewAuth(executor *db.Executor) *Auth { return &Auth{Base: NewBase(executor)} }

func userResult(user *models.User, err error) (*models.User, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return user, err
}

func (r *Auth) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.Ctx(ctx).Where("email = ?", email).Take(&user).Error
	return userResult(&user, err)
}

// EmailTaken enforces the database uniqueness constraint without exposing a deleted account.
func (r *Auth) EmailTaken(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Unscoped().Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *Auth) UserByID(ctx context.Context, id string, lock bool) (*models.User, error) {
	var user models.User
	query := r.Ctx(ctx).Where("id = ?", id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return userResult(&user, query.Take(&user).Error)
}

func (r *Auth) UsernameExists(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Raw(`SELECT count(*) FROM (SELECT username FROM users WHERE username = ? UNION ALL SELECT username FROM username_registry WHERE username = ? AND (expires_at IS NULL OR expires_at > now())) AS occupied`, username, username).Scan(&count).Error
	return count > 0, err
}

func (r *Auth) CreateUser(ctx context.Context, user *models.User) error {
	query := r.Ctx(ctx)
	if user.Email == "" {
		query = query.Omit("Email", "EmailVerifiedAt")
	}
	err := query.Create(user).Error
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return services.ErrRegistrationUnavailable
	}
	if err != nil {
		return err
	}
	// Account creation is called inside a serializable transaction: claiming the
	// name and inserting the user must commit together, including OAuth signup.
	claim := r.Ctx(ctx).Exec(`INSERT INTO username_registry (username, user_id, expires_at) VALUES (?, ?, NULL)
		ON CONFLICT (username) DO UPDATE SET user_id = excluded.user_id, expires_at = NULL
		WHERE username_registry.expires_at IS NOT NULL AND username_registry.expires_at <= now()`, user.Username, user.ID)
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		return services.ErrRegistrationUnavailable
	}
	return nil
}

func (r *Auth) AddEmail(ctx context.Context, id, email string, verifiedAt time.Time) error {
	result := r.Ctx(ctx).Model(&models.User{}).Where("id = ? AND email IS NULL", id).Updates(map[string]any{"email": email, "email_verified_at": verifiedAt})
	var pgError *pgconn.PgError
	if errors.As(result.Error, &pgError) && pgError.Code == "23505" {
		return services.ErrRegistrationUnavailable
	}
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return services.ErrRegistrationUnavailable
	}
	return nil
}

func (r *Auth) CreateChallenge(ctx context.Context, challenge *models.AuthChallenge) error {
	return r.Ctx(ctx).Create(challenge).Error
}
func (r *Auth) ChallengeForUpdate(ctx context.Context, id string) (*models.AuthChallenge, error) {
	var challenge models.AuthChallenge
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &challenge, err
}
func (r *Auth) SaveChallenge(ctx context.Context, c *models.AuthChallenge) error {
	return r.Ctx(ctx).Model(&models.AuthChallenge{}).Where("id = ?", c.ID).Updates(map[string]any{"code_hash": c.CodeHash, "attempts": c.Attempts, "expires_at": c.ExpiresAt, "sent_at": c.SentAt}).Error
}
func (r *Auth) DeleteChallenge(ctx context.Context, id string) error {
	return r.Ctx(ctx).Where("id = ?", id).Delete(&models.AuthChallenge{}).Error
}
func (r *Auth) CreateSession(ctx context.Context, s *models.AuthSession) error {
	return r.Ctx(ctx).Create(s).Error
}
func (r *Auth) SessionUser(ctx context.Context, hash string, now time.Time) (*models.User, error) {
	var user models.User
	err := r.Ctx(ctx).Model(&models.User{}).Select("users.*").Joins("JOIN auth_sessions ON auth_sessions.user_id = users.id").Where("auth_sessions.token_hash = ? AND auth_sessions.expires_at > ?", hash, now).Take(&user).Error
	return userResult(&user, err)
}
func (r *Auth) DeleteSession(ctx context.Context, hash string) error {
	return r.Ctx(ctx).Where("token_hash = ?", hash).Delete(&models.AuthSession{}).Error
}
func (r *Auth) DeleteUserSessions(ctx context.Context, id string) error {
	return r.Ctx(ctx).Where("user_id = ?", id).Delete(&models.AuthSession{}).Error
}
func (r *Auth) DeleteUserChallenges(ctx context.Context, id string) error {
	return r.Ctx(ctx).Where("user_id = ?", id).Delete(&models.AuthChallenge{}).Error
}
func (r *Auth) UpdatePassword(ctx context.Context, id, hash string) error {
	return r.Ctx(ctx).Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *Auth) Allow(ctx context.Context, key string, maximum int, now, expiry time.Time) (bool, error) {
	var rows []struct{ Count int }
	err := r.Ctx(ctx).Raw(`INSERT INTO auth_rate_limits (key, count, expires_at) VALUES (?, 1, ?)
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
		if err := r.Ctx(ctx).Exec(query.sql, query.cutoff).Error; err != nil {
			return err
		}
	}
	return nil
}
