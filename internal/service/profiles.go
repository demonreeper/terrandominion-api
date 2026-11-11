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

// ProfileView is a player's public profile: what anyone may see, on the profile screen and (P2) the
// web page. Nothing private is in it: no real name, email, sign-ins, resources, gear, items,
// Tether Node places or last-seen time. Your own profile adds those from /v1/me on the phone.
type ProfileView struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	// Faction is the standing line: null for a freelancer.
	Faction *ProfileFaction     `json:"faction"`
	Badges  []store.Badge       `json:"badges"`
	Record  store.ServiceRecord `json:"record"`
	// IsMe: the asker is looking at their own profile.
	IsMe     bool   `json:"isMe"`
	ShareURL string `json:"shareUrl"`
}

// ProfileFaction is a member's faction as their profile shows it, in the faction's colour.
type ProfileFaction struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Rank      int    `json:"rank"`
	RankTitle string `json:"rankTitle"`
	// Perks are what every member of this faction gets, so a perk is visible on anyone in it.
	Perks []store.PerkLine `json:"perks"`
}

// Profile is the public profile behind a share code, for a signed-in player (viewerID) or the web
// page (viewerID 0). An unknown, deleted or banned player is NotFound.
func (s *Service) Profile(ctx context.Context, viewerID int64, code string) (*ProfileView, error) {
	playerID, err := playerIDByCode(ctx, s.db, code)
	if err != nil {
		return nil, err
	}
	player, err := store.GetPlayer(ctx, s.db, playerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFound
	}
	if err != nil {
		return nil, err
	}
	view := &ProfileView{
		Code: player.Code, Name: player.Name, Level: player.Level,
		IsMe: viewerID != 0 && viewerID == playerID, ShareURL: s.ShareURL(player.Code),
	}
	membership, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return nil, err
	}
	if membership != nil {
		describeMembership(membership)
		color, err := store.FactionColor(ctx, s.db, membership.FactionID)
		if err != nil {
			return nil, err
		}
		view.Faction = &ProfileFaction{
			Code: membership.FactionCode, Name: membership.FactionName, Color: color,
			Rank: membership.Rank, RankTitle: membership.RankTitle,
			Perks: perkLines(s.Settings(ctx), membership.FactionID),
		}
	}
	if view.Badges, err = store.Badges(ctx, s.db, playerID); err != nil {
		return nil, err
	}
	if view.Record, err = store.PlayerRecord(ctx, s.db, playerID); err != nil {
		return nil, err
	}
	return view, nil
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
