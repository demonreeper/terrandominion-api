package store

import (
	"context"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// ScheduleInactive puts accounts unseen for `days` into the deletion grace period. Founders, bots
// and system accounts are never touched.
func ScheduleInactive(ctx context.Context, q db.Querier, days, graceDays int) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.players
		SET status = 'pending_delete', delete_after = now() + make_interval(days => $2), updated_at = now()
		WHERE status = 'active' AND type = 'player'
		  AND COALESCE(last_seen_at, created_at) < now() - make_interval(days => $1)`, days, graceDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DueDeletions lists accounts whose grace period has ended.
func DueDeletions(ctx context.Context, q db.Querier, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.players
		WHERE status = 'pending_delete' AND delete_after < now()
		ORDER BY delete_after LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func LeadsFaction(ctx context.Context, q db.Querier, playerID int64) (bool, error) {
	var leads bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM td.factions WHERE leader_player_id = $1)`, playerID).Scan(&leads)
	return leads, err
}

// AnonymizePlayer erases a deleted account's personal data but keeps the row, so harvest logs,
// XP history and faction records stay consistent. Sign-in links and sessions are removed, which
// means the same Google account starts fresh if it ever signs in again.
func AnonymizePlayer(ctx context.Context, q db.Querier, playerID int64) error {
	steps := []string{
		`DELETE FROM td.player_identities WHERE player_id = $1`,
		`DELETE FROM td.sessions WHERE player_id = $1`,
		`DELETE FROM td.faction_members WHERE player_id = $1`,
		`UPDATE td.faction_branches SET leader_player_id = NULL WHERE leader_player_id = $1`,
		`UPDATE td.players SET status = 'deleted', display_name = 'Deleted player', photo_file = NULL,
		        delete_after = NULL, last_app_version = NULL, updated_at = now() WHERE id = $1`,
	}
	for _, step := range steps {
		if _, err := q.ExecContext(ctx, step, playerID); err != nil {
			return err
		}
	}
	return nil
}
