package services

import (
	"context"
	"strings"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type GoogleRepository interface {
	CreateFlow(context.Context, *models.AuthGoogleFlow) error
	ConsumeFlow(context.Context, string, time.Time) (*models.AuthGoogleFlow, error)
	UserBySubject(context.Context, string) (*models.User, error)
	IdentityByUser(context.Context, string) (*models.AuthGoogleIdentity, error)
	CreateIdentity(context.Context, *models.AuthGoogleIdentity) error
	CreatePending(context.Context, *models.AuthGooglePending) error
	PendingForUpdate(context.Context, string, time.Time) (*models.AuthGooglePending, error)
	DeletePending(context.Context, string) error
	Cleanup(context.Context, time.Time) error
}

type GoogleProvider interface {
	Enabled() bool
	AuthorizationURL(string, string) string
	Exchange(context.Context, string, string) (security.GoogleIdentity, error)
}

type Google struct {
	auth     *Auth
	repo     GoogleRepository
	provider GoogleProvider
}

func NewGoogle(auth *Auth, repo GoogleRepository, provider GoogleProvider) *Google {
	return &Google{auth: auth, repo: repo, provider: provider}
}

type GoogleResult struct {
	Mode          string
	Pending       string
	Authenticated Authenticated
}

func (g *Google) Enabled() bool { return g.provider.Enabled() }

func (g *Google) Cleanup(ctx context.Context) error { return g.repo.Cleanup(ctx, g.auth.now()) }

func (g *Google) Linked(ctx context.Context, session string) (bool, error) {
	user, err := g.auth.Current(ctx, session)
	if err != nil {
		return false, err
	}
	identity, err := g.repo.IdentityByUser(ctx, user.ID)
	return identity != nil, err
}

func (g *Google) Start(ctx context.Context, mode, session string) (string, string, error) {
	if !g.Enabled() || (mode != "login" && mode != "link") {
		return "", "", ErrInvalidInput
	}
	var sessionHash string
	if mode == "link" {
		if _, err := g.auth.Current(ctx, session); err != nil {
			return "", "", err
		}
		sessionHash = security.Digest(session)
	} else if user, err := g.auth.Current(ctx, session); err == nil && user != nil {
		return "", "", ErrRegistrationUnavailable
	} else if err != nil && err != ErrSession {
		return "", "", err
	}
	state, verifier := security.Token(), security.Token()
	flow := &models.AuthGoogleFlow{StateHash: security.Digest(state), Verifier: verifier, Mode: mode, SessionHash: sessionHash, ExpiresAt: g.auth.now().Add(10 * time.Minute)}
	if err := g.repo.CreateFlow(ctx, flow); err != nil {
		return "", "", err
	}
	return state, g.provider.AuthorizationURL(state, verifier), nil
}

func (g *Google) Callback(ctx context.Context, state, session, code string) (GoogleResult, error) {
	if len(state) != 43 {
		return GoogleResult{}, ErrInvalidInput
	}
	flow, err := g.repo.ConsumeFlow(ctx, security.Digest(state), g.auth.now())
	if err != nil {
		return GoogleResult{}, err
	}
	if flow == nil {
		return GoogleResult{}, ErrInvalidInput
	}
	if flow.Mode == "link" && (len(session) != 43 || flow.SessionHash != security.Digest(session)) {
		return GoogleResult{}, ErrSession
	}
	if flow.Mode == "login" && len(session) == 43 {
		if user, err := g.auth.Current(ctx, session); err == nil && user != nil {
			return GoogleResult{}, ErrRegistrationUnavailable
		} else if err != nil && err != ErrSession {
			return GoogleResult{}, err
		}
	}
	identity, err := g.provider.Exchange(ctx, code, flow.Verifier)
	if err != nil {
		return GoogleResult{}, err
	}
	email, err := normalizeEmail(identity.Email)
	if err != nil || !identity.EmailVerified || identity.Subject == "" {
		return GoogleResult{}, ErrInvalidInput
	}
	if flow.Mode == "link" {
		return g.link(ctx, session, identity.Subject)
	}
	user, err := g.repo.UserBySubject(ctx, identity.Subject)
	if err != nil {
		return GoogleResult{}, err
	}
	if user != nil {
		var result Authenticated
		err = g.auth.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			current, err := g.auth.repo.UserByID(txCtx, user.ID, true)
			if err != nil {
				return err
			}
			if current == nil {
				return ErrSession
			}
			var s *models.AuthSession
			result, s = g.auth.newSession(current)
			return g.auth.repo.CreateSession(txCtx, s)
		})
		return GoogleResult{Mode: "login", Authenticated: result}, err
	}
	existing, err := g.auth.repo.UserByEmail(ctx, email)
	if err != nil {
		return GoogleResult{}, err
	}
	if existing != nil {
		return GoogleResult{}, ErrRegistrationUnavailable
	}
	token := security.Token()
	err = g.repo.CreatePending(ctx, &models.AuthGooglePending{TokenHash: security.Digest(token), Subject: identity.Subject, Email: email, ExpiresAt: g.auth.now().Add(10 * time.Minute)})
	return GoogleResult{Mode: "register", Pending: token}, err
}

func (g *Google) link(ctx context.Context, session, subject string) (GoogleResult, error) {
	err := g.auth.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := g.auth.Current(txCtx, session)
		if err != nil {
			return err
		}
		linked, err := g.repo.UserBySubject(txCtx, subject)
		if err != nil {
			return err
		}
		if linked != nil {
			if linked.ID == user.ID {
				return nil
			}
			return ErrRegistrationUnavailable
		}
		other, err := g.repo.IdentityByUser(txCtx, user.ID)
		if err != nil {
			return err
		}
		if other != nil {
			return ErrRegistrationUnavailable
		}
		return g.repo.CreateIdentity(txCtx, &models.AuthGoogleIdentity{Subject: subject, UserID: user.ID, CreatedAt: g.auth.now()})
	})
	return GoogleResult{Mode: "link"}, err
}

func (g *Google) Complete(ctx context.Context, token, username, session string) (Authenticated, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(token) != 43 {
		return Authenticated{}, ErrInvalidInput
	}
	if !usernamePattern.MatchString(username) {
		return Authenticated{}, ErrUsernameInvalid
	}
	if len(session) == 43 {
		if user, err := g.auth.Current(ctx, session); err == nil && user != nil {
			return Authenticated{}, ErrRegistrationUnavailable
		} else if err != nil && err != ErrSession {
			return Authenticated{}, err
		}
	}
	var result Authenticated
	err := g.auth.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		pending, err := g.repo.PendingForUpdate(txCtx, security.Digest(token), g.auth.now())
		if err != nil {
			return err
		}
		if pending == nil {
			return ErrInvalidInput
		}
		linked, err := g.repo.UserBySubject(txCtx, pending.Subject)
		if err != nil {
			return err
		}
		if linked != nil {
			return ErrRegistrationUnavailable
		}
		now := g.auth.now()
		user := &models.User{ID: security.UUID(), Username: username, DisplayName: username, Email: pending.Email, EmailVerifiedAt: now, CreatedAt: now}
		if err := g.auth.repo.CreateUser(txCtx, user); err != nil {
			return err
		}
		if err := g.repo.CreateIdentity(txCtx, &models.AuthGoogleIdentity{Subject: pending.Subject, UserID: user.ID, CreatedAt: now}); err != nil {
			return err
		}
		var s *models.AuthSession
		result, s = g.auth.newSession(user)
		if err := g.auth.repo.CreateSession(txCtx, s); err != nil {
			return err
		}
		return g.repo.DeletePending(txCtx, security.Digest(token))
	})
	return result, err
}
