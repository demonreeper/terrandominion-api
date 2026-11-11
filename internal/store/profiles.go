package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// ServiceRecord is what a player has done, as their profile shows it to anyone: counts only,
// nothing with a place or an exact time in it.
type ServiceRecord struct {
	// ActiveSince is the first day of the month the account was made (the profile shows month and year).
	ActiveSince       time.Time `json:"activeSince"`
	NodesHarvested    int       `json:"nodesHarvested"`
	TethersBuilt      int       `json:"tethersBuilt"`
	TethersActive     int       `json:"tethersActive"`
	TethersDestroyed  int       `json:"tethersDestroyed"`
	DronesIntercepted int       `json:"dronesIntercepted"`
	MissionsCompleted int       `json:"missionsCompleted"`
}

// PlayerRecord counts a player's service record from the tables the actions already write.
func PlayerRecord(ctx context.Context, q db.Querier, playerID int64) (ServiceRecord, error) {
	var r ServiceRecord
	err := q.QueryRowContext(ctx, `
		SELECT date_trunc('month', p.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC',
		       (SELECT count(*) FROM td.harvests h WHERE h.player_id = p.id),
		       (SELECT count(*) FROM td.tether_nodes t WHERE t.owner_id = p.id),
		       (SELECT count(*) FROM td.tether_nodes t WHERE t.owner_id = p.id AND t.status = 'active'),
		       (SELECT count(*) FROM td.tether_nodes t WHERE t.destroyed_by = p.id),
		       (SELECT count(*) FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		         WHERE d.intercepted AND t.owner_id = p.id),
		       (SELECT count(*) FROM td.player_missions m WHERE m.player_id = p.id AND m.status IN ('done', 'claimed'))
		FROM td.players p WHERE p.id = $1`, playerID).
		Scan(&r.ActiveSince, &r.NodesHarvested, &r.TethersBuilt, &r.TethersActive, &r.TethersDestroyed,
			&r.DronesIntercepted, &r.MissionsCompleted)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// FactionColor is a faction's colour ("#4fd8ff"), for a profile's standing line.
func FactionColor(ctx context.Context, q db.Querier, factionID int) (string, error) {
	var color string
	err := q.QueryRowContext(ctx, `SELECT color FROM td.factions WHERE id = $1`, factionID).Scan(&color)
	return color, err
}

// SetTetherDestroyer records who destroyed a Tether Node.
func SetTetherDestroyer(ctx context.Context, q db.Querier, tetherID, attackerID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.tether_nodes SET destroyed_by = $2 WHERE id = $1`, tetherID, attackerID)
	return err
}
