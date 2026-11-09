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
	// ReadyInS is how long until this player may harvest the node again (0 = ready now).
	ReadyInS int `json:"readyInS"`

	Yield  int    `json:"-"`
	Status string `json:"-"`
}

// NearbyNodes uses the GiST index on location; distance is metres on the spheroid. Each node
// also says how long until this player's harvest cooldown on it ends, so the app can show it.
func NearbyNodes(ctx context.Context, q db.Querier, playerID int64, lat, lon, radiusM float64, limit int,
	cooldownSec int) ([]Node, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g)
		SELECT n.id, n.name, n.resource, ST_Y(n.location::geometry), ST_X(n.location::geometry),
		       n.reserve, ST_Distance(n.location, here.g),
		       COALESCE(GREATEST(0, CEIL(EXTRACT(EPOCH FROM
		           (h.last_at + make_interval(secs => $6)) - now())))::int, 0)
		FROM td.nodes n
		CROSS JOIN here
		LEFT JOIN LATERAL (
		    SELECT created_at AS last_at FROM td.harvests
		    WHERE player_id = $5 AND node_id = n.id
		    ORDER BY created_at DESC LIMIT 1
		) h ON true
		WHERE n.status = 'active' AND ST_DWithin(n.location, here.g, $3)
		ORDER BY n.location <-> here.g
		LIMIT $4`, lat, lon, radiusM, limit, playerID, cooldownSec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Node, 0, limit)
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Resource, &n.Lat, &n.Lon, &n.Reserve, &n.DistanceM, &n.ReadyInS); err != nil {
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

// DepletedNodes lists active nodes whose reserve has fallen below the threshold.
func DepletedNodes(ctx context.Context, q db.Querier, threshold float64) ([]int64, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id FROM td.nodes WHERE status = 'active' AND reserve < $1 ORDER BY id`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, 16)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ReplaceNode retires a node and creates its replacement at a new spot, full, with the same
// resource, yield and origin. The old row stays, so harvest history still points at it.
func ReplaceNode(ctx context.Context, q db.Querier, id int64, lat, lon float64) (int64, error) {
	if _, err := q.ExecContext(ctx,
		`UPDATE td.nodes SET status = 'retired', updated_at = now() WHERE id = $1`, id); err != nil {
		return 0, err
	}
	var newID int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.nodes (name, resource, location, country_code, region_code, yield_per_harvest,
		                      regen_per_hour, source, source_ref, created_by)
		SELECT name, resource, ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography, country_code, region_code,
		       yield_per_harvest, regen_per_hour, source, source_ref, created_by
		FROM td.nodes WHERE id = $1
		RETURNING id`, id, lat, lon).Scan(&newID)
	return newID, err
}

type NewNode struct {
	Name         string  `json:"name"`
	Resource     string  `json:"resource"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	RegionCode   string  `json:"regionCode"`
	Yield        int     `json:"yield"`
	RegenPerHour float64 `json:"regenPerHour"`
	// Source records where a node came from: manual (default), scatter (test nodes), osm.
	Source string `json:"-"`
}

func CreateNode(ctx context.Context, q db.Querier, n NewNode, adminID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.nodes (name, resource, location, region_code, yield_per_harvest, regen_per_hour,
		                      created_by, source)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography, NULLIF($5, ''), $6, $7, $8,
		        COALESCE(NULLIF($9, ''), 'manual'))
		RETURNING id`, n.Name, n.Resource, n.Lat, n.Lon, n.RegionCode, n.Yield, n.RegenPerHour, adminID,
		n.Source).Scan(&id)
	return id, err
}

// RetireNodesBySource takes every active node from one source off the map (they stay in the
// table, so harvest history still points at them).
func RetireNodesBySource(ctx context.Context, q db.Querier, source string) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.nodes SET status = 'retired', updated_at = now()
		WHERE source = $1 AND status <> 'retired'`, source)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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
