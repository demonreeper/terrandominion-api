package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Items returns the consumables a player owns; items they have none of are absent.
func Items(ctx context.Context, q db.Querier, playerID int64) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT item, count FROM td.player_items WHERE player_id = $1 AND count > 0`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var item string
		var count int64
		if err := rows.Scan(&item, &count); err != nil {
			return nil, err
		}
		out[item] = count
	}
	return out, rows.Err()
}

// AddItem adds to a consumable's count.
func AddItem(ctx context.Context, q db.Querier, playerID int64, item string, count int) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_items (player_id, item, count) VALUES ($1, $2, $3)
		ON CONFLICT (player_id, item) DO UPDATE SET count = td.player_items.count + $3`,
		playerID, item, count)
	return err
}

// OwnedGear is a piece of gear being changed. UpgradesTo is empty at the top of its line.
type OwnedGear struct {
	Durability    int
	MaxDurability int
	UpgradesTo    string
}

// OwnedGearForUpdate locks one of the player's items, or returns ErrNotFound.
func OwnedGearForUpdate(ctx context.Context, q db.Querier, playerID, gearID int64) (OwnedGear, error) {
	var out OwnedGear
	var next sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT pg.durability, gt.max_durability, gt.upgrades_to
		FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		WHERE pg.id = $1 AND pg.player_id = $2
		FOR UPDATE OF pg`, gearID, playerID).Scan(&out.Durability, &out.MaxDurability, &next)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	out.UpgradesTo = next.String
	return out, err
}

// GearType is the part of the catalogue an upgrade needs.
type GearType struct {
	ID            string
	MinLevel      int
	MaxDurability int
	Cost          map[string]int64
}

func GearTypeByID(ctx context.Context, q db.Querier, id string) (GearType, error) {
	out := GearType{ID: id}
	var cost []byte
	err := q.QueryRowContext(ctx, `SELECT min_level, max_durability, craft_cost FROM td.gear_types WHERE id = $1`, id).
		Scan(&out.MinLevel, &out.MaxDurability, &cost)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(cost, &out.Cost)
}

// SetGearType makes an item the given type at full durability.
func SetGearType(ctx context.Context, q db.Querier, gearID int64, typeID string, durability int) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.player_gear SET gear_type_id = $2, durability = $3, acquired_from = 'upgrade' WHERE id = $1`,
		gearID, typeID, durability)
	return err
}

func SetGearDurability(ctx context.Context, q db.Querier, gearID int64, durability int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.player_gear SET durability = $2 WHERE id = $1`, gearID, durability)
	return err
}
