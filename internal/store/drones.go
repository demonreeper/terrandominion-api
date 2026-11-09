package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// DroneBay is the player's working equipped drone bay.
type DroneBay struct {
	Drones int     `json:"drones"`
	RangeM float64 `json:"range_m"`
	Damage float64 `json:"damage"`
}

// EquippedDroneBay returns ErrNotFound when the player has none that works.
func EquippedDroneBay(ctx context.Context, q db.Querier, playerID int64) (DroneBay, error) {
	var bay DroneBay
	var stats []byte
	err := q.QueryRowContext(ctx, `
		SELECT gt.stats FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		WHERE pg.player_id = $1 AND pg.slot = 'drone_bay' AND pg.equipped AND pg.durability > 0`, playerID).Scan(&stats)
	if errors.Is(err, sql.ErrNoRows) {
		return bay, ErrNotFound
	}
	if err != nil {
		return bay, err
	}
	return bay, json.Unmarshal(stats, &bay)
}

func DronesInFlight(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.drone_strikes WHERE launched_by = $1 AND status = 'flying'`, playerID).Scan(&n)
	return n, err
}

func LaunchDrone(ctx context.Context, q db.Querier, tetherID, playerID int64, damage float64, arrivesAt time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.drone_strikes (tn_id, launched_by, damage, arrives_at) VALUES ($1, $2, $3, $4)`,
		tetherID, playerID, damage, arrivesAt)
	return err
}

// DueDrone is a strike that has reached its target.
type DueDrone struct {
	ID, TetherID, OwnerID, LaunchedBy int64
	Damage                            float64
}

func DueDrones(ctx context.Context, q db.Querier) ([]DueDrone, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT d.id, d.tn_id, t.owner_id, d.launched_by, d.damage
		FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		WHERE d.status = 'flying' AND d.arrives_at <= now() ORDER BY d.arrives_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DueDrone, 0, 4)
	for rows.Next() {
		var d DueDrone
		if err := rows.Scan(&d.ID, &d.TetherID, &d.OwnerID, &d.LaunchedBy, &d.Damage); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// LandDrone marks a strike landed, and reports false if something else got to it first.
func LandDrone(ctx context.Context, q db.Querier, id int64) (bool, error) {
	res, err := q.ExecContext(ctx, `UPDATE td.drone_strikes SET status = 'hit' WHERE id = $1 AND status = 'flying'`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// CraftableGear is a piece of gear the player can make from nothing on the Craft tab.
type CraftableGear struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	MinLevel int              `json:"minLevel"`
	Cost     map[string]int64 `json:"cost"`
}

// CraftableGearFor lists the given types whose slot the player has nothing in.
func CraftableGearFor(ctx context.Context, q db.Querier, playerID int64, ids []string) ([]CraftableGear, error) {
	out := make([]CraftableGear, 0, len(ids))
	for _, id := range ids {
		var g CraftableGear
		var cost []byte
		err := q.QueryRowContext(ctx, `
			SELECT gt.id, gt.name, gt.min_level, gt.craft_cost FROM td.gear_types gt
			WHERE gt.id = $1 AND NOT EXISTS (
			    SELECT 1 FROM td.player_gear pg WHERE pg.player_id = $2 AND pg.slot = gt.slot)`, id, playerID).
			Scan(&g.ID, &g.Name, &g.MinLevel, &cost)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(cost, &g.Cost); err != nil {
			return nil, fmt.Errorf("gear %s cost: %w", id, err)
		}
		out = append(out, g)
	}
	return out, nil
}

// GrantCraftedGear gives the player a new item, equipped, at full durability.
func GrantCraftedGear(ctx context.Context, q db.Querier, playerID int64, typeID string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_gear (player_id, gear_type_id, slot, durability, equipped, acquired_from)
		SELECT $1, gt.id, gt.slot, gt.max_durability, true, 'craft' FROM td.gear_types gt WHERE gt.id = $2`,
		playerID, typeID)
	return err
}
