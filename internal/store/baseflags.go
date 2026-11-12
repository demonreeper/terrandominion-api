package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Faction bases, B1 (base-building-plan.md 1b): flags (proposals), the Commander's answer, the Spire project, and
// what the map shows.

// TakenBaseNames lists every name a standing base or an open flag holds.
func TakenBaseNames(ctx context.Context, q db.Querier, now time.Time) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT name FROM td.bases WHERE status <> 'destroyed'
		UNION SELECT name FROM td.base_proposals WHERE status = 'open' AND expires_at > $1`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// ProposerState is what the anti-spam limits read: an open flag, and the last declined or expired one.
func ProposerState(ctx context.Context, q db.Querier, playerID int64, now time.Time) (open bool, lastRefused *time.Time, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.base_proposals WHERE proposer_id = $1 AND status = 'open' AND expires_at > $2),
		       (SELECT max(decided_at) FROM td.base_proposals WHERE proposer_id = $1 AND status IN ('declined', 'expired'))`,
		playerID, now).Scan(&open, &lastRefused)
	return open, lastRefused, err
}

// BranchProposalsOff says whether the Commander turned proposals off for the faction's region.
func BranchProposalsOff(ctx context.Context, q db.Querier, factionID int, region string) (bool, error) {
	var off bool
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE((SELECT base_proposals_off FROM td.faction_branches
		                 WHERE faction_id = $1 AND country_code = $2::text), false)`, factionID, region).Scan(&off)
	return off, err
}

// SetBranchProposalsOff turns proposals off (or on) for a branch.
func SetBranchProposalsOff(ctx context.Context, q db.Querier, branchID int64, off bool) error {
	_, err := q.ExecContext(ctx, `UPDATE td.faction_branches SET base_proposals_off = $2 WHERE id = $1`, branchID, off)
	return err
}

// NewFlag is a proposal to insert.
type NewFlag struct {
	FactionID  int
	Region     string
	ProposerID int64
	Name       string
	Lat, Lon   float64
	AccuracyM  float64
	ExpiresAt  time.Time
}

func InsertFlag(ctx context.Context, q db.Querier, f NewFlag) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.base_proposals (faction_id, region, proposer_id, name, location, accuracy_m, expires_at)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography, $7, $8) RETURNING id`,
		f.FactionID, f.Region, f.ProposerID, f.Name, f.Lat, f.Lon, f.AccuracyM, f.ExpiresAt).Scan(&id)
	return id, err
}

// Flag is a proposal as the PROPOSALS list and the map show it.
type Flag struct {
	ID         int64     `json:"id"`
	FactionID  int       `json:"-"`
	Faction    string    `json:"faction"`
	Color      string    `json:"color"`
	Region     string    `json:"region"`
	Name       string    `json:"name"`
	ProposerID int64     `json:"-"`
	Proposer   PlayerRef `json:"proposer"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	// Near is the closest standing bases, for the Commander (PROPOSALS only).
	Near []NearBaseRef `json:"near,omitempty"`
}

// NearBaseRef is a base near a flag: its name, faction and distance.
type NearBaseRef struct {
	Name      string  `json:"name"`
	Faction   string  `json:"faction"`
	DistanceM float64 `json:"distanceM"`
}

const flagCols = `p.id, p.faction_id, f.code, f.color, p.region, p.name, p.proposer_id, ` + `td.player_code(a.public_code, a.status), td.player_name(a.callsign, a.public_code, a.status),
	ST_Y(p.location::geometry), ST_X(p.location::geometry), p.status, p.created_at, p.expires_at`

const flagFrom = ` FROM td.base_proposals p JOIN td.factions f ON f.id = p.faction_id JOIN td.players a ON a.id = p.proposer_id`

func scanFlag(sc interface{ Scan(...any) error }) (Flag, error) {
	var x Flag
	err := sc.Scan(&x.ID, &x.FactionID, &x.Faction, &x.Color, &x.Region, &x.Name, &x.ProposerID, &x.Proposer.Code,
		&x.Proposer.Name, &x.Lat, &x.Lon, &x.Status, &x.CreatedAt, &x.ExpiresAt)
	return x, err
}

// FlagForUpdate reads and locks one proposal.
func FlagForUpdate(ctx context.Context, q db.Querier, id int64) (Flag, error) {
	x, err := scanFlag(q.QueryRowContext(ctx, `SELECT `+flagCols+flagFrom+` WHERE p.id = $1 FOR UPDATE OF p`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	return x, err
}

func queryFlags(ctx context.Context, q db.Querier, where string, args ...any) ([]Flag, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+flagCols+flagFrom+` WHERE `+where+` ORDER BY p.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Flag
	for rows.Next() {
		x, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// RegionFlags lists a faction's open flags in a region, oldest first.
func RegionFlags(ctx context.Context, q db.Querier, factionID int, region string, now time.Time) ([]Flag, error) {
	return queryFlags(ctx, q, `p.faction_id = $1 AND p.region = $2 AND p.status = 'open' AND p.expires_at > $3`,
		factionID, region, now)
}

// PlayerFlags lists a player's open flags (one at most).
func PlayerFlags(ctx context.Context, q db.Querier, playerID int64, now time.Time) ([]Flag, error) {
	return queryFlags(ctx, q, `p.proposer_id = $1 AND p.status = 'open' AND p.expires_at > $2`, playerID, now)
}

// FlagsNear lists every open flag within m metres (the map).
func FlagsNear(ctx context.Context, q db.Querier, lat, lon, m float64, now time.Time) ([]Flag, error) {
	return queryFlags(ctx, q, `p.status = 'open' AND p.expires_at > $4
		AND ST_DWithin(p.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)`, lat, lon, m, now)
}

// NearestBases names the standing bases closest to a point, nearest first.
func NearestBases(ctx context.Context, q db.Querier, lat, lon, m float64, limit int) ([]NearBaseRef, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.name, f.code, ST_Distance(b.location, pt.g)
		FROM td.bases b JOIN td.factions f ON f.id = b.faction_id,
		     (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g) pt
		WHERE b.status <> 'destroyed' AND ST_DWithin(b.location, pt.g, $3)
		ORDER BY 3 LIMIT $4`, lat, lon, m, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NearBaseRef
	for rows.Next() {
		var r NearBaseRef
		if err := rows.Scan(&r.Name, &r.Faction, &r.DistanceM); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DecideFlag answers an open proposal: approved (with the base), declined or expired.
func DecideFlag(ctx context.Context, q db.Querier, id int64, status string, by, baseID int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_proposals SET status = $2, decided_by = NULLIF($3, 0), decided_at = $4, base_id = NULLIF($5, 0)
		WHERE id = $1 AND status = 'open'`, id, status, by, now, baseID)
	return err
}

// DueFlags lists open flags whose time is up.
func DueFlags(ctx context.Context, q db.Querier, now time.Time, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.base_proposals WHERE status = 'open' AND expires_at <= $1 ORDER BY expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// NewProject is a base project to insert (funding, B2 gives toward it).
type NewProject struct {
	BaseID      int64
	Kind        string
	ToTier      int
	Ring, Index int
	Module      string
	Mk          int
	Bill        [4]int
	AssemblySec int
	StartedBy   int64
}

func InsertProject(ctx context.Context, q db.Querier, p NewProject) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.base_projects (base_id, kind, to_tier, ring, idx, module, mk, bill_alloy, bill_crystal, bill_data,
		                              bill_credit, assembly_sec, started_by)
		VALUES ($1, $2, NULLIF($3, 0), NULLIF($4, 0), CASE WHEN $4 = 0 THEN NULL ELSE $5::smallint END, NULLIF($6, ''),
		        NULLIF($7, 0), $8, $9, $10, $11, $12, NULLIF($13, 0)) RETURNING id`,
		p.BaseID, p.Kind, p.ToTier, p.Ring, p.Index, p.Module, p.Mk, p.Bill[0], p.Bill[1], p.Bill[2], p.Bill[3],
		p.AssemblySec, p.StartedBy).Scan(&id)
	return id, err
}

// Project is a base project as the base card shows it.
type Project struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	ToTier      *int       `json:"toTier,omitempty"`
	Ring        *int       `json:"ring,omitempty"`
	Index       *int       `json:"index,omitempty"`
	Module      *string    `json:"module,omitempty"`
	Mk          *int       `json:"mk,omitempty"`
	Bill        Resources4 `json:"bill"`
	Given       Resources4 `json:"given"`
	AssemblySec int        `json:"assemblySec"`
	AssembledS  float64    `json:"assembledSec"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	FundedAt    *time.Time `json:"fundedAt,omitempty"`
}

// Resources4 is an amount of each of the four resources.
type Resources4 struct {
	Alloy   int64 `json:"alloy"`
	Crystal int64 `json:"crystal"`
	Data    int64 `json:"data"`
	Credit  int64 `json:"credit"`
}

const projectCols = `id, kind, to_tier, ring, idx, module, mk, bill_alloy, bill_crystal, bill_data, bill_credit,
	given_alloy, given_crystal, given_data, given_credit, assembly_sec, assembled_sec, status, started_at, funded_at`

func scanProject(sc interface{ Scan(...any) error }) (Project, error) {
	var p Project
	err := sc.Scan(&p.ID, &p.Kind, &p.ToTier, &p.Ring, &p.Index, &p.Module, &p.Mk, &p.Bill.Alloy, &p.Bill.Crystal,
		&p.Bill.Data, &p.Bill.Credit, &p.Given.Alloy, &p.Given.Crystal, &p.Given.Data, &p.Given.Credit, &p.AssemblySec,
		&p.AssembledS, &p.Status, &p.StartedAt, &p.FundedAt)
	return p, err
}

// RunningProjects lists a base's projects that are funding or assembling, oldest first.
func RunningProjects(ctx context.Context, q db.Querier, baseID int64) ([]Project, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+projectCols+` FROM td.base_projects
		WHERE base_id = $1 AND status IN ('funding', 'assembling') ORDER BY started_at`, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MapBase is a standing base as the map and the base card show it to everyone.
type MapBase struct {
	ID         int64   `json:"id"`
	FactionID  int     `json:"-"`
	Faction    string  `json:"faction"`
	FactionNm  string  `json:"factionName"`
	Color      string  `json:"color"`
	Region     string  `json:"region"`
	Name       string  `json:"name"`
	Tier       int     `json:"tier"`
	TierName   string  `json:"tierName"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Status     string  `json:"status"`
	Faded      bool    `json:"faded"`
	CoreHealth int     `json:"coreHealth"`
	CoreMax    int     `json:"coreMax"`
	ZoneM      float64 `json:"zoneM"`
}

const mapBaseCols = `b.id, b.faction_id, f.code, f.name, f.color, b.region, b.name, b.tier, ST_Y(b.location::geometry),
	ST_X(b.location::geometry), b.status, b.faded_at IS NOT NULL, b.core_health, COALESCE(z.radius_m, 0)`

const mapBaseFrom = ` FROM td.bases b JOIN td.factions f ON f.id = b.faction_id LEFT JOIN td.base_zones z ON z.base_id = b.id`

func scanMapBase(sc interface{ Scan(...any) error }) (MapBase, error) {
	var b MapBase
	err := sc.Scan(&b.ID, &b.FactionID, &b.Faction, &b.FactionNm, &b.Color, &b.Region, &b.Name, &b.Tier, &b.Lat, &b.Lon,
		&b.Status, &b.Faded, &b.CoreHealth, &b.ZoneM)
	return b, err
}

// MapBasesNear lists the standing bases whose zone reaches within m metres of a point.
func MapBasesNear(ctx context.Context, q db.Querier, lat, lon, m float64) ([]MapBase, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+mapBaseCols+mapBaseFrom+`
		WHERE b.status <> 'destroyed'
		  AND ST_DWithin(b.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3 + COALESCE(z.radius_m, 0))
		ORDER BY b.id`, lat, lon, m)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapBase
	for rows.Next() {
		b, err := scanMapBase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// MapBaseByID reads one standing base for its card.
func MapBaseByID(ctx context.Context, q db.Querier, id int64) (MapBase, error) {
	b, err := scanMapBase(q.QueryRowContext(ctx, `SELECT `+mapBaseCols+mapBaseFrom+`
		WHERE b.id = $1 AND b.status <> 'destroyed'`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// SlotRow is one module slot as members see it.
type SlotRow struct {
	Ring   int     `json:"ring"`
	Index  int     `json:"index"`
	Module *string `json:"module,omitempty"`
	Mk     *int    `json:"mk,omitempty"`
	Health *int    `json:"health,omitempty"`
	Max    *int    `json:"maxHealth,omitempty"`
	Online bool    `json:"online"`
}

func BaseSlots(ctx context.Context, q db.Querier, baseID int64) ([]SlotRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT ring, idx, module, mk, health, max_health, online FROM td.base_slots WHERE base_id = $1 ORDER BY ring, idx`,
		baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SlotRow
	for rows.Next() {
		var r SlotRow
		if err := rows.Scan(&r.Ring, &r.Index, &r.Module, &r.Mk, &r.Health, &r.Max, &r.Online); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RegionCommanders lists the players who answer a faction's flags in a region: rank Regional Commander and up
// (succession puts an acting Commander there too).
func RegionCommanders(ctx context.Context, q db.Querier, factionID int, region string, minRank int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id FROM td.faction_members m JOIN td.faction_branches b ON b.id = m.branch_id
		WHERE m.faction_id = $1 AND b.country_code = $2::text AND m.rank >= $3 AND m.rank < 100`, factionID, region, minRank)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
