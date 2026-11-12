package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
)

// Faction bases, B5 (base-building-plan.md 8): the base under attack.

// BaseCombat is a base as an attack sees it, locked.
type BaseCombat struct {
	BaseRef
	CoreHealth     int
	ProtectedUntil *time.Time
	AttackPushedAt *time.Time
}

func BaseCombatForUpdate(ctx context.Context, q db.Querier, id int64) (BaseCombat, error) {
	var b BaseCombat
	err := q.QueryRowContext(ctx, `
		SELECT b.id, b.faction_id, b.region, b.name, b.tier, b.status, ST_Y(b.location::geometry), ST_X(b.location::geometry),
		       COALESCE(z.radius_m, 0), b.core_health, b.protected_until, b.attack_pushed_at
		FROM td.bases b LEFT JOIN td.base_zones z ON z.base_id = b.id
		WHERE b.id = $1 AND b.status <> 'destroyed' FOR UPDATE OF b`, id).
		Scan(&b.ID, &b.FactionID, &b.Region, &b.Name, &b.Tier, &b.Status, &b.Lat, &b.Lon, &b.ZoneM, &b.CoreHealth,
			&b.ProtectedUntil, &b.AttackPushedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// PowerSlots lists a base's modules for the power rule (with Mk for the shield and refunds).
func PowerSlots(ctx context.Context, q db.Querier, baseID int64) ([]game.PowerSlot, map[[2]int]int, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT ring, idx, COALESCE(module, ''), COALESCE(health, 0), COALESCE(mk, 0) FROM td.base_slots
		WHERE base_id = $1 ORDER BY ring, idx`, baseID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []game.PowerSlot
	mks := map[[2]int]int{}
	for rows.Next() {
		var p game.PowerSlot
		var mk int
		if err := rows.Scan(&p.Ring, &p.Index, &p.Module, &p.Health, &mk); err != nil {
			return nil, nil, err
		}
		out = append(out, p)
		mks[[2]int{p.Ring, p.Index}] = mk
	}
	return out, mks, rows.Err()
}

// SetPowered writes which modules are online.
func SetPowered(ctx context.Context, q db.Querier, baseID int64, online map[[2]int]bool) error {
	rings := []int64{}
	idxs := []int64{}
	for k := range online {
		rings = append(rings, int64(k[0]))
		idxs = append(idxs, int64(k[1]))
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_slots s SET online = EXISTS (
		    SELECT 1 FROM unnest($2::bigint[], $3::bigint[]) AS l(ring, idx) WHERE l.ring = s.ring AND l.idx = s.idx)
		WHERE s.base_id = $1`, baseID, rings, idxs)
	return err
}

// BestShieldMk is the Mk of the base's best online Shield Matrix (0: none).
func BestShieldMk(ctx context.Context, q db.Querier, baseID int64) (int, error) {
	var mk int
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(max(mk), 0) FROM td.base_slots WHERE base_id = $1 AND module = 'shield' AND online`, baseID).Scan(&mk)
	return mk, err
}

// ShieldOverTether is the best online Shield Matrix of a base of the TN owner's faction whose zone holds the TN
// (members' TNs inside the zone use the better of their own shield and this one, section 5).
func ShieldOverTether(ctx context.Context, q db.Querier, tetherID int64) (int, error) {
	var mk int
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(max(s.mk), 0)
		FROM td.tether_nodes t
		JOIN td.faction_members m ON m.player_id = t.owner_id
		JOIN td.bases b ON b.faction_id = m.faction_id AND b.status <> 'destroyed'
		JOIN td.base_zones z ON z.base_id = b.id
		JOIN td.base_slots s ON s.base_id = b.id AND s.module = 'shield' AND s.online
		WHERE t.id = $1 AND ST_DWithin(b.location, t.location, z.radius_m)`, tetherID).Scan(&mk)
	return mk, err
}

// DamageSlot takes damage off a module; returns its health after and its module (empty: no module there).
func DamageSlot(ctx context.Context, q db.Querier, baseID int64, ring, index int, damage float64) (int, string, error) {
	var health int
	var module sql.NullString
	err := q.QueryRowContext(ctx, `
		UPDATE td.base_slots SET health = GREATEST(0, health - CEIL($4)::int)
		WHERE base_id = $1 AND ring = $2 AND idx = $3 AND module IS NOT NULL
		RETURNING health, module`, baseID, ring, index, damage).Scan(&health, &module)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	return health, module.String, err
}

func DamageCore(ctx context.Context, q db.Querier, baseID int64, damage float64) (int, error) {
	var health int
	err := q.QueryRowContext(ctx, `
		UPDATE td.bases SET core_health = GREATEST(0, core_health - CEIL($2)::int), updated_at = now()
		WHERE id = $1 RETURNING core_health`, baseID, damage).Scan(&health)
	return health, err
}

// SlotHealth reads a module's health and full health.
func SlotHealth(ctx context.Context, q db.Querier, baseID int64, ring, index int) (health, max int, module string, err error) {
	var m sql.NullString
	err = q.QueryRowContext(ctx, `
		SELECT COALESCE(health, 0), COALESCE(max_health, 0), module FROM td.base_slots
		WHERE base_id = $1 AND ring = $2 AND idx = $3`, baseID, ring, index).Scan(&health, &max, &m)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", ErrNotFound
	}
	return health, max, m.String, err
}

func SetSlotHealth(ctx context.Context, q db.Querier, baseID int64, ring, index, health int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.base_slots SET health = $4 WHERE base_id = $1 AND ring = $2 AND idx = $3`,
		baseID, ring, index, health)
	return err
}

func SetCoreHealth(ctx context.Context, q db.Querier, baseID int64, health int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET core_health = $2, updated_at = now() WHERE id = $1`, baseID, health)
	return err
}

func SetBaseProtected(ctx context.Context, q db.Querier, baseID int64, until time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET protected_until = $2 WHERE id = $1`, baseID, until)
	return err
}

func SetAttackPushed(ctx context.Context, q db.Querier, baseID int64, at time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET attack_pushed_at = $2 WHERE id = $1`, baseID, at)
	return err
}

// DestroyBase ends a base: destroyed, its running projects cancelled, its collector links let go.
func DestroyBase(ctx context.Context, q db.Querier, baseID int64, now time.Time) error {
	if _, err := q.ExecContext(ctx, `
		UPDATE td.bases SET status = 'destroyed', destroyed_at = $2, updated_at = $2 WHERE id = $1`, baseID, now); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `
		UPDATE td.base_projects SET status = 'cancelled', cancelled_at = $2
		WHERE base_id = $1 AND status IN ('funding', 'assembling')`, baseID, now); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE td.base_strikes SET status = 'void' WHERE base_id = $1 AND status = 'pending'`, baseID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `DELETE FROM td.base_collector_links WHERE base_id = $1`, baseID)
	return err
}

// InsertBaseDrop leaves what a destroyed base held on the map, to be picked up until it expires.
func InsertBaseDrop(ctx context.Context, q db.Querier, baseID int64, lat, lon float64, contents map[string]int64, expires time.Time) error {
	encoded, err := json.Marshal(contents)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.map_drops (base_id, location, contents, expires_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography, $4::jsonb, $5)`,
		baseID, lat, lon, string(encoded), expires)
	return err
}

// FactionMembersInRegion lists every member of a faction whose home is the region (the attack push).
func FactionMembersInRegion(ctx context.Context, q db.Querier, factionID int, region string) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id FROM td.faction_members m JOIN td.faction_branches b ON b.id = m.branch_id
		WHERE m.faction_id = $1 AND b.country_code = $2::text`, factionID, region)
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

// WalkLinkHasFaction says whether a live walk link has a member of a faction (attacking its base ends the link).
func WalkLinkHasFaction(ctx context.Context, q db.Querier, linkID int64, factionID int) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.walk_link_members w JOIN td.faction_members m ON m.player_id = w.player_id
		               WHERE w.link_id = $1 AND w.left_at IS NULL AND m.faction_id = $2)`, linkID, factionID).Scan(&ok)
	return ok, err
}

// ---- strikes: drones in flight and spikes armed ----------------------------------------------

type BaseStrike struct {
	ID         int64      `json:"id"`
	BaseID     int64      `json:"-"`
	Ring       *int       `json:"ring,omitempty"`
	Index      *int       `json:"index,omitempty"`
	Kind       string     `json:"kind"`
	LaunchedBy int64      `json:"-"`
	Damage     float64    `json:"-"`
	Mk         int        `json:"mk"`
	FromLat    *float64   `json:"fromLat,omitempty"`
	FromLon    *float64   `json:"fromLon,omitempty"`
	DueAt      time.Time  `json:"dueAt"`
	Intercept  float64    `json:"interceptShare"`
	Status     string     `json:"-"`
	CreatedAt  *time.Time `json:"-"`
}

const strikeCols = `id, base_id, ring, idx, kind, launched_by, damage, mk, from_lat, from_lon, due_at, intercept_share, status`

func scanStrike(sc interface{ Scan(...any) error }) (BaseStrike, error) {
	var x BaseStrike
	err := sc.Scan(&x.ID, &x.BaseID, &x.Ring, &x.Index, &x.Kind, &x.LaunchedBy, &x.Damage, &x.Mk, &x.FromLat, &x.FromLon,
		&x.DueAt, &x.Intercept, &x.Status)
	return x, err
}

// InsertStrike adds a drone in flight or an armed spike. A nil ring targets the core.
func InsertStrike(ctx context.Context, q db.Querier, x BaseStrike) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.base_strikes (base_id, ring, idx, kind, launched_by, damage, mk, from_lat, from_lon, due_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		x.BaseID, x.Ring, x.Index, x.Kind, x.LaunchedBy, x.Damage, x.Mk, x.FromLat, x.FromLon, x.DueAt).Scan(&id)
	return id, err
}

func DueStrikeIDs(ctx context.Context, q db.Querier, now time.Time, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.base_strikes WHERE status = 'pending' AND due_at <= $1 ORDER BY due_at LIMIT $2`, now, limit)
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

func StrikeForUpdate(ctx context.Context, q db.Querier, id int64) (BaseStrike, error) {
	x, err := scanStrike(q.QueryRowContext(ctx, `SELECT `+strikeCols+` FROM td.base_strikes WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	return x, err
}

// PendingStrikes lists the drones flying at and spikes armed on a base, soonest first.
func PendingStrikes(ctx context.Context, q db.Querier, baseID int64) ([]BaseStrike, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+strikeCols+` FROM td.base_strikes
		WHERE base_id = $1 AND status = 'pending' ORDER BY due_at`, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaseStrike{}
	for rows.Next() {
		x, err := scanStrike(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func SetStrikeStatus(ctx context.Context, q db.Querier, id int64, status string) error {
	_, err := q.ExecContext(ctx, `UPDATE td.base_strikes SET status = $2 WHERE id = $1`, id, status)
	return err
}

func SetStrikeIntercept(ctx context.Context, q db.Querier, id int64, share float64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.base_strikes SET intercept_share = $2 WHERE id = $1`, id, share)
	return err
}

// BaseDronesInFlight counts a player's drones flying at bases (with DronesInFlight, the Drone Bay's limit).
func BaseDronesInFlight(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.base_strikes WHERE launched_by = $1 AND kind = 'drone' AND status = 'pending'`, playerID).Scan(&n)
	return n, err
}

// ---- hacks -------------------------------------------------------------------------------------

func LastHack(ctx context.Context, q db.Querier, playerID, baseID int64) (*time.Time, error) {
	var at *time.Time
	err := q.QueryRowContext(ctx, `SELECT max(at) FROM td.base_hacks WHERE player_id = $1 AND base_id = $2`, playerID, baseID).Scan(&at)
	return at, err
}

func InsertHack(ctx context.Context, q db.Querier, baseID, playerID int64, stolen map[string]int64, now time.Time) error {
	encoded, err := json.Marshal(stolen)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO td.base_hacks (base_id, player_id, at, stolen) VALUES ($1, $2, $3, $4::jsonb)`,
		baseID, playerID, now, string(encoded))
	return err
}

// FactionHasDamagedModule says whether any of a faction's standing bases has a module below full health (the repair
// mission is offered then).
func FactionHasDamagedModule(ctx context.Context, q db.Querier, factionID int) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.base_slots s JOIN td.bases b ON b.id = s.base_id
		               WHERE b.faction_id = $1 AND b.status <> 'destroyed' AND s.module IS NOT NULL
		                 AND s.health < s.max_health)`, factionID).Scan(&ok)
	return ok, err
}

// UpkeepBase is a standing base as the upkeep tick reads it.
type UpkeepBase struct {
	ID        int64
	Tier      int
	LastVisit time.Time
	FadedAt   *time.Time
	DroppedAt *time.Time
}

func UpkeepBases(ctx context.Context, q db.Querier) ([]UpkeepBase, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, tier, last_visit_at, faded_at, fade_dropped_at FROM td.bases WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UpkeepBase
	for rows.Next() {
		var u UpkeepBase
		if err := rows.Scan(&u.ID, &u.Tier, &u.LastVisit, &u.FadedAt, &u.DroppedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func SetBaseFaded(ctx context.Context, q db.Querier, baseID int64, at time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET faded_at = $2, fade_dropped_at = NULL WHERE id = $1 AND faded_at IS NULL`, baseID, at)
	return err
}

func SetFadeDropped(ctx context.Context, q db.Querier, baseID int64, at time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.bases SET fade_dropped_at = $2 WHERE id = $1`, baseID, at)
	return err
}

// BaseFaded says whether a base is fading.
func BaseFaded(ctx context.Context, q db.Querier, baseID int64) (bool, error) {
	var faded bool
	err := q.QueryRowContext(ctx, `SELECT faded_at IS NOT NULL FROM td.bases WHERE id = $1`, baseID).Scan(&faded)
	return faded, err
}

// BaseFlight is a strike on a base as the map draws it: a drone in flight, or a hit that just landed (drone or spike).
type BaseFlight struct {
	ID         int64     `json:"id"`
	BaseID     int64     `json:"baseId"`
	Kind       string    `json:"kind"`
	FromLat    *float64  `json:"fromLat,omitempty"`
	FromLon    *float64  `json:"fromLon,omitempty"`
	ToLat      float64   `json:"toLat"`
	ToLon      float64   `json:"toLon"`
	LaunchedAt time.Time `json:"launchedAt"`
	DueAt      time.Time `json:"dueAt"`
	Landed     bool      `json:"landed"`
	// Mine: the viewer launched it (outgoing); otherwise it flies at the viewer's faction's base (incoming).
	Mine     bool `json:"mine"`
	Shielded bool `json:"shielded"`
}

// BaseFlights lists what the map may show a player about strikes on bases (the same rule as for Tether Nodes): their own
// drones going out, and drones and spikes at their faction's bases; pending ones, and hits from the last minutes.
func BaseFlights(ctx context.Context, q db.Querier, playerID int64, factionID int, since time.Time) ([]BaseFlight, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT s.id, s.base_id, s.kind, s.from_lat, s.from_lon, ST_Y(b.location::geometry), ST_X(b.location::geometry),
		       s.created_at, s.due_at, s.status = 'hit', s.launched_by = $1,
		       EXISTS (SELECT 1 FROM td.base_slots sl WHERE sl.base_id = b.id AND sl.module = 'shield' AND sl.online)
		FROM td.base_strikes s JOIN td.bases b ON b.id = s.base_id
		WHERE (s.launched_by = $1 OR b.faction_id = $2)
		  AND ((s.status = 'pending' AND s.kind = 'drone') OR (s.status = 'hit' AND s.due_at > $3))
		ORDER BY s.due_at`, playerID, factionID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaseFlight{}
	for rows.Next() {
		var f BaseFlight
		if err := rows.Scan(&f.ID, &f.BaseID, &f.Kind, &f.FromLat, &f.FromLon, &f.ToLat, &f.ToLon, &f.LaunchedAt, &f.DueAt,
			&f.Landed, &f.Mine, &f.Shielded); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
