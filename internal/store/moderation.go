package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Reports and moderation (docs/profile-plan.md, M1).

// HasOpenReport says whether the reporter already has an open report on the target.
func HasOpenReport(ctx context.Context, q db.Querier, reporterID, targetID int64) (bool, error) {
	var open bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.player_reports
		               WHERE reporter_id = $1 AND target_id = $2 AND status = 'open')`, reporterID, targetID).Scan(&open)
	return open, err
}

// ReportsSince counts the reports a player made since a time (the daily limit).
func ReportsSince(ctx context.Context, q db.Querier, reporterID int64, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM td.player_reports WHERE reporter_id = $1 AND created_at >= $2`, reporterID, since).Scan(&n)
	return n, err
}

// Interactions is what one player did to another's Tether Nodes and nodes in a stretch of time.
type Interactions struct {
	Attacks      int `json:"attacks"`      // weapon hits on the other's Tether Nodes
	DroneStrikes int `json:"droneStrikes"` // drones launched at the other's Tether Nodes
	Intercepts   int `json:"intercepts"`   // the other's drones this player intercepted
	Contests     int `json:"contests"`     // node contests this player started against the other
}

// InteractionsBetween counts what `from` did to `to` since a time.
func InteractionsBetween(ctx context.Context, q db.Querier, from, to int64, since time.Time) (Interactions, error) {
	var i Interactions
	err := q.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM td.tether_attacks a JOIN td.tether_nodes t ON t.id = a.tn_id
		         WHERE a.attacker_id = $1 AND t.owner_id = $2 AND a.weapon <> 'drone' AND a.created_at >= $3),
		       (SELECT count(*) FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		         WHERE d.launched_by = $1 AND t.owner_id = $2 AND d.created_at >= $3),
		       (SELECT count(*) FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		         WHERE d.launched_by = $2 AND t.owner_id = $1 AND d.intercepted AND d.created_at >= $3),
		       (SELECT count(*) FROM td.node_contests c
		         WHERE c.challenger_id = $1 AND c.defender_id = $2 AND c.created_at >= $3)`,
		from, to, since).Scan(&i.Attacks, &i.DroneStrikes, &i.Intercepts, &i.Contests)
	return i, err
}

// InsertReport files a report and returns its id.
func InsertReport(ctx context.Context, q db.Querier, reporterID, targetID int64, reason, detail, note string, context any) (int64, error) {
	encoded, err := json.Marshal(context)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `
		INSERT INTO td.player_reports (reporter_id, target_id, reason, detail, note, context)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb) RETURNING id`,
		reporterID, targetID, reason, detail, note, string(encoded)).Scan(&id)
	return id, err
}

// Report is one report as a moderator sees it: no reporter name, only what they said.
type Report struct {
	ID        int64           `json:"id"`
	TargetID  int64           `json:"-"`
	Reporter  int64           `json:"-"`
	Reason    string          `json:"reason"`
	Detail    string          `json:"detail"`
	Note      string          `json:"note"`
	Context   json.RawMessage `json:"context"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
}

// OpenReports lists every open report, oldest first.
func OpenReports(ctx context.Context, q db.Querier, limit int) ([]Report, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, target_id, reporter_id, reason, detail, note, context, status, created_at
		FROM td.player_reports WHERE status = 'open' ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		var r Report
		var raw []byte
		if err := rows.Scan(&r.ID, &r.TargetID, &r.Reporter, &r.Reason, &r.Detail, &r.Note, &raw, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Context = json.RawMessage(raw)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReportForUpdate reads one report and locks it.
func ReportForUpdate(ctx context.Context, q db.Querier, id int64) (Report, error) {
	var r Report
	var raw []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, target_id, reporter_id, reason, detail, note, context, status, created_at
		FROM td.player_reports WHERE id = $1 FOR UPDATE`, id).
		Scan(&r.ID, &r.TargetID, &r.Reporter, &r.Reason, &r.Detail, &r.Note, &raw, &r.Status, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	r.Context = json.RawMessage(raw)
	return r, err
}

// CloseReports marks open reports on one player handled (dismissed or actioned) and returns who
// made them, so each reporter can be told it was reviewed. ids empty means every open report on
// the player. sanctionID is 0 for a dismissal.
func CloseReports(ctx context.Context, q db.Querier, targetID int64, ids []int64, status string, handledBy, sanctionID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		UPDATE td.player_reports
		SET status = $3, handled_by = $4, handled_at = now(), sanction_id = NULLIF($5::bigint, 0)
		WHERE target_id = $1 AND status = 'open' AND (cardinality($2::bigint[]) = 0 OR id = ANY($2::bigint[]))
		RETURNING reporter_id`, targetID, int64Array(ids), status, handledBy, sanctionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reporters := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		reporters = append(reporters, id)
	}
	return reporters, rows.Err()
}

// ReopenReport puts a dismissed report back in the queue. False when it is not dismissed.
func ReopenReport(ctx context.Context, q db.Querier, id int64) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.player_reports SET status = 'open', handled_by = NULL, handled_at = NULL
		WHERE id = $1 AND status = 'dismissed'
		  AND NOT EXISTS (SELECT 1 FROM td.player_reports o
		                  WHERE o.reporter_id = td.player_reports.reporter_id
		                    AND o.target_id = td.player_reports.target_id AND o.status = 'open')`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// int64Array writes ids as a Postgres array literal ("{1,2,3}"), which every driver accepts.
func int64Array(ids []int64) string {
	out := []byte{'{'}
	for i, id := range ids {
		if i > 0 {
			out = append(out, ',')
		}
		out = appendInt(out, id)
	}
	return string(append(out, '}'))
}

func appendInt(b []byte, v int64) []byte {
	if v == 0 {
		return append(b, '0')
	}
	if v < 0 {
		b = append(b, '-')
		v = -v
	}
	var digits [20]byte
	n := len(digits)
	for v > 0 {
		n--
		digits[n] = byte('0' + v%10)
		v /= 10
	}
	return append(b, digits[n:]...)
}

// Sanction is one moderation action on a player. By and LiftedBy are callsigns.
type Sanction struct {
	ID         int64      `json:"id"`
	PlayerID   int64      `json:"-"`
	Player     PlayerRef  `json:"player"`
	Kind       string     `json:"kind"`
	Reason     string     `json:"reason"`
	Until      *time.Time `json:"until,omitempty"`
	By         string     `json:"by"`
	CreatedAt  time.Time  `json:"createdAt"`
	LiftedAt   *time.Time `json:"liftedAt,omitempty"`
	LiftedBy   string     `json:"liftedBy,omitempty"`
	LiftReason string     `json:"liftReason,omitempty"`
	// Active: a suspension still running or a ban, not lifted. Filled when listed.
	Active bool `json:"active"`
}

// The player is named by callsign even when banned (staff need to know who): not td.player_name.
const sanctionColumns = `
	s.id, s.player_id, p.public_code, COALESCE(p.callsign, 'OPERATIVE-' || p.public_code), s.kind, s.reason, s.until,
	COALESCE(b.callsign, 'OPERATIVE-' || b.public_code, ''), s.created_at, s.lifted_at,
	COALESCE(l.callsign, 'OPERATIVE-' || l.public_code, ''), COALESCE(s.lift_reason, '')`

const sanctionJoins = `
	FROM td.player_sanctions s
	JOIN td.players p ON p.id = s.player_id
	LEFT JOIN td.players b ON b.id = s.by_id
	LEFT JOIN td.players l ON l.id = s.lifted_by`

func scanSanction(row interface{ Scan(...any) error }, now time.Time) (Sanction, error) {
	var s Sanction
	var until, lifted sql.NullTime
	if err := row.Scan(&s.ID, &s.PlayerID, &s.Player.Code, &s.Player.Name, &s.Kind, &s.Reason, &until, &s.By,
		&s.CreatedAt, &lifted, &s.LiftedBy, &s.LiftReason); err != nil {
		return s, err
	}
	s.Until, s.LiftedAt = timePtr(until), timePtr(lifted)
	s.Active = s.LiftedAt == nil && (s.Kind == "ban" || (s.Kind == "suspend" && s.Until != nil && s.Until.After(now)))
	return s, nil
}

// InsertSanction records an action and returns its id.
func InsertSanction(ctx context.Context, q db.Querier, playerID int64, kind, reason string, until *time.Time, byID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.player_sanctions (player_id, kind, reason, until, by_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		playerID, kind, reason, until, byID).Scan(&id)
	return id, err
}

// SanctionsOf lists a player's sanctions, newest first.
func SanctionsOf(ctx context.Context, q db.Querier, playerID int64, now time.Time) ([]Sanction, error) {
	return listSanctions(ctx, q, now, `WHERE s.player_id = $1 ORDER BY s.id DESC LIMIT 50`, playerID)
}

// AllSanctions lists every sanction, active ones first, then newest first.
func AllSanctions(ctx context.Context, q db.Querier, now time.Time, limit int) ([]Sanction, error) {
	return listSanctions(ctx, q, now, `
		ORDER BY (s.lifted_at IS NULL AND (s.kind = 'ban' OR (s.kind = 'suspend' AND s.until > now()))) DESC, s.id DESC
		LIMIT $1`, limit)
}

func listSanctions(ctx context.Context, q db.Querier, now time.Time, tail string, arg any) ([]Sanction, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+sanctionColumns+sanctionJoins+` `+tail, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sanction{}
	for rows.Next() {
		s, err := scanSanction(rows, now)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SanctionForUpdate reads one sanction and locks it.
func SanctionForUpdate(ctx context.Context, q db.Querier, id int64, now time.Time) (Sanction, error) {
	// Lock the row first: FOR UPDATE can't be used across the outer joins.
	var found int64
	err := q.QueryRowContext(ctx, `SELECT id FROM td.player_sanctions WHERE id = $1 FOR UPDATE`, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return Sanction{}, ErrNotFound
	}
	if err != nil {
		return Sanction{}, err
	}
	return scanSanction(q.QueryRowContext(ctx, `SELECT `+sanctionColumns+sanctionJoins+` WHERE s.id = $1`, id), now)
}

// LiftSanction ends a sanction early; the row stays as history.
func LiftSanction(ctx context.Context, q db.Querier, id, adminID int64, reason string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.player_sanctions SET lifted_at = now(), lifted_by = $2, lift_reason = $3 WHERE id = $1`,
		id, adminID, reason)
	return err
}

// SetAccountStatus sets a player's status and, for a suspension, when it ends (nil clears it).
func SetAccountStatus(ctx context.Context, q db.Querier, playerID int64, status string, suspendedUntil *time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.players SET status = $2::td.account_status, suspended_until = $3, updated_at = now() WHERE id = $1`,
		playerID, status, suspendedUntil)
	return err
}

// SuspendedUntil is when a suspended player may sign in again (zero when not set).
func SuspendedUntil(ctx context.Context, q db.Querier, playerID int64) (time.Time, error) {
	var until sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT suspended_until FROM td.players WHERE id = $1`, playerID).Scan(&until)
	return until.Time, err
}

// EndExpiredSuspensions returns every suspension that has run out to active, and says how many.
func EndExpiredSuspensions(ctx context.Context, q db.Querier) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.players SET status = 'active', suspended_until = NULL, updated_at = now()
		WHERE status = 'suspended' AND (suspended_until IS NULL OR suspended_until <= now())`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StrongestActiveSanction is the status a player's remaining sanctions call for after one is
// lifted: "banned", "suspended" (with its end) or "active".
func StrongestActiveSanction(ctx context.Context, q db.Querier, playerID int64) (string, *time.Time, error) {
	var banned bool
	var until sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(bool_or(kind = 'ban'), false), max(until) FILTER (WHERE kind = 'suspend' AND until > now())
		FROM td.player_sanctions WHERE player_id = $1 AND lifted_at IS NULL`, playerID).Scan(&banned, &until)
	switch {
	case err != nil:
		return "", nil, err
	case banned:
		return "banned", nil, nil
	case until.Valid:
		return "suspended", &until.Time, nil
	}
	return "active", nil, nil
}

// InsertSystemMessage sends a player a written message from the game itself (sender), not a power.
func InsertSystemMessage(ctx context.Context, q db.Querier, playerID int64, sender, kind, body string, at time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_messages (player_id, sender, kind, body, due_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, kind) DO NOTHING`, playerID, sender, kind, body, at)
	return err
}

// StaffOf is a player's roles and status, for the checks on who may act on whom.
type StaffOf struct {
	ID      int64
	IsAdmin bool
	IsMod   bool
	Status  string
	Level   int
	Ref     PlayerRef
}

// StaffForUpdate reads a player's roles and locks the row. The name is the callsign even when the
// account is banned, since staff need to know who it is.
func StaffForUpdate(ctx context.Context, q db.Querier, playerID int64) (StaffOf, error) {
	var s StaffOf
	err := q.QueryRowContext(ctx, `
		SELECT id, is_admin, is_mod, status, level, public_code, COALESCE(callsign, 'OPERATIVE-' || public_code)
		FROM td.players WHERE id = $1 FOR UPDATE`, playerID).
		Scan(&s.ID, &s.IsAdmin, &s.IsMod, &s.Status, &s.Level, &s.Ref.Code, &s.Ref.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// PlayerIDByAnyCode finds a player by share code whatever their status: staff act on banned
// accounts too (to lift a ban).
func PlayerIDByAnyCode(ctx context.Context, q db.Querier, code string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx,
		`SELECT id FROM td.players WHERE public_code = $1 AND status <> 'deleted' AND type <> 'system'`, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// DismissedReport is a report a moderator closed with no action, for an admin to look at again.
type DismissedReport struct {
	ID        int64     `json:"id"`
	Player    PlayerRef `json:"player"`
	Detail    string    `json:"detail"`
	Note      string    `json:"note"`
	HandledBy string    `json:"handledBy"`
	HandledAt time.Time `json:"handledAt"`
}

// DismissedReports lists the latest dismissed reports, newest first.
func DismissedReports(ctx context.Context, q db.Querier, limit int) ([]DismissedReport, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.id, p.public_code, COALESCE(p.callsign, 'OPERATIVE-' || p.public_code), r.detail, r.note,
		       COALESCE(h.callsign, 'OPERATIVE-' || h.public_code, ''), r.handled_at
		FROM td.player_reports r
		JOIN td.players p ON p.id = r.target_id
		LEFT JOIN td.players h ON h.id = r.handled_by
		WHERE r.status = 'dismissed' ORDER BY r.handled_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DismissedReport{}
	for rows.Next() {
		var d DismissedReport
		if err := rows.Scan(&d.ID, &d.Player.Code, &d.Player.Name, &d.Detail, &d.Note, &d.HandledBy, &d.HandledAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
