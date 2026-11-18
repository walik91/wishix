package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	security "wishix/server/internal/auth"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Social struct{ Base }

func NewSocial(executor *db.Executor) *Social { return &Social{Base: NewBase(executor)} }

func (r *Social) Settings(ctx context.Context, owner string) (*services.ProfileSettings, error) {
	var settings []services.ProfileSettings
	err := r.Ctx(ctx).Table("users").Select("username, biography, birth_date::STRING AS birth_date, show_age, show_biography, show_avatar, show_followers, show_following, username_changed_at, EXISTS (SELECT 1 FROM profile_avatars a WHERE a.user_id = users.id) AS has_avatar").Where("id = ?", owner).Limit(1).Scan(&settings).Error
	if err != nil || len(settings) == 0 {
		return nil, err
	}
	return &settings[0], nil
}
func (r *Social) PublicInfo(ctx context.Context, username string) (*services.PublicSocialProfile, error) {
	var rows []services.PublicSocialProfile
	err := r.Ctx(ctx).Table("users").Select("username, CASE WHEN show_biography THEN biography ELSE '' END AS biography, CASE WHEN show_age THEN birth_date ELSE NULL END AS birth_date, show_followers, show_following, (show_avatar AND EXISTS (SELECT 1 FROM profile_avatars a WHERE a.user_id = users.id)) AS has_avatar").Where("username = ?", username).Limit(1).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (r *Social) SaveSettings(ctx context.Context, owner string, settings services.ProfileSettings) error {
	return r.Ctx(ctx).Model(&models.User{}).Where("id = ?", owner).Updates(map[string]any{"biography": settings.Biography, "birth_date": settings.BirthDate, "show_age": settings.ShowAge, "show_biography": settings.ShowBiography, "show_avatar": settings.ShowAvatar, "show_followers": settings.ShowFollowers, "show_following": settings.ShowFollowing}).Error
}
func (r *Social) Rename(ctx context.Context, owner, username string, redirect bool, now time.Time) error {
	var user struct {
		Username          string
		UsernameChangedAt *time.Time
	}
	query := r.Ctx(ctx).Raw("SELECT username, username_changed_at FROM users WHERE id = ? FOR UPDATE", owner).Scan(&user)
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected == 0 {
		return services.ErrSession
	}
	if user.Username == username {
		return nil
	}
	if user.UsernameChangedAt != nil && now.Before(user.UsernameChangedAt.Add(30*24*time.Hour)) {
		return services.ErrUsernameCooldown
	}
	claim := r.Ctx(ctx).Exec(`INSERT INTO username_registry (username, user_id, expires_at) VALUES (?, ?, NULL)
		ON CONFLICT (username) DO UPDATE SET user_id = excluded.user_id, expires_at = NULL
		WHERE username_registry.user_id = ? AND username_registry.expires_at IS NOT NULL`, username, owner, owner)
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		return services.ErrUsernameTaken
	}
	result := r.Ctx(ctx).Model(&models.User{}).Where("id = ?", owner).Updates(map[string]any{"username": username, "username_changed_at": now})
	var pg *pgconn.PgError
	if errors.As(result.Error, &pg) && pg.Code == "23505" {
		return services.ErrUsernameTaken
	}
	if result.Error != nil {
		return result.Error
	}
	expires := now
	if redirect {
		expires = now.Add(60 * 24 * time.Hour)
	}
	return r.Ctx(ctx).Exec(`INSERT INTO username_registry (username, user_id, expires_at) VALUES (?, ?, ?) ON CONFLICT (username) DO UPDATE SET user_id = excluded.user_id, expires_at = excluded.expires_at`, user.Username, owner, expires).Error
}
func (r *Social) TargetID(ctx context.Context, username string) (string, error) {
	var ids []string
	err := r.Ctx(ctx).Model(&models.User{}).Where("username = ?", username).Limit(1).Pluck("id", &ids).Error
	if err != nil || len(ids) == 0 {
		return "", err
	}
	return ids[0], nil
}
func (r *Social) LockPair(ctx context.Context, first, second string) error {
	var rows []struct{ ID string }
	return r.Ctx(ctx).Raw("SELECT id FROM users WHERE id IN (?, ?) ORDER BY id FOR UPDATE", first, second).Scan(&rows).Error
}
func (r *Social) BlockedEither(ctx context.Context, first, second string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Table("user_blocks").Where("(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)", first, second, second, first).Count(&count).Error
	return count > 0, err
}
func (r *Social) State(ctx context.Context, owner, username string) (services.SocialState, error) {
	target, err := r.TargetID(ctx, username)
	if err != nil {
		return services.SocialState{}, err
	}
	if target == "" {
		return services.SocialState{}, services.ErrListNotFound
	}
	if target == owner {
		return services.SocialState{Self: true}, nil
	}
	var row struct {
		Following    bool
		Blocked      bool
		OtherBlocked bool
	}
	err = r.Ctx(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM user_follows WHERE follower_id = ? AND followed_id = ?) AS following,
		EXISTS (SELECT 1 FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?) AS blocked,
		EXISTS (SELECT 1 FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?) AS other_blocked`, owner, target, owner, target, target, owner).Scan(&row).Error
	return services.SocialState{Following: row.Following, Blocked: row.Blocked, CanFollow: !row.Blocked && !row.OtherBlocked}, err
}
func (r *Social) Follow(ctx context.Context, owner, target string, follow bool, now time.Time) error {
	if !follow {
		return r.Ctx(ctx).Exec("DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?", owner, target).Error
	}
	return r.Ctx(ctx).Exec("INSERT INTO user_follows (follower_id, followed_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", owner, target, now).Error
}
func (r *Social) Block(ctx context.Context, owner, target string, block bool, now time.Time) error {
	if !block {
		return r.Ctx(ctx).Exec("DELETE FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?", owner, target).Error
	}
	if err := r.Ctx(ctx).Exec("INSERT INTO user_blocks (blocker_id, blocked_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", owner, target, now).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Exec("DELETE FROM user_follows WHERE (follower_id = ? AND followed_id = ?) OR (follower_id = ? AND followed_id = ?)", owner, target, target, owner).Error
}
func (r *Social) Report(ctx context.Context, owner, target, message string, now time.Time) error {
	return r.Ctx(ctx).Exec("INSERT INTO user_reports (id, reporter_id, reported_id, message, created_at) VALUES (?, ?, ?, ?, ?)", security.UUID(), owner, target, message, now).Error
}
func (r *Social) People(ctx context.Context, viewer, username, kind, cursor string, limit int) ([]services.SocialPerson, bool, error) {
	var users []struct {
		ID            string
		ShowFollowers bool
		ShowFollowing bool
	}
	err := r.Ctx(ctx).Table("users").Select("id, show_followers, show_following").Where("username = ?", username).Limit(1).Scan(&users).Error
	if err != nil || len(users) == 0 {
		return nil, false, err
	}
	owner := users[0]
	if viewer != owner.ID && (kind == "blocked" || (kind == "followers" && !owner.ShowFollowers) || (kind == "following" && !owner.ShowFollowing)) {
		return nil, false, nil
	}
	people := make([]services.SocialPerson, 0)
	query := r.Ctx(ctx).Table("users").Select("users.username, CASE WHEN users.show_display_name THEN users.display_name ELSE '' END AS display_name, (users.show_avatar AND EXISTS (SELECT 1 FROM profile_avatars a WHERE a.user_id = users.id)) AS has_avatar").Where("users.username > ?", cursor)
	// Recheck visibility in the same statement that reads memberships, so a
	// concurrent privacy change cannot disclose a previously public list.
	if viewer != owner.ID && kind == "followers" {
		query = query.Where("EXISTS (SELECT 1 FROM users privacy WHERE privacy.id = ? AND privacy.show_followers = true)", owner.ID)
	} else if viewer != owner.ID && kind == "following" {
		query = query.Where("EXISTS (SELECT 1 FROM users privacy WHERE privacy.id = ? AND privacy.show_following = true)", owner.ID)
	}
	switch kind {
	case "followers":
		query = query.Joins("JOIN user_follows f ON f.follower_id = users.id").Where("f.followed_id = ?", owner.ID)
	case "following":
		query = query.Joins("JOIN user_follows f ON f.followed_id = users.id").Where("f.follower_id = ?", owner.ID)
	case "blocked":
		query = query.Joins("JOIN user_blocks b ON b.blocked_id = users.id").Where("b.blocker_id = ?", owner.ID)
	}
	err = query.Order("users.username ASC").Limit(limit).Scan(&people).Error
	return people, true, err
}
func (r *Social) Avatar(ctx context.Context, owner string, public bool) ([]byte, error) {
	var rows []struct{ Data []byte }
	query := r.Ctx(ctx).Table("profile_avatars a").Select("a.data").Joins("JOIN users ON users.id = a.user_id")
	if public {
		query = query.Where("users.username = ? AND users.show_avatar = true", owner)
	} else {
		query = query.Where("users.id = ?", owner)
	}
	err := query.Limit(1).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0].Data, nil
}
func (r *Social) SaveAvatar(ctx context.Context, owner string, data []byte) error {
	return r.Ctx(ctx).Exec("UPSERT INTO profile_avatars (user_id, version, data) VALUES (?, ?, ?)", owner, security.UUID(), data).Error
}
func (r *Social) DeleteAvatar(ctx context.Context, owner string) error {
	return r.Ctx(ctx).Exec("DELETE FROM profile_avatars WHERE user_id = ?", owner).Error
}
