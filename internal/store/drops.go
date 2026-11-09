package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Drop is a pickup lying on the map.
type Drop struct {
	ID        int64            `json:"id"`
	Lat       float64          `json:"lat"`
	Lon       float64          `json:"lon"`
	Contents  map[string]int64 `json:"contents"`
	ExpiresAt time.Time        `json:"expiresAt"`
	DistanceM float64          `json:"distanceM"`
}

func NearbyDrops(ctx context.Context, q db.Querier, lat, lon, radiusM float64, limit int) ([]Drop, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g)
		SELECT d.id, ST_Y(d.location::geometry), ST_X(d.location::geometry), d.contents, d.expires_at,
		       ST_Distance(d.location, here.g)
		FROM td.map_drops d CROSS JOIN here
		WHERE d.picked_at IS NULL AND d.expires_at > now() AND ST_DWithin(d.location, here.g, $3)
		ORDER BY d.location <-> here.g
		LIMIT $4`, lat, lon, radiusM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Drop, 0, 8)
	for rows.Next() {
		var d Drop
		var contents []byte
		if err := rows.Scan(&d.ID, &d.Lat, &d.Lon, &contents, &d.ExpiresAt, &d.DistanceM); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(contents, &d.Contents); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func InsertDrop(ctx context.Context, q db.Querier, tetherID int64, lat, lon float64, contents map[string]int64,
	expires time.Time) error {
	encoded, err := json.Marshal(contents)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.map_drops (tn_id, location, contents, expires_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography, $4::jsonb, $5)`,
		tetherID, lat, lon, string(encoded), expires)
	return err
}

// DropForUpdate reads a drop that can still be picked up, and locks it.
func DropForUpdate(ctx context.Context, q db.Querier, id int64) (Drop, error) {
	var d Drop
	var contents []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, ST_Y(location::geometry), ST_X(location::geometry), contents, expires_at
		FROM td.map_drops
		WHERE id = $1 AND picked_at IS NULL AND expires_at > now() FOR UPDATE`, id).
		Scan(&d.ID, &d.Lat, &d.Lon, &contents, &d.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(contents, &d.Contents)
}

// SetDropContents leaves what did not fit on the map.
func SetDropContents(ctx context.Context, q db.Querier, id int64, contents map[string]int64) error {
	encoded, err := json.Marshal(contents)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE td.map_drops SET contents = $2::jsonb WHERE id = $1`, id, string(encoded))
	return err
}

func MarkDropPicked(ctx context.Context, q db.Querier, id, playerID int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE td.map_drops SET picked_by = $2, picked_at = now() WHERE id = $1`, id, playerID)
	return err
}

// DeleteExpiredDrops removes drops nobody picked up in time.
func DeleteExpiredDrops(ctx context.Context, q db.Querier) (int64, error) {
	res, err := q.ExecContext(ctx, `DELETE FROM td.map_drops WHERE picked_at IS NULL AND expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
