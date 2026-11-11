// loadcountries fills td.countries from Natural Earth's admin-0 countries (public domain), once per database
// (docs/social-signal-plan.md Part B, R1). It replaces the real countries and keeps admin test regions.
//
//	go run ./cmd/loadcountries [path-or-url]
//
// The default is ne_10m_admin_0_countries.geojson from the natural-earth-vector repository on GitHub. The 1:50m
// set has two polygons for the Maldives; 1:10m has 176. TD_DATABASE_URL names the database (from .env).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultSource = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_10m_admin_0_countries.geojson"

type collection struct {
	Features []struct {
		Properties map[string]any  `json:"properties"`
		Geometry   json.RawMessage `json:"geometry"`
	} `json:"features"`
}

func main() {
	source := defaultSource
	if len(os.Args) > 1 {
		source = os.Args[1]
	}
	raw, err := read(source)
	if err != nil {
		fail(err)
	}
	var c collection
	if err := json.Unmarshal(raw, &c); err != nil {
		fail(err)
	}
	conn, err := sql.Open("pgx", os.Getenv("TD_DATABASE_URL"))
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		fail(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM td.countries WHERE NOT test`); err != nil {
		fail(err)
	}
	loaded, skipped := 0, 0
	for _, f := range c.Features {
		code := text(f.Properties, "ISO_A2_EH")
		if len(code) != 2 || code == "-9" || strings.ToUpper(code) != code {
			skipped++
			continue
		}
		name := text(f.Properties, "NAME_EN")
		if name == "" {
			name = text(f.Properties, "NAME")
		}
		// A territory filed under its sovereign's code (Indian Ocean Ter. as AU) is not the country itself.
		main := text(f.Properties, "ISO_A2") == code || text(f.Properties, "ADMIN") == text(f.Properties, "SOVEREIGNT")
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO td.countries (code, name, main, geom)
			VALUES ($1, $2, $3, ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($4), 4326)), 3)))`,
			code, name, main, string(f.Geometry)); err != nil {
			fail(fmt.Errorf("%s %s: %w", code, name, err))
		}
		loaded++
	}
	if err := tx.Commit(); err != nil {
		fail(err)
	}
	fmt.Printf("loaded %d country shapes, skipped %d without a two-letter code\n", loaded, skipped)
}

func read(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http") {
		client := &http.Client{Timeout: 5 * time.Minute}
		res, err := client.Get(source)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", source, res.Status)
		}
		return io.ReadAll(res.Body)
	}
	return os.ReadFile(source)
}

func text(p map[string]any, key string) string {
	if v, ok := p[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "loadcountries:", err)
	os.Exit(1)
}
