package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Faction ranks v2 and Nomads (docs/faction-ranks-plan.md, R1): rank slots, Captain squads, Nomad asks and moves,
// and the promotions the tick makes.

// CountRankInBranch counts a branch's members holding a rank from level up to (not including) below.
func CountRankInBranch(ctx context.Context, q db.Querier, branchID int64, level, below int) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.faction_members WHERE branch_id = $1 AND rank >= $2 AND rank < $3 AND NOT nomad`,
		branchID, level, below).Scan(&n)
	return n, err
}

// BranchInRegion finds a faction's branch in a region: its id and whether it has a Regional Commander (unlocked).
func BranchInRegion(ctx context.Context, q db.Querier, factionID int, region string) (BranchInfo, error) {
	var b BranchInfo
	err := q.QueryRowContext(ctx, `
		SELECT id, faction_id, country_code, leader_player_id IS NOT NULL FROM td.faction_branches
		WHERE faction_id = $1 AND country_code = $2::text`, factionID, region).
		Scan(&b.ID, &b.FactionID, &b.Region, &b.Unlocked)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// ClearSeat empties a branch seat the player holds (they gave up their rank).
func ClearSeat(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.faction_branches SET leader_player_id = NULL WHERE leader_player_id = $1`, playerID)
	return err
}

// ---- squads ----------------------------------------------------------------------------------

func SquadSize(ctx context.Context, q db.Querier, captainID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.squad_members WHERE captain_id = $1`, captainID).Scan(&n)
	return n, err
}

// SquadCaptainOf is the Captain whose squad the player is in (nil: none).
func SquadCaptainOf(ctx context.Context, q db.Querier, playerID int64) (*int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT captain_id FROM td.squad_members WHERE player_id = $1`, playerID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

func AddSquadMember(ctx context.Context, q db.Querier, captainID, playerID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO td.squad_members (player_id, captain_id) VALUES ($1, $2)`, playerID, captainID)
	return err
}

// LeaveSquad takes a player out of whatever squad they are in.
func LeaveSquad(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.squad_members WHERE player_id = $1`, playerID)
	return err
}

// DissolveSquad empties a Captain's squad.
func DissolveSquad(ctx context.Context, q db.Querier, captainID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.squad_members WHERE captain_id = $1`, captainID)
	return err
}

// ---- Nomads ------------------------------------------------------------------------------------

// SetNomad makes a member a Nomad (rank Member) or settles them. cooldownFrom is kept when nil.
func SetNomad(ctx context.Context, q db.Querier, playerID int64, nomad bool, rank int, cooldownFrom *time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.faction_members SET nomad = $2, rank = $3,
		       nomad_cooldown_from = COALESCE($4, nomad_cooldown_from),
		       was_nomad = was_nomad OR NOT $2
		WHERE player_id = $1`, playerID, nomad, rank, cooldownFrom)
	return err
}

// WasNomad says whether the member was a Nomad before and settled (becoming one again restarts the cooldown).
func WasNomad(ctx context.Context, q db.Querier, playerID int64) (bool, error) {
	var was bool
	err := q.QueryRowContext(ctx, `SELECT was_nomad FROM td.faction_members WHERE player_id = $1`, playerID).Scan(&was)
	return was, err
}

// MoveNomadHome moves a Nomad's home region: Member there, the move's time starts the cooldown.
func MoveNomadHome(ctx context.Context, q db.Querier, playerID, branchID int64, rank int, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.faction_members SET branch_id = $2, viewing_branch_id = NULL, rank = $3, nomad_cooldown_from = $4
		WHERE player_id = $1`, playerID, branchID, rank, now)
	return err
}

// ClearNomadCooldown lets a Nomad move again at once (admin).
func ClearNomadCooldown(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.faction_members SET nomad_cooldown_from = NULL WHERE player_id = $1`, playerID)
	return err
}

// NomadAsk is a Nomad's ask to move home regions.
type NomadAsk struct {
	ID         int64      `json:"id"`
	PlayerID   int64      `json:"-"`
	Player     PlayerRef  `json:"player"`
	Level      int        `json:"level"`
	FactionID  int        `json:"-"`
	FromRegion string     `json:"fromRegion"`
	ToRegion   string     `json:"toRegion"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	DecidedAt  *time.Time `json:"decidedAt,omitempty"`
}

const askCols = `a.id, a.player_id, td.player_code(p.public_code, p.status), td.player_name(p.callsign, p.public_code, p.status),
	p.level, a.faction_id, a.from_region, a.to_region, a.status, a.created_at, a.expires_at, a.decided_at`

func scanAsk(sc interface{ Scan(...any) error }) (NomadAsk, error) {
	var a NomadAsk
	err := sc.Scan(&a.ID, &a.PlayerID, &a.Player.Code, &a.Player.Name, &a.Level, &a.FactionID, &a.FromRegion, &a.ToRegion,
		&a.Status, &a.CreatedAt, &a.ExpiresAt, &a.DecidedAt)
	return a, err
}

func InsertNomadAsk(ctx context.Context, q db.Querier, playerID int64, factionID int, from, to string, expires time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.nomad_asks (player_id, faction_id, from_region, to_region, expires_at) VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, playerID, factionID, from, to, expires).Scan(&id)
	return id, err
}

func NomadAskForUpdate(ctx context.Context, q db.Querier, id int64) (NomadAsk, error) {
	a, err := scanAsk(q.QueryRowContext(ctx, `SELECT `+askCols+` FROM td.nomad_asks a JOIN td.players p ON p.id = a.player_id
		WHERE a.id = $1 FOR UPDATE OF a`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func queryAsks(ctx context.Context, q db.Querier, where string, args ...any) ([]NomadAsk, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+askCols+` FROM td.nomad_asks a JOIN td.players p ON p.id = a.player_id
		WHERE `+where+` ORDER BY a.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NomadAsk{}
	for rows.Next() {
		a, err := scanAsk(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// OpenAskOf is the player's open ask (nil: none).
func OpenAskOf(ctx context.Context, q db.Querier, playerID int64, now time.Time) (*NomadAsk, error) {
	asks, err := queryAsks(ctx, q, `a.player_id = $1 AND a.status = 'open' AND a.expires_at > $2`, playerID, now)
	if err != nil || len(asks) == 0 {
		return nil, err
	}
	return &asks[0], nil
}

// AsksTo lists open asks to move into a faction's region.
func AsksTo(ctx context.Context, q db.Querier, factionID int, region string, now time.Time) ([]NomadAsk, error) {
	return queryAsks(ctx, q, `a.faction_id = $1 AND a.to_region = $2 AND a.status = 'open' AND a.expires_at > $3`,
		factionID, region, now)
}

func DecideAsk(ctx context.Context, q db.Querier, id int64, status string, by int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.nomad_asks SET status = $2, decided_by = NULLIF($3, 0), decided_at = $4 WHERE id = $1 AND status = 'open'`,
		id, status, by, now)
	return err
}

// CancelOpenAsks cancels a player's open ask (settling, or asking elsewhere).
func CancelOpenAsks(ctx context.Context, q db.Querier, playerID int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.nomad_asks SET status = 'cancelled', decided_at = $2 WHERE player_id = $1 AND status = 'open'`, playerID, now)
	return err
}

// DueAsks lists open asks whose time is up.
func DueAsks(ctx context.Context, q db.Querier, now time.Time, limit int) ([]NomadAsk, error) {
	return queryAsks(ctx, q, `a.status = 'open' AND a.expires_at <= $1 LIMIT $2`, now, limit)
}

// BranchApprovers lists the members of a branch at or above a rank (who hear about a Nomad's ask).
func BranchApprovers(ctx context.Context, q db.Querier, branchID int64, minRank int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT player_id FROM td.faction_members WHERE branch_id = $1 AND rank >= $2 AND rank < 100 AND NOT nomad`,
		branchID, minRank)
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

// ---- promotions ------------------------------------------------------------------------------

// RankCandidate is a member the tick may promote.
type RankCandidate struct {
	PlayerID int64
	BranchID int64
	Rank     int
	Nomad    bool
	Points   int64
	JoinedAt time.Time
}

// RankCandidates lists Prospects, and members who could climb by points (most points first).
func RankCandidates(ctx context.Context, q db.Querier, prospect, below, minPoints int) ([]RankCandidate, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id, m.branch_id, m.rank, m.nomad, p.contribution_points, m.joined_at
		FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.branch_id IS NOT NULL AND m.rank < $2
		  AND (m.rank <= $1 OR (NOT m.nomad AND $3 > 0 AND p.contribution_points >= $3))
		ORDER BY p.contribution_points DESC, m.joined_at`, prospect, below, minPoints)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RankCandidate
	for rows.Next() {
		var c RankCandidate
		if err := rows.Scan(&c.PlayerID, &c.BranchID, &c.Rank, &c.Nomad, &c.Points, &c.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AwayFromHome says whether a member's last accepted fix lies outside their home region (its shape, or farther than
// maxKm from it). Freelancers, and members with no fix yet, are never away. (faction-ranks-plan.md: a move changes
// where a player belongs, not where they can play from.)
func AwayFromHome(ctx context.Context, q db.Querier, playerID int64, maxKm float64) (bool, error) {
	var away bool
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE((
		    SELECT NOT EXISTS (
		        SELECT 1 FROM td.countries c
		        WHERE c.code = b.country_code
		          AND ST_DWithin(c.geom::geography, ST_SetSRID(ST_MakePoint(f.lon, f.lat), 4326)::geography, $2::float8 * 1000))
		    FROM td.faction_members m
		    JOIN td.faction_branches b ON b.id = m.branch_id
		    JOIN td.player_last_fix f ON f.player_id = m.player_id
		    WHERE m.player_id = $1), false)`, playerID, maxKm).Scan(&away)
	return away, err
}
