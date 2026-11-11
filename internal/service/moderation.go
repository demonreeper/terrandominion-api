package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Reports and moderation (docs/profile-plan.md, M1). Players report another player for griefing;
// mods warn, suspend or ban; admins lift. A ban or a suspension only stops sign-in: nothing the
// player owns is removed, and their Tether Nodes decay as they always would.

// ReportContext is what the server attaches to a report: what each of the two did to the other in
// the last ReportContextDays.
type ReportContext struct {
	Days             int                `json:"days"`
	TargetOnReporter store.Interactions `json:"targetOnReporter"`
	ReporterOnTarget store.Interactions `json:"reporterOnTarget"`
}

// ReportPlayer files a report on the player behind a share code. Limits: not yourself, one open
// report per reporter per player, ReportsPerDay a day.
func (s *Service) ReportPlayer(ctx context.Context, reporterID int64, code, detail, note string) error {
	settings := s.Settings(ctx)
	note = game.CleanReportNote(note)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		// The reporter's row is locked, so two taps can't both pass the limits.
		if _, err := store.GetPlayerForUpdate(ctx, tx, reporterID); err != nil {
			return err
		}
		targetID, err := playerIDByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		open, err := store.HasOpenReport(ctx, tx, reporterID, targetID)
		if err != nil {
			return err
		}
		now := s.now()
		today, err := store.ReportsSince(ctx, tx, reporterID, now.Add(-24*time.Hour))
		if err != nil {
			return err
		}
		switch game.CheckReport(settings, reporterID, targetID, detail, note, open, today) {
		case game.ReportSelf:
			return apperr.ReportSelf
		case game.ReportBadDetail, game.ReportNoteLong:
			return apperr.ReportBad
		case game.ReportDuplicate:
			return apperr.ReportOpen
		case game.ReportTooMany:
			return apperr.ReportLimit.WithData(map[string]any{"perDay": settings.ReportsPerDay})
		}

		since := now.Add(-time.Duration(settings.ReportContextDays) * 24 * time.Hour)
		reportContext := ReportContext{Days: settings.ReportContextDays}
		if reportContext.TargetOnReporter, err = store.InteractionsBetween(ctx, tx, targetID, reporterID, since); err != nil {
			return err
		}
		if reportContext.ReporterOnTarget, err = store.InteractionsBetween(ctx, tx, reporterID, targetID, since); err != nil {
			return err
		}
		if _, err := store.InsertReport(ctx, tx, reporterID, targetID, game.ReportReasonGriefing, detail, note, reportContext); err != nil {
			if isUniqueViolation(err) {
				return apperr.ReportOpen
			}
			return err
		}
		return nil
	})
}

// ReportedPlayer is one player in the mod queue with everything said about them. Mods see the
// callsign, level and faction: no real name, email, device or location, and not who reported.
type ReportedPlayer struct {
	Player    store.PlayerRef `json:"player"`
	Level     int             `json:"level"`
	Faction   string          `json:"faction"`
	RankTitle string          `json:"rankTitle"`
	// Reporters is how many different players reported them.
	Reporters int              `json:"reporters"`
	Reports   []store.Report   `json:"reports"`
	Sanctions []store.Sanction `json:"sanctions"`
	// Suggested is the next step up from their history: shown, never forced.
	Suggested game.SanctionSuggestion `json:"suggested"`
	// CanAct is false when the asking moderator may not act on this player (staff, or themselves).
	CanAct bool `json:"canAct"`
}

// ModQueue is the report queue and the choices a mod has.
type ModQueue struct {
	Players     []ReportedPlayer `json:"players"`
	SuspendDays []int            `json:"suspendDays"`
	Details     []string         `json:"details"`
}

const modQueueLimit = 500

// ModReports lists open reports grouped by reported player, the most reported first.
func (s *Service) ModReports(ctx context.Context, modID int64) (ModQueue, error) {
	settings := s.Settings(ctx)
	queue := ModQueue{Players: []ReportedPlayer{}, SuspendDays: game.SuspendDayChoices(settings), Details: game.ReportDetails}
	reports, err := store.OpenReports(ctx, s.db, modQueueLimit)
	if err != nil {
		return queue, err
	}
	actor, err := store.GetPlayer(ctx, s.db, modID)
	if err != nil {
		return queue, err
	}
	now := s.now()
	byTarget := map[int64]*ReportedPlayer{}
	reporters := map[int64]map[int64]bool{}
	order := []int64{}
	for _, report := range reports {
		entry, seen := byTarget[report.TargetID]
		if !seen {
			target, err := store.GetPlayer(ctx, s.db, report.TargetID)
			if err != nil {
				return queue, err
			}
			entry = &ReportedPlayer{
				// Staff see the callsign even on a banned account: they need to know who it is.
				Player: store.PlayerRef{Code: target.Code, Name: staffName(target)}, Level: target.Level,
				CanAct: game.CanSanction(staffOf(actor), staffOf(target)),
			}
			membership, err := store.MembershipOf(ctx, s.db, report.TargetID)
			if err != nil {
				return queue, err
			}
			if membership != nil {
				describeMembership(membership)
				entry.Faction, entry.RankTitle = membership.FactionName, membership.RankTitle
			}
			if entry.Sanctions, err = store.SanctionsOf(ctx, s.db, report.TargetID, now); err != nil {
				return queue, err
			}
			entry.Suggested = game.SuggestSanction(settings, pastSanctions(entry.Sanctions), now)
			byTarget[report.TargetID] = entry
			reporters[report.TargetID] = map[int64]bool{}
			order = append(order, report.TargetID)
		}
		entry.Reports = append(entry.Reports, report)
		reporters[report.TargetID][report.Reporter] = true
	}
	for _, id := range order {
		byTarget[id].Reporters = len(reporters[id])
		queue.Players = append(queue.Players, *byTarget[id])
	}
	sort.SliceStable(queue.Players, func(i, j int) bool { return queue.Players[i].Reporters > queue.Players[j].Reporters })
	return queue, nil
}

// staffName is a player as staff see them: the callsign (or placeholder), banned or not.
func staffName(p store.Player) string {
	if p.Callsign.Valid {
		return p.Callsign.String
	}
	return game.PlaceholderCallsign(p.Code)
}

func staffOf(p store.Player) game.Staff {
	return game.Staff{ID: p.ID, IsAdmin: p.IsAdmin, IsMod: p.IsMod}
}

// pastSanctions is a player's history as the suggestion reads it: lifted ones left out.
func pastSanctions(sanctions []store.Sanction) []game.PastSanction {
	past := []game.PastSanction{}
	for _, sanction := range sanctions {
		if sanction.LiftedAt != nil {
			continue
		}
		days := 0
		if sanction.Until != nil {
			days = int(sanction.Until.Sub(sanction.CreatedAt).Hours()/24 + 0.5)
		}
		past = append(past, game.PastSanction{Kind: sanction.Kind, Days: days, At: sanction.CreatedAt})
	}
	return past
}

// DismissReport closes a report with no action. all also closes every other open report on the
// same player. Each reporter is told their report was reviewed.
func (s *Service) DismissReport(ctx context.Context, modID, reportID int64, reason string, all bool) error {
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < game.SanctionReasonMin || n > game.SanctionReasonMax {
		return apperr.SanctionBad
	}
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		report, err := store.ReportForUpdate(ctx, tx, reportID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && report.Status != "open") {
			return apperr.NotFound.With("open report %d", reportID)
		}
		if err != nil {
			return err
		}
		ids := []int64{reportID}
		if all {
			ids = nil
		}
		reporters, err := store.CloseReports(ctx, tx, report.TargetID, ids, "dismissed", modID, 0)
		if err != nil {
			return err
		}
		if err := tellReporters(ctx, tx, reporters); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"report": reportID, "all": all, "reason": reason, "closed": len(reporters)})
		return store.LogAdminAction(ctx, tx, modID, "report.dismiss", fmt.Sprint(report.TargetID), string(detail))
	})
}

// tellReporters sends each reporter one "Your report was reviewed." notice.
func tellReporters(ctx context.Context, tx *sql.Tx, reporters []int64) error {
	told := map[int64]bool{}
	for _, reporter := range reporters {
		if told[reporter] {
			continue
		}
		told[reporter] = true
		if err := notify(ctx, tx, store.NewNotice{PlayerID: reporter, Kind: "report_reviewed", Body: game.NoticeReportReviewed()}); err != nil {
			return err
		}
	}
	return nil
}

// SanctionRequest is one action on a player.
type SanctionRequest struct {
	Kind      string
	Days      int
	Reason    string
	ReportIDs []int64
}

// Sanction warns, suspends or bans the player behind a share code. A reason is required; a
// suspension takes one of the three allowed lengths; a mod can't act on staff or themselves. A
// suspension or ban signs the player out everywhere at once. The reports named are closed as
// actioned and their reporters told.
func (s *Service) Sanction(ctx context.Context, actorID int64, code string, req SanctionRequest) (store.Sanction, error) {
	settings := s.Settings(ctx)
	req.Reason = strings.TrimSpace(req.Reason)
	switch game.CheckSanction(settings, req.Kind, req.Days, req.Reason) {
	case game.SanctionOK:
	default:
		return store.Sanction{}, apperr.SanctionBad.WithData(map[string]any{"days": game.SuspendDayChoices(settings)})
	}

	var out store.Sanction
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		targetID, err := staffTargetByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		// Both rows locked lowest id first, like every action between two players.
		if err := lockPlayerPair(ctx, tx, actorID, targetID); err != nil {
			return err
		}
		actor, err := store.StaffForUpdate(ctx, tx, actorID)
		if err != nil {
			return err
		}
		target, err := store.StaffForUpdate(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if !game.CanSanction(game.Staff{ID: actor.ID, IsAdmin: actor.IsAdmin, IsMod: actor.IsMod},
			game.Staff{ID: target.ID, IsAdmin: target.IsAdmin, IsMod: target.IsMod}) {
			return apperr.SanctionNotYours
		}

		now := s.now()
		var until *time.Time
		switch req.Kind {
		case game.SanctionSuspend:
			end := now.Add(time.Duration(req.Days) * 24 * time.Hour)
			until = &end
		}
		id, err := store.InsertSanction(ctx, tx, targetID, req.Kind, req.Reason, until, actorID)
		if err != nil {
			return err
		}

		switch req.Kind {
		case game.SanctionWarn:
			if err := store.InsertSystemMessage(ctx, tx, targetID, game.ModerationSender,
				fmt.Sprintf("mod_warning_%d", id), game.ModerationWarning(req.Reason), now); err != nil {
				return err
			}
		case game.SanctionSuspend, game.SanctionBan:
			// A ban outranks a suspension; a longer suspension already running is kept.
			status, end, err := store.StrongestActiveSanction(ctx, tx, targetID)
			if err != nil {
				return err
			}
			if err := store.SetAccountStatus(ctx, tx, targetID, status, end); err != nil {
				return err
			}
			if err := store.DeletePlayerPushTokens(ctx, tx, targetID); err != nil {
				return err
			}
			// A ban also bans the player's phones and sign-ins (freelancer-path-plan.md phase 2).
			if req.Kind == game.SanctionBan {
				if err := s.markBan(ctx, tx, targetID, id); err != nil {
					return err
				}
			}
			if err := store.RevokeAllSessions(ctx, tx, targetID); err != nil {
				return err
			}
		}

		reporters, err := store.CloseReports(ctx, tx, targetID, req.ReportIDs, "actioned", actorID, id)
		if err != nil {
			return err
		}
		if err := tellReporters(ctx, tx, reporters); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"sanction": id, "kind": req.Kind, "days": req.Days, "reason": req.Reason, "reports": len(reporters)})
		if err := store.LogAdminAction(ctx, tx, actorID, "sanction."+req.Kind, fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		out, err = store.SanctionForUpdate(ctx, tx, id, now)
		return err
	})
	return out, err
}

// staffTargetByCode finds the player staff are acting on, whatever their status (a banned account
// can still be found, to lift the ban). Unknown or deleted: NotFound.
func staffTargetByCode(ctx context.Context, q db.Querier, code string) (int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !game.ValidPublicCode(code) {
		return 0, apperr.NotFound.With("player code %q", code)
	}
	id, err := store.PlayerIDByAnyCode(ctx, q, code)
	if errors.Is(err, store.ErrNotFound) {
		return 0, apperr.NotFound.With("player code %q", code)
	}
	return id, err
}

const sanctionsListed = 200

// AdminModeration is what only admins see of moderation: every sanction (active first), to lift,
// and the reports mods dismissed, to reopen.
type AdminModeration struct {
	Sanctions []store.Sanction        `json:"sanctions"`
	Dismissed []store.DismissedReport `json:"dismissed"`
}

func (s *Service) AdminSanctions(ctx context.Context) (AdminModeration, error) {
	var out AdminModeration
	var err error
	if out.Sanctions, err = store.AllSanctions(ctx, s.db, s.now(), sanctionsListed); err != nil {
		return out, err
	}
	out.Dismissed, err = store.DismissedReports(ctx, s.db, 50)
	return out, err
}

// LiftSanction ends a suspension or a ban early (admins only). A reason is required; the player
// can sign in again at once, unless another sanction still holds; the history keeps both entries.
func (s *Service) LiftSanction(ctx context.Context, adminID, sanctionID int64, reason string) (store.Sanction, error) {
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < game.SanctionReasonMin || n > game.SanctionReasonMax {
		return store.Sanction{}, apperr.SanctionBad
	}
	var out store.Sanction
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		now := s.now()
		sanction, err := store.SanctionForUpdate(ctx, tx, sanctionID, now)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("sanction %d", sanctionID)
		}
		if err != nil {
			return err
		}
		if !sanction.Active {
			return apperr.SanctionNotLive
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, sanction.PlayerID); err != nil {
			return err
		}
		if err := store.LiftSanction(ctx, tx, sanctionID, adminID, reason); err != nil {
			return err
		}
		// Lifting a ban frees the phones and sign-ins it marked.
		if sanction.Kind == game.SanctionBan {
			if err := store.LiftBanMarks(ctx, tx, sanctionID); err != nil {
				return err
			}
		}
		status, end, err := store.StrongestActiveSanction(ctx, tx, sanction.PlayerID)
		if err != nil {
			return err
		}
		if err := store.SetAccountStatus(ctx, tx, sanction.PlayerID, status, end); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"sanction": sanctionID, "kind": sanction.Kind, "reason": reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "sanction.lift", fmt.Sprint(sanction.PlayerID), string(detail)); err != nil {
			return err
		}
		out, err = store.SanctionForUpdate(ctx, tx, sanctionID, now)
		return err
	})
	return out, err
}

// ReopenReport puts a dismissed report back in the queue (admins only).
func (s *Service) ReopenReport(ctx context.Context, adminID, reportID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		reopened, err := store.ReopenReport(ctx, tx, reportID)
		if err != nil {
			return err
		}
		if !reopened {
			return apperr.ReportNotDismissed
		}
		return store.LogAdminAction(ctx, tx, adminID, "report.reopen", fmt.Sprint(reportID), "{}")
	})
}

// IsStaff says whether the player is a moderator or an admin right now (read from the database,
// so a removed role stops working at once).
func (s *Service) IsStaff(ctx context.Context, playerID int64) (bool, error) {
	player, err := store.GetPlayer(ctx, s.db, playerID)
	if err != nil {
		return false, err
	}
	return (player.IsAdmin || player.IsMod) && player.Status == "active", nil
}

// endSuspensions returns suspensions that have run out to active (the tick calls it).
func (s *Service) endSuspensions(ctx context.Context) {
	if _, err := store.EndExpiredSuspensions(ctx, s.db); err != nil && ctx.Err() == nil {
		slogError("suspension expiry failed", err)
	}
}

// checkSuspension refuses a suspended player, or returns them to active when the suspension has
// run out. q must be the transaction holding the player's row.
func (s *Service) checkSuspension(ctx context.Context, q db.Querier, player store.Player) error {
	if player.Status != "suspended" {
		return nil
	}
	until, err := store.SuspendedUntil(ctx, q, player.ID)
	if err != nil {
		return err
	}
	if !until.IsZero() && until.After(s.now()) {
		return apperr.AccountSuspended.
			Say("Suspended until %s.", until.UTC().Format("2 Jan 2006 15:04 UTC")).
			WithData(map[string]any{"until": until.UTC()})
	}
	return store.SetAccountStatus(ctx, q, player.ID, "active", nil)
}

func slogError(message string, err error) {
	slog.Error("E1900 "+message, "err", err)
}
