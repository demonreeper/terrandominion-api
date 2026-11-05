package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type Node struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Resource  string  `json:"resource"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Reserve   float64 `json:"reserve"`
	DistanceM float64 `json:"distanceM"`

	Yield  int    `json:"-"`
	Status string `json:"-"`
}

// NearbyNodes uses the GiST index on location; distance is metres on the spheroid.
func NearbyNodes(ctx context.Context, q db.Querier, lat, lon, radiusM float64, limit int) ([]Node, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g)
		SELECT n.id, n.name, n.resource, ST_Y(n.location::geometry), ST_X(n.location::geometry),
		       n.reserve, ST_Distance(n.location, here.g)
		FROM td.nodes n, here
		WHERE n.status = 'active' AND ST_DWithin(n.location, here.g, $3)
		ORDER BY n.location <-> here.g
		LIMIT $4`, lat, lon, radiusM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Node, 0, limit)
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Resource, &n.Lat, &n.Lon, &n.Reserve, &n.DistanceM); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// NodeForUpdate locks the node so simultaneous harvests by different players drain it correctly.
func NodeForUpdate(ctx context.Context, q db.Querier, id int64) (Node, error) {
	var n Node
	err := q.QueryRowContext(ctx, `
		SELECT id, name, resource, ST_Y(location::geometry), ST_X(location::geometry), reserve,
		       yield_per_harvest, status
		FROM td.nodes WHERE id = $1 FOR UPDATE`, id).
		Scan(&n.ID, &n.Name, &n.Resource, &n.Lat, &n.Lon, &n.Reserve, &n.Yield, &n.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNotFound
	}
	return n, err
}

func DrainNode(ctx context.Context, q db.Querier, id int64, cost float64) (float64, error) {
	var reserve float64
	err := q.QueryRowContext(ctx, `
		UPDATE td.nodes SET reserve = GREATEST(0, reserve - $2), updated_at = now()
		WHERE id = $1 RETURNING reserve`, id, cost).Scan(&reserve)
	return reserve, err
}

func InsertHarvest(ctx context.Context, q db.Querier, playerID, nodeID int64, amount int,
	lat, lon, accuracyM, distanceM float64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.harvests (player_id, node_id, amount, reported, accuracy_m, distance_m)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($5, $4), 4326)::geography, $6, $7)
		RETURNING id`, playerID, nodeID, amount, lat, lon, accuracyM, distanceM).Scan(&id)
	return id, err
}

func LastHarvestAtNode(ctx context.Context, q db.Querier, playerID, nodeID int64) (*time.Time, error) {
	var at time.Time
	err := q.QueryRowContext(ctx, `
		SELECT created_at FROM td.harvests WHERE player_id = $1 AND node_id = $2
		ORDER BY created_at DESC LIMIT 1`, playerID, nodeID).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &at, nil
}

type LastFix struct {
	Lat, Lon, AccuracyM float64
	At                  time.Time
}

func LastHarvestFix(ctx context.Context, q db.Querier, playerID int64) (*LastFix, error) {
	var f LastFix
	err := q.QueryRowContext(ctx, `
		SELECT ST_Y(reported::geometry), ST_X(reported::geometry), accuracy_m, created_at
		FROM td.harvests WHERE player_id = $1 ORDER BY created_at DESC LIMIT 1`, playerID).
		Scan(&f.Lat, &f.Lon, &f.AccuracyM, &f.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

type NewNode struct {
	Name         string  `json:"name"`
	Resource     string  `json:"resource"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	RegionCode   string  `json:"regionCode"`
	Yield        int     `json:"yield"`
	RegenPerHour float64 `json:"regenPerHour"`
}

func CreateNode(ctx context.Context, q db.Querier, n NewNode, adminID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.nodes (name, resource, location, region_code, yield_per_harvest, regen_per_hour, created_by)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography, NULLIF($5, ''), $6, $7, $8)
		RETURNING id`, n.Name, n.Resource, n.Lat, n.Lon, n.RegionCode, n.Yield, n.RegenPerHour, adminID).Scan(&id)
	return id, err
}

// RegrowNodes tops up every node by its hourly rate for the time since it last regrew. It is
// safe to run at any interval, and running it twice in a row adds nothing extra.
func RegrowNodes(ctx context.Context, q db.Querier) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.nodes
		SET reserve = LEAST(1, reserve + regen_per_hour * EXTRACT(EPOCH FROM now() - regen_at) / 3600),
		    regen_at = now()
		WHERE status = 'active' AND reserve < 1`)
	if err != nil {
		return 0, err
	}
	// Nodes already full still need their clock moved, or they would regrow a burst later.
	if _, err := q.ExecContext(ctx, `UPDATE td.nodes SET regen_at = now() WHERE reserve >= 1`); err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
