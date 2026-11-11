package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Anti GPS spoofing (docs/social-signal-plan.md Part C).

// LastAcceptedFix is the last fix the server accepted from a player (nil when none).
func LastAcceptedFix(ctx context.Context, q db.Querier, playerID int64) (*FixRow, error) {
	var f FixRow
	err := q.QueryRowContext(ctx, `
		SELECT lat, lon, accuracy_m, fixed_at FROM td.player_last_fix WHERE player_id = $1`, playerID).
		Scan(&f.Lat, &f.Lon, &f.AccuracyM, &f.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &f, err
}

// FixRow is a stored position.
type FixRow struct {
	Lat, Lon, AccuracyM float64
	At                  time.Time
}

func SaveLastFix(ctx context.Context, q db.Querier, playerID int64, f FixRow) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_last_fix (player_id, lat, lon, accuracy_m, fixed_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id) DO UPDATE SET lat = $2, lon = $3, accuracy_m = $4, fixed_at = $5`,
		playerID, f.Lat, f.Lon, f.AccuracyM, f.At)
	return err
}

// SpoofState is what the fix check needs about a player: whether they are exempt and the lock they are under.
type SpoofState struct {
	Exempt      bool
	LockedUntil *time.Time
}

func SpoofStateOf(ctx context.Context, q db.Querier, playerID int64, now time.Time) (SpoofState, error) {
	var st SpoofState
	var until sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT p.spoof_exempt,
		       (SELECT max(s.until) FROM td.player_sanctions s
		        WHERE s.player_id = p.id AND s.kind = 'spoof_lock' AND s.lifted_at IS NULL AND s.until > $2)
		FROM td.players p WHERE p.id = $1`, playerID, now).Scan(&st.Exempt, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ErrNotFound
	}
	st.LockedUntil = timePtr(until)
	return st, err
}

// CountedIncidents is how many hard incidents of a player count toward the step-up since a time
// (dismissed ones don't).
func CountedIncidents(ctx context.Context, q db.Querier, playerID int64, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.infringements
		WHERE player_id = $1 AND hard AND NOT dismissed AND created_at >= $2`, playerID, since).Scan(&n)
	return n, err
}

// InsertIncident stores a hard (or, for an exempt player, noted) spoofing signal and returns its id.
// evidence holds the positions and is shown to admins only.
func InsertIncident(ctx context.Context, q db.Querier, playerID int64, kind, detail, path string, hard bool, evidence any) (int64, error) {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `
		INSERT INTO td.infringements (player_id, kind, detail, path, hard, evidence)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb) RETURNING id`,
		playerID, kind, detail, path, hard, string(encoded)).Scan(&id)
	return id, err
}

// DismissIncidents takes incidents out of the step-up count (a mod called them false alarms).
func DismissIncidents(ctx context.Context, q db.Querier, playerID int64, ids []int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.infringements SET dismissed = true WHERE player_id = $1 AND id = ANY($2::bigint[])`,
		playerID, int64Array(ids))
	return err
}

// LiftSpoofLocks ends every running signal lock on a player; the rows stay as history.
func LiftSpoofLocks(ctx context.Context, q db.Querier, playerID, byID int64, reason string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.player_sanctions SET lifted_at = now(), lifted_by = $2, lift_reason = $3
		WHERE player_id = $1 AND kind = 'spoof_lock' AND lifted_at IS NULL AND until > now()`,
		playerID, byID, reason)
	return err
}

// SpoofReportContext is what a Signal watch report carries: the incidents it groups and the lock given.
type SpoofReportContext struct {
	Auto      bool          `json:"auto"`
	Incidents []int64       `json:"incidents"`
	Signals   []SpoofSignal `json:"signals"`
}

// SpoofSignal is one incident as moderators read it: never a position.
type SpoofSignal struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Detail      string    `json:"detail"`
	LockMinutes int       `json:"lockMinutes"`
	At          time.Time `json:"at"`
}

// OpenSpoofReport is the open Signal watch report on a player, if any, locked for update.
func OpenSpoofReport(ctx context.Context, q db.Querier, reporterID, targetID int64) (int64, SpoofReportContext, error) {
	var id int64
	var raw []byte
	var c SpoofReportContext
	err := q.QueryRowContext(ctx, `
		SELECT id, context FROM td.player_reports
		WHERE reporter_id = $1 AND target_id = $2 AND status = 'open' AND reason = 'spoof' FOR UPDATE`,
		reporterID, targetID).Scan(&id, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, c, nil
	}
	if err != nil {
		return 0, c, err
	}
	err = json.Unmarshal(raw, &c)
	return id, c, err
}

func UpdateReportContext(ctx context.Context, q db.Querier, id int64, detail string, context any) error {
	encoded, err := json.Marshal(context)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE td.player_reports SET context = $2::jsonb, detail = $3 WHERE id = $1`, id, string(encoded), detail)
	return err
}

// SpoofReportIncidents reads the incident ids a report groups (none for a player's report).
func SpoofReportIncidents(ctx context.Context, q db.Querier, reportID int64) ([]int64, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT context FROM td.player_reports WHERE id = $1 AND reason = 'spoof'`, reportID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c SpoofReportContext
	err = json.Unmarshal(raw, &c)
	return c.Incidents, err
}

// OpenSpoofReportIDs lists a player's open Signal watch reports.
func OpenSpoofReportIDs(ctx context.Context, q db.Querier, targetID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.player_reports WHERE target_id = $1 AND status = 'open' AND reason = 'spoof'`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func SetSpoofExempt(ctx context.Context, q db.Querier, playerID int64, exempt bool) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET spoof_exempt = $2 WHERE id = $1`, playerID, exempt)
	return err
}

// OpenReportCount is how many players have open reports (the number on the ADMIN button).
func OpenReportCount(ctx context.Context, q db.Querier) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(DISTINCT target_id) FROM td.player_reports WHERE status = 'open'`).Scan(&n)
	return n, err
}
