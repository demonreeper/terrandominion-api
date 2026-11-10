// Package config reads the server's settings from the environment. Secrets never live in the
// repository: the server keeps them in an env file loaded by systemd (see deploy/).
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Env is "live" or "dev". Dev and live run as separate services on separate databases.
	Env string
	// Addr is where the HTTP server listens. nginx terminates TLS and proxies to it.
	Addr string
	// DatabaseURL is a Postgres connection string for this environment's database.
	DatabaseURL string
	// JWTSecret signs access tokens. At least 32 bytes.
	JWTSecret []byte
	// DevicePepper is mixed into device-id hashes so raw device ids are never stored.
	DevicePepper []byte
	// GoogleClientIDs are the OAuth client ids whose ID tokens are accepted (web, Android, iOS).
	GoogleClientIDs []string
	// AppleBundleIDs are the app bundle ids whose Sign in with Apple tokens are accepted.
	AppleBundleIDs []string
	// TrustProxy reads the client IP from X-Real-IP (set by nginx). Only enable behind nginx.
	TrustProxy bool
	// FCMProjectID and FCMCredentialsFile enable phone notifications. The file holds a private key.
	FCMProjectID       string
	FCMCredentialsFile string
}

func (c Config) IsLive() bool { return c.Env == "live" }

func Load() (Config, error) {
	c := Config{
		Env:                getenv("TD_ENV", "dev"),
		Addr:               getenv("TD_ADDR", "127.0.0.1:8090"),
		DatabaseURL:        os.Getenv("TD_DATABASE_URL"),
		JWTSecret:          []byte(os.Getenv("TD_JWT_SECRET")),
		DevicePepper:       []byte(os.Getenv("TD_DEVICE_PEPPER")),
		GoogleClientIDs:    splitList(os.Getenv("TD_GOOGLE_CLIENT_IDS")),
		AppleBundleIDs:     splitList(os.Getenv("TD_APPLE_BUNDLE_IDS")),
		FCMProjectID:       os.Getenv("TD_FCM_PROJECT_ID"),
		FCMCredentialsFile: os.Getenv("TD_FCM_CREDENTIALS_FILE"),
	}

	trust, err := strconv.ParseBool(getenv("TD_TRUST_PROXY", "true"))
	if err != nil {
		return c, fmt.Errorf("TD_TRUST_PROXY: %w", err)
	}
	c.TrustProxy = trust

	var problems []string
	if c.Env != "dev" && c.Env != "live" {
		problems = append(problems, `TD_ENV must be "dev" or "live"`)
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "TD_DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		problems = append(problems, "TD_JWT_SECRET must be at least 32 bytes")
	}
	if len(c.DevicePepper) < 16 {
		problems = append(problems, "TD_DEVICE_PEPPER must be at least 16 bytes")
	}
	if c.IsLive() && len(c.GoogleClientIDs) == 0 {
		problems = append(problems, "TD_GOOGLE_CLIENT_IDS is required on live")
	}
	if len(problems) > 0 {
		return c, errors.New(strings.Join(problems, "; "))
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
