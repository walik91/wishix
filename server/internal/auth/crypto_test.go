package auth

import (
	"strings"
	"testing"
)

func TestPasswordsAndSecrets(t *testing.T) {
	password := "a long passphrase with spaces"
	first, second := HashPassword(password), HashPassword(password)
	if first == second || strings.Contains(first, password) {
		t.Fatal("password hashes must be salted and irreversible")
	}
	if !CheckPassword(password, first) || CheckPassword(password+"!", first) || CheckPassword(password, "$argon2id$malformed") {
		t.Fatal("password verification failed")
	}
	token := Token()
	if len(token) != 43 || token == Token() || Digest(token) == token {
		t.Fatal("invalid session token")
	}
	if MAC("secret", "register:123456") == MAC("secret", "reset:123456") {
		t.Fatal("code purposes must be separated")
	}
}

func TestProductionConfiguration(t *testing.T) {
	for key, value := range map[string]string{"APP_ENV": "production", "APP_ORIGIN": "https://example.test", "AUTH_SECRET": strings.Repeat("a", 48), "SMTP_HOST": "smtp.example.test", "SMTP_PORT": "587", "SMTP_FROM": "support@example.test", "SMTP_SECURITY": "starttls", "SMTP_USERNAME": "", "SMTP_PASSWORD": "", "AUTH_TRUSTED_PROXY_CIDRS": ""} {
		t.Setenv(key, value)
	}
	config, err := LoadConfig()
	if err != nil || !config.SecureCookie {
		t.Fatal("valid production config rejected", err)
	}
	t.Setenv("APP_ORIGIN", "http://example.test")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("production allowed insecure cookie")
	}
	t.Setenv("APP_ORIGIN", "https://example.test")
	t.Setenv("SMTP_SECURITY", "none")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("production allowed unencrypted SMTP")
	}
}

func TestLocalizedMail(t *testing.T) {
	for _, locale := range []string{"en", "ru", "ro"} {
		for _, purpose := range []string{"register", "reset"} {
			subject, body := codeMessage(locale, purpose, "012345")
			if !strings.Contains(subject, "Wishix") || !strings.Contains(body, "012345") || !strings.Contains(body, "10") {
				t.Fatal("incomplete code email")
			}
		}
	}
}
