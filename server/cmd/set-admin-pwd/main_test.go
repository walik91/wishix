package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wishix/server/internal/auth"
)

func TestUpdatePasswordPreservesEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	original := "ADMIN_USERNAME=admin\nKEEP=unchanged\nADMIN_PASSWORD_HASH=old\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{path}, strings.NewReader("long-enough-password\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "ADMIN_USERNAME=admin\nKEEP=unchanged\n") || strings.Contains(content, "long-enough-password") {
		t.Fatal("env not preserved")
	}
	encoded := strings.Split(strings.Split(content, "ADMIN_PASSWORD_HASH=")[1], "\n")[0]
	hash, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || !auth.CheckPassword("long-enough-password", string(hash)) {
		t.Fatal("invalid hash")
	}
	for _, password := range []string{"12345", "", strings.Repeat("a", 600)} {
		if err = run([]string{path}, strings.NewReader(password+"\n")); err != nil {
			t.Fatalf("set password: %v", err)
		}
		updated, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		value := strings.Split(strings.Split(string(updated), "ADMIN_PASSWORD_HASH=")[1], "\n")[0]
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(value)
		if decodeErr != nil || !auth.CheckPassword(password, string(decoded)) {
			t.Fatal("password hash did not verify")
		}
	}
}
