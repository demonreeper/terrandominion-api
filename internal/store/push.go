package store

import (
	"context"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// SavePushToken ties a phone to the session that registered it, taking it from any earlier owner.
func SavePushToken(ctx context.Context, q db.Querier, token string, playerID, sessionID int64, platform string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.push_tokens (token, player_id, session_id, platform) VALUES ($1, $2, $3, $4)
		ON CONFLICT (token) DO UPDATE SET player_id = $2, session_id = $3, platform = $4, updated_at = now()`,
		token, playerID, sessionID, platform)
	return err
}

func DeleteSessionPushTokens(ctx context.Context, q db.Querier, sessionID, playerID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.push_tokens WHERE session_id = $1 AND player_id = $2`, sessionID, playerID)
	return err
}

func DeletePlayerPushTokens(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.push_tokens WHERE player_id = $1`, playerID)
	return err
}

func DeletePushToken(ctx context.Context, q db.Querier, token string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.push_tokens WHERE token = $1`, token)
	return err
}

// ActivePushTokens lists a player's phones whose session is still valid.
func ActivePushTokens(ctx context.Context, q db.Querier, playerID int64) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT t.token FROM td.push_tokens t JOIN td.sessions s ON s.id = t.session_id
		WHERE t.player_id = $1 AND s.revoked_at IS NULL AND s.expires_at > now()`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 2)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

func PushPrefs(ctx context.Context, q db.Querier, playerID int64) (attacks, results bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT push_attacks, push_results FROM td.players WHERE id = $1`, playerID).Scan(&attacks, &results)
	return attacks, results, err
}

func SetPushPrefs(ctx context.Context, q db.Querier, playerID int64, attacks, results bool) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET push_attacks = $2, push_results = $3 WHERE id = $1`, playerID, attacks, results)
	return err
}

// PendingPush is a notice waiting to go to the phone.
type PendingPush struct {
	ID, PlayerID, TetherID int64
	Kind, Body             string
	// ActorCode is the other player's share code (friend requests open their profile), "" when none.
	ActorCode        string
	CreatedAt        time.Time
	Attacks, Results bool
}

// ClaimPendingPushes locks the oldest notices waiting to be sent.
func ClaimPendingPushes(ctx context.Context, q db.Querier, limit int) ([]PendingPush, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id, n.player_id, COALESCE(n.tn_id, 0), n.kind, n.body, COALESCE(a.public_code, ''), n.created_at,
		       p.push_attacks, p.push_results
		FROM td.notices n JOIN td.players p ON p.id = n.player_id LEFT JOIN td.players a ON a.id = n.actor_id
		WHERE n.push_state = 'pending' ORDER BY n.id LIMIT $1 FOR UPDATE OF n SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PendingPush, 0, limit)
	for rows.Next() {
		var p PendingPush
		if err := rows.Scan(&p.ID, &p.PlayerID, &p.TetherID, &p.Kind, &p.Body, &p.ActorCode, &p.CreatedAt, &p.Attacks, &p.Results); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func MarkPush(ctx context.Context, q db.Querier, id int64, state string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.notices SET push_state = $2, pushed_at = CASE WHEN $2 = 'sent' THEN now() END WHERE id = $1`, id, state)
	return err
}

// PushedSince reports whether this kind of push already went out for this Tether Node lately.
func PushedSince(ctx context.Context, q db.Querier, playerID, tetherID int64, kind string, since time.Time) (bool, error) {
	var found bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.notices
		               WHERE player_id = $1 AND tn_id = $2 AND kind = $3 AND push_state = 'sent' AND pushed_at > $4)`,
		playerID, tetherID, kind, since).Scan(&found)
	return found, err
}
