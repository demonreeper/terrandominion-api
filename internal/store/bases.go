package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
)

// Faction bases (docs/base-building-plan.md), B0: bases, zones, slots, flags and vaults.

// LockBasePlacement serialises everything that places, grows or flags a base, so two at once can't both pass the
// spacing and zone rules.
func LockBasePlacement(ctx context.Context, q db.Querier) error {
	_, err := q.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('td.bases.placement'))`)
	return err
}

// NearBases lists the standing bases within m metres of a point, with their zone radius.
func NearBases(ctx context.Context, q db.Querier, lat, lon, m float64) ([]game.NearBase, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.id, b.faction_id, b.name, ST_Y(b.location::geometry), ST_X(b.location::geometry), z.radius_m
		FROM td.bases b JOIN td.base_zones z ON z.base_id = b.id
		WHERE b.status <> 'destroyed'
		  AND ST_DWithin(b.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)`, lat, lon, m)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.NearBase
	for rows.Next() {
		var b game.NearBase
		if err := rows.Scan(&b.ID, &b.FactionID, &b.Name, &b.Lat, &b.Lon, &b.ZoneM); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// NearFlags lists the open, unexpired proposals within m metres of a point.
func NearFlags(ctx context.Context, q db.Querier, lat, lon, m float64, now time.Time) ([]game.NearFlag, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, faction_id, ST_Y(location::geometry), ST_X(location::geometry)
		FROM td.base_proposals
		WHERE status = 'open' AND expires_at > $4
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)`, lat, lon, m, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.NearFlag
	for rows.Next() {
		var f game.NearFlag
		if err := rows.Scan(&f.ID, &f.FactionID, &f.Lat, &f.Lon); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// BaseSpotMap asks the OSM layers about a spot: on land, within walkM of a walkable way, not inside a building,
// water or airfield, and not within sensitiveM of a sensitive place.
func BaseSpotMap(ctx context.Context, q db.Querier, lat, lon, walkM, sensitiveM float64) (game.SpotMap, error) {
	var m game.SpotMap
	err := q.QueryRowContext(ctx, `
		WITH p AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326) AS g)
		SELECT
		    EXISTS (SELECT 1 FROM td.osm_land l, p WHERE ST_Intersects(l.geom, p.g)),
		    EXISTS (SELECT 1 FROM td.osm_walkways w, p WHERE ST_DWithin(w.geom::geography, p.g::geography, $3)),
		    EXISTS (SELECT 1 FROM td.osm_blocked x, p WHERE ST_Intersects(x.geom, p.g)),
		    EXISTS (SELECT 1 FROM td.osm_sensitive s, p WHERE ST_DWithin(s.geom::geography, p.g::geography, $4)),
		    coalesce((SELECT x.kind FROM td.osm_blocked x, p WHERE ST_Intersects(x.geom, p.g)
		              ORDER BY x.kind = 'building' DESC LIMIT 1), '')
		FROM p`, lat, lon, walkM, sensitiveM).
		Scan(&m.OnLand, &m.NearWalkway, &m.InBlocked, &m.NearSensitive, &m.BlockedKind)
	return m, err
}

// BasesHeld counts a faction's standing bases and open flags in a region (the base cap counts both), leaving out
// one proposal (the one being approved; 0 for none).
func BasesHeld(ctx context.Context, q db.Querier, factionID int, region string, exceptFlag int64, now time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM td.bases WHERE faction_id = $1 AND region = $2 AND status <> 'destroyed')
		     + (SELECT count(*) FROM td.base_proposals
		        WHERE faction_id = $1 AND region = $2 AND status = 'open' AND expires_at > $4 AND id <> $3)`,
		factionID, region, exceptFlag, now).Scan(&n)
	return n, err
}

// ActivePlayersInRegion is the region's signal population for the base cap (owner, 2026-11-12): every player,
// freelancers included, whose last accepted fix lies in the region (its shape, or within maxKm of it) and is no
// older than days.
func ActivePlayersInRegion(ctx context.Context, q db.Querier, region string, days, maxKm float64, now time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.player_last_fix f
		JOIN td.players p ON p.id = f.player_id
		WHERE p.type = 'player' AND p.status = 'active'
		  AND f.fixed_at > $3::timestamptz - make_interval(secs => $2::float8 * 86400)
		  AND EXISTS (SELECT 1 FROM td.countries c
		              WHERE c.code = $1
		                AND ST_DWithin(c.geom::geography, ST_SetSRID(ST_MakePoint(f.lon, f.lat), 4326)::geography,
		                               $4::float8 * 1000))`, region, days, now, maxKm).Scan(&n)
	return n, err
}

// NewBase is a base to insert.
type NewBase struct {
	FactionID  int
	Region     string
	Name       string
	Tier       int
	Lat, Lon   float64
	Status     string
	CoreHealth int
	ZoneM      float64
	ProposalID int64
	RaisedBy   int64
}

// InsertBase adds a base with its zone and an empty vault. Slots are set with SetBaseSlots.
func InsertBase(ctx context.Context, q db.Querier, b NewBase) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.bases (faction_id, region, name, tier, location, status, core_health, proposal_id, raised_by)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography, $7, $8, NULLIF($9, 0), NULLIF($10, 0))
		RETURNING id`,
		b.FactionID, b.Region, b.Name, b.Tier, b.Lat, b.Lon, b.Status, b.CoreHealth, b.ProposalID, b.RaisedBy).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := SetBaseZone(ctx, q, id, b.ZoneM); err != nil {
		return 0, err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO td.base_vaults (base_id) VALUES ($1)`, id)
	return id, err
}

// SetBaseZone sets a base's zone radius and its circle.
func SetBaseZone(ctx context.Context, q db.Querier, baseID int64, radiusM float64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.base_zones (base_id, radius_m, area)
		SELECT id, $2, ST_Buffer(location, $2, 'quad_segs=16')::geography FROM td.bases WHERE id = $1
		ON CONFLICT (base_id) DO UPDATE SET radius_m = EXCLUDED.radius_m, area = EXCLUDED.area, updated_at = now()`,
		baseID, radiusM)
	return err
}

// SetBaseSlots makes the base's slots exactly this layout: missing slots are added empty, slots outside it
// removed. It returns the modules that stood in removed slots.
func SetBaseSlots(ctx context.Context, q db.Querier, baseID int64, layout []game.BaseSlot) ([]string, error) {
	rings := make([]int64, len(layout))
	idxs := make([]int64, len(layout))
	for i, sl := range layout {
		rings[i], idxs[i] = int64(sl.Ring), int64(sl.Index)
	}
	rows, err := q.QueryContext(ctx, `
		DELETE FROM td.base_slots s
		WHERE s.base_id = $1
		  AND NOT EXISTS (SELECT 1 FROM unnest($2::bigint[], $3::bigint[]) AS l(ring, idx)
		                  WHERE l.ring = s.ring AND l.idx = s.idx)
		RETURNING module`, baseID, rings, idxs)
	if err != nil {
		return nil, err
	}
	var removed []string
	for rows.Next() {
		var m sql.NullString
		if err := rows.Scan(&m); err != nil {
			rows.Close()
			return nil, err
		}
		if m.Valid {
			removed = append(removed, m.String)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.base_slots (base_id, ring, idx)
		SELECT $1, ring, idx FROM unnest($2::bigint[], $3::bigint[]) AS l(ring, idx)
		ON CONFLICT DO NOTHING`, baseID, rings, idxs)
	return removed, err
}

// BaseRow is a base as the server keeps it.
type BaseRow struct {
	ID         int64      `json:"id"`
	Faction    string     `json:"faction"`
	FactionID  int        `json:"-"`
	Region     string     `json:"region"`
	Name       string     `json:"name"`
	Tier       int        `json:"tier"`
	TierName   string     `json:"tierName"`
	Lat        float64    `json:"lat"`
	Lon        float64    `json:"lon"`
	Status     string     `json:"status"`
	CoreHealth int        `json:"coreHealth"`
	ZoneM      float64    `json:"zoneM"`
	Slots      int        `json:"slots"`
	FadedAt    *time.Time `json:"fadedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

const baseCols = `b.id, f.code, b.faction_id, b.region, b.name, b.tier, ST_Y(b.location::geometry), ST_X(b.location::geometry),
	b.status, b.core_health, COALESCE(z.radius_m, 0),
	(SELECT count(*) FROM td.base_slots s WHERE s.base_id = b.id), b.faded_at, b.created_at`

const baseFrom = ` FROM td.bases b JOIN td.factions f ON f.id = b.faction_id LEFT JOIN td.base_zones z ON z.base_id = b.id`

func scanBase(sc interface{ Scan(...any) error }) (BaseRow, error) {
	var b BaseRow
	err := sc.Scan(&b.ID, &b.Faction, &b.FactionID, &b.Region, &b.Name, &b.Tier, &b.Lat, &b.Lon,
		&b.Status, &b.CoreHealth, &b.ZoneM, &b.Slots, &b.FadedAt, &b.CreatedAt)
	b.TierName = game.BaseTierName(b.Tier)
	return b, err
}

// BaseForUpdate reads and locks one standing base.
func BaseForUpdate(ctx context.Context, q db.Querier, id int64) (BaseRow, error) {
	b, err := scanBase(q.QueryRowContext(ctx, `SELECT `+baseCols+baseFrom+
		` WHERE b.id = $1 AND b.status <> 'destroyed' FOR UPDATE OF b`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// SetBaseTier changes a base's tier and core health (the zone and slots are set apart).
func SetBaseTier(ctx context.Context, q db.Querier, id int64, tier, core int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET tier = $2, core_health = $3, updated_at = now() WHERE id = $1`,
		id, tier, core)
	return err
}

// DeleteBase removes a base and everything of it (zone, slots, projects, vault, log). Its proposal stays as history.
func DeleteBase(ctx context.Context, q db.Querier, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.bases WHERE id = $1`, id)
	return err
}

// FlagRow is a proposal (flag) as the server keeps it.
type FlagRow struct {
	ID         int64     `json:"id"`
	Faction    string    `json:"faction"`
	Region     string    `json:"region"`
	ProposerID int64     `json:"proposerId"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// ListBases lists a region's standing bases (every region with ""), newest first.
func ListBases(ctx context.Context, q db.Querier, region string) ([]BaseRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+baseCols+baseFrom+`
		WHERE b.status <> 'destroyed' AND ($1 = '' OR b.region = $1) ORDER BY b.created_at DESC`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BaseRow
	for rows.Next() {
		b, err := scanBase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListOpenFlags lists a region's open proposals (every region with "").
func ListOpenFlags(ctx context.Context, q db.Querier, region string, now time.Time) ([]FlagRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, f.code, p.region, p.proposer_id, ST_Y(p.location::geometry), ST_X(p.location::geometry),
		       p.status, p.created_at, p.expires_at
		FROM td.base_proposals p JOIN td.factions f ON f.id = p.faction_id
		WHERE p.status = 'open' AND p.expires_at > $2 AND ($1 = '' OR p.region = $1)
		ORDER BY p.created_at DESC`, region, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FlagRow
	for rows.Next() {
		var f FlagRow
		if err := rows.Scan(&f.ID, &f.Faction, &f.Region, &f.ProposerID, &f.Lat, &f.Lon, &f.Status,
			&f.CreatedAt, &f.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ReleaseFlag takes an open flag down (admin). It reports whether there was one to release.
func ReleaseFlag(ctx context.Context, q db.Querier, id, by int64, now time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.base_proposals SET status = 'released', decided_by = $2, decided_at = $3
		WHERE id = $1 AND status = 'open'`, id, by, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
