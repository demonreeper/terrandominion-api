package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/auth"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type DeviceInfo struct {
	DeviceID   string
	Platform   string
	AppVersion string
}

type AuthResult struct {
	AccessToken     string    `json:"accessToken"`
	AccessExpiresAt time.Time `json:"accessExpiresAt"`
	RefreshToken    string    `json:"refreshToken"`
	NewAccount      bool      `json:"newAccount"`
	Player          *MeView   `json:"player"`
}

func (d DeviceInfo) validate() *apperr.Error {
	if len(d.DeviceID) < 8 || len(d.DeviceID) > 200 {
		return apperr.DeviceRequired
	}
	switch d.Platform {
	case "android", "ios", "editor", "pc":
	default:
		return apperr.BadRequest.With("unknown platform %q", d.Platform)
	}
	return nil
}

// SignInGoogle verifies a Google ID token and signs the player in, creating the account on first
// sign-in. A new account gets starter gear, and the welcome resources only if this phone has not
// already had them.
func (s *Service) SignInGoogle(ctx context.Context, idToken, nonce string, device DeviceInfo) (AuthResult, error) {
	if err := device.validate(); err != nil {
		return AuthResult{}, err
	}
	identity, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return AuthResult{}, apperr.GoogleTokenInvalid.With("%v", err)
	}
	// The app asks Google to put its own random nonce in the token, so a token lifted from
	// elsewhere can't be replayed here. Older apps send none.
	if nonce != "" && identity.Nonce != nonce {
		return AuthResult{}, apperr.GoogleTokenInvalid.With("nonce does not match")
	}
	return s.signIn(ctx, "google", identity.Subject, identity.Email, identity.Name, device)
}

// SignInDev signs in without Google, for local testing only. The HTTP layer exposes it only when
// TD_DEV_LOGIN is on and TD_ENV is dev.
func (s *Service) SignInDev(ctx context.Context, name string, device DeviceInfo) (AuthResult, error) {
	if err := device.validate(); err != nil {
		return AuthResult{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AuthResult{}, apperr.BadRequest.With("dev sign-in needs a name")
	}
	return s.signIn(ctx, "dev", strings.ToLower(name), "", name, device)
}

func (s *Service) signIn(ctx context.Context, provider, subject, email, name string, device DeviceInfo) (AuthResult, error) {
	settings := s.Settings(ctx)
	deviceHash := auth.HashDevice(device.DeviceID, s.cfg.DevicePepper)
	var result AuthResult

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		playerID, err := store.FindIdentity(ctx, tx, provider, subject)
		isNew := errors.Is(err, store.ErrNotFound)
		if err != nil && !isNew {
			return err
		}

		if isNew {
			playerID, err = store.CreatePlayer(ctx, tx, defaultDisplayName(name), "MV")
			if err != nil {
				return err
			}
			if err := store.LinkIdentity(ctx, tx, provider, subject, playerID, email); err != nil {
				return err
			}
			if err := store.GrantStarterGear(ctx, tx, playerID); err != nil {
				return err
			}
		}

		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		switch player.Status {
		case "banned":
			return apperr.AccountBanned
		case "deleted":
			return apperr.AccountDeleted
		}

		granted, err := store.RegisterDevice(ctx, tx, deviceHash, playerID, isNew)
		if err != nil {
			return err
		}
		if isNew && !granted {
			// Uploaded, so it doesn't fill the carry limit: building and crafting pay from there.
			for resource, amount := range map[string]int{
				"crystal": settings.WelcomeCrystal, "alloy": settings.WelcomeAlloy, "data": settings.WelcomeData,
			} {
				if err := store.AddUploaded(ctx, tx, playerID, resource, int64(amount)); err != nil {
					return err
				}
			}
			if settings.WelcomeCells > 0 {
				if err := store.AddItem(ctx, tx, playerID, "energy_cell", settings.WelcomeCells); err != nil {
					return err
				}
			}
			if err := store.MarkWelcomeGranted(ctx, tx, deviceHash); err != nil {
				return err
			}
		}

		if err := store.TouchPlayer(ctx, tx, playerID, device.AppVersion); err != nil {
			return err
		}

		refresh, refreshHash, err := auth.NewRefreshToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(settings.SessionDays) * 24 * time.Hour)
		sessionID, err := store.CreateSession(ctx, tx, playerID, refreshHash, deviceHash, device.Platform, device.AppVersion, expires)
		if err != nil {
			return err
		}

		access, accessExpires, err := s.tokens.IssueAccess(playerID, sessionID, player.IsAdmin,
			time.Duration(settings.AccessTokenMinutes)*time.Minute)
		if err != nil {
			return err
		}

		me, err := s.meView(ctx, tx, playerID, settings)
		if err != nil {
			return err
		}
		result = AuthResult{
			AccessToken: access, AccessExpiresAt: accessExpires, RefreshToken: refresh,
			NewAccount: isNew, Player: me,
		}
		if isNew {
			slog.Info("player created", "player", playerID, "provider", provider, "welcome", !granted)
		}
		return nil
	})
	return result, err
}

// Refresh spends a refresh token and returns a new pair. A spent, revoked or expired token fails,
// and the player's status is checked again, so a ban lands within one access-token lifetime.
func (s *Service) Refresh(ctx context.Context, refreshToken, appVersion string) (AuthResult, error) {
	settings := s.Settings(ctx)
	var result AuthResult

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		session, err := store.SessionByRefreshForUpdate(ctx, tx, auth.HashRefreshToken(refreshToken))
		if errors.Is(err, store.ErrNotFound) {
			return apperr.SessionExpired
		}
		if err != nil {
			return err
		}
		if session.RevokedAt.Valid || s.now().After(session.ExpiresAt) {
			return apperr.SessionExpired
		}

		player, err := store.GetPlayer(ctx, tx, session.PlayerID)
		if err != nil {
			return err
		}
		switch player.Status {
		case "banned":
			_ = store.RevokeAllSessions(ctx, tx, player.ID)
			return apperr.AccountBanned
		case "deleted":
			return apperr.AccountDeleted
		}

		refresh, refreshHash, err := auth.NewRefreshToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(settings.SessionDays) * 24 * time.Hour)
		if err := store.RotateSession(ctx, tx, session.ID, refreshHash, appVersion, expires); err != nil {
			return err
		}
		if err := store.TouchPlayer(ctx, tx, player.ID, appVersion); err != nil {
			return err
		}

		access, accessExpires, err := s.tokens.IssueAccess(player.ID, session.ID, player.IsAdmin,
			time.Duration(settings.AccessTokenMinutes)*time.Minute)
		if err != nil {
			return err
		}
		result = AuthResult{AccessToken: access, AccessExpiresAt: accessExpires, RefreshToken: refresh}
		return nil
	})
	return result, err
}

func (s *Service) Logout(ctx context.Context, playerID, sessionID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.DeleteSessionPushTokens(ctx, tx, sessionID, playerID); err != nil {
			return err
		}
		return store.RevokeSession(ctx, tx, sessionID, playerID)
	})
}

// RequestDeletion starts the grace period and signs every device out. Signing in again within
// DeleteGraceDays cancels it.
func (s *Service) RequestDeletion(ctx context.Context, playerID int64) (time.Time, error) {
	settings := s.Settings(ctx)
	after := s.now().Add(time.Duration(settings.DeleteGraceDays) * 24 * time.Hour)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.ScheduleDeletion(ctx, tx, playerID, after); err != nil {
			return err
		}
		if err := store.DeletePlayerPushTokens(ctx, tx, playerID); err != nil {
			return err
		}
		return store.RevokeAllSessions(ctx, tx, playerID)
	})
	return after, err
}

// MeView is the signed-in player's own view of their account.
type MeView struct {
	ID                int64                 `json:"id"`
	DisplayName       string                `json:"displayName"`
	Type              string                `json:"type"`
	IsAdmin           bool                  `json:"isAdmin,omitempty"`
	Progress          game.Progress         `json:"progress"`
	Carried           map[string]int64      `json:"carried"`
	Uploaded          map[string]int64      `json:"uploaded"`
	TethersOwned      int                   `json:"tethersOwned"`
	TetherMax         int                   `json:"tetherMax"`
	TetherCost        map[string]int64      `json:"tetherCost"`
	TetherUpgradeCost map[string]int64      `json:"tetherUpgradeCost"`
	CarryCap          int64                 `json:"carryCapacity"`
	ScanRangeM        float64               `json:"scanRangeM"`
	Gear              []store.GearItem      `json:"gear"`
	Items             map[string]int64      `json:"items"`
	Recipes           []game.Recipe         `json:"recipes"`
	CraftableGear     []store.CraftableGear `json:"craftableGear"`
	Faction           *store.Membership     `json:"faction"`
	Freelancer        bool                  `json:"freelancer"`
	PushAttacks       bool                  `json:"pushAttacks"`
	PushResults       bool                  `json:"pushResults"`
	DeleteAfter       *time.Time            `json:"deleteAfter,omitempty"`
}

func (s *Service) Me(ctx context.Context, playerID int64) (*MeView, error) {
	return s.meView(ctx, s.db, playerID, s.Settings(ctx))
}

func (s *Service) meView(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (*MeView, error) {
	player, err := store.GetPlayer(ctx, q, playerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.Unauthorized
	}
	if err != nil {
		return nil, err
	}
	carried, err := store.Carried(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	uploaded, err := store.Uploaded(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	tethersOwned, err := store.CountActiveTethers(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	gear, err := store.Gear(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	for i := range gear {
		gear[i].RepairCost = game.RepairCost(settings, gear[i].Durability, gear[i].MaxDurability)
	}
	items, err := store.Items(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	craftable, err := store.CraftableGearFor(ctx, q, playerID, game.CraftableGearIDs)
	if err != nil {
		return nil, err
	}
	stats, err := store.Equipped(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return nil, err
	}

	pushAttacks, pushResults, err := store.PushPrefs(ctx, q, playerID)
	if err != nil {
		return nil, err
	}

	me := &MeView{
		PushAttacks: pushAttacks, PushResults: pushResults,
		ID: player.ID, DisplayName: player.DisplayName, Type: player.Type, IsAdmin: player.IsAdmin,
		Progress: game.ProgressFor(player.XP, settings), Carried: carried, Uploaded: uploaded, TethersOwned: tethersOwned,
		TetherMax: settings.TnMaxPerPlayer, TetherCost: game.TetherBuildCost(settings), TetherUpgradeCost: game.TetherUpgradeCost(settings), CarryCap: stats.CarryCapacity,
		ScanRangeM: stats.ScanRangeM, Gear: gear, Items: items, Recipes: game.Recipes(settings), CraftableGear: craftable, Faction: membership, Freelancer: membership == nil,
	}
	if player.DeleteAfter.Valid {
		t := player.DeleteAfter.Time
		me.DeleteAfter = &t
	}
	return me, nil
}

// awardXP adds XP and keeps the stored level in step. Every action calls it.
func awardXP(ctx context.Context, q db.Querier, playerID int64, action string, xp int, refID int64,
	settings game.Settings) (game.Progress, bool, error) {
	before, err := store.GetPlayer(ctx, q, playerID)
	if err != nil {
		return game.Progress{}, false, err
	}
	total, err := store.AddXP(ctx, q, playerID, action, xp, refID)
	if err != nil {
		return game.Progress{}, false, err
	}
	progress := game.ProgressFor(total, settings)
	// Levels only ever go up: an admin may grant a level by hand, and XP must not take it away.
	if progress.Level > before.Level {
		if err := store.SetLevel(ctx, q, playerID, progress.Level); err != nil {
			return progress, false, err
		}
		return progress, true, nil
	}
	progress.Level = before.Level
	return progress, false, nil
}

func defaultDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 24 {
		return fmt.Sprintf("Freelancer-%04d", time.Now().UnixNano()%10000)
	}
	return name
}
