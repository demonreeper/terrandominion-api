package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Viewing another region of your faction (docs/social-signal-plan.md Part B, R2).

// UnlockedRegion is one region of a faction that has a Regional Commander.
type UnlockedRegion struct {
	BranchID  int64      `json:"-"`
	Region    string     `json:"region"`
	Name      string     `json:"name"`
	Commander *PlayerRef `json:"commander"`
	Members   int        `json:"members"`
	Home      bool       `json:"home"`
	Viewing   bool       `json:"viewing"`
}

// UnlockedRegions lists a faction's regions with a commander, by name. The region name comes from td.countries.
func UnlockedRegions(ctx context.Context, q db.Querier, factionID int) ([]UnlockedRegion, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.id, b.country_code,
		       COALESCE((SELECT c.name FROM td.countries c WHERE c.code = b.country_code ORDER BY c.test DESC, c.main DESC, c.id LIMIT 1), b.country_code),
		       `+playerRefSQL("p")+`,
		       (SELECT count(*) FROM td.faction_members m WHERE m.branch_id = b.id)
		FROM td.faction_branches b JOIN td.players p ON p.id = b.leader_player_id
		WHERE b.faction_id = $1 ORDER BY 3`, factionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UnlockedRegion{}
	for rows.Next() {
		var r UnlockedRegion
		var code, name string
		if err := rows.Scan(&r.BranchID, &r.Region, &r.Name, &code, &name, &r.Members); err != nil {
			return nil, err
		}
		r.Commander = &PlayerRef{Code: code, Name: name}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ViewingBranch is the branch a member is viewing (0 = home).
func ViewingBranch(ctx context.Context, q db.Querier, playerID int64) (int64, error) {
	var id sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT viewing_branch_id FROM td.faction_members WHERE player_id = $1`, playerID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id.Int64, err
}

// SetViewingBranch sets the region a member views (0 = home).
func SetViewingBranch(ctx context.Context, q db.Querier, playerID, branchID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.faction_members SET viewing_branch_id = NULLIF($2::bigint, 0) WHERE player_id = $1`,
		playerID, branchID)
	return err
}

// BranchVisitors lists the members of other regions viewing a branch now.
func BranchVisitors(ctx context.Context, q db.Querier, branchID int64) ([]BranchMember, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, `+playerRefSQL("p")+`, p.level, m.rank, COALESCE(b.country_code, '')
		FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		LEFT JOIN td.faction_branches b ON b.id = m.branch_id
		WHERE m.viewing_branch_id = $1 AND m.branch_id IS DISTINCT FROM $1
		ORDER BY p.level DESC, p.id`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BranchMember{}
	for rows.Next() {
		var m BranchMember
		if err := rows.Scan(&m.PlayerID, &m.Code, &m.Name, &m.Level, &m.Rank, &m.From); err != nil {
			return nil, err
		}
		m.Visitor = true
		out = append(out, m)
	}
	return out, rows.Err()
}

// BranchInfo is a branch's region, name and whether it has a commander.
type BranchInfo struct {
	ID        int64
	FactionID int
	Region    string
	Unlocked  bool
}

func BranchByID(ctx context.Context, q db.Querier, id int64) (BranchInfo, error) {
	var b BranchInfo
	err := q.QueryRowContext(ctx, `
		SELECT id, faction_id, country_code, leader_player_id IS NOT NULL FROM td.faction_branches WHERE id = $1`, id).
		Scan(&b.ID, &b.FactionID, &b.Region, &b.Unlocked)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// MoveMemberHome changes a member's home branch (an admin's MOVE REGION). They become a plain member there; a
// Regional Commander's seat they held is left empty.
func MoveMemberHome(ctx context.Context, q db.Querier, playerID, branchID int64, rank int) error {
	if _, err := q.ExecContext(ctx, `UPDATE td.faction_branches SET leader_player_id = NULL WHERE leader_player_id = $1`, playerID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.faction_members SET branch_id = $2, viewing_branch_id = NULL, rank = $3 WHERE player_id = $1`,
		playerID, branchID, rank)
	return err
}
