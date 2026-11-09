package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Carried returns what the player carries, with every resource type present (zero if none).
func Carried(ctx context.Context, q db.Querier, playerID int64) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT rt.code, COALESCE(pr.carried, 0)
		FROM td.resource_types rt
		LEFT JOIN td.player_resources pr ON pr.resource = rt.code AND pr.player_id = $1
		ORDER BY rt.sort`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var code string
		var amount int64
		if err := rows.Scan(&code, &amount); err != nil {
			return nil, err
		}
		out[code] = amount
	}
	return out, rows.Err()
}

// spend removes a resource with a plain UPDATE. An upsert can't remove: Postgres checks the row it
// would insert, with its negative amount, against the CHECK before it looks for a conflict.
func spend(ctx context.Context, q db.Querier, query string, playerID int64, resource string, amount int64) error {
	res, err := q.ExecContext(ctx, query, playerID, resource, amount)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return fmt.Errorf("player %d holds no %s to spend", playerID, resource)
	}
	return err
}

// AddCarried adds (or with a negative amount, removes) a resource. The table's CHECK refuses to
// go below zero, so a bad caller fails instead of minting debt.
func AddCarried(ctx context.Context, q db.Querier, playerID int64, resource string, amount int64) error {
	if amount < 0 {
		return spend(ctx, q, `UPDATE td.player_resources SET carried = carried + $3
			WHERE player_id = $1 AND resource = $2`, playerID, resource, amount)
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_resources (player_id, resource, carried) VALUES ($1, $2, $3)
		ON CONFLICT (player_id, resource) DO UPDATE SET carried = td.player_resources.carried + $3`,
		playerID, resource, amount)
	return err
}

// Uploaded returns the player's uploaded inventory, with every resource type present (zero if none).
func Uploaded(ctx context.Context, q db.Querier, playerID int64) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT rt.code, COALESCE(pu.amount, 0)
		FROM td.resource_types rt
		LEFT JOIN td.player_uploaded pu ON pu.resource = rt.code AND pu.player_id = $1
		ORDER BY rt.sort`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var code string
		var amount int64
		if err := rows.Scan(&code, &amount); err != nil {
			return nil, err
		}
		out[code] = amount
	}
	return out, rows.Err()
}

// AddUploaded adds (or with a negative amount, removes) a resource. The table's CHECK refuses to
// go below zero, as with AddCarried.
func AddUploaded(ctx context.Context, q db.Querier, playerID int64, resource string, amount int64) error {
	if amount < 0 {
		return spend(ctx, q, `UPDATE td.player_uploaded SET amount = amount + $3
			WHERE player_id = $1 AND resource = $2`, playerID, resource, amount)
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_uploaded (player_id, resource, amount) VALUES ($1, $2, $3)
		ON CONFLICT (player_id, resource) DO UPDATE SET amount = td.player_uploaded.amount + $3`,
		playerID, resource, amount)
	return err
}

type GearItem struct {
	ID            int64           `json:"id"`
	TypeID        string          `json:"typeId"`
	Name          string          `json:"name"`
	Slot          string          `json:"slot"`
	Tier          int             `json:"tier"`
	Durability    int             `json:"durability"`
	MaxDurability int             `json:"maxDurability"`
	Equipped      bool            `json:"equipped"`
	Stats         json.RawMessage `json:"stats"`

	// The next tier, when there is one. RepairCost is filled by the service, which has the settings.
	UpgradeName     string           `json:"upgradeName,omitempty"`
	UpgradeMinLevel int              `json:"upgradeMinLevel,omitempty"`
	UpgradeCost     map[string]int64 `json:"upgradeCost,omitempty"`
	RepairCost      map[string]int64 `json:"repairCost,omitempty"`
}

// GrantStarterGear gives a new player the Mk I scanner and extractor, equipped.
func GrantStarterGear(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_gear (player_id, gear_type_id, slot, durability, equipped, acquired_from)
		SELECT $1, gt.id, gt.slot, gt.max_durability, true, 'starter'
		FROM td.gear_types gt WHERE gt.id IN ('scanner_mk1', 'extractor_mk1')`, playerID)
	return err
}

func Gear(ctx context.Context, q db.Querier, playerID int64) ([]GearItem, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT pg.id, gt.id, gt.name, gt.slot, gt.tier, pg.durability, gt.max_durability, pg.equipped, gt.stats,
		       nx.name, nx.min_level, nx.craft_cost
		FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		LEFT JOIN td.gear_types nx ON nx.id = gt.upgrades_to
		WHERE pg.player_id = $1
		ORDER BY pg.equipped DESC, gt.slot, gt.tier DESC`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GearItem, 0, 8)
	for rows.Next() {
		var g GearItem
		var stats, upgradeCost []byte
		var upgradeName sql.NullString
		var upgradeLevel sql.NullInt64
		if err := rows.Scan(&g.ID, &g.TypeID, &g.Name, &g.Slot, &g.Tier, &g.Durability, &g.MaxDurability,
			&g.Equipped, &stats, &upgradeName, &upgradeLevel, &upgradeCost); err != nil {
			return nil, err
		}
		g.Stats = stats
		if upgradeName.Valid {
			g.UpgradeName = upgradeName.String
			g.UpgradeMinLevel = int(upgradeLevel.Int64)
			if err := json.Unmarshal(upgradeCost, &g.UpgradeCost); err != nil {
				return nil, err
			}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// EquippedStats folds the equipped gear's stats into the numbers the rules need. Worn-out gear
// (durability 0) contributes nothing until repaired.
type EquippedStats struct {
	ScanRangeM    float64
	YieldMult     float64
	CarryCapacity int64
}

func Equipped(ctx context.Context, q db.Querier, playerID int64) (EquippedStats, error) {
	out := EquippedStats{YieldMult: 1}
	err := q.QueryRowContext(ctx, `
		SELECT
		  COALESCE(MAX((gt.stats->>'range_m')::float8)   FILTER (WHERE gt.slot = 'scanner'), 0),
		  COALESCE(MAX((gt.stats->>'yield_mult')::float8) FILTER (WHERE gt.slot = 'extractor'), 1),
		  COALESCE(MAX((gt.stats->>'carry')::int8)       FILTER (WHERE gt.slot = 'extractor'), 0)
		FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		WHERE pg.player_id = $1 AND pg.equipped AND pg.durability > 0`, playerID).
		Scan(&out.ScanRangeM, &out.YieldMult, &out.CarryCapacity)
	return out, err
}
