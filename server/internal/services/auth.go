package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type AuthError string

func (e AuthError) Error() string { return string(e) }

const (
	ErrInvalidInput            AuthError = "invalid_input"
	ErrUsernameInvalid         AuthError = "username_invalid"
	ErrPasswordInvalid         AuthError = "password_invalid"
	ErrRegistrationUnavailable AuthError = "registration_unavailable"
	ErrCredentials             AuthError = "invalid_credentials"
	ErrCode                    AuthError = "invalid_code"
	ErrRateLimit               AuthError = "rate_limited"
	ErrSession                 AuthError = "unauthorized"
)

type AuthRepository interface {
	UserByEmail(context.Context, string) (*models.User, error)
	UserByID(context.Context, string, bool) (*models.User, error)
	UsernameExists(context.Context, string) (bool, error)
	CreateUser(context.Context, *models.User) error
	CreateChallenge(context.Context, *models.AuthChallenge) error
	ChallengeForUpdate(context.Context, string) (*models.AuthChallenge, error)
	SaveChallenge(context.Context, *models.AuthChallenge) error
	DeleteChallenge(context.Context, string) error
	CreateSession(context.Context, *models.AuthSession) error
	SessionUser(context.Context, string, time.Time) (*models.User, error)
	DeleteSession(context.Context, string) error
	DeleteUserSessions(context.Context, string) error
	DeleteUserChallenges(context.Context, string) error
	UpdatePassword(context.Context, string, string) error
	AddEmail(context.Context, string, string, time.Time) error
	Allow(context.Context, string, int, time.Time, time.Time) (bool, error)
	Cleanup(context.Context, time.Time) error
}

type CodeMailer interface {
	SendCode(context.Context, string, string, string, string) error
}

type Auth struct {
	repo          AuthRepository
	tx            Transactor
	mail          CodeMailer
	config        *security.Config
	dummyPassword string
	now           func() time.Time
}

func NewAuth(repo AuthRepository, tx Transactor, mail CodeMailer, config *security.Config) *Auth {
	return &Auth{repo: repo, tx: tx, mail: mail, config: config, dummyPassword: security.HashPassword(security.Token()), now: func() time.Time { return time.Now().UTC() }}
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)
var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		return "", ErrInvalidInput
	}
	for _, r := range email {
		if r > 127 {
			return "", ErrInvalidInput
		}
	}
	return email, nil
}

func validatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 4 || utf8.RuneCountInString(password) > 128 || len(password) > 512 {
		return ErrPasswordInvalid
	}
	return nil
}

func normalizedLocale(locale string) string {
	if locale == "ru" || locale == "ro" {
		return locale
	}
	return "en"
}

func (a *Auth) limit(ctx context.Context, scope, subject string, maximum int, duration time.Duration) error {
	now := a.now()
	key := security.MAC(a.config.Secret, "rate:"+scope+":"+subject)
	allowed, err := a.repo.Allow(ctx, key, maximum, now, now.Add(duration))
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimit
	}
	return nil
}

// LimitIP must run before password hashing or other expensive authentication work.
func (a *Auth) LimitIP(ctx context.Context, action, ip string) error {
	maximum := 100
	if action == "register" || action == "forgot-password" || action == "resend" {
		maximum = 20
	}
	return a.limit(ctx, "ip:"+action, ip, maximum, 15*time.Minute)
}

// LimitProductImport shares the persistent, atomic limiter, keyed by private account ID.
func (a *Auth) LimitProductImport(ctx context.Context, userID string) error {
	return a.limit(ctx, "product-import", userID, 20, 10*time.Minute)
}

func (a *Auth) LimitSocialReport(ctx context.Context, userID string) error {
	return a.limit(ctx, "social-report", userID, 10, time.Hour)
}

func (a *Auth) newChallenge(purpose, email, username, passwordHash, locale string, userID *string) (*models.AuthChallenge, string, string) {
	token, code := security.Token(), security.Code()
	id := security.Digest(token)
	now := a.now()
	challenge := &models.AuthChallenge{ID: id, Purpose: purpose, Email: email, Username: username, PasswordHash: passwordHash, UserID: userID, Locale: normalizedLocale(locale), CodeHash: a.codeHash(id, purpose, code), ExpiresAt: now.Add(10 * time.Minute), SentAt: now}
	return challenge, token, code
}

func (a *Auth) codeHash(id, purpose, code string) string {
	return security.MAC(a.config.Secret, "code:"+id+":"+purpose+":"+code)
}

func (a *Auth) Register(ctx context.Context, email, username, password, locale string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return "", ErrUsernameInvalid
	}
	if err := validatePassword(password); err != nil {
		return "", err
	}
	if err := a.limit(ctx, "send-email", email, 5, time.Hour); err != nil {
		return "", err
	}
	user, err := a.repo.UserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	exists, err := a.repo.UsernameExists(ctx, username)
	if err != nil {
		return "", err
	}
	if user != nil || exists {
		return "", ErrRegistrationUnavailable
	}
	challenge, token, code := a.newChallenge("register", email, username, security.HashPassword(password), locale, nil)
	if err := a.repo.CreateChallenge(ctx, challenge); err != nil {
		return "", err
	}
	if err := a.mail.SendCode(ctx, email, challenge.Locale, challenge.Purpose, code); err != nil {
		return "", err
	}
	return token, nil
}

type Authenticated struct {
	User      *models.User
	Token     string
	ExpiresAt time.Time
}

func (a *Auth) newSession(user *models.User) (Authenticated, *models.AuthSession) {
	token, now := security.Token(), a.now()
	expiry := now.Add(7 * 24 * time.Hour)
	return Authenticated{User: user, Token: token, ExpiresAt: expiry}, &models.AuthSession{TokenHash: security.Digest(token), UserID: user.ID, CreatedAt: now, ExpiresAt: expiry}
}

// consume commits invalid-code attempt counts as well as successful consumption.
// Domain errors are returned only after commit; returning them inside fn would
// roll back the attempt counter. All attempt-local results reset on DB retries.
func (a *Auth) consume(ctx context.Context, token, code, purpose string, success func(context.Context, *models.AuthChallenge) error) error {
	if len(token) != 43 || !codePattern.MatchString(code) {
		return ErrCode
	}
	id := security.Digest(token)
	var outcome error
	err := a.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		outcome = nil
		challenge, err := a.repo.ChallengeForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if challenge == nil || challenge.Purpose != purpose || !a.now().Before(challenge.ExpiresAt) || challenge.Attempts >= 5 {
			outcome = ErrCode
			return nil
		}
		expected := a.codeHash(id, purpose, code)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(challenge.CodeHash)) != 1 {
			challenge.Attempts++
			outcome = ErrCode
			return a.repo.SaveChallenge(txCtx, challenge)
		}
		if err := success(txCtx, challenge); err != nil {
			return err
		}
		return a.repo.DeleteChallenge(txCtx, id)
	})
	if err != nil {
		return err
	}
	return outcome
}

func (a *Auth) Verify(ctx context.Context, token, code string) (Authenticated, error) {
	var result Authenticated
	err := a.consume(ctx, token, code, "register", func(txCtx context.Context, challenge *models.AuthChallenge) error {
		if challenge.UserID != nil {
			return ErrCode
		}
		now := a.now()
		user := &models.User{ID: security.UUID(), Username: challenge.Username, Email: challenge.Email, PasswordHash: challenge.PasswordHash, EmailVerifiedAt: now, CreatedAt: now}
		if err := a.repo.CreateUser(txCtx, user); err != nil {
			return err
		}
		var session *models.AuthSession
		result, session = a.newSession(user)
		return a.repo.CreateSession(txCtx, session)
	})
	return result, err
}

// An email added to a social account is usable only after proof of control.
func (a *Auth) StartAddEmail(ctx context.Context, session, email, locale string) (string, error) {
	user, err := a.Current(ctx, session)
	if err != nil {
		return "", err
	}
	if user.Email != "" {
		return "", ErrRegistrationUnavailable
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return "", err
	}
	if err = a.limit(ctx, "send-email", email, 5, time.Hour); err != nil {
		return "", err
	}
	if existing, err := a.repo.UserByEmail(ctx, email); err != nil {
		return "", err
	} else if existing != nil {
		return "", ErrRegistrationUnavailable
	}
	challenge, token, code := a.newChallenge("register", email, "", "", locale, &user.ID)
	if err = a.repo.CreateChallenge(ctx, challenge); err != nil {
		return "", err
	}
	if err = a.mail.SendCode(ctx, email, challenge.Locale, challenge.Purpose, code); err != nil {
		return "", err
	}
	return token, nil
}

func (a *Auth) FinishAddEmail(ctx context.Context, session, token, code string) error {
	user, err := a.Current(ctx, session)
	if err != nil {
		return err
	}
	return a.consume(ctx, token, code, "register", func(txCtx context.Context, challenge *models.AuthChallenge) error {
		current, err := a.Current(txCtx, session)
		if err != nil {
			return err
		}
		if challenge.UserID == nil || *challenge.UserID != user.ID || current.ID != user.ID {
			return ErrCode
		}
		return a.repo.AddEmail(txCtx, user.ID, challenge.Email, a.now())
	})
}

func (a *Auth) Resend(ctx context.Context, token string) error {
	if len(token) != 43 {
		return ErrCode
	}
	var message *models.AuthChallenge
	code := security.Code()
	err := a.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		message = nil
		challenge, err := a.repo.ChallengeForUpdate(txCtx, security.Digest(token))
		if err != nil {
			return err
		}
		if challenge == nil || a.now().Sub(challenge.SentAt) < time.Minute {
			return ErrRateLimit
		}
		// Expired challenges may be resent for up to 24 hours.
		if a.now().Sub(challenge.ExpiresAt) > 24*time.Hour {
			return ErrCode
		}
		if err := a.limit(txCtx, "send-email", challenge.Email, 5, time.Hour); err != nil {
			return err
		}
		for challenge.CodeHash == a.codeHash(challenge.ID, challenge.Purpose, code) {
			// Ensure the replacement differs even if the RNG repeats six digits.
			code = security.Code()
		}
		challenge.CodeHash = a.codeHash(challenge.ID, challenge.Purpose, code)
		challenge.Attempts = 0
		challenge.SentAt = a.now()
		challenge.ExpiresAt = a.now().Add(10 * time.Minute)
		if err := a.repo.SaveChallenge(txCtx, challenge); err != nil {
			return err
		}
		message = challenge
		return nil
	})
	if err != nil {
		return err
	}
	if message.Purpose == "reset" && message.UserID == nil {
		return nil
	}
	return a.mail.SendCode(ctx, message.Email, message.Locale, message.Purpose, code)
}

func (a *Auth) Login(ctx context.Context, email, password string) (Authenticated, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) > 512 || password == "" {
		return Authenticated{}, ErrCredentials
	}
	if err := a.limit(ctx, "login-email", email, 15, 15*time.Minute); err != nil {
		return Authenticated{}, err
	}
	user, err := a.repo.UserByEmail(ctx, email)
	if err != nil {
		return Authenticated{}, err
	}
	encoded := a.dummyPassword
	if user != nil && user.PasswordHash != "" {
		encoded = user.PasswordHash
	}
	valid := security.CheckPassword(password, encoded)
	if !valid || user == nil || user.PasswordHash == "" {
		return Authenticated{}, ErrCredentials
	}
	var result Authenticated
	err = a.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		// Serialize session issuance with resets; a previously checked password
		// cannot create a session after a concurrent reset has committed.
		current, err := a.repo.UserByID(txCtx, user.ID, true)
		if err != nil {
			return err
		}
		if current == nil || current.PasswordHash != encoded {
			return ErrCredentials
		}
		var session *models.AuthSession
		result, session = a.newSession(current)
		return a.repo.CreateSession(txCtx, session)
	})
	return result, err
}

func (a *Auth) Current(ctx context.Context, token string) (*models.User, error) {
	if len(token) != 43 {
		return nil, ErrSession
	}
	user, err := a.repo.SessionUser(ctx, security.Digest(token), a.now())
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrSession
	}
	return user, nil
}

func (a *Auth) Logout(ctx context.Context, token string) error {
	if len(token) != 43 {
		return nil
	}
	return a.repo.DeleteSession(ctx, security.Digest(token))
}

func (a *Auth) ForgotPassword(ctx context.Context, email, locale string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	if err := a.limit(ctx, "send-email", email, 5, time.Hour); err != nil {
		return "", err
	}
	user, err := a.repo.UserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	var userID *string
	if user != nil && user.PasswordHash != "" {
		userID = &user.ID
	}
	challenge, token, code := a.newChallenge("reset", email, "", "", locale, userID)
	if err := a.repo.CreateChallenge(ctx, challenge); err != nil {
		return "", err
	}
	if userID == nil {
		return token, nil
	}
	if err := a.mail.SendCode(ctx, email, challenge.Locale, challenge.Purpose, code); err != nil {
		return "", err
	}
	return token, nil
}

func (a *Auth) ResetPassword(ctx context.Context, token, code, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash := security.HashPassword(password)
	return a.consume(ctx, token, code, "reset", func(txCtx context.Context, challenge *models.AuthChallenge) error {
		if challenge.UserID == nil {
			return ErrCode
		}
		user, err := a.repo.UserByID(txCtx, *challenge.UserID, true)
		if err != nil {
			return err
		}
		if user == nil || user.PasswordHash == "" {
			return ErrCode
		}
		if err := a.repo.UpdatePassword(txCtx, user.ID, hash); err != nil {
			return err
		}
		if err := a.repo.DeleteUserSessions(txCtx, user.ID); err != nil {
			return err
		}
		return a.repo.DeleteUserChallenges(txCtx, user.ID)
	})
}

func (a *Auth) Cleanup(ctx context.Context) error { return a.repo.Cleanup(ctx, a.now()) }

func ErrorCode(err error) string {
	var authError AuthError
	if errors.As(err, &authError) {
		return string(authError)
	}
	if errors.Is(err, security.ErrDelivery) {
		return "email_unavailable"
	}
	return "unavailable"
}
