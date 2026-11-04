//go:build integration

package services_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	security "wishix/server/internal/auth"
	"wishix/server/internal/controllers"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

type lifecycle struct{ hooks []fx.Hook }

func (l *lifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }

type testMail struct {
	mu    sync.Mutex
	codes map[string]string
	fail  bool
}

func (m *testMail) SendCode(_ context.Context, email, locale, purpose, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return security.ErrDelivery
	}
	m.codes[email+":"+purpose] = code
	return nil
}
func (m *testMail) code(email, purpose string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[email+":"+purpose]
}

type authFixture struct {
	database *gorm.DB
	repo     *repositories.Auth
	auth     *services.Auth
	mail     *testMail
	handler  http.Handler
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	l := &lifecycle{}
	admin, err := db.New(l)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, hook := range l.hooks {
		if err := hook.OnStart(ctx); err != nil {
			t.Fatal(err)
		}
	}
	schema := "auth_test_" + strings.ReplaceAll(security.UUID(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		for i := len(l.hooks) - 1; i >= 0; i-- {
			if err := l.hooks[i].OnStop(context.Background()); err != nil {
				t.Error(err)
			}
		}
	})
	dsn := url.URL{Scheme: "postgresql", Host: "db:26257", Path: "/" + os.Getenv("DB_NAME"), User: url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"))}
	dsn.RawQuery = url.Values{"sslmode": {"verify-full"}, "sslrootcert": {"/db-certs/ca.crt"}, "search_path": {schema}, "connect_timeout": {"5"}}.Encode()
	database, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("connect to isolated test schema:", err)
	}
	pool, _ := database.DB()
	pool.SetMaxOpenConns(10)
	t.Cleanup(func() { _ = pool.Close() })
	migrator, err := db.NewMigrator(database)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal("migration replay:", err)
	}
	executor := db.NewExecutor(database)
	repo := repositories.NewAuth(executor)
	mail := &testMail{codes: map[string]string{}}
	config := &security.Config{Origin: "https://wishix.test", SecureCookie: true, Secret: strings.Repeat("integration-secret", 3)}
	auth := services.NewAuth(repo, db.NewTransactionManager(executor), mail, config)
	mux := http.NewServeMux()
	controllers.NewAuth(auth, config, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)
	return &authFixture{database, repo, auth, mail, mux}
}

func (f *authFixture) request(t *testing.T, action string, body map[string]string, cookie *http.Cookie) (int, map[string]any, []*http.Cookie) {
	t.Helper()
	raw, _ := json.Marshal(body)
	method := "POST"
	if action == "me" {
		method = "GET"
	}
	request := httptest.NewRequest(method, "/api/auth/"+action, bytes.NewReader(raw))
	request.Header.Set("Origin", "https://wishix.test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Wishix-Request", "1")
	request.RemoteAddr = "192.0.2.1:1234"
	if cookie != nil {
		request.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	f.handler.ServeHTTP(result, request)
	var response map[string]any
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatalf("bad JSON response: %d", result.Code)
	}
	// No passwords, tokens or verification secrets in account responses.
	for _, key := range []string{"password", "password_hash", "token", "code", "code_hash"} {
		if _, ok := response[key]; ok {
			t.Fatalf("private field returned: %s", key)
		}
	}
	return result.Code, response, result.Result().Cookies()
}

const initialPassword = "1234"

func (f *authFixture) registration(t *testing.T, email, username string) string {
	t.Helper()
	status, body, _ := f.request(t, "register", map[string]string{"email": email, "username": username, "password": initialPassword, "locale": "ru"}, nil)
	if status != 202 {
		t.Fatalf("registration failed: %d %v", status, body)
	}
	return body["challenge"].(string)
}

func (f *authFixture) account(t *testing.T, email, username string) *http.Cookie {
	t.Helper()
	token := f.registration(t, email, username)
	status, body, cookies := f.request(t, "verify-email", map[string]string{"challenge": token, "code": f.mail.code(email, "register")}, nil)
	if status != 200 || len(cookies) != 1 {
		t.Fatalf("verification failed: %d %v", status, body)
	}
	return cookies[0]
}

func TestIntegrationEmailAccountLifecycle(t *testing.T) {
	f := newAuthFixture(t)
	token := f.registration(t, " PERSON@EXAMPLE.TEST ", "Test_User")
	status, _, _ := f.request(t, "login", map[string]string{"email": "person@example.test", "password": initialPassword}, nil)
	if status != 401 {
		t.Fatal("unconfirmed registration allowed login")
	}
	code := f.mail.code("person@example.test", "register")
	var challenge models.AuthChallenge
	if err := f.database.Where("id = ?", security.Digest(token)).Take(&challenge).Error; err != nil {
		t.Fatal(err)
	}
	if challenge.CodeHash == code || strings.Contains(challenge.PasswordHash, initialPassword) || challenge.ID == token {
		t.Fatal("challenge stored plaintext secrets")
	}
	status, body, cookies := f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
	if status != 200 || len(cookies) != 1 {
		t.Fatalf("verification failed: %d %v", status, body)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 604800 {
		t.Fatal("unsafe session cookie")
	}
	user := body["user"].(map[string]any)
	if len(user) != 3 || user["username"] != "test_user" || user["email"] != "person@example.test" {
		t.Fatal("unexpected account DTO")
	}
	status, _, _ = f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
	if status != 400 {
		t.Fatal("code reused")
	}
	status, _, _ = f.request(t, "me", nil, cookie)
	if status != 200 {
		t.Fatal("session missing")
	}
	status, _, _ = f.request(t, "me", nil, nil)
	if status != 401 {
		t.Fatal("anonymous private access")
	}
	status, _, _ = f.request(t, "register", map[string]string{"email": "PERSON@example.test", "username": "another_user", "password": initialPassword}, nil)
	if status != 409 {
		t.Fatal("case-insensitive email uniqueness failed")
	}
	status, _, newCookies := f.request(t, "login", map[string]string{"email": "Person@example.test", "password": initialPassword}, cookie)
	if status != 200 || newCookies[0].Value == cookie.Value {
		t.Fatal("login did not rotate token")
	}
	status, _, _ = f.request(t, "me", nil, cookie)
	if status != 401 {
		t.Fatal("previous browser token survived login")
	}
	cookie = newCookies[0]
	status, _, _ = f.request(t, "logout", map[string]string{}, cookie)
	if status != 200 {
		t.Fatal("logout failed")
	}
	status, _, _ = f.request(t, "me", nil, cookie)
	if status != 401 {
		t.Fatal("logged-out session remained active")
	}
}

func TestIntegrationCodesAttemptsResendAndExpiry(t *testing.T) {
	f := newAuthFixture(t)
	email := "codes@example.test"
	token := f.registration(t, email, "code_user")
	code := f.mail.code(email, "register")
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		status, _, _ := f.request(t, "verify-email", map[string]string{"challenge": token, "code": wrong}, nil)
		if status != 400 {
			t.Fatal("invalid code accepted")
		}
	}
	status, _, _ := f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
	if status != 400 {
		t.Fatal("attempt cap not enforced")
	}
	status, _, _ = f.request(t, "resend", map[string]string{"challenge": token}, nil)
	if status != 429 {
		t.Fatal("resend cooldown not enforced")
	}
	f.database.Exec("UPDATE auth_challenges SET sent_at = ? WHERE id = ?", time.Now().Add(-61*time.Second), security.Digest(token))
	status, _, _ = f.request(t, "resend", map[string]string{"challenge": token}, nil)
	if status != 200 {
		t.Fatal("resend failed")
	}
	renewed := f.mail.code(email, "register")
	if renewed != code {
		status, _, _ = f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
		if status != 400 {
			t.Fatal("old code survived resend")
		}
	}
	f.database.Exec("UPDATE auth_challenges SET expires_at = ? WHERE id = ?", time.Now().Add(-time.Second), security.Digest(token))
	status, _, _ = f.request(t, "verify-email", map[string]string{"challenge": token, "code": renewed}, nil)
	if status != 400 {
		t.Fatal("expired code accepted")
	}
}

func TestIntegrationPasswordResetRevokesSessionsAndCodes(t *testing.T) {
	f := newAuthFixture(t)
	email := "reset@example.test"
	cookie := f.account(t, email, "reset_user")
	status, _, other := f.request(t, "login", map[string]string{"email": email, "password": initialPassword}, nil)
	if status != 200 {
		t.Fatal("login failed")
	}
	status, body, _ := f.request(t, "forgot-password", map[string]string{"email": email, "locale": "ro"}, nil)
	if status != 202 {
		t.Fatal("reset request failed")
	}
	token := body["challenge"].(string)
	code := f.mail.code(email, "reset")
	status, _, _ = f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
	if status != 400 {
		t.Fatal("reset code used to register")
	}
	newPassword := "5678"
	status, _, _ = f.request(t, "reset-password", map[string]string{"challenge": token, "code": code, "password": "abc"}, nil)
	if status != 400 {
		t.Fatal("three-character reset password accepted")
	}
	status, _, _ = f.request(t, "reset-password", map[string]string{"challenge": token, "code": code, "password": newPassword}, nil)
	if status != 200 {
		t.Fatal("password reset failed")
	}
	for _, session := range []*http.Cookie{cookie, other[0]} {
		status, _, _ = f.request(t, "me", nil, session)
		if status != 401 {
			t.Fatal("old session survived reset")
		}
	}
	status, _, _ = f.request(t, "reset-password", map[string]string{"challenge": token, "code": code, "password": initialPassword}, nil)
	if status != 400 {
		t.Fatal("reset code reused")
	}
	status, _, _ = f.request(t, "login", map[string]string{"email": email, "password": initialPassword}, nil)
	if status != 401 {
		t.Fatal("old password still works")
	}
	status, _, _ = f.request(t, "login", map[string]string{"email": email, "password": newPassword}, nil)
	if status != 200 {
		t.Fatal("new password rejected")
	}
	status, missing, _ := f.request(t, "forgot-password", map[string]string{"email": "missing@example.test"}, nil)
	if status != 202 || len(missing) != len(body) || len(missing["challenge"].(string)) != 43 {
		t.Fatal("reset reveals account existence")
	}
	f.database.Exec("UPDATE auth_challenges SET sent_at = ? WHERE id = ?", time.Now().Add(-61*time.Second), security.Digest(missing["challenge"].(string)))
	status, _, _ = f.request(t, "resend", map[string]string{"challenge": missing["challenge"].(string)}, nil)
	if status != 200 {
		t.Fatal("resend reveals account existence")
	}
}

func TestIntegrationConcurrentVerificationAndRateLimit(t *testing.T) {
	f := newAuthFixture(t)
	email := "race@example.test"
	token := f.registration(t, email, "race_user")
	code := f.mail.code(email, "register")
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, _ := f.request(t, "verify-email", map[string]string{"challenge": token, "code": code}, nil)
			if status == 200 {
				successes.Add(1)
			} else if status != 400 {
				t.Errorf("unexpected concurrent status: %d", status)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("concurrent verification created multiple sessions")
	}
	var count int64
	f.database.Model(&models.AuthSession{}).Count(&count)
	if count != 1 {
		t.Fatal("multiple sessions issued for one code")
	}
	successes.Store(0)
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := f.repo.Allow(context.Background(), "concurrent-test-key", 4, now, now.Add(time.Hour))
			if err != nil {
				t.Error(err)
			}
			if ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 4 {
		t.Fatalf("rate limit allowed %d calls, want 4", successes.Load())
	}
	ok, err := f.repo.Allow(context.Background(), "concurrent-test-key", 4, now.Add(2*time.Hour), now.Add(3*time.Hour))
	if err != nil || !ok {
		t.Fatal("rate window did not expire")
	}
}

func TestIntegrationPendingSignupCannotReplaceAccount(t *testing.T) {
	f := newAuthFixture(t)
	email := "duplicate@example.test"
	first := f.registration(t, email, "first_user")
	firstCode := f.mail.code(email, "register")
	second := f.registration(t, email, "second_user")
	secondCode := f.mail.code(email, "register")
	status, _, _ := f.request(t, "verify-email", map[string]string{"challenge": first, "code": firstCode}, nil)
	if status != 200 {
		t.Fatal("first verification failed")
	}
	status, _, _ = f.request(t, "verify-email", map[string]string{"challenge": second, "code": secondCode}, nil)
	if status != 409 {
		t.Fatal("pending signup replaced existing identity")
	}
	var users int64
	f.database.Model(&models.User{}).Count(&users)
	if users != 1 {
		t.Fatal("duplicate email persisted")
	}
}

type delayedUserLock struct {
	services.AuthRepository
	reached chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (r *delayedUserLock) UserByID(ctx context.Context, id string, lock bool) (*models.User, error) {
	r.once.Do(func() { close(r.reached) })
	select {
	case <-r.proceed:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return r.AuthRepository.UserByID(ctx, id, lock)
}

func TestIntegrationResetDuringLoginCannotIssueStaleSession(t *testing.T) {
	f := newAuthFixture(t)
	email := "stale-login@example.test"
	f.account(t, email, "stale_login_user")
	token, err := f.auth.ForgotPassword(context.Background(), email, "en")
	if err != nil {
		t.Fatal(err)
	}
	delayed := &delayedUserLock{AuthRepository: f.repo, reached: make(chan struct{}), proceed: make(chan struct{})}
	executor := db.NewExecutor(f.database)
	loginService := services.NewAuth(delayed, db.NewTransactionManager(executor), f.mail, &security.Config{Secret: strings.Repeat("stale-test-secret", 3)})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := loginService.Login(ctx, email, initialPassword); result <- err }()
	select {
	case <-delayed.reached:
	case <-ctx.Done():
		t.Fatal("login did not reach the session issuance boundary")
	}
	resetErr := f.auth.ResetPassword(ctx, token, f.mail.code(email, "reset"), "a new password after the concurrent reset")
	close(delayed.proceed)
	if resetErr != nil {
		t.Fatal(resetErr)
	}
	if err := <-result; err != services.ErrCredentials {
		t.Fatalf("stale login was not rejected: %v", err)
	}
	var sessions int64
	if err := f.database.Model(&models.AuthSession{}).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatal("old password issued a session after password reset")
	}
}

func TestIntegrationSessionExpiryCleanup(t *testing.T) {
	f := newAuthFixture(t)
	cookie := f.account(t, "expiry@example.test", "expiry_user")
	f.database.Exec("UPDATE auth_sessions SET expires_at = ?", time.Now().Add(-time.Hour))
	status, _, _ := f.request(t, "me", nil, cookie)
	if status != 401 {
		t.Fatal("expired session accepted")
	}
	if err := f.auth.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.database.Model(&models.AuthSession{}).Count(&count)
	if count != 0 {
		t.Fatal("expired sessions not removed")
	}
}

func TestIntegrationAuthValidationAndEmailFailure(t *testing.T) {
	f := newAuthFixture(t)
	for _, body := range []map[string]string{
		{"email": "Display <a@example.test>", "username": "valid_user", "password": initialPassword},
		{"email": "a@example.test", "username": "a", "password": initialPassword},
		{"email": "a@example.test", "username": "valid_user", "password": "abc"},
	} {
		status, _, _ := f.request(t, "register", body, nil)
		if status != 400 {
			t.Fatalf("invalid registration accepted: %d", status)
		}
	}
	f.mail.fail = true
	status, body, _ := f.request(t, "register", map[string]string{"email": "mail@example.test", "username": "mail_user", "password": initialPassword}, nil)
	if status != 503 || body["error"] != "email_unavailable" {
		t.Fatal("mail failure concealed")
	}
	var count int64
	f.database.Model(&models.User{}).Count(&count)
	if count != 0 {
		t.Fatal("SMTP failure created an account")
	}
	for i := 0; i < 15; i++ {
		_, err := f.auth.Login(context.Background(), "unknown@example.test", initialPassword)
		if err != services.ErrCredentials {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	_, err := f.auth.Login(context.Background(), "unknown@example.test", initialPassword)
	if err != services.ErrRateLimit {
		t.Fatal("email login rate limit not enforced")
	}
}
