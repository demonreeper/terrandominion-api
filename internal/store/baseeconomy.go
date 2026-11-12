package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Faction bases, B4 (base-building-plan.md 5, 6, 7, 10): Collector Array links, the vault log, the Uplink, visits.

// CollectorBase is a base with Collector Arrays online, and how many nodes they tether in all.
type CollectorBase struct {
	ID        int64
	FactionID int
	Lat, Lon  float64
	ZoneM     float64
	Mks       []int
	LastVisit time.Time
}

// CollectorBases lists every standing base with an online Collector Array, with each array's Mk.
func CollectorBases(ctx context.Context, q db.Querier) ([]CollectorBase, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.id, b.faction_id, ST_Y(b.location::geometry), ST_X(b.location::geometry), z.radius_m, b.last_visit_at,
		       string_agg(s.mk::text, ',' ORDER BY s.ring, s.idx)
		FROM td.bases b JOIN td.base_zones z ON z.base_id = b.id
		JOIN td.base_slots s ON s.base_id = b.id AND s.module = 'collector' AND s.online
		WHERE b.status = 'active'
		GROUP BY b.id, z.radius_m`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CollectorBase
	for rows.Next() {
		var c CollectorBase
		var mks string
		if err := rows.Scan(&c.ID, &c.FactionID, &c.Lat, &c.Lon, &c.ZoneM, &c.LastVisit, &mks); err != nil {
			return nil, err
		}
		for _, m := range strings.Split(mks, ",") {
			if n, err := strconv.Atoi(m); err == nil {
				c.Mks = append(c.Mks, n)
			}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func CollectorLinkCount(ctx context.Context, q db.Querier, baseID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.base_collector_links WHERE base_id = $1`, baseID).Scan(&n)
	return n, err
}

func InsertCollectorLink(ctx context.Context, q db.Querier, baseID, nodeID int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.base_collector_links (node_id, base_id, next_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		nodeID, baseID, now)
	return err
}

// TrimCollectorLinks keeps at most keep links for a base (an array went offline or was packed away).
func TrimCollectorLinks(ctx context.Context, q db.Querier, baseID int64, keep int) error {
	_, err := q.ExecContext(ctx, `
		DELETE FROM td.base_collector_links WHERE base_id = $1 AND node_id NOT IN (
		    SELECT node_id FROM td.base_collector_links WHERE base_id = $1 ORDER BY node_id LIMIT $2)`, baseID, keep)
	return err
}

// FreeNodesInZone lists active nodes inside a zone that no Tether Node or base holds, locked.
func FreeNodesInZone(ctx context.Context, q db.Querier, lat, lon, radiusM, minReserve float64, limit int) ([]FreeNode, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id, n.resource FROM td.nodes n
		WHERE n.status = 'active' AND n.reserve >= $4
		  AND ST_DWithin(n.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)
		  AND NOT EXISTS (SELECT 1 FROM td.tether_links l WHERE l.node_id = n.id)
		  AND NOT EXISTS (SELECT 1 FROM td.base_collector_links c WHERE c.node_id = n.id)
		ORDER BY random() LIMIT $5
		FOR UPDATE OF n SKIP LOCKED`, lat, lon, radiusM, minReserve, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FreeNode
	for rows.Next() {
		var c FreeNode
		if err := rows.Scan(&c.NodeID, &c.Resource); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CollectorLink is one node a base tethers.
type CollectorLink struct {
	NodeID int64
	BaseID int64
}

// DueCollectorLinks lists links whose next harvest is due.
func DueCollectorLinks(ctx context.Context, q db.Querier, now time.Time, limit int) ([]CollectorLink, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT node_id, base_id FROM td.base_collector_links WHERE next_at <= $1 ORDER BY next_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CollectorLink
	for rows.Next() {
		var c CollectorLink
		if err := rows.Scan(&c.NodeID, &c.BaseID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func SetCollectorLinkNext(ctx context.Context, q db.Querier, nodeID int64, next time.Time, gathered int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_collector_links SET next_at = $2, gathered = gathered + $3 WHERE node_id = $1`, nodeID, next, gathered)
	return err
}

func DeleteCollectorLink(ctx context.Context, q db.Querier, nodeID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.base_collector_links WHERE node_id = $1`, nodeID)
	return err
}

// CollectorNodes lists the nodes a base tethers (for the card and the map).
func CollectorNodes(ctx context.Context, q db.Querier, baseID int64) ([]TetherLinkView, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id, n.resource, ST_Y(n.location::geometry), ST_X(n.location::geometry)
		FROM td.base_collector_links c JOIN td.nodes n ON n.id = c.node_id WHERE c.base_id = $1 ORDER BY n.id`, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TetherLinkView{}
	for rows.Next() {
		var v TetherLinkView
		if err := rows.Scan(&v.NodeID, &v.Resource, &v.Lat, &v.Lon); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TetherLinkView is a linked node as the map draws the line to it.
type TetherLinkView struct {
	NodeID   int64   `json:"nodeId"`
	Resource string  `json:"resource"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

// BaseHasModule says whether a base has an online module of a kind.
func BaseHasModule(ctx context.Context, q db.Querier, baseID int64, module string) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.base_slots WHERE base_id = $1 AND module = $2 AND online)`, baseID, module).Scan(&ok)
	return ok, err
}

// HasStandingBase says whether a faction has a standing (active) base in a region.
func HasStandingBase(ctx context.Context, q db.Querier, factionID int, region string) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.bases WHERE faction_id = $1 AND region = $2 AND status = 'active')`,
		factionID, region).Scan(&ok)
	return ok, err
}

// OnSite is a member standing inside one of their faction's base zones.
type OnSite struct {
	PlayerID int64
	BaseID   int64
}

// MembersOnSiteAll lists members whose last fix (no older than since) is inside a standing base of their faction.
func MembersOnSiteAll(ctx context.Context, q db.Querier, since time.Time) ([]OnSite, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT ON (m.player_id) m.player_id, b.id
		FROM td.faction_members m
		JOIN td.player_last_fix f ON f.player_id = m.player_id
		JOIN td.bases b ON b.faction_id = m.faction_id AND b.status <> 'destroyed'
		JOIN td.base_zones z ON z.base_id = b.id
		WHERE f.fixed_at > $1
		  AND ST_DWithin(b.location, ST_SetSRID(ST_MakePoint(f.lon, f.lat), 4326)::geography, z.radius_m)
		ORDER BY m.player_id, b.id`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OnSite
	for rows.Next() {
		var o OnSite
		if err := rows.Scan(&o.PlayerID, &o.BaseID); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MarkVisited: a member stood on site, so the base is looked after (upkeep, section 7) and any fade ends.
func MarkVisited(ctx context.Context, q db.Querier, baseIDs []int64, now time.Time) error {
	if len(baseIDs) == 0 {
		return nil
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.bases SET last_visit_at = $2, faded_at = NULL WHERE id = ANY($1)`, baseIDs, now)
	return err
}

// VaultEntry is one line of the VAULT LOG.
type VaultEntry struct {
	At      time.Time  `json:"at"`
	Player  *PlayerRef `json:"player,omitempty"`
	Kind    string     `json:"kind"`
	Amounts Resources4 `json:"amounts"`
}

func VaultLog(ctx context.Context, q db.Querier, baseID int64, limit int) ([]VaultEntry, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT l.at, td.player_code(p.public_code, p.status), td.player_name(p.callsign, p.public_code, p.status),
		       l.kind, l.alloy, l.crystal, l.data, l.credit
		FROM td.base_vault_log l LEFT JOIN td.players p ON p.id = l.player_id
		WHERE l.base_id = $1 ORDER BY l.at DESC, l.id DESC LIMIT $2`, baseID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VaultEntry{}
	for rows.Next() {
		var e VaultEntry
		var code, name *string
		if err := rows.Scan(&e.At, &code, &name, &e.Kind, &e.Amounts.Alloy, &e.Amounts.Crystal, &e.Amounts.Data,
			&e.Amounts.Credit); err != nil {
			return nil, err
		}
		if name != nil {
			e.Player = &PlayerRef{Name: *name}
			if code != nil {
				e.Player.Code = *code
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
