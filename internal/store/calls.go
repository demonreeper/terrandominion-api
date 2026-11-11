package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// The powers' calls and written messages, and the playstyle score that decides which power sends
// them. Each belongs to one faction's power; nothing here joins the six.

// AddPlaystyle adds to a player's score for one power.
func AddPlaystyle(ctx context.Context, q db.Querier, playerID int64, factionID, points int) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_playstyle (player_id, faction_id, points) VALUES ($1, $2, $3)
		ON CONFLICT (player_id, faction_id) DO UPDATE SET points = td.player_playstyle.points + $3`,
		playerID, factionID, points)
	return err
}

// PlaystyleScores reads a player's score per power.
func PlaystyleScores(ctx context.Context, q db.Querier, playerID int64) (map[int]int, error) {
	rows, err := q.QueryContext(ctx, `SELECT faction_id, points FROM td.player_playstyle WHERE player_id = $1`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var faction int
		var points int64
		if err := rows.Scan(&faction, &points); err != nil {
			return nil, err
		}
		out[faction] = int(points)
	}
	return out, rows.Err()
}

// InsertCall records a call; a player gets each kind once. It reports whether this one was new.
func InsertCall(ctx context.Context, q db.Querier, playerID int64, factionID int, kind, line string, dueAt time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.player_calls (player_id, faction_id, kind, line, due_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, kind) DO NOTHING`, playerID, factionID, kind, line, dueAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// InsertMessage records a written message; a player gets each kind once.
func InsertMessage(ctx context.Context, q db.Querier, playerID int64, factionID int, kind, body string, dueAt time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_messages (player_id, faction_id, kind, body, due_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, kind) DO NOTHING`, playerID, factionID, kind, body, dueAt)
	return err
}

// CallerOf is the faction whose power made a call of this kind to the player; 0 when none did.
func CallerOf(ctx context.Context, q db.Querier, playerID int64, kind string) (int, error) {
	var faction int
	err := q.QueryRowContext(ctx, `SELECT faction_id FROM td.player_calls WHERE player_id = $1 AND kind = $2`,
		playerID, kind).Scan(&faction)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return faction, err
}

// CancelOffer drops the repeat call and the written message that have not come due: the player
// reached the faction level first.
func CancelOffer(ctx context.Context, q db.Querier, playerID int64, now time.Time) error {
	if _, err := q.ExecContext(ctx, `
		DELETE FROM td.player_calls WHERE player_id = $1 AND kind LIKE 'level19%' AND due_at > $2`, playerID, now); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		DELETE FROM td.player_messages WHERE player_id = $1 AND kind LIKE 'level19%' AND due_at > $2`, playerID, now)
	return err
}

// Call is a power's call waiting for the player.
type Call struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Persona     string    `json:"persona"`
	FactionCode string    `json:"factionCode"`
	Color       string    `json:"color"`
	Line        string    `json:"line"`
	DueAt       time.Time `json:"dueAt"`
	// Voice and Audio name the app's sound file (Audio/Voices/<voice>/<audio>), and Missed says the
	// call has rung out; the service fills them.
	Voice     string `json:"voice"`
	Audio     string `json:"audio"`
	Missed    bool   `json:"missed"`
	FactionID int    `json:"-"`
}

// DueCalls lists the player's calls that have come due and have not been played, oldest first.
func DueCalls(ctx context.Context, q db.Querier, playerID int64, now time.Time) ([]Call, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.kind, f.persona, f.code, f.color, c.line, c.due_at, c.faction_id
		FROM td.player_calls c JOIN td.factions f ON f.id = c.faction_id
		WHERE c.player_id = $1 AND c.played_at IS NULL AND c.due_at <= $2
		ORDER BY c.due_at, c.id`, playerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Call, 0, 2)
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.ID, &c.Kind, &c.Persona, &c.FactionCode, &c.Color, &c.Line, &c.DueAt, &c.FactionID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkCallsDelivered notes that the app has fetched the player's due calls, so no push follows.
func MarkCallsDelivered(ctx context.Context, q db.Querier, playerID int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.player_calls SET delivered_at = $2
		WHERE player_id = $1 AND delivered_at IS NULL AND played_at IS NULL AND due_at <= $2`, playerID, now)
	return err
}

// MarkCallPlayed ends a call for good. It reports false when the call is not the player's, not due
// or already played.
func MarkCallPlayed(ctx context.Context, q db.Querier, playerID, id int64, now time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.player_calls SET played_at = $3, delivered_at = COALESCE(delivered_at, $3)
		WHERE id = $1 AND player_id = $2 AND played_at IS NULL AND due_at <= $3`, id, playerID, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UnansweredCall is a call the app has not fetched.
type UnansweredCall struct {
	ID, PlayerID int64
	Persona      string
}

// ClaimUnansweredCalls locks the calls that came due before the cutoff and that the app has not
// fetched, to push them.
func ClaimUnansweredCalls(ctx context.Context, q db.Querier, cutoff time.Time, limit int) ([]UnansweredCall, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.player_id, f.persona
		FROM td.player_calls c JOIN td.factions f ON f.id = c.faction_id
		WHERE c.delivered_at IS NULL AND c.pushed_at IS NULL AND c.played_at IS NULL AND c.due_at <= $1
		ORDER BY c.due_at LIMIT $2 FOR UPDATE OF c SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnansweredCall
	for rows.Next() {
		var c UnansweredCall
		if err := rows.Scan(&c.ID, &c.PlayerID, &c.Persona); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func MarkCallPushed(ctx context.Context, q db.Querier, id int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.player_calls SET pushed_at = $2 WHERE id = $1`, id, now)
	return err
}

// Message is one of the powers' written messages.
type Message struct {
	ID          int64     `json:"id"`
	Persona     string    `json:"persona"`
	FactionCode string    `json:"factionCode"`
	Color       string    `json:"color"`
	Body        string    `json:"body"`
	At          time.Time `json:"at"`
	Read        bool      `json:"read"`
}

// Messages lists the player's messages that have arrived, newest first.
func Messages(ctx context.Context, q db.Querier, playerID int64, now time.Time, limit int) ([]Message, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.id, f.persona, f.code, f.color, m.body, m.due_at, m.read_at IS NOT NULL
		FROM td.player_messages m JOIN td.factions f ON f.id = m.faction_id
		WHERE m.player_id = $1 AND m.due_at <= $2
		ORDER BY m.due_at DESC, m.id DESC LIMIT $3`, playerID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Message, 0, 2)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Persona, &m.FactionCode, &m.Color, &m.Body, &m.At, &m.Read); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkMessageRead reports false when the message is not the player's or has not arrived.
func MarkMessageRead(ctx context.Context, q db.Querier, playerID, id int64, now time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.player_messages SET read_at = COALESCE(read_at, $3)
		WHERE id = $1 AND player_id = $2 AND due_at <= $3`, id, playerID, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// InvitedFaction is the faction that invited the player at the faction level; 0 when none did.
func InvitedFaction(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var faction int
	err := q.QueryRowContext(ctx, `SELECT faction_id FROM td.player_invitations WHERE player_id = $1`, playerID).Scan(&faction)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return faction, err
}
