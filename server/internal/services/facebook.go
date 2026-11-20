package services

import (
	"context"
	"strings"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type FacebookRepository interface {
	CreateFlow(context.Context, *models.AuthFacebookFlow) error
	ConsumeFlow(context.Context, string, time.Time) (*models.AuthFacebookFlow, error)
	UserBySubject(context.Context, string) (*models.User, error)
	IdentityByUser(context.Context, string) (*models.AuthFacebookIdentity, error)
	CreateIdentity(context.Context, *models.AuthFacebookIdentity) error
	CreatePending(context.Context, *models.AuthFacebookPending) error
	PendingForUpdate(context.Context, string, time.Time) (*models.AuthFacebookPending, error)
	DeletePending(context.Context, string) error
	Cleanup(context.Context, time.Time) error
}

type FacebookProvider interface {
	Enabled() bool
	AuthorizationURL(string, string) string
	Exchange(context.Context, string, string) (security.FacebookIdentity, error)
}

type Facebook struct {
	auth     *Auth
	repo     FacebookRepository
	provider FacebookProvider
}

func NewFacebook(auth *Auth, repo FacebookRepository, provider FacebookProvider) *Facebook {
	return &Facebook{auth: auth, repo: repo, provider: provider}
}

type FacebookResult struct {
	Mode          string
	Pending       string
	Authenticated Authenticated
}

func (g *Facebook) Enabled() bool { return g.provider.Enabled() }

func (g *Facebook) Cleanup(ctx context.Context) error { return g.repo.Cleanup(ctx, g.auth.now()) }

func (g *Facebook) Linked(ctx context.Context, session string) (bool, error) {
	user, err := g.auth.Current(ctx, session)
	if err != nil {
		return false, err
	}
	identity, err := g.repo.IdentityByUser(ctx, user.ID)
	return identity != nil, err
}

func (g *Facebook) Start(ctx context.Context, mode, session string) (string, string, error) {
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
	flow := &models.AuthFacebookFlow{StateHash: security.Digest(state), Verifier: verifier, Mode: mode, SessionHash: sessionHash, ExpiresAt: g.auth.now().Add(10 * time.Minute)}
	if err := g.repo.CreateFlow(ctx, flow); err != nil {
		return "", "", err
	}
	return state, g.provider.AuthorizationURL(state, verifier), nil
}

func (g *Facebook) Callback(ctx context.Context, state, session, code string) (FacebookResult, error) {
	if !g.Enabled() || len(state) != 43 {
		return FacebookResult{}, ErrInvalidInput
	}
	flow, err := g.repo.ConsumeFlow(ctx, security.Digest(state), g.auth.now())
	if err != nil {
		return FacebookResult{}, err
	}
	if flow == nil {
		return FacebookResult{}, ErrInvalidInput
	}
	if flow.Mode == "link" && (len(session) != 43 || flow.SessionHash != security.Digest(session)) {
		return FacebookResult{}, ErrSession
	}
	if flow.Mode == "login" && len(session) == 43 {
		if user, err := g.auth.Current(ctx, session); err == nil && user != nil {
			return FacebookResult{}, ErrRegistrationUnavailable
		} else if err != nil && err != ErrSession {
			return FacebookResult{}, err
		}
	}
	identity, err := g.provider.Exchange(ctx, code, flow.Verifier)
	if err != nil {
		return FacebookResult{}, err
	}
	if identity.Subject == "" {
		return FacebookResult{}, ErrInvalidInput
	}
	if flow.Mode == "link" {
		return g.link(ctx, session, identity.Subject)
	}
	user, err := g.repo.UserBySubject(ctx, identity.Subject)
	if err != nil {
		return FacebookResult{}, err
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
		return FacebookResult{Mode: "login", Authenticated: result}, err
	}
	email := ""
	if identity.Email != "" {
		email, err = normalizeEmail(identity.Email)
		if err != nil {
			return FacebookResult{}, ErrInvalidInput
		}
		occupied, err := g.auth.repo.EmailTaken(ctx, email)
		if err != nil {
			return FacebookResult{}, err
		}
		if occupied {
			return FacebookResult{}, ErrRegistrationUnavailable
		}
	}
	token := security.Token()
	err = g.repo.CreatePending(ctx, &models.AuthFacebookPending{TokenHash: security.Digest(token), Subject: identity.Subject, Email: email, ExpiresAt: g.auth.now().Add(10 * time.Minute)})
	return FacebookResult{Mode: "register", Pending: token}, err
}

func (g *Facebook) link(ctx context.Context, session, subject string) (FacebookResult, error) {
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
		return g.repo.CreateIdentity(txCtx, &models.AuthFacebookIdentity{Subject: subject, UserID: user.ID, CreatedAt: g.auth.now()})
	})
	return FacebookResult{Mode: "link"}, err
}

func (g *Facebook) Complete(ctx context.Context, token, username, session string) (Authenticated, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !g.Enabled() || len(token) != 43 {
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
		user := &models.User{ID: security.UUID(), Username: username, DisplayName: username, Email: pending.Email, CreatedAt: now}
		if pending.Email != "" {
			user.EmailVerifiedAt = now
		}
		if err := g.auth.repo.CreateUser(txCtx, user); err != nil {
			return err
		}
		if err := g.repo.CreateIdentity(txCtx, &models.AuthFacebookIdentity{Subject: pending.Subject, UserID: user.ID, CreatedAt: now}); err != nil {
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
