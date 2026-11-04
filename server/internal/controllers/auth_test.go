package controllers

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	security "wishix/server/internal/auth"
)

func TestAuthRejectsCSRFAndMalformedRequests(t *testing.T) {
	controller := NewAuth(nil, &security.Config{Origin: "https://wishix.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, test := range []struct {
		name, origin, header, contentType, body string
		status                                  int
	}{
		{"foreign origin", "https://attacker.test", "1", "application/json", "{}", 403},
		{"missing origin", "", "1", "application/json", "{}", 403},
		{"no custom header", "https://wishix.test", "", "application/json", "{}", 403},
		{"html form", "https://wishix.test", "1", "application/x-www-form-urlencoded", "email=a", 415},
		{"trailing json", "https://wishix.test", "1", "application/json", "{}{}", 400},
		{"unknown fields", "https://wishix.test", "1", "application/json", `{"token":"secret"}`, 400},
		{"oversized", "https://wishix.test", "1", "application/json", `{"email":"` + strings.Repeat("x", 5000) + `"}`, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(test.body))
			request.Header.Set("Origin", test.origin)
			request.Header.Set("X-Wishix-Request", test.header)
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			controller.mutation("login", nil)(response, request)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
				t.Fatalf("unexpected response: %d", response.Code)
			}
		})
	}
}

func TestClientIPRequiresTrustedProxy(t *testing.T) {
	config := &security.Config{}
	controller := NewAuth(nil, config, nil)
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "192.0.2.5:1234"
	request.Header.Set("X-Forwarded-For", "1.1.1.1")
	request.Header.Set("X-Real-IP", "203.0.113.5")
	if controller.clientIP(request) != "192.0.2.5" {
		t.Fatal("untrusted headers changed client IP")
	}
	_, network, _ := net.ParseCIDR("192.0.2.0/24")
	config.TrustedProxies = []*net.IPNet{network}
	if controller.clientIP(request) != "203.0.113.5" {
		t.Fatal("trusted proxy ignored")
	}
	request.Header.Set("X-Real-IP", "spoofed,203.0.113.5")
	if controller.clientIP(request) != "192.0.2.5" {
		t.Fatal("invalid IP header accepted")
	}
	request.RemoteAddr = "[2001:db8::1234]:1234"
	if controller.clientIP(request) != "2001:db8::" {
		t.Fatal("IPv6 prefix not normalized")
	}
}
