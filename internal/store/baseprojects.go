package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Faction bases, B2 (base-building-plan.md 4, 6): projects, giving, the vault and its log, the assembly tick.

// ProjectRow is a project as the service decides on it.
type ProjectRow struct {
	ID          int64
	BaseID      int64
	Kind        string
	ToTier      int
	Ring, Index int
	Module      string
	Mk          int
	Bill, Given [4]int64
	AssemblySec int
	Assembled   float64
	Status      string
	StartedBy   int64
	TickedAt    *time.Time
}

const projectRowCols = `id, base_id, kind, COALESCE(to_tier, 0), COALESCE(ring, 0), COALESCE(idx, 0), COALESCE(module, ''),
	COALESCE(mk, 0), bill_alloy, bill_crystal, bill_data, bill_credit, given_alloy, given_crystal, given_data, given_credit,
	assembly_sec, assembled_sec, status, COALESCE(started_by, 0), ticked_at`

func scanProjectRow(sc interface{ Scan(...any) error }) (ProjectRow, error) {
	var p ProjectRow
	err := sc.Scan(&p.ID, &p.BaseID, &p.Kind, &p.ToTier, &p.Ring, &p.Index, &p.Module, &p.Mk, &p.Bill[0], &p.Bill[1],
		&p.Bill[2], &p.Bill[3], &p.Given[0], &p.Given[1], &p.Given[2], &p.Given[3], &p.AssemblySec, &p.Assembled,
		&p.Status, &p.StartedBy, &p.TickedAt)
	return p, err
}

func ProjectForUpdate(ctx context.Context, q db.Querier, id int64) (ProjectRow, error) {
	p, err := scanProjectRow(q.QueryRowContext(ctx, `SELECT `+projectRowCols+` FROM td.base_projects WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// RunningProjectRows lists a base's funding and assembling projects.
func RunningProjectRows(ctx context.Context, q db.Querier, baseID int64) ([]ProjectRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+projectRowCols+` FROM td.base_projects
		WHERE base_id = $1 AND status IN ('funding', 'assembling')`, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectRow
	for rows.Next() {
		p, err := scanProjectRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetProjectGiven writes what has been given; funded moves it to assembling.
func SetProjectGiven(ctx context.Context, q db.Querier, id int64, given [4]int64, funded bool, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_projects SET given_alloy = $2, given_crystal = $3, given_data = $4, given_credit = $5,
		       status = CASE WHEN $6 THEN 'assembling' ELSE status END,
		       funded_at = CASE WHEN $6 THEN $7 ELSE funded_at END,
		       ticked_at = CASE WHEN $6 THEN $7 ELSE ticked_at END
		WHERE id = $1`, id, given[0], given[1], given[2], given[3], funded, now)
	return err
}

func CancelProject(ctx context.Context, q db.Querier, id, by int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_projects SET status = 'cancelled', cancelled_by = $2, cancelled_at = $3 WHERE id = $1`, id, by, now)
	return err
}

// AdvanceProject moves an assembling project on, or finishes it.
func AdvanceProject(ctx context.Context, q db.Querier, id int64, assembled float64, done bool, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_projects SET assembled_sec = $2, ticked_at = $4,
		       status = CASE WHEN $3 THEN 'done' ELSE status END, done_at = CASE WHEN $3 THEN $4 ELSE done_at END
		WHERE id = $1`, id, assembled, done, now)
	return err
}

// AssemblingIDs lists every project assembling now.
func AssemblingIDs(ctx context.Context, q db.Querier, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM td.base_projects WHERE status = 'assembling' ORDER BY id LIMIT $1`, limit)
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

// MembersOnSite counts a faction's members whose last fix (no older than since) lies inside the base's zone.
func MembersOnSite(ctx context.Context, q db.Querier, baseID int64, factionID int, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.faction_members m
		JOIN td.player_last_fix f ON f.player_id = m.player_id
		JOIN td.bases b ON b.id = $1
		JOIN td.base_zones z ON z.base_id = b.id
		WHERE m.faction_id = $2 AND f.fixed_at > $3
		  AND ST_DWithin(b.location, ST_SetSRID(ST_MakePoint(f.lon, f.lat), 4326)::geography, z.radius_m)`,
		baseID, factionID, since).Scan(&n)
	return n, err
}

// SlotModule reads one slot (exists false when the base has no such slot).
func SlotModule(ctx context.Context, q db.Querier, baseID int64, ring, index int) (module string, mk int, exists bool, err error) {
	var m sql.NullString
	var k sql.NullInt64
	err = q.QueryRowContext(ctx, `SELECT module, mk FROM td.base_slots WHERE base_id = $1 AND ring = $2 AND idx = $3`,
		baseID, ring, index).Scan(&m, &k)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	return m.String, int(k.Int64), err == nil, err
}

// SetSlotModule puts a module (or its new Mk) on a slot at full health, online.
func SetSlotModule(ctx context.Context, q db.Querier, baseID int64, ring, index int, module string, mk, health int) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.base_slots SET module = $4, mk = $5, health = $6, max_health = $6, online = true
		WHERE base_id = $1 AND ring = $2 AND idx = $3`, baseID, ring, index, module, mk, health)
	return err
}

// SetBaseActive: the Spire stands. New bases are protected for a while (section 8).
func SetBaseActive(ctx context.Context, q db.Querier, baseID int64, protectedUntil time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.bases SET status = 'active', protected_until = $2, updated_at = now() WHERE id = $1`, baseID, protectedUntil)
	return err
}

// BaseRefForUpdate locks a base and gives what giving and starting need.
type BaseRef struct {
	ID        int64
	FactionID int
	Region    string
	Name      string
	Tier      int
	Status    string
	Lat, Lon  float64
	ZoneM     float64
}

func BaseRefForUpdate(ctx context.Context, q db.Querier, id int64) (BaseRef, error) {
	var b BaseRef
	err := q.QueryRowContext(ctx, `
		SELECT b.id, b.faction_id, b.region, b.name, b.tier, b.status, ST_Y(b.location::geometry), ST_X(b.location::geometry),
		       COALESCE(z.radius_m, 0)
		FROM td.bases b LEFT JOIN td.base_zones z ON z.base_id = b.id
		WHERE b.id = $1 AND b.status <> 'destroyed' FOR UPDATE OF b`, id).
		Scan(&b.ID, &b.FactionID, &b.Region, &b.Name, &b.Tier, &b.Status, &b.Lat, &b.Lon, &b.ZoneM)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// ---- contributions and points --------------------------------------------------------------------

func InsertContribution(ctx context.Context, q db.Querier, baseID, projectID, playerID int64, fromVault bool, amounts [4]int64, points int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.base_contributions (base_id, project_id, player_id, from_vault, alloy, crystal, data, credit, points)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6, $7, $8, $9)`,
		baseID, projectID, playerID, fromVault, amounts[0], amounts[1], amounts[2], amounts[3], points)
	return err
}

func AddContributionPoints(ctx context.Context, q db.Querier, playerID, points int64) error {
	if points <= 0 {
		return nil
	}
	_, err := q.ExecContext(ctx, `UPDATE td.players SET contribution_points = contribution_points + $2 WHERE id = $1`, playerID, points)
	return err
}

// VaultGivenSince is how much a player has paid from vaults since a time (the Captain's daily cap).
func VaultGivenSince(ctx context.Context, q db.Querier, playerID int64, since time.Time) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(sum(alloy + crystal + data + credit), 0) FROM td.base_contributions
		WHERE player_id = $1 AND from_vault AND at >= $2`, playerID, since).Scan(&n)
	return n, err
}

// ---- the vault ---------------------------------------------------------------------------------

func VaultForUpdate(ctx context.Context, q db.Querier, baseID int64) ([4]int64, error) {
	var v [4]int64
	err := q.QueryRowContext(ctx, `SELECT alloy, crystal, data, credit FROM td.base_vaults WHERE base_id = $1 FOR UPDATE`, baseID).
		Scan(&v[0], &v[1], &v[2], &v[3])
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	return v, err
}

// AddVault moves amounts into (or, negative, out of) a vault and logs it. The CHECKs refuse to go below zero.
func AddVault(ctx context.Context, q db.Querier, baseID, playerID, projectID int64, kind string, delta [4]int64, now time.Time) error {
	if delta == [4]int64{} {
		return nil
	}
	// Update first: an upsert would check its (negative) insert row against the CHECKs before the conflict.
	res, err := q.ExecContext(ctx, `
		UPDATE td.base_vaults SET alloy = alloy + $2, crystal = crystal + $3, data = data + $4, credit = credit + $5
		WHERE base_id = $1`, baseID, delta[0], delta[1], delta[2], delta[3])
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO td.base_vaults (base_id, alloy, crystal, data, credit) VALUES ($1, $2, $3, $4, $5)`,
			baseID, delta[0], delta[1], delta[2], delta[3]); err != nil {
			return err
		}
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.base_vault_log (base_id, at, player_id, kind, project_id, alloy, crystal, data, credit)
		VALUES ($1, $2, NULLIF($3, 0), $4, NULLIF($5, 0), $6, $7, $8, $9)`,
		baseID, now, playerID, kind, projectID, delta[0], delta[1], delta[2], delta[3])
	return err
}
