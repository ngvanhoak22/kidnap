package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment       string
	Address           string
	DatabaseURL       string
	AllowedOrigins    []string
	TrustedProxies    []string
	CookieSecure      bool
	SessionTTL        time.Duration
	InvitationTTL     time.Duration
	AuthRatePerMinute int
	DBMaxConnections  int32
}

func Load() (Config, error) {
	c := Config{Environment: value("APP_ENV", "development"), Address: value("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"), AllowedOrigins: split(value("ALLOWED_ORIGINS", "http://localhost:5173")),
		TrustedProxies: split(os.Getenv("TRUSTED_PROXIES"))}
	var err error
	if c.CookieSecure, err = strconv.ParseBool(value("COOKIE_SECURE", "false")); err != nil {
		return c, fmt.Errorf("COOKIE_SECURE must be boolean")
	}
	if c.SessionTTL, err = time.ParseDuration(value("SESSION_TTL", "168h")); err != nil || c.SessionTTL < time.Minute || c.SessionTTL > 30*24*time.Hour {
		return c, fmt.Errorf("SESSION_TTL must be between 1m and 720h")
	}
	if c.InvitationTTL, err = time.ParseDuration(value("INVITATION_TTL", "72h")); err != nil || c.InvitationTTL < time.Minute || c.InvitationTTL > 7*24*time.Hour {
		return c, fmt.Errorf("INVITATION_TTL must be between 1m and 168h")
	}
	if c.AuthRatePerMinute, err = strconv.Atoi(value("AUTH_RATE_PER_MINUTE", "10")); err != nil || c.AuthRatePerMinute < 1 || c.AuthRatePerMinute > 1000 {
		return c, fmt.Errorf("AUTH_RATE_PER_MINUTE must be between 1 and 1000")
	}
	n, err := strconv.Atoi(value("DB_MAX_CONNECTIONS", "10"))
	if err != nil || n < 2 || n > 100 {
		return c, fmt.Errorf("DB_MAX_CONNECTIONS must be between 2 and 100")
	}
	c.DBMaxConnections = int32(n)
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return c, fmt.Errorf("APP_ENV must be development, test or production")
	}
	dbURL, err := url.Parse(c.DatabaseURL)
	if err != nil || (dbURL.Scheme != "postgres" && dbURL.Scheme != "postgresql") || dbURL.Host == "" || dbURL.Path == "" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	if len(c.AllowedOrigins) == 0 {
		return c, fmt.Errorf("ALLOWED_ORIGINS must not be empty")
	}
	for _, origin := range c.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return c, fmt.Errorf("ALLOWED_ORIGINS must contain exact origins without paths")
		}
		if c.Environment == "production" && u.Scheme != "https" {
			return c, fmt.Errorf("production origins require HTTPS")
		}
	}
	if c.Environment == "production" && !c.CookieSecure {
		return c, fmt.Errorf("production requires COOKIE_SECURE=true")
	}
	return c, nil
}

func (c Config) CookieName() string {
	if c.CookieSecure {
		return "__Host-journey_session"
	}
	return "journey_session"
}
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func split(v string) []string {
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
