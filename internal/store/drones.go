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

// DroneBay is the player's working equipped drone bay. Mk is its tier; Cells is what one strike
// costs (1 when the stats don't say).
type DroneBay struct {
	GearID int64   `json:"-"`
	Mk     int     `json:"-"`
	Drones int     `json:"drones"`
	RangeM float64 `json:"range_m"`
	Damage float64 `json:"damage"`
	Cells  int     `json:"cells"`
}

// EquippedDroneBay returns ErrNotFound when the player has none that works.
func EquippedDroneBay(ctx context.Context, q db.Querier, playerID int64) (DroneBay, error) {
	bay := DroneBay{Cells: 1}
	var stats []byte
	err := q.QueryRowContext(ctx, `
		SELECT pg.id, gt.tier, gt.stats FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		WHERE pg.player_id = $1 AND pg.slot = 'drone_bay' AND pg.equipped AND pg.durability > 0`, playerID).
		Scan(&bay.GearID, &bay.Mk, &stats)
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

// WearGear takes durability from a piece of gear, never below 0.
func WearGear(ctx context.Context, q db.Querier, gearID int64, wear int) error {
	if wear <= 0 {
		return nil
	}
	_, err := q.ExecContext(ctx,
		`UPDATE td.player_gear SET durability = GREATEST(0, durability - $2) WHERE id = $1`, gearID, wear)
	return err
}

// LaunchDrone records a strike in flight from the launch fix and returns its id and launch time.
func LaunchDrone(ctx context.Context, q db.Querier, tetherID, playerID int64, damage float64, mk int,
	fromLat, fromLon float64, arrivesAt time.Time) (int64, time.Time, error) {
	var id int64
	var launched time.Time
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.drone_strikes (tn_id, launched_by, damage, mk, from_lat, from_lon, arrives_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`,
		tetherID, playerID, damage, mk, fromLat, fromLon, arrivesAt).Scan(&id, &launched)
	return id, launched, err
}

// DueDrone is a strike that has reached its target.
type DueDrone struct {
	ID, TetherID, OwnerID, LaunchedBy int64
	Damage                            float64
	Mk                                int
	InterceptShare                    float64
}

func DueDrones(ctx context.Context, q db.Querier) ([]DueDrone, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT d.id, d.tn_id, t.owner_id, d.launched_by, d.damage, d.mk, d.intercept_share
		FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		WHERE d.status = 'flying' AND d.arrives_at <= now() ORDER BY d.arrives_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DueDrone, 0, 4)
	for rows.Next() {
		var d DueDrone
		if err := rows.Scan(&d.ID, &d.TetherID, &d.OwnerID, &d.LaunchedBy, &d.Damage, &d.Mk, &d.InterceptShare); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// LandDrone marks a strike landed, and reports false if something else got to it first (an
// intercept that shot it down, or the node ending). The intercept locks the same row, so a drone
// intercepted in the tick it lands is resolved once.
func LandDrone(ctx context.Context, q db.Querier, id int64) (bool, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE td.drone_strikes SET status = 'hit', ended_at = now() WHERE id = $1 AND status = 'flying'`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SetDroneLandedDamage records what actually reached the node.
func SetDroneLandedDamage(ctx context.Context, q db.Querier, id int64, damage float64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.drone_strikes SET landed_damage = $2 WHERE id = $1`, id, damage)
	return err
}

// InboundDrone is a strike flying at a Tether Node, locked for an intercept.
type InboundDrone struct {
	ID, LaunchedBy int64
	Mk             int
	ArrivesAt      time.Time
}

// InboundDroneForUpdate locks the soonest-landing drone at the node that hasn't been intercepted.
// anyFlying says whether any drone is flying at it at all (all already intercepted when ok is false).
func InboundDroneForUpdate(ctx context.Context, q db.Querier, tetherID int64) (drone InboundDrone, ok, anyFlying bool, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT id, launched_by, mk, arrives_at FROM td.drone_strikes
		WHERE tn_id = $1 AND status = 'flying' AND NOT intercepted AND arrives_at > now()
		ORDER BY arrives_at LIMIT 1 FOR UPDATE`, tetherID).Scan(&drone.ID, &drone.LaunchedBy, &drone.Mk, &drone.ArrivesAt)
	if err == nil {
		return drone, true, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return drone, false, false, err
	}
	err = q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM td.drone_strikes WHERE tn_id = $1 AND status = 'flying')`, tetherID).Scan(&anyFlying)
	return drone, false, anyFlying, err
}

// InterceptDrone records an intercept. A drone whose whole damage is stopped is void at once.
func InterceptDrone(ctx context.Context, q db.Querier, id int64, share float64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.drone_strikes
		SET intercepted = true, intercept_share = $2::real,
		    status = CASE WHEN $2::real >= 1 THEN 'void' ELSE status END,
		    ended_at = CASE WHEN $2::real >= 1 THEN now() ELSE ended_at END
		WHERE id = $1 AND status = 'flying'`, id, share)
	return err
}

// Point is a place on the map.
type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// PlayerDrone is one of a player's strikes, in flight or recently ended, for GET /v1/drones.
type PlayerDrone struct {
	ID          int64     `json:"id"`
	TetherID    int64     `json:"tnId"`
	Mk          int       `json:"mk"`
	Status      string    `json:"status"` // flying, hit or void
	LaunchedAt  time.Time `json:"launchedAt"`
	ArrivesAt   time.Time `json:"arrivesAt"`
	From        *Point    `json:"from"`
	To          Point     `json:"to"`
	Intercepted bool      `json:"intercepted"`
	// Damage is what landed, once it has.
	Damage *float64 `json:"damage"`
}

// PlayerDrones lists the player's strikes still flying, and those that ended since the given time.
func PlayerDrones(ctx context.Context, q db.Querier, playerID int64, since time.Time) ([]PlayerDrone, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT d.id, d.tn_id, d.mk, d.status, d.created_at, d.arrives_at, d.from_lat, d.from_lon,
		       ST_Y(t.location::geometry), ST_X(t.location::geometry), d.intercepted, d.landed_damage
		FROM td.drone_strikes d JOIN td.tether_nodes t ON t.id = d.tn_id
		WHERE d.launched_by = $1 AND (d.status = 'flying' OR COALESCE(d.ended_at, d.arrives_at) >= $2)
		ORDER BY d.arrives_at DESC LIMIT 20`, playerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerDrone, 0, 4)
	for rows.Next() {
		var d PlayerDrone
		var fromLat, fromLon, damage sql.NullFloat64
		if err := rows.Scan(&d.ID, &d.TetherID, &d.Mk, &d.Status, &d.LaunchedAt, &d.ArrivesAt, &fromLat, &fromLon,
			&d.To.Lat, &d.To.Lon, &d.Intercepted, &damage); err != nil {
			return nil, err
		}
		if fromLat.Valid && fromLon.Valid {
			d.From = &Point{fromLat.Float64, fromLon.Float64}
		}
		if damage.Valid {
			v := damage.Float64
			d.Damage = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
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
