package services

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type SocialError string

func (e SocialError) Error() string { return string(e) }

const (
	ErrSocialUnavailable SocialError = "social_unavailable"
	ErrUsernameTaken     SocialError = "username_taken"
	ErrUsernameCooldown  SocialError = "username_cooldown"
)

type ProfileSettings struct {
	Username          string     `json:"username"`
	Biography         string     `json:"biography"`
	BirthDate         *string    `json:"birth_date"`
	ShowAge           bool       `json:"show_age"`
	ShowFollowers     bool       `json:"show_followers"`
	ShowFollowing     bool       `json:"show_following"`
	HasAvatar         bool       `json:"has_avatar"`
	UsernameChangedAt *time.Time `json:"username_changed_at"`
}
type SocialPerson struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	HasAvatar   bool   `json:"has_avatar,omitempty"`
}
type SocialState struct {
	Self      bool `json:"self"`
	Following bool `json:"following"`
	Blocked   bool `json:"blocked"`
	CanFollow bool `json:"can_follow"`
}
type PublicSocialProfile struct {
	Username      string     `json:"username"`
	Biography     string     `json:"biography,omitempty"`
	BirthDate     *time.Time `json:"-"`
	Age           *int       `json:"age,omitempty"`
	HasAvatar     bool       `json:"has_avatar"`
	ShowFollowers bool       `json:"show_followers"`
	ShowFollowing bool       `json:"show_following"`
}
type SocialRepository interface {
	Settings(context.Context, string) (*ProfileSettings, error)
	PublicInfo(context.Context, string) (*PublicSocialProfile, error)
	SaveSettings(context.Context, string, ProfileSettings) error
	Rename(context.Context, string, string, bool, time.Time) error
	TargetID(context.Context, string) (string, error)
	LockPair(context.Context, string, string) error
	BlockedEither(context.Context, string, string) (bool, error)
	State(context.Context, string, string) (SocialState, error)
	Follow(context.Context, string, string, bool, time.Time) error
	Block(context.Context, string, string, bool, time.Time) error
	Report(context.Context, string, string, string, time.Time) error
	People(context.Context, string, string, string, string, int) ([]SocialPerson, bool, error)
	Avatar(context.Context, string, bool) ([]byte, error)
	SaveAvatar(context.Context, string, []byte) error
	DeleteAvatar(context.Context, string) error
}
type Social struct {
	repo SocialRepository
	tx   Transactor
}

func NewSocial(repo SocialRepository, tx Transactor) *Social { return &Social{repo: repo, tx: tx} }

func (s *Social) Settings(ctx context.Context, owner string) (*ProfileSettings, error) {
	if owner == "" {
		return nil, ErrSession
	}
	return s.repo.Settings(ctx, owner)
}
func (s *Social) PublicInfo(ctx context.Context, username string) (*PublicSocialProfile, error) {
	username = strings.ToLower(username)
	if !usernamePattern.MatchString(username) {
		return nil, ErrListNotFound
	}
	info, err := s.repo.PublicInfo(ctx, username)
	if err == nil && info == nil {
		err = ErrListNotFound
	}
	if info != nil && info.BirthDate != nil {
		today := time.Now().UTC()
		age := today.Year() - info.BirthDate.Year()
		if today.Month() < info.BirthDate.Month() || (today.Month() == info.BirthDate.Month() && today.Day() < info.BirthDate.Day()) {
			age--
		}
		info.Age = &age
	}
	return info, err
}
func (s *Social) SaveSettings(ctx context.Context, owner string, value ProfileSettings) error {
	if owner == "" {
		return ErrSession
	}
	value.Biography = strings.TrimSpace(value.Biography)
	if !utf8.ValidString(value.Biography) || utf8.RuneCountInString(value.Biography) > 2000 || strings.ContainsRune(value.Biography, 0) {
		return ErrInvalidInput
	}
	if value.BirthDate != nil {
		date, err := time.Parse("2006-01-02", *value.BirthDate)
		if err != nil || date.Year() < 1 || date.After(time.Now().UTC()) {
			return ErrInvalidInput
		}
	} else if value.ShowAge {
		return ErrInvalidInput
	}
	return s.repo.SaveSettings(ctx, owner, value)
}
func (s *Social) Rename(ctx context.Context, owner, username string, redirect bool) error {
	if owner == "" {
		return ErrSession
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return ErrUsernameInvalid
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.repo.Rename(ctx, owner, username, redirect, time.Now().UTC())
	})
}
func (s *Social) withPair(ctx context.Context, owner, username string, fn func(context.Context, string) error) error {
	if owner == "" {
		return ErrSession
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return ErrListNotFound
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		target, err := s.repo.TargetID(ctx, username)
		if err != nil {
			return err
		}
		if target == "" {
			return ErrListNotFound
		}
		if target == owner {
			return ErrInvalidInput
		}
		if err := s.repo.LockPair(ctx, owner, target); err != nil {
			return err
		}
		return fn(ctx, target)
	})
}
func (s *Social) State(ctx context.Context, owner, username string) (SocialState, error) {
	if owner == "" {
		return SocialState{}, ErrSession
	}
	return s.repo.State(ctx, owner, strings.ToLower(username))
}
func (s *Social) Follow(ctx context.Context, owner, username string, follow bool) error {
	return s.withPair(ctx, owner, username, func(ctx context.Context, target string) error {
		if follow {
			blocked, err := s.repo.BlockedEither(ctx, owner, target)
			if err != nil {
				return err
			}
			if blocked {
				return ErrSocialUnavailable
			}
		}
		return s.repo.Follow(ctx, owner, target, follow, time.Now().UTC())
	})
}
func (s *Social) Block(ctx context.Context, owner, username string, block bool) error {
	return s.withPair(ctx, owner, username, func(ctx context.Context, target string) error {
		return s.repo.Block(ctx, owner, target, block, time.Now().UTC())
	})
}
func (s *Social) Report(ctx context.Context, owner, username, message string) error {
	message = strings.TrimSpace(message)
	if !utf8.ValidString(message) || utf8.RuneCountInString(message) < 1 || utf8.RuneCountInString(message) > 1000 || strings.ContainsRune(message, 0) {
		return ErrInvalidInput
	}
	return s.withPair(ctx, owner, username, func(ctx context.Context, target string) error {
		return s.repo.Report(ctx, owner, target, message, time.Now().UTC())
	})
}
func (s *Social) People(ctx context.Context, viewer, username, kind, cursor string) ([]SocialPerson, string, error) {
	username = strings.ToLower(username)
	if !usernamePattern.MatchString(username) || (kind != "followers" && kind != "following" && kind != "blocked") || (cursor != "" && !usernamePattern.MatchString(cursor)) {
		return nil, "", ErrInvalidInput
	}
	people, allowed, err := s.repo.People(ctx, viewer, username, kind, cursor, 21)
	if err != nil {
		return nil, "", err
	}
	if !allowed {
		return nil, "", ErrListNotFound
	}
	next := ""
	if len(people) > 20 {
		people = people[:20]
		next = people[len(people)-1].Username
	}
	return people, next, nil
}
func (s *Social) Avatar(ctx context.Context, owner string, public bool) ([]byte, error) {
	data, err := s.repo.Avatar(ctx, owner, public)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrListNotFound
	}
	return data, nil
}
func (s *Social) SaveAvatar(ctx context.Context, owner string, data []byte) error {
	if owner == "" {
		return ErrSession
	}
	return s.repo.SaveAvatar(ctx, owner, data)
}
func (s *Social) DeleteAvatar(ctx context.Context, owner string) error {
	if owner == "" {
		return ErrSession
	}
	return s.repo.DeleteAvatar(ctx, owner)
}
