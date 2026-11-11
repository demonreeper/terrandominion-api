package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Profiles (docs/profile-plan.md): callsigns, share codes and the public profile.

// ShareURL is the link to a player's profile: this server's public address plus /p/<code>.
func (s *Service) ShareURL(code string) string {
	return s.cfg.PublicURL + "/p/" + code
}

// PlayerIDByCode turns a share code from a request into the player's id. An unknown, deleted or
// banned player, or text that is not a code, is NotFound.
func (s *Service) PlayerIDByCode(ctx context.Context, code string) (int64, error) {
	return playerIDByCode(ctx, s.db, code)
}

func playerIDByCode(ctx context.Context, q db.Querier, code string) (int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !game.ValidPublicCode(code) {
		return 0, apperr.NotFound.With("player code %q", code)
	}
	id, err := store.PlayerIDByCode(ctx, q, code)
	if errors.Is(err, store.ErrNotFound) {
		return 0, apperr.NotFound.With("player code %q", code)
	}
	return id, err
}

// SetCallsign gives the player the callsign they typed: checked for shape, reserved and blocked
// words, that nobody else has it (in any letter case), and that CallsignChangeDays have passed
// since the last change. The first pick is free of the wait. Returns the player's new /me.
func (s *Service) SetCallsign(ctx context.Context, playerID int64, typed string) (*MeView, error) {
	settings := s.Settings(ctx)
	callsign, ok := game.CleanCallsign(typed)
	if !ok {
		return nil, apperr.CallsignInvalid.WithData(map[string]any{"error": "callsign_invalid"})
	}

	var me *MeView
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.Unauthorized
		}
		if err != nil {
			return err
		}
		// The same callsign again (or only its letter case changed) is not a change: no wait, no cost.
		if player.Callsign.Valid && player.Callsign.String == callsign {
			me, err = s.meView(ctx, tx, playerID, settings)
			return err
		}
		caseOnly := player.Callsign.Valid && strings.EqualFold(player.Callsign.String, callsign)

		blocked, err := store.BlockedCallsignWords(ctx, tx)
		if err != nil {
			return err
		}
		if game.CallsignBlocked(callsign, blocked) {
			// Said the same way as a taken name, so the blocked list can't be read back by trying words.
			return apperr.CallsignTaken.WithData(map[string]any{"error": "callsign_taken"}).With("blocked %q", callsign)
		}
		taken, err := store.CallsignTaken(ctx, tx, callsign, playerID)
		if err != nil {
			return err
		}
		if taken {
			return apperr.CallsignTaken.WithData(map[string]any{"error": "callsign_taken"})
		}
		if player.CallsignChangedAt.Valid && !caseOnly {
			at := game.CallsignChangeAt(&player.CallsignChangedAt.Time, settings.CallsignChangeDays)
			if s.now().Before(at) {
				return apperr.CallsignTooSoon.
					Say("You can change your callsign again on %s.", at.UTC().Format("2 Jan 2006")).
					WithData(map[string]any{"error": "callsign_too_soon", "changeAt": at.UTC(), "days": settings.CallsignChangeDays})
			}
		}

		changedAt := s.now()
		if caseOnly {
			changedAt = player.CallsignChangedAt.Time // fixing the capitals doesn't restart the wait
		}
		if err := store.SetCallsign(ctx, tx, playerID, callsign, changedAt); err != nil {
			// Two players asking for the same new callsign at once: the unique index decides.
			if isUniqueViolation(err) {
				return apperr.CallsignTaken.WithData(map[string]any{"error": "callsign_taken"})
			}
			return err
		}
		me, err = s.meView(ctx, tx, playerID, settings)
		return err
	})
	return me, err
}

// isUniqueViolation says whether a database error is Postgres's unique_violation (23505).
func isUniqueViolation(err error) bool {
	var coded interface{ SQLState() string }
	if errors.As(err, &coded) {
		return coded.SQLState() == "23505"
	}
	return err != nil && strings.Contains(err.Error(), "23505")
}
