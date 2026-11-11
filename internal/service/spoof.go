package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Anti GPS spoofing, G0 (docs/social-signal-plan.md Part C). Every request that acts on a place passes
// GuardFix first (httpapi wraps those routes): a signal lock refuses it; an old or future fix is refused
// and noted; a fake-location flag or a teleport refuses it, starts a lock that steps up on repeat, and
// files a report from Signal watch. Otherwise the fix becomes the player's last fix.

// GuardFix checks a request's fix before the action runs. path is the route, for the record.
func (s *Service) GuardFix(ctx context.Context, playerID int64, fix game.FixReport, path string) error {
	settings := s.Settings(ctx)
	now := s.now()
	state, err := store.SpoofStateOf(ctx, s.db, playerID, now)
	if err != nil {
		return err
	}
	if state.LockedUntil != nil && !state.Exempt {
		return s.lockedError(ctx, playerID, *state.LockedUntil, settings, now)
	}
	if !fix.HasPosition() {
		return nil
	}
	if game.FixStale(settings, fix.FixAt, now) {
		return apperr.FixStale
	}
	// A fix without an accuracy can't be measured against the last one; it is checked for the lock only.
	if fix.AccuracyM <= 0 {
		return nil
	}
	current := game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now}
	last, err := store.LastAcceptedFix(ctx, s.db, playerID)
	if err != nil {
		return err
	}
	var lastFix *game.Fix
	if last != nil {
		lastFix = &game.Fix{Lat: last.Lat, Lon: last.Lon, AccuracyM: last.AccuracyM, At: last.At}
	}

	incident := game.SpoofIncident{}
	if fix.Mock && settings.SpoofCheckMockFlag == 1 {
		incident.Kind = game.InfringementMock
	} else if jump, teleport := game.TeleportJump(settings, lastFix, current); teleport {
		incident = game.SpoofIncident{Kind: game.InfringementTeleport, Km: jump.Km, Kmh: jump.Kmh, Seconds: jump.Seconds}
	}
	if incident.Kind == "" {
		accepted := store.FixRow{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now}
		if !state.Exempt {
			s.notePattern(ctx, playerID, accepted, settings, path)
		}
		return store.SaveLastFix(ctx, s.db, playerID, accepted)
	}
	evidence := map[string]any{"to": current, "jump": incident}
	if lastFix != nil {
		evidence["from"] = lastFix
	}
	if state.Exempt {
		// A tester: noted, never locked or reported, and the fix counts.
		if _, err := store.InsertIncident(ctx, s.db, playerID, incident.Kind, "exempt: "+game.SpoofDetail(incident), path, false, evidence); err != nil {
			return err
		}
		return store.SaveLastFix(ctx, s.db, playerID, store.FixRow{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now})
	}
	return s.spoofIncident(ctx, playerID, incident, evidence, path, settings, now)
}

// spoofIncident records a hard signal, gives the lock (unless locks are off or one is running) and files
// or extends the Signal watch report. It returns the error the request gets.
func (s *Service) spoofIncident(ctx context.Context, playerID int64, incident game.SpoofIncident, evidence any,
	path string, settings game.Settings, now time.Time) error {
	window := now.Add(-time.Duration(settings.SpoofRepeatDays) * 24 * time.Hour)
	var refusal error
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		earlier, err := store.CountedIncidents(ctx, tx, playerID, window)
		if err != nil {
			return err
		}
		detail := game.SpoofDetail(incident)
		id, err := store.InsertIncident(ctx, tx, playerID, incident.Kind, detail, path, true, evidence)
		if err != nil {
			return err
		}
		state, err := store.SpoofStateOf(ctx, tx, playerID, now)
		if err != nil {
			return err
		}
		minutes := 0
		switch {
		case settings.SpoofLockEnabled != 1:
			refusal = apperr.HarvestImplausible.Say("That position can't be right.")
		case state.LockedUntil != nil:
			// Locks stack with nothing: the incident is recorded, the lock keeps its end.
			refusal = lockError(*state.LockedUntil, game.LockLine(earlier), now)
		default:
			minutes = game.LockMinutes(settings, earlier)
			until := now.Add(time.Duration(minutes) * time.Minute)
			if _, err := store.InsertSanction(ctx, tx, playerID, game.SanctionSpoofLock,
				"Signal lock: "+detail, &until, game.SignalWatchID); err != nil {
				return err
			}
			refusal = lockError(until, game.LockLine(earlier), now)
		}
		signal := store.SpoofSignal{ID: id, Kind: incident.Kind, Detail: detail, LockMinutes: minutes, At: now}
		reportID, context, err := store.OpenSpoofReport(ctx, tx, game.SignalWatchID, playerID)
		if err != nil {
			return err
		}
		if reportID == 0 {
			_, err = store.InsertReport(ctx, tx, game.SignalWatchID, playerID, game.ReportReasonSpoof, incident.Kind, "",
				store.SpoofReportContext{Auto: true, Incidents: []int64{id}, Signals: []store.SpoofSignal{signal}})
			return err
		}
		context.Auto = true
		context.Incidents = append(context.Incidents, id)
		context.Signals = append(context.Signals, signal)
		return store.UpdateReportContext(ctx, tx, reportID, incident.Kind, context)
	})
	if err != nil {
		return err
	}
	// Walk together L1/L2: a fake flag or a lock on any member ends the link for everyone.
	if err := s.endWalkFor(ctx, s.db, playerID, game.WalkEndLocked); err != nil {
		return err
	}
	return refusal
}

func lockError(until time.Time, line string, now time.Time) *apperr.Error {
	left := int(until.Sub(now).Minutes() + 0.999)
	return apperr.SignalLocked.Say("Signal rejected. Back in %d min.", left).
		WithData(map[string]any{"until": until.UTC(), "incident": line})
}

func (s *Service) lockedError(ctx context.Context, playerID int64, until time.Time, settings game.Settings, now time.Time) error {
	line, err := s.lockLine(ctx, s.db, playerID, settings, now)
	if err != nil {
		return err
	}
	return lockError(until, line, now)
}

// lockLine is the narrator line for the lock a player is under: first, unless an earlier counted incident
// in the window came before the latest one.
func (s *Service) lockLine(ctx context.Context, q db.Querier, playerID int64, settings game.Settings, now time.Time) (string, error) {
	n, err := store.CountedIncidents(ctx, q, playerID, now.Add(-time.Duration(settings.SpoofRepeatDays)*24*time.Hour))
	if err != nil {
		return "", err
	}
	return game.LockLine(n - 1), nil
}

// signalLock fills /v1/me: when the running lock ends and which line it carries.
func (s *Service) signalLock(ctx context.Context, q db.Querier, me *MeView, settings game.Settings) error {
	now := s.now()
	state, err := store.SpoofStateOf(ctx, q, me.ID, now)
	if err != nil || state.LockedUntil == nil || state.Exempt {
		return err
	}
	me.SignalLockedUntil = state.LockedUntil
	me.SignalLockLine, err = s.lockLine(ctx, q, me.ID, settings, now)
	return err
}

// dismissSpoof runs when moderators dismiss Signal watch reports: the incidents in them stop counting and
// any running lock is lifted at once.
func (s *Service) dismissSpoof(ctx context.Context, tx *sql.Tx, modID, targetID int64, reportIDs []int64, reason string) error {
	var incidents []int64
	for _, id := range reportIDs {
		ids, err := store.SpoofReportIncidents(ctx, tx, id)
		if err != nil {
			return err
		}
		incidents = append(incidents, ids...)
	}
	if len(incidents) == 0 {
		return nil
	}
	if err := store.DismissIncidents(ctx, tx, targetID, incidents); err != nil {
		return err
	}
	return store.LiftSpoofLocks(ctx, tx, targetID, modID, "Report dismissed: "+reason)
}

// SetSpoofExempt marks a player as a tester who may use a fake location (admins only, logged).
func (s *Service) SetSpoofExempt(ctx context.Context, adminID int64, code string, exempt bool, reason string) (*PlayerInfo, error) {
	admin, err := store.GetPlayer(ctx, s.db, adminID)
	if err != nil {
		return nil, err
	}
	if !admin.IsAdmin {
		return nil, apperr.Forbidden
	}
	targetID, err := staffTargetByCode(ctx, s.db, code)
	if err != nil {
		return nil, err
	}
	var info *PlayerInfo
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, targetID); err != nil {
			return err
		}
		if err := store.SetSpoofExempt(ctx, tx, targetID, exempt); err != nil {
			return err
		}
		if exempt {
			if err := store.LiftSpoofLocks(ctx, tx, targetID, adminID, "Marked spoof-exempt"); err != nil {
				return err
			}
		}
		detail, _ := json.Marshal(map[string]any{"exempt": exempt, "reason": reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "player.spoof_exempt", fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}

// notePattern adds an accepted fix to the player's trail and, now and then, looks for too-perfect movement (G2,
// D3). A finding is a Signal watch report only: never a lock, and it doesn't count toward the step-up.
func (s *Service) notePattern(ctx context.Context, playerID int64, fix store.FixRow, settings game.Settings, path string) {
	if settings.SpoofPatternEnabled != 1 || settings.SpoofPatternFixes <= 0 {
		return
	}
	if err := store.AppendFixTrail(ctx, s.db, playerID, fix, settings.SpoofPatternFixes*2); err != nil {
		slogError("fix trail not stored", err)
		return
	}
	rows, err := store.FixTrail(ctx, s.db, playerID, settings.SpoofPatternFixes)
	if err != nil || len(rows) < settings.SpoofPatternFixes {
		return
	}
	fixes := make([]game.Fix, len(rows))
	for i, r := range rows {
		fixes[i] = game.Fix{Lat: r.Lat, Lon: r.Lon, AccuracyM: r.AccuracyM, At: r.At}
	}
	finding := game.PatternFinding(fixes, settings.SpoofPatternFixes)
	if finding == "" {
		return
	}
	now := s.now()
	if seen, err := store.RecentIncidentOf(ctx, s.db, playerID, game.InfringementPattern,
		now.Add(-time.Duration(settings.SpoofPatternGapHours)*time.Hour)); err != nil || seen {
		return
	}
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		detail := "movement pattern: " + finding
		id, err := store.InsertIncident(ctx, tx, playerID, game.InfringementPattern, detail, path, false,
			map[string]any{"fixes": len(fixes), "first": fixes[0], "last": fixes[len(fixes)-1]})
		if err != nil {
			return err
		}
		signal := store.SpoofSignal{ID: id, Kind: game.InfringementPattern, Detail: detail, At: now}
		reportID, context, err := store.OpenSpoofReport(ctx, tx, game.SignalWatchID, playerID)
		if err != nil {
			return err
		}
		if reportID == 0 {
			_, err = store.InsertReport(ctx, tx, game.SignalWatchID, playerID, game.ReportReasonSpoof, game.InfringementPattern, "",
				store.SpoofReportContext{Auto: true, Incidents: []int64{id}, Signals: []store.SpoofSignal{signal}})
			return err
		}
		context.Auto = true
		context.Incidents = append(context.Incidents, id)
		context.Signals = append(context.Signals, signal)
		return store.UpdateReportContext(ctx, tx, reportID, context.Signals[0].Kind, context)
	})
	if err != nil {
		slogError("pattern report not filed", err)
	}
}
