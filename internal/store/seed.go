package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Node seeding: the OpenStreetMap layers the seeder stands nodes on, the cells it has filled and
// the areas an admin pre-seeds.

// Box is a cell or area in degrees.
type Box struct{ MinLat, MinLon, MaxLat, MaxLon float64 }

// envelope is the SQL for a Box given as four parameters starting at $n, in lon/lat order.
const cellEnvelope = `ST_MakeEnvelope($2, $1, $4, $3, 4326)`

// OSMFeature is one shape from the extract: Geom is a GeoJSON geometry. BufferM (blocked lines
// only, such as runways) widens a line into an area.
type OSMFeature struct {
	Kind    string          `json:"k"`
	Geom    json.RawMessage `json:"g"`
	BufferM float64         `json:"buf,omitempty"`
}

// ReplaceOSM swaps every OSM layer for a new import and forgets which cells were seeded, so each
// is looked at again on the next scan or tick (existing seed nodes still count toward it).
func ReplaceOSM(ctx context.Context, q db.Querier, land, walk, blocked []OSMFeature) error {
	if _, err := q.ExecContext(ctx, `TRUNCATE td.osm_land, td.osm_walkways, td.osm_blocked`); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE td.seed_cells SET seeded_at = NULL, topped_at = NULL`); err != nil {
		return err
	}
	insert := func(sqlText string, features []OSMFeature) error {
		const chunk = 2000
		for start := 0; start < len(features); start += chunk {
			body, err := json.Marshal(features[start:min(start+chunk, len(features))])
			if err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, sqlText, string(body)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := insert(`
		INSERT INTO td.osm_land (geom)
		SELECT g FROM (
		    SELECT ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON(f->>'g'), 4326)), 3) AS g
		    FROM json_array_elements($1::json) f) x
		WHERE NOT ST_IsEmpty(g)`, land); err != nil {
		return err
	}
	if err := insert(`
		INSERT INTO td.osm_walkways (kind, geom)
		SELECT f->>'k', ST_SetSRID(ST_GeomFromGeoJSON(f->>'g'), 4326)
		FROM json_array_elements($1::json) f`, walk); err != nil {
		return err
	}
	return insert(`
		INSERT INTO td.osm_blocked (kind, geom)
		SELECT k, g FROM (
		    SELECT f->>'k' AS k,
		           CASE WHEN COALESCE((f->>'buf')::float, 0) > 0
		                THEN ST_Buffer(ST_SetSRID(ST_GeomFromGeoJSON(f->>'g'), 4326)::geography, (f->>'buf')::float)::geometry
		                ELSE ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON(f->>'g'), 4326)), 3)
		           END AS g
		    FROM json_array_elements($1::json) f) x
		WHERE NOT ST_IsEmpty(g)`, blocked)
}

// OSMLoaded says whether an import has run; without one the seeder does nothing (and marks
// nothing as seeded).
func OSMLoaded(ctx context.Context, q db.Querier) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM td.osm_land) AND EXISTS (SELECT 1 FROM td.osm_walkways)`).Scan(&ok)
	return ok, err
}

type SeedCellRow struct {
	ID       string
	Box      Box
	Target   int
	WalkM    float64
	SeededAt *time.Time
}

// LockSeedCell makes the cell's row if it is new and locks it for the transaction.
func LockSeedCell(ctx context.Context, q db.Querier, id string, b Box) (SeedCellRow, error) {
	if _, err := q.ExecContext(ctx, `
		INSERT INTO td.seed_cells (cell_id, min_lat, min_lon, max_lat, max_lon)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (cell_id) DO NOTHING`,
		id, b.MinLat, b.MinLon, b.MaxLat, b.MaxLon); err != nil {
		return SeedCellRow{}, err
	}
	r := SeedCellRow{ID: id}
	err := q.QueryRowContext(ctx, `
		SELECT min_lat, min_lon, max_lat, max_lon, target, walk_m, seeded_at
		FROM td.seed_cells WHERE cell_id = $1 FOR UPDATE`, id).
		Scan(&r.Box.MinLat, &r.Box.MinLon, &r.Box.MaxLat, &r.Box.MaxLon, &r.Target, &r.WalkM, &r.SeededAt)
	return r, err
}

// SeededCells says which of these cells have had their first seeding pass.
func SeededCells(ctx context.Context, q db.Querier, ids []string) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT cell_id FROM td.seed_cells WHERE cell_id = ANY($1) AND seeded_at IS NOT NULL`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// WalkLengthInCell is the metres of walkable way inside the cell.
func WalkLengthInCell(ctx context.Context, q db.Querier, b Box) (float64, error) {
	var m float64
	err := q.QueryRowContext(ctx, `
		WITH env AS (SELECT `+cellEnvelope+` AS g)
		SELECT COALESCE(SUM(ST_Length(ST_Intersection(w.geom, env.g)::geography)), 0)
		FROM td.osm_walkways w, env WHERE w.geom && env.g`,
		b.MinLat, b.MinLon, b.MaxLat, b.MaxLon).Scan(&m)
	return m, err
}

// CountSeedNodesInCell counts active seed nodes standing in the cell, wherever they were first
// seeded (a respawn can move one into another cell). Scatter test nodes don't count.
func CountSeedNodesInCell(ctx context.Context, q db.Querier, b Box, source string) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.nodes
		WHERE status = 'active' AND source = $5
		  AND ST_Intersects(location::geometry, `+cellEnvelope+`)`,
		b.MinLat, b.MinLon, b.MaxLat, b.MaxLon, source).Scan(&n)
	return n, err
}

// SeedCandidate is a place a seed node may stand.
type SeedCandidate struct{ Lat, Lon float64 }

// SeedCandidates walks every walkable way in the cell every stepM metres, moves each point up to
// snapM in a random direction, and keeps the ones on land, outside buildings, water and runways,
// at least nodeGapM from any active node and tnGapM from any active Tether Node. Random order.
func SeedCandidates(ctx context.Context, q db.Querier, b Box, stepM, snapM, nodeGapM, tnGapM float64,
	limit int) ([]SeedCandidate, error) {
	rows, err := q.QueryContext(ctx, `
		WITH env AS (SELECT `+cellEnvelope+` AS g),
		lines AS (
		    SELECT (ST_Dump(ST_Intersection(w.geom, env.g))).geom AS g
		    FROM td.osm_walkways w, env WHERE w.geom && env.g),
		pts AS (
		    SELECT (ST_DumpPoints(ST_Segmentize(l.g::geography, $5)::geometry)).geom AS g
		    FROM lines l WHERE GeometryType(l.g) = 'LINESTRING'),
		cand AS (
		    SELECT ST_Project(p.g::geography, random() * $6, random() * 2 * pi()) AS g FROM pts p)
		SELECT ST_Y(c.g::geometry), ST_X(c.g::geometry)
		FROM cand c, env
		WHERE ST_Intersects(c.g::geometry, env.g)
		  AND EXISTS (SELECT 1 FROM td.osm_land l WHERE ST_Intersects(l.geom, c.g::geometry))
		  AND NOT EXISTS (SELECT 1 FROM td.osm_blocked x WHERE ST_Intersects(x.geom, c.g::geometry))
		  AND NOT EXISTS (SELECT 1 FROM td.nodes n
		                  WHERE n.status = 'active' AND ST_DWithin(n.location, c.g, $7))
		  AND NOT EXISTS (SELECT 1 FROM td.tether_nodes t
		                  WHERE t.status = 'active' AND ST_DWithin(t.location, c.g, $8))
		ORDER BY random()
		LIMIT $9`,
		b.MinLat, b.MinLon, b.MaxLat, b.MaxLon, stepM, snapM, nodeGapM, tnGapM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SeedCandidate, 0, limit)
	for rows.Next() {
		var c SeedCandidate
		if err := rows.Scan(&c.Lat, &c.Lon); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkCellSeeded records a seeding pass. topUp passes keep the first pass's target and path.
func MarkCellSeeded(ctx context.Context, q db.Querier, id string, target int, walkM float64, topUp bool) error {
	if topUp {
		_, err := q.ExecContext(ctx, `UPDATE td.seed_cells SET topped_at = now() WHERE cell_id = $1`, id)
		return err
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.seed_cells SET target = $2, walk_m = $3, seeded_at = now(), topped_at = now()
		WHERE cell_id = $1`, id, target, walkM)
	return err
}

// RetireNodesInCell takes one source's active nodes in the cell off the map.
func RetireNodesInCell(ctx context.Context, q db.Querier, b Box, source string) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.nodes SET status = 'retired', updated_at = now()
		WHERE status = 'active' AND source = $5
		  AND ST_Intersects(location::geometry, `+cellEnvelope+`)`,
		b.MinLat, b.MinLon, b.MaxLat, b.MaxLon, source)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CellsToTopUp lists seeded cells holding fewer seed nodes than their target that the tick
// hasn't looked at for a while, longest-waiting first.
func CellsToTopUp(ctx context.Context, q db.Querier, source string, waitMinutes float64, limit int) ([]SeedCellRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.cell_id, c.min_lat, c.min_lon, c.max_lat, c.max_lon, c.target, c.walk_m, c.seeded_at
		FROM td.seed_cells c
		WHERE c.seeded_at IS NOT NULL AND c.target > 0
		  AND (c.topped_at IS NULL OR c.topped_at < now() - make_interval(secs => $2 * 60))
		  AND c.target > (
		      SELECT count(*) FROM td.nodes n
		      WHERE n.status = 'active' AND n.source = $1
		        AND ST_Intersects(n.location::geometry,
		                          ST_MakeEnvelope(c.min_lon, c.min_lat, c.max_lon, c.max_lat, 4326)))
		ORDER BY c.topped_at NULLS FIRST
		LIMIT $3`, source, waitMinutes, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SeedCellRow
	for rows.Next() {
		var r SeedCellRow
		if err := rows.Scan(&r.ID, &r.Box.MinLat, &r.Box.MinLon, &r.Box.MaxLat, &r.Box.MaxLon, &r.Target,
			&r.WalkM, &r.SeededAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SeededLandCells counts cells that hold (or should hold) seed nodes, and picks one at random
// (for a rare node); ok is false when there are none.
func SeededLandCells(ctx context.Context, q db.Querier) (count int, random SeedCellRow, ok bool, err error) {
	if err = q.QueryRowContext(ctx,
		`SELECT count(*) FROM td.seed_cells WHERE seeded_at IS NOT NULL AND target > 0`).Scan(&count); err != nil || count == 0 {
		return count, random, false, err
	}
	err = q.QueryRowContext(ctx, `
		SELECT cell_id, min_lat, min_lon, max_lat, max_lon, target, walk_m, seeded_at
		FROM td.seed_cells WHERE seeded_at IS NOT NULL AND target > 0
		ORDER BY random() LIMIT 1`).
		Scan(&random.ID, &random.Box.MinLat, &random.Box.MinLon, &random.Box.MaxLat, &random.Box.MaxLon,
			&random.Target, &random.WalkM, &random.SeededAt)
	if errors.Is(err, sql.ErrNoRows) {
		return count, random, false, nil
	}
	return count, random, err == nil, err
}

// RetireEndedNodes takes nodes of one source off the map once they expire or are depleted.
// Rare nodes don't respawn; the seeder makes new ones elsewhere.
func RetireEndedNodes(ctx context.Context, q db.Querier, source string, depleted float64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.nodes SET status = 'retired', updated_at = now()
		WHERE status = 'active' AND source = $1 AND (expires_at <= now() OR reserve < $2)`, source, depleted)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetNodeSourceRef moves a node's origin note (a respawned seed node's new cell).
func SetNodeSourceRef(ctx context.Context, q db.Querier, id int64, ref string) error {
	_, err := q.ExecContext(ctx, `UPDATE td.nodes SET source_ref = $2 WHERE id = $1`, id, ref)
	return err
}

// NodeSource reads where a node came from.
func NodeSource(ctx context.Context, q db.Querier, id int64) (string, error) {
	var source string
	err := q.QueryRowContext(ctx, `SELECT source FROM td.nodes WHERE id = $1`, id).Scan(&source)
	return source, err
}

type SeedArea struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Box  Box    `json:"-"`
}

func SeedAreaByID(ctx context.Context, q db.Querier, id string) (SeedArea, error) {
	a := SeedArea{ID: id}
	err := q.QueryRowContext(ctx, `
		SELECT name, min_lat, min_lon, max_lat, max_lon FROM td.seed_areas WHERE id = $1`, id).
		Scan(&a.Name, &a.Box.MinLat, &a.Box.MinLon, &a.Box.MaxLat, &a.Box.MaxLon)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func SeedAreas(ctx context.Context, q db.Querier) ([]SeedArea, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, min_lat, min_lon, max_lat, max_lon FROM td.seed_areas ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SeedArea
	for rows.Next() {
		var a SeedArea
		if err := rows.Scan(&a.ID, &a.Name, &a.Box.MinLat, &a.Box.MinLon, &a.Box.MaxLat, &a.Box.MaxLon); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SeedTotals is the admin readout: seeded cells and active seed nodes everywhere.
func SeedTotals(ctx context.Context, q db.Querier, source string) (cells, nodes int, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM td.seed_cells WHERE seeded_at IS NOT NULL),
		       (SELECT count(*) FROM td.nodes WHERE status = 'active' AND source = $1)`, source).
		Scan(&cells, &nodes)
	return cells, nodes, err
}

// SeedCellInfo reads a cell's row without locking it; ok is false if it was never looked at.
func SeedCellInfo(ctx context.Context, q db.Querier, id string) (SeedCellRow, bool, error) {
	r := SeedCellRow{ID: id}
	err := q.QueryRowContext(ctx, `
		SELECT min_lat, min_lon, max_lat, max_lon, target, walk_m, seeded_at
		FROM td.seed_cells WHERE cell_id = $1`, id).
		Scan(&r.Box.MinLat, &r.Box.MinLon, &r.Box.MaxLat, &r.Box.MaxLon, &r.Target, &r.WalkM, &r.SeededAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}
