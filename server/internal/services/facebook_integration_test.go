//go:build integration

package services_test

import (
	"context"
	"testing"

	security "wishix/server/internal/auth"
	"wishix/server/internal/db"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

type fakeFacebook struct{ identity security.FacebookIdentity }

func (*fakeFacebook) Enabled() bool { return true }
func (*fakeFacebook) AuthorizationURL(state, _ string) string {
	return "https://facebook.test/?state=" + state
}
func (f *fakeFacebook) Exchange(context.Context, string, string) (security.FacebookIdentity, error) {
	return f.identity, nil
}

func TestIntegrationFacebookWithoutEmailAndEmailConfirmation(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	provider := &fakeFacebook{identity: security.FacebookIdentity{Subject: "facebook-one"}}
	facebook := services.NewFacebook(f.auth, repositories.NewFacebook(db.NewExecutor(f.database)), provider)
	create := func(username, subject string) services.Authenticated {
		provider.identity.Subject = subject
		state, _, err := facebook.Start(ctx, "login", "")
		if err != nil {
			t.Fatal(err)
		}
		pending, err := facebook.Callback(ctx, state, "", "code")
		if err != nil {
			t.Fatal(err)
		}
		account, err := facebook.Complete(ctx, pending.Pending, username, "")
		if err != nil {
			t.Fatal(err)
		}
		return account
	}
	first := create("facebook_first", "facebook-one")
	second := create("facebook_second", "facebook-two")
	current, err := f.auth.Current(ctx, first.Token)
	if err != nil || current.Email != "" || !current.EmailVerifiedAt.IsZero() || current.DisplayName != "facebook_first" {
		t.Fatalf("email-less account: %v", err)
	}
	provider.identity.Subject = "facebook-one"
	state, _, _ := facebook.Start(ctx, "login", "")
	login, err := facebook.Callback(ctx, state, "", "code")
	if err != nil || login.Authenticated.User.ID != first.User.ID {
		t.Fatal("email-less login failed", err)
	}
	challenge, err := f.auth.StartAddEmail(ctx, first.Token, "new@example.test", "en")
	if err != nil {
		t.Fatal(err)
	}
	code := f.mail.code("new@example.test", "register")
	if err := f.auth.FinishAddEmail(ctx, second.Token, challenge, code); err == nil {
		t.Fatal("another account confirmed email")
	}
	if _, err := f.auth.Verify(ctx, challenge, code); err == nil {
		t.Fatal("profile email challenge used for registration")
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if err := f.auth.FinishAddEmail(ctx, first.Token, challenge, wrong); err == nil {
		t.Fatal("incorrect code accepted")
	}
	if err := f.auth.FinishAddEmail(ctx, first.Token, challenge, code); err != nil {
		t.Fatal(err)
	}
	current, err = f.auth.Current(ctx, first.Token)
	if err != nil || current.Email != "new@example.test" || current.EmailVerifiedAt.IsZero() {
		t.Fatal("email not saved", err)
	}
	if _, err := f.auth.StartAddEmail(ctx, second.Token, "new@example.test", "en"); err != services.ErrRegistrationUnavailable {
		t.Fatal("duplicate email accepted")
	}
}

func TestIntegrationFacebookLinkRequiresSameSessionAndNoEmailMerge(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	existing := f.account(t, "existing@example.test", "existing_user")
	other := f.account(t, "other@example.test", "other_user")
	provider := &fakeFacebook{identity: security.FacebookIdentity{Subject: "facebook-link", Email: "existing@example.test"}}
	facebook := services.NewFacebook(f.auth, repositories.NewFacebook(db.NewExecutor(f.database)), provider)
	state, _, _ := facebook.Start(ctx, "login", "")
	if _, err := facebook.Callback(ctx, state, "", "code"); err != services.ErrRegistrationUnavailable {
		t.Fatal("accounts merged")
	}
	state, _, _ = facebook.Start(ctx, "link", existing.Value)
	if _, err := facebook.Callback(ctx, state, other.Value, "code"); err != services.ErrSession {
		t.Fatal("link accepted another session")
	}
	state, _, _ = facebook.Start(ctx, "link", existing.Value)
	if _, err := facebook.Callback(ctx, state, existing.Value, "code"); err != nil {
		t.Fatal(err)
	}
	provider.identity.Email = ""
	state, _, _ = facebook.Start(ctx, "login", "")
	result, err := facebook.Callback(ctx, state, "", "code")
	user, _ := f.auth.Current(ctx, existing.Value)
	if err != nil || result.Authenticated.User.ID != user.ID {
		t.Fatal("linked login failed", err)
	}
}
