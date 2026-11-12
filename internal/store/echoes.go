package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Signal echoes (docs/no-ar-support-plan.md milestone 7) and the CODEX of lines each player has read.

// Echo is one echo standing in the Undersignal. Its line is not sent until it is read.
type Echo struct {
	ID        int64   `json:"id"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	DistanceM float64 `json:"distanceM"`
	// Known: the asker already has this echo's line in their CODEX.
	Known bool `json:"known"`
	Set   int  `json:"-"`
	Line  int  `json:"-"`
}

// LockEchoCell holds the cell for this transaction, so two players arriving at once don't both fill it.
func LockEchoCell(ctx context.Context, q db.Querier, cell string) error {
	_, err := q.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('td.echo:' || $1))`, cell)
	return err
}

// ClearMovedEchoes removes a cell's echoes whose time is up, and says how many are left.
func ClearMovedEchoes(ctx context.Context, q db.Querier, cell string, now time.Time) (int, error) {
	if _, err := q.ExecContext(ctx, `DELETE FROM td.signal_echoes WHERE cell = $1 AND moves_at <= $2`, cell, now); err != nil {
		return 0, err
	}
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.signal_echoes WHERE cell = $1`, cell).Scan(&n)
	return n, err
}

// EchoAnchor is a place an echo may stand beside: an active node, Tether Node or faction base.
type EchoAnchor struct{ Lat, Lon float64 }

// EchoAnchors picks up to limit active nodes and Tether Nodes inside a cell's box, in random order.
func EchoAnchors(ctx context.Context, q db.Querier, box Box, limit int) ([]EchoAnchor, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT lat, lon FROM (
		    SELECT ST_Y(n.location::geometry) AS lat, ST_X(n.location::geometry) AS lon
		    FROM td.nodes n
		    WHERE n.status = 'active' AND n.location::geometry && `+cellEnvelope+`
		    UNION ALL
		    SELECT ST_Y(t.location::geometry), ST_X(t.location::geometry)
		    FROM td.tether_nodes t
		    WHERE t.status = 'active' AND t.location::geometry && `+cellEnvelope+`
		    UNION ALL
		    -- Faction bases too (base-building-plan.md 9, B7): same placement and CODEX rules.
		    SELECT ST_Y(b.location::geometry), ST_X(b.location::geometry)
		    FROM td.bases b
		    WHERE b.status <> 'destroyed' AND b.location::geometry && `+cellEnvelope+`
		) a
		ORDER BY random()
		LIMIT $5`, box.MinLat, box.MinLon, box.MaxLat, box.MaxLon, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EchoAnchor, 0, limit)
	for rows.Next() {
		var a EchoAnchor
		if err := rows.Scan(&a.Lat, &a.Lon); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func CreateEcho(ctx context.Context, q db.Querier, cell string, lat, lon float64, set, line int, movesAt time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.signal_echoes (location, cell, echo_set, line, moves_at)
		VALUES (ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3, $4, $5, $6)`,
		lat, lon, cell, set, line, movesAt)
	return err
}

// NearbyEchoes lists the echoes standing around a point, nearest first, each marked known when the
// asker already has its line.
func NearbyEchoes(ctx context.Context, q db.Querier, playerID int64, lat, lon, radiusM float64, now time.Time, limit int) ([]Echo, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g)
		SELECT e.id, ST_Y(e.location::geometry), ST_X(e.location::geometry), ST_Distance(e.location, here.g),
		       e.echo_set, e.line,
		       EXISTS (SELECT 1 FROM td.echo_reads r
		               WHERE r.player_id = $5 AND r.echo_set = e.echo_set AND r.line = e.line)
		FROM td.signal_echoes e CROSS JOIN here
		WHERE e.moves_at > $6 AND ST_DWithin(e.location, here.g, $3)
		ORDER BY e.location <-> here.g
		LIMIT $4`, lat, lon, radiusM, limit, playerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Echo, 0, 8)
	for rows.Next() {
		var e Echo
		if err := rows.Scan(&e.ID, &e.Lat, &e.Lon, &e.DistanceM, &e.Set, &e.Line, &e.Known); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EchoByID reads one echo still standing; ErrNotFound once it has moved.
func EchoByID(ctx context.Context, q db.Querier, id int64, now time.Time) (Echo, error) {
	var e Echo
	err := q.QueryRowContext(ctx, `
		SELECT id, ST_Y(location::geometry), ST_X(location::geometry), echo_set, line
		FROM td.signal_echoes WHERE id = $1 AND moves_at > $2`, id, now).
		Scan(&e.ID, &e.Lat, &e.Lon, &e.Set, &e.Line)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// AddEchoRead puts a line in the player's CODEX and says whether it is new there.
func AddEchoRead(ctx context.Context, q db.Querier, playerID int64, set, line int) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.echo_reads (player_id, echo_set, line) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, playerID, set, line)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// EchoReads lists the lines a player has found, as set → lines.
func EchoReads(ctx context.Context, q db.Querier, playerID int64) (map[int][]int, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT echo_set, line FROM td.echo_reads WHERE player_id = $1 ORDER BY echo_set, line`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var set, line int
		if err := rows.Scan(&set, &line); err != nil {
			return nil, err
		}
		out[set] = append(out[set], line)
	}
	return out, rows.Err()
}
