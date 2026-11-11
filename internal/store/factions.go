package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type Faction struct {
	ID           int    `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Motto        string `json:"motto"`
	Color        string `json:"color"`
	Status       string `json:"status"`
	LockedReason string `json:"lockedReason,omitempty"`
	// Leader is the faction's Regional Commander in the asked region (null when nobody holds it).
	Leader    *PlayerRef `json:"leader"`
	Region    string     `json:"region"`
	ClaimedAt *time.Time `json:"claimedAt,omitempty"`
	Members   int        `json:"members"`
	// Invited marks the faction that invited the asking player at the faction level; the service
	// fills it.
	Invited bool `json:"invited"`
	// Perks are what members get, with their numbers; Mine marks the asking player's own faction,
	// where they are working on them now. Both filled by the service.
	Perks []PerkLine `json:"perks"`
	Mine  bool       `json:"mine"`
	// Bound marks the faction a pledge made on this phone or sign-in binds the player to.
	Bound bool `json:"bound,omitempty"`

	LeaderID sql.NullInt64 `json:"-"`
}

// ListFactions lists every faction with its Regional Commander in one region (a country) and its
// members there. A faction is active once any region of it has a commander.
func ListFactions(ctx context.Context, q db.Querier, region string) ([]Faction, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT f.id, f.code, f.name, f.motto, f.color, f.status, f.locked_reason,
		       p.id, `+playerRefSQL("p")+`, f.claimed_at,
		       (SELECT count(*) FROM td.faction_members m WHERE m.faction_id = f.id AND m.branch_id = b.id)
		FROM td.factions f
		LEFT JOIN td.faction_branches b ON b.faction_id = f.id AND b.country_code = $1::text AND b.region_code = $1::text
		LEFT JOIN td.players p ON p.id = b.leader_player_id
		ORDER BY f.sort`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Faction, 0, 8)
	for rows.Next() {
		var f Faction
		var claimed sql.NullTime
		var leaderID sql.NullInt64
		var leaderCode, leaderName sql.NullString
		if err := rows.Scan(&f.ID, &f.Code, &f.Name, &f.Motto, &f.Color, &f.Status, &f.LockedReason,
			&leaderID, &leaderCode, &leaderName, &claimed, &f.Members); err != nil {
			return nil, err
		}
		if leaderID.Valid {
			f.Leader = &PlayerRef{Code: leaderCode.String, Name: leaderName.String}
		}
		if claimed.Valid {
			f.ClaimedAt = &claimed.Time
		}
		if f.Status != "locked" {
			f.LockedReason = ""
		}
		f.Region = region
		out = append(out, f)
	}
	return out, rows.Err()
}

// PerkLine is one perk as players read it (game.PerkLine, repeated here so store needs no game import).
type PerkLine struct {
	Key  string `json:"key"`
	Line string `json:"line"`
	Tag  string `json:"tag"`
}

func FactionByCodeForUpdate(ctx context.Context, q db.Querier, code string) (Faction, error) {
	var f Faction
	err := q.QueryRowContext(ctx, `
		SELECT id, code, name, motto, color, status, leader_player_id FROM td.factions
		WHERE code = $1 FOR UPDATE`, code).
		Scan(&f.ID, &f.Code, &f.Name, &f.Motto, &f.Color, &f.Status, &f.LeaderID)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// Branch is a faction's presence in one region (a country). FoundedBy is its first commander ever.
type Branch struct {
	ID        int64
	LeaderID  sql.NullInt64
	FoundedBy sql.NullInt64
}

// BranchForUpdate makes the faction's branch in a region if it is new and locks it.
func BranchForUpdate(ctx context.Context, q db.Querier, factionID int, region, name string) (Branch, error) {
	if _, err := q.ExecContext(ctx, `
		INSERT INTO td.faction_branches (faction_id, country_code, region_code, name) VALUES ($1, $2::text, $2::text, $3)
		ON CONFLICT (faction_id, country_code, region_code) DO NOTHING`, factionID, region, name); err != nil {
		return Branch{}, err
	}
	var b Branch
	err := q.QueryRowContext(ctx, `
		SELECT id, leader_player_id, founded_by FROM td.faction_branches
		WHERE faction_id = $1 AND country_code = $2::text AND region_code = $2::text FOR UPDATE`, factionID, region).
		Scan(&b.ID, &b.LeaderID, &b.FoundedBy)
	return b, err
}

// ClaimBranch makes a player the branch's Regional Commander. The WHERE clause is the real guard:
// if two players race, only one update matches and the other sees false.
func ClaimBranch(ctx context.Context, q db.Querier, branchID, playerID int64) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.faction_branches SET leader_player_id = $2
		WHERE id = $1 AND leader_player_id IS NULL`, branchID, playerID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// FoundBranch records the branch's first commander and gives them the permanent Founder badge.
func FoundBranch(ctx context.Context, q db.Querier, branchID, playerID int64, factionID int, region string) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE td.faction_branches SET founded_by = $2 WHERE id = $1 AND founded_by IS NULL`, branchID, playerID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.player_badges (player_id, badge, faction_id, region) VALUES ($1, 'founder', $2, $3::text)
		ON CONFLICT DO NOTHING`, playerID, factionID, region)
	return err
}

// Badge is a permanent mark a player earned, such as founding a faction's branch in a country.
type Badge struct {
	Badge       string `json:"badge"`
	FactionCode string `json:"factionCode"`
	FactionName string `json:"factionName"`
	Region      string `json:"region"`
}

// Badges lists a player's badges, oldest first.
func Badges(ctx context.Context, q db.Querier, playerID int64) ([]Badge, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.badge, COALESCE(f.code, ''), COALESCE(f.name, ''), b.region
		FROM td.player_badges b LEFT JOIN td.factions f ON f.id = b.faction_id
		WHERE b.player_id = $1 ORDER BY b.earned_at`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Badge, 0, 2)
	for rows.Next() {
		var b Badge
		if err := rows.Scan(&b.Badge, &b.FactionCode, &b.FactionName, &b.Region); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GrantTitle gives a player a badge that belongs to no faction (an Unbound title). It reports
// whether the badge is new.
func GrantTitle(ctx context.Context, q db.Querier, playerID int64, badge string) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.player_badges (player_id, badge, faction_id, region) VALUES ($1, $2, NULL, '')
		ON CONFLICT DO NOTHING`, playerID, badge)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// WakeFaction marks a faction active when its first region is claimed.
func WakeFaction(ctx context.Context, q db.Querier, factionID int) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.factions SET status = 'active', claimed_at = COALESCE(claimed_at, now()) WHERE id = $1`, factionID)
	return err
}

func AddMember(ctx context.Context, q db.Querier, playerID int64, factionID int, branchID int64, rank int) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.faction_members (player_id, faction_id, branch_id, rank) VALUES ($1, $2, $3, $4)`,
		playerID, factionID, branchID, rank)
	return err
}

type Membership struct {
	FactionID   int    `json:"factionId"`
	FactionCode string `json:"factionCode"`
	FactionName string `json:"factionName"`
	Rank        int    `json:"rank"`
	BranchID    *int64 `json:"branchId,omitempty"`
	// Region is the branch's country (MV); RankTitle names the rank. Filled by the service.
	Region    string `json:"region,omitempty"`
	RankTitle string `json:"rankTitle,omitempty"`
	// PowerName and PowerLine are only on a pledge's answer: the chosen power and its short line
	// (its motto), shown once after HOLD TO PLEDGE.
	PowerName string `json:"powerName,omitempty"`
	PowerLine string `json:"powerLine,omitempty"`
}

// FactionVoice is a faction's power and its motto.
func FactionVoice(ctx context.Context, q db.Querier, factionID int) (persona, motto string, err error) {
	err = q.QueryRowContext(ctx, `SELECT persona, motto FROM td.factions WHERE id = $1`, factionID).Scan(&persona, &motto)
	return persona, motto, err
}

// ReleaseMember takes a player out of their faction (an admin's RELEASE PLEDGE). A Regional
// Commander's seat in that country is left empty. Reports the faction released from (0 when none).
func ReleaseMember(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var faction int
	err := q.QueryRowContext(ctx, `DELETE FROM td.faction_members WHERE player_id = $1 RETURNING faction_id`, playerID).Scan(&faction)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_, err = q.ExecContext(ctx, `UPDATE td.faction_branches SET leader_player_id = NULL WHERE leader_player_id = $1`, playerID)
	return faction, err
}

// MembershipOf returns nil for a freelancer.
func MembershipOf(ctx context.Context, q db.Querier, playerID int64) (*Membership, error) {
	var m Membership
	var branch sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT f.id, f.code, f.name, m.rank, m.branch_id, COALESCE(b.country_code, '')
		FROM td.faction_members m JOIN td.factions f ON f.id = m.faction_id
		LEFT JOIN td.faction_branches b ON b.id = m.branch_id
		WHERE m.player_id = $1`, playerID).Scan(&m.FactionID, &m.FactionCode, &m.FactionName, &m.Rank, &branch, &m.Region)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if branch.Valid {
		m.BranchID = &branch.Int64
	}
	return &m, nil
}

// FactionBranch is one region of a faction as an admin looking inside sees it.
type FactionBranch struct {
	ID        int64          `json:"-"`
	Region    string         `json:"region"`
	Name      string         `json:"name"`
	Commander *PlayerRef     `json:"commander"`
	Members   []BranchMember `json:"members"`
}

// FactionBranches lists a faction's branches (members are filled by the caller).
func FactionBranches(ctx context.Context, q db.Querier, factionID int) ([]FactionBranch, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.id, b.region_code, b.name, p.id, `+playerRefSQL("p")+`
		FROM td.faction_branches b LEFT JOIN td.players p ON p.id = b.leader_player_id
		WHERE b.faction_id = $1 ORDER BY b.id`, factionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FactionBranch{}
	for rows.Next() {
		var b FactionBranch
		var leaderID sql.NullInt64
		var code, name sql.NullString
		if err := rows.Scan(&b.ID, &b.Region, &b.Name, &leaderID, &code, &name); err != nil {
			return nil, err
		}
		if leaderID.Valid {
			b.Commander = &PlayerRef{Code: code.String, Name: name.String}
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
