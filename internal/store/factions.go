package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type Faction struct {
	ID           int        `json:"id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Motto        string     `json:"motto"`
	Color        string     `json:"color"`
	Status       string     `json:"status"`
	LockedReason string     `json:"lockedReason,omitempty"`
	LeaderName   string     `json:"leaderName,omitempty"`
	ClaimedAt    *time.Time `json:"claimedAt,omitempty"`
	Members      int        `json:"members"`

	LeaderID sql.NullInt64 `json:"-"`
}

func ListFactions(ctx context.Context, q db.Querier) ([]Faction, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT f.id, f.code, f.name, f.motto, f.color, f.status, f.locked_reason,
		       COALESCE(p.display_name, ''), f.claimed_at,
		       (SELECT count(*) FROM td.faction_members m WHERE m.faction_id = f.id)
		FROM td.factions f LEFT JOIN td.players p ON p.id = f.leader_player_id
		ORDER BY f.sort`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Faction, 0, 8)
	for rows.Next() {
		var f Faction
		var claimed sql.NullTime
		if err := rows.Scan(&f.ID, &f.Code, &f.Name, &f.Motto, &f.Color, &f.Status, &f.LockedReason,
			&f.LeaderName, &claimed, &f.Members); err != nil {
			return nil, err
		}
		if claimed.Valid {
			f.ClaimedAt = &claimed.Time
		}
		if f.Status != "locked" {
			f.LockedReason = ""
		}
		out = append(out, f)
	}
	return out, rows.Err()
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

// ClaimFaction wakes a locked faction under its first leader. The WHERE clause is the real guard:
// if two players race, only one update matches and the other sees false.
func ClaimFaction(ctx context.Context, q db.Querier, factionID int, playerID int64) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.factions SET status = 'active', leader_player_id = $2, claimed_at = now()
		WHERE id = $1 AND status = 'locked' AND leader_player_id IS NULL`, factionID, playerID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func AddMember(ctx context.Context, q db.Querier, playerID int64, factionID int, rank int) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.faction_members (player_id, faction_id, rank) VALUES ($1, $2, $3)`,
		playerID, factionID, rank)
	return err
}

type Membership struct {
	FactionID   int    `json:"factionId"`
	FactionCode string `json:"factionCode"`
	FactionName string `json:"factionName"`
	Rank        int    `json:"rank"`
	BranchID    *int64 `json:"branchId,omitempty"`
}

// MembershipOf returns nil for a freelancer.
func MembershipOf(ctx context.Context, q db.Querier, playerID int64) (*Membership, error) {
	var m Membership
	var branch sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT f.id, f.code, f.name, m.rank, m.branch_id
		FROM td.faction_members m JOIN td.factions f ON f.id = m.faction_id
		WHERE m.player_id = $1`, playerID).Scan(&m.FactionID, &m.FactionCode, &m.FactionName, &m.Rank, &branch)
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
