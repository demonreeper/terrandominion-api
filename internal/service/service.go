// Package service is where requests become game actions. Each exported method is one player or
// admin action, runs in one transaction, loads the state the rules need, asks package game what
// happens, and writes the result. HTTP handlers (and later, Nakama RPCs) only translate to and
// from these calls.
package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/auth"
	"github.com/demonreeper/terrandominion/api/internal/config"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type Service struct {
	db     *sql.DB
	cfg    config.Config
	tokens *auth.Tokens
	google *auth.GoogleVerifier
	now    func() time.Time

	settingsMu     sync.RWMutex
	settings       game.Settings
	settingsLoaded time.Time
}

func New(conn *sql.DB, cfg config.Config) *Service {
	return &Service{
		db:       conn,
		cfg:      cfg,
		tokens:   auth.NewTokens(cfg.JWTSecret, cfg.Env),
		google:   auth.NewGoogleVerifier(cfg.GoogleClientIDs),
		now:      time.Now,
		settings: game.DefaultSettings(),
	}
}

func (s *Service) Tokens() *auth.Tokens { return s.tokens }

const settingsTTL = time.Minute

// Settings returns the cached rules, reloading them at most once a minute. If the reload fails
// the last good values stay in force.
func (s *Service) Settings(ctx context.Context) game.Settings {
	s.settingsMu.RLock()
	fresh := s.now().Sub(s.settingsLoaded) < settingsTTL
	current := s.settings
	s.settingsMu.RUnlock()
	if fresh {
		return current
	}

	values, err := store.LoadSettings(ctx, s.db)
	if err != nil {
		slog.Error("E1900 settings reload failed; keeping previous values", "err", err)
		return current
	}
	loaded := game.SettingsFrom(values)

	s.settingsMu.Lock()
	s.settings = loaded
	s.settingsLoaded = s.now()
	s.settingsMu.Unlock()
	return loaded
}

func (s *Service) invalidateSettings() {
	s.settingsMu.Lock()
	s.settingsLoaded = time.Time{}
	s.settingsMu.Unlock()
}
