package config

import "testing"

func setup(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{"APP_ENV": "development", "DATABASE_URL": "postgres://test:test@localhost:5432/journey_test?sslmode=disable", "HTTP_ADDR": "127.0.0.1:8080", "ALLOWED_ORIGINS": "http://localhost:5173", "COOKIE_SECURE": "false", "SESSION_TTL": "168h", "INVITATION_TTL": "72h", "AUTH_RATE_PER_MINUTE": "10", "DB_MAX_CONNECTIONS": "10", "TRUSTED_PROXIES": ""} {
		t.Setenv(k, v)
	}
}
func TestProductionFailsClosed(t *testing.T) {
	setup(t)
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted insecure settings")
	}
	t.Setenv("ALLOWED_ORIGINS", "https://journey.example")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted non-secure cookie")
	}
	t.Setenv("COOKIE_SECURE", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.CookieName() != "__Host-journey_session" {
		t.Fatal("missing host cookie prefix")
	}
}
func TestRejectInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"ALLOWED_ORIGINS", "*"}, {"ALLOWED_ORIGINS", "https://example.com/path"}, {"ALLOWED_ORIGINS", "https://example.com?secret=x"}, {"DATABASE_URL", "secret-not-a-url"}, {"SESSION_TTL", "0s"}, {"INVITATION_TTL", "1000h"}, {"DB_MAX_CONNECTIONS", "0"}, {"AUTH_RATE_PER_MINUTE", "0"}, {"APP_ENV", "prod"}, {"COOKIE_SECURE", "no"}} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			setup(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
