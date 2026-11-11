package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Regions from GPS (docs/social-signal-plan.md Part B, R1).

// Country is a region a fix resolved to.
type Country struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Test bool   `json:"test,omitempty"`
	Live bool   `json:"live"`
}

// CountryAt is the country a point is in, or the nearest within maxKm (a reef, a lagoon, the sea). A test region
// counts only for a point inside it, and then wins over the country around it; only when withTest (never on
// production). ErrNotFound
// when nothing is near.
func CountryAt(ctx context.Context, q db.Querier, lat, lon, maxKm float64, withTest bool) (Country, error) {
	var c Country
	// The box prefilter uses the index; one degree is at most ~111 km.
	err := q.QueryRowContext(ctx, `
		WITH pt AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326) AS g)
		SELECT c.code, COALESCE((SELECT m.name FROM td.countries m WHERE m.code = c.code AND m.main ORDER BY m.id LIMIT 1), c.name),
		       c.test, EXISTS (SELECT 1 FROM td.live_regions l WHERE l.code = c.code) OR c.test
		FROM td.countries c, pt
		WHERE (NOT c.test OR ($4 AND ST_Covers(c.geom, pt.g)))
		  AND c.geom && ST_Expand(pt.g, $3 / 111.0 + 0.01)
		  AND ST_DWithin(c.geom::geography, pt.g::geography, $3 * 1000)
		ORDER BY ST_Covers(c.geom, pt.g) DESC, c.test DESC, ST_Distance(c.geom::geography, pt.g::geography)
		LIMIT 1`, lat, lon, maxKm, withTest).Scan(&c.Code, &c.Name, &c.Test, &c.Live)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// CountryName is a region's name for players ("" when unknown).
func CountryName(ctx context.Context, q db.Querier, code string) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `
		SELECT name FROM td.countries WHERE code = $1 ORDER BY test DESC, main DESC, id LIMIT 1`, code).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// LiveRegions lists the countries that can be claimed.
func LiveRegions(ctx context.Context, q db.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT code FROM td.live_regions ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func SetLiveRegion(ctx context.Context, q db.Querier, code string, live bool) error {
	var err error
	if live {
		_, err = q.ExecContext(ctx, `INSERT INTO td.live_regions (code) VALUES ($1) ON CONFLICT DO NOTHING`, code)
	} else {
		_, err = q.ExecContext(ctx, `DELETE FROM td.live_regions WHERE code = $1`, code)
	}
	return err
}

// TestRegion is an admin's test region on dev: a code XA..XZ, a name and a ring of points (lat, lon).
type TestRegion struct {
	Code   string       `json:"code"`
	Name   string       `json:"name"`
	Points [][2]float64 `json:"points,omitempty"`
}

// SaveTestRegion replaces a test region's shape (a closed ring is made from the points).
func SaveTestRegion(ctx context.Context, q db.Querier, r TestRegion) error {
	if len(r.Points) < 3 {
		return fmt.Errorf("a test region needs at least 3 points")
	}
	coords := make([][2]float64, 0, len(r.Points)+1)
	for _, p := range r.Points {
		coords = append(coords, [2]float64{p[1], p[0]}) // GeoJSON is lon, lat
	}
	coords = append(coords, coords[0])
	shape, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][2]float64{coords}})
	if _, err := q.ExecContext(ctx, `DELETE FROM td.countries WHERE test AND code = $1`, r.Code); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.countries (code, name, test, geom)
		VALUES ($1, $2, true, ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($3), 4326)), 3)))`,
		r.Code, r.Name, string(shape))
	return err
}

func DeleteTestRegion(ctx context.Context, q db.Querier, code string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.countries WHERE test AND code = $1`, code)
	return err
}

func TestRegions(ctx context.Context, q db.Querier) ([]TestRegion, error) {
	rows, err := q.QueryContext(ctx, `SELECT code, name FROM td.countries WHERE test ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TestRegion{}
	for rows.Next() {
		var r TestRegion
		if err := rows.Scan(&r.Code, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
