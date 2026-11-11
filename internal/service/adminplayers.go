package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Admin players and stats (docs/profile-plan.md, A2). Moderators may search and read a player's
// public side; everything private (real name, email, sign-ins, phone, failed location checks,
// what they hold) and every change is for admins.

const (
	playersFound     = 30
	infringementsMax = 20
	playerActionsMax = 20
)

// SearchPlayers is the staff player search: a piece of a callsign or share code, and for admins
// also the player's id or a piece of an email.
func (s *Service) SearchPlayers(ctx context.Context, actorID int64, typed string) ([]store.FoundPlayer, error) {
	actor, err := store.GetPlayer(ctx, s.db, actorID)
	if err != nil {
		return nil, err
	}
	q, ok := game.CleanPlayerSearch(typed)
	if !ok {
		return nil, apperr.BadRequest.Say("Type at least 2 characters.")
	}
	return store.SearchPlayers(ctx, s.db, q, actor.IsAdmin, playersFound)
}

// PlayerInfo is a player as staff see them. Private is nil for a moderator.
type PlayerInfo struct {
	Player         store.PlayerRef  `json:"player"`
	Level          int              `json:"level"`
	XP             int64            `json:"xp"`
	Status         string           `json:"status"`
	SuspendedUntil *time.Time       `json:"suspendedUntil,omitempty"`
	IsAdmin        bool             `json:"isAdmin"`
	IsMod          bool             `json:"isMod"`
	Faction        string           `json:"faction,omitempty"`
	RankTitle      string           `json:"rankTitle,omitempty"`
	TethersActive  int              `json:"tethersActive"`
	Registered     time.Time        `json:"registered"`
	LastSeen       *time.Time       `json:"lastSeen,omitempty"`
	ReportsAgainst int              `json:"reportsAgainst"`
	ReportsBy      int              `json:"reportsBy"`
	Sanctions      []store.Sanction `json:"sanctions"`
	// CanAct: this member of staff may warn, suspend or ban them.
	CanAct bool `json:"canAct"`
	// SuspendDays are the three lengths a suspension may have.
	SuspendDays []int `json:"suspendDays"`
	// SpoofExempt: a tester who may use a fake location; never signal-locked.
	SpoofExempt bool           `json:"spoofExempt"`
	Private     *PlayerPrivate `json:"private,omitempty"`
}

// PlayerPrivate is the part of a player only admins see.
type PlayerPrivate struct {
	ID                int64                    `json:"id"`
	RealName          string                   `json:"realName"`
	SignIns           []store.SignIn           `json:"signIns"`
	Platform          string                   `json:"platform"`
	AppVersion        string                   `json:"appVersion"`
	Carried           map[string]int64         `json:"carried"`
	Stored            map[string]int64         `json:"stored"`
	SharedPhone       []store.PlayerRef        `json:"sharedPhone"`
	InfringementKinds []store.InfringementKind `json:"infringementKinds"`
	// Marks are the device record's entries on this player's phones and sign-ins.
	Marks         []store.DeviceMark   `json:"marks"`
	Infringements []store.Infringement `json:"infringements"`
	Actions       []store.PlayerAction `json:"actions"`
}

// PlayerInfo reads one player for staff, by share code (a banned account too).
func (s *Service) PlayerInfo(ctx context.Context, actorID int64, code string) (*PlayerInfo, error) {
	actor, err := store.GetPlayer(ctx, s.db, actorID)
	if err != nil {
		return nil, err
	}
	targetID, err := staffTargetByCode(ctx, s.db, code)
	if err != nil {
		return nil, err
	}
	return s.playerInfo(ctx, s.db, actor, targetID)
}

func (s *Service) playerInfo(ctx context.Context, q db.Querier, actor store.Player, targetID int64) (*PlayerInfo, error) {
	target, err := store.GetPlayer(ctx, q, targetID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFound
	}
	if err != nil {
		return nil, err
	}
	activity, err := store.PlayerActivityOf(ctx, q, targetID)
	if err != nil {
		return nil, err
	}
	info := &PlayerInfo{
		Player: store.PlayerRef{Code: target.Code, Name: staffName(target)},
		Level:  target.Level, XP: target.XP, Status: target.Status, SuspendedUntil: activity.SuspendedUntil,
		IsAdmin: target.IsAdmin, IsMod: target.IsMod,
		TethersActive: activity.TethersActive, Registered: activity.Registered, LastSeen: activity.LastSeen,
		ReportsAgainst: activity.ReportsAgainst, ReportsBy: activity.ReportsBy,
		CanAct:      game.CanSanction(staffOf(actor), staffOf(target)),
		SuspendDays: game.SuspendDayChoices(s.Settings(ctx)),
	}
	membership, err := store.MembershipOf(ctx, q, targetID)
	if err != nil {
		return nil, err
	}
	if membership != nil {
		describeMembership(membership)
		info.Faction, info.RankTitle = membership.FactionName, membership.RankTitle
	}
	if info.Sanctions, err = store.SanctionsOf(ctx, q, targetID, s.now()); err != nil {
		return nil, err
	}
	if spoof, err := store.SpoofStateOf(ctx, q, targetID, s.now()); err != nil {
		return nil, err
	} else {
		info.SpoofExempt = spoof.Exempt
	}
	if !actor.IsAdmin {
		return info, nil
	}

	private := &PlayerPrivate{ID: target.ID, RealName: target.DisplayName}
	if private.SignIns, err = store.PlayerSignIns(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.Platform, private.AppVersion, err = store.LastSession(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.Carried, err = store.Carried(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.Stored, err = store.Uploaded(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.SharedPhone, err = store.SharedPhone(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.InfringementKinds, err = store.InfringementKinds(ctx, q, targetID); err != nil {
		return nil, err
	}
	if private.Infringements, err = store.RecentInfringements(ctx, q, targetID, infringementsMax); err != nil {
		return nil, err
	}
	if private.Actions, err = store.PlayerActions(ctx, q, targetID, playerActionsMax); err != nil {
		return nil, err
	}
	devices, signins, err := s.playerHashes(ctx, q, targetID)
	if err != nil {
		return nil, err
	}
	if private.Marks, err = store.MarksOf(ctx, q, devices, signins); err != nil {
		return nil, err
	}
	info.Private = private
	return info, nil
}

// AdminAdjust adds to (or with a minus, takes from) a player's XP, carried and stored resources.
// A reason is required and the change is logged. The carry limit is not applied: an admin may
// overfill. The level follows the new XP, up or down.
func (s *Service) AdminAdjust(ctx context.Context, adminID int64, code string, a game.Adjustment) (*PlayerInfo, error) {
	settings := s.Settings(ctx)
	a.Reason = strings.TrimSpace(a.Reason)
	var info *PlayerInfo
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		targetID, err := staffTargetByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		target, err := store.GetPlayerForUpdate(ctx, tx, targetID)
		if err != nil {
			return err
		}
		carried, err := store.Carried(ctx, tx, targetID)
		if err != nil {
			return err
		}
		stored, err := store.Uploaded(ctx, tx, targetID)
		if err != nil {
			return err
		}
		switch game.CheckAdjust(a, target.XP, carried, stored) {
		case game.AdjustOK:
		case game.AdjustReason:
			return apperr.AdjustBad.Say("Give a reason (3 to 500 characters).")
		case game.AdjustNothing:
			return apperr.AdjustBad.Say("Nothing to change: every amount is 0.")
		case game.AdjustTooMuch:
			return apperr.AdjustBad.Say("One adjustment can change at most %d of anything.", game.AdjustMax)
		case game.AdjustResource:
			return apperr.AdjustBad.Say("That isn't a resource the game has.")
		default:
			return apperr.AdjustBad.Say("That would take more than the player has.")
		}

		for resource, amount := range a.Carried {
			if amount == 0 {
				continue
			}
			if err := store.AddCarried(ctx, tx, targetID, resource, amount); err != nil {
				return err
			}
		}
		for resource, amount := range a.Stored {
			if amount == 0 {
				continue
			}
			if err := store.AddUploaded(ctx, tx, targetID, resource, amount); err != nil {
				return err
			}
		}
		if a.XP != 0 {
			total, err := store.AddXP(ctx, tx, targetID, "admin_adjust", int(a.XP), 0, 0)
			if err != nil {
				return err
			}
			if err := store.SetLevel(ctx, tx, targetID, game.LevelForXP(total, settings)); err != nil {
				return err
			}
		}
		detail, _ := json.Marshal(map[string]any{"xp": a.XP, "carried": nonZero(a.Carried), "stored": nonZero(a.Stored), "reason": a.Reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "player.adjust", fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		admin, err := store.GetPlayer(ctx, tx, adminID)
		if err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}

func nonZero(amounts map[string]int64) map[string]int64 {
	out := map[string]int64{}
	for resource, amount := range amounts {
		if amount != 0 {
			out[resource] = amount
		}
	}
	return out
}

// AdminResetCallsign removes a player's callsign (for an offensive one): they are OPERATIVE-<code>
// until they pick again, which they may do at once, and they are told in a written message.
func (s *Service) AdminResetCallsign(ctx context.Context, adminID int64, code, reason string) (*PlayerInfo, error) {
	reason = strings.TrimSpace(reason)
	if game.CheckSanction(s.Settings(ctx), game.SanctionWarn, 0, reason) != game.SanctionOK {
		return nil, apperr.AdjustBad.Say("Give a reason (3 to 500 characters).")
	}
	var info *PlayerInfo
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		targetID, err := staffTargetByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		target, err := store.GetPlayerForUpdate(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if !target.Callsign.Valid {
			return apperr.AdjustBad.Say("This player has no callsign to reset.")
		}
		if err := store.ClearCallsign(ctx, tx, targetID); err != nil {
			return err
		}
		now := s.now()
		if err := store.InsertSystemMessage(ctx, tx, targetID, game.ModerationSender,
			fmt.Sprintf("callsign_reset_%d", now.Unix()), game.NoticeCallsignReset, now); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"callsign": target.Callsign.String, "reason": reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "player.callsign_reset", fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		admin, err := store.GetPlayer(ctx, tx, adminID)
		if err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}

// AdminSetMod makes a player a moderator or takes the role away. It takes effect on their next call.
func (s *Service) AdminSetMod(ctx context.Context, adminID int64, code string, isMod bool) (*PlayerInfo, error) {
	var info *PlayerInfo
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		targetID, err := staffTargetByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		target, err := store.GetPlayerForUpdate(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if target.IsMod != isMod {
			if err := store.SetMod(ctx, tx, targetID, isMod); err != nil {
				return err
			}
			detail, _ := json.Marshal(map[string]any{"isMod": isMod})
			if err := store.LogAdminAction(ctx, tx, adminID, "player.mod", fmt.Sprint(targetID), string(detail)); err != nil {
				return err
			}
		}
		admin, err := store.GetPlayer(ctx, tx, adminID)
		if err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}

// NoteInfringement stores a failed location check when the error is one: a fix too poor to act on,
// or one the player could not have reached. Anything else is ignored. It never fails the request.
func (s *Service) NoteInfringement(ctx context.Context, playerID int64, e *apperr.Error, path string) {
	if e == nil || playerID == 0 {
		return
	}
	var kind, detail string
	switch e.Code {
	case apperr.HarvestGPSWeak.Code:
		kind = game.InfringementGPSWeak
		detail = fmt.Sprintf("accuracy %v m, limit %v m", e.Data["accuracyM"], e.Data["maxAccuracyM"])
	case apperr.HarvestImplausible.Code:
		kind, detail = game.InfringementTravelSpeed, e.Detail
	case apperr.FixStale.Code:
		kind, detail = game.InfringementStaleFix, "fix too old or from the future"
	default:
		return
	}
	gap := time.Duration(s.Settings(ctx).InfringementGapMinutes) * time.Minute
	// The request's context may already be done; the record is small and must not be lost with it.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := store.InsertInfringement(writeCtx, s.db, playerID, kind, detail, path, gap); err != nil {
		slogError("infringement not stored", err)
	}
}

// GameStatsView is the game's totals for admins and for the share-stats card: counts only.
type GameStatsView struct {
	store.GameStats
	RetentionPct int       `json:"retentionPct"`
	AsOf         time.Time `json:"asOf"`
}

func (s *Service) AdminStats(ctx context.Context) (GameStatsView, error) {
	now := s.now()
	stats, err := store.LoadGameStats(ctx, s.db, now)
	if err != nil {
		return GameStatsView{}, err
	}
	return GameStatsView{GameStats: stats, RetentionPct: game.RetentionPct(stats.Retained, stats.RetainedOf), AsOf: now.UTC()}, nil
}

// AdminReleasePledge returns a member to freelancer: the fix for a mistaken pledge, since a pledge
// can't be undone by the player (freelancer-path-plan.md C5). A reason is required and it is logged.
// A Regional Commander's seat is left empty.
func (s *Service) AdminReleasePledge(ctx context.Context, adminID int64, code, reason string) (*PlayerInfo, error) {
	reason = strings.TrimSpace(reason)
	if game.CheckSanction(s.Settings(ctx), game.SanctionWarn, 0, reason) != game.SanctionOK {
		return nil, apperr.AdjustBad.Say("Give a reason (3 to 500 characters).")
	}
	var info *PlayerInfo
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		targetID, err := staffTargetByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, targetID); err != nil {
			return err
		}
		faction, err := store.ReleaseMember(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if faction == 0 {
			return apperr.AdjustBad.Say("This player has not pledged to a faction.")
		}
		if err := releaseDeviceMarks(ctx, tx, s, targetID); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"faction": faction, "reason": reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "player.release_pledge", fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		admin, err := store.GetPlayer(ctx, tx, adminID)
		if err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}
