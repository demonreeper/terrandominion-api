package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Orders and daily missions. Each mission comes from one faction's lore power (its persona).

// MissionReward mirrors game.MissionReward as stored on the mission row.
type MissionReward struct {
	XP        int    `json:"xp"`
	Credits   int    `json:"credits"`
	Item      string `json:"item,omitempty"`
	ItemCount int    `json:"itemCount,omitempty"`
}

// PlayerMission is one of a player's missions for a day, with its persona.
type PlayerMission struct {
	ID          int64         `json:"id"`
	Persona     string        `json:"persona"`
	FactionCode string        `json:"factionCode"`
	Color       string        `json:"color"`
	Audience    string        `json:"audience"`
	Goal        string        `json:"goal"`
	GoalKey     string        `json:"goalKey,omitempty"`
	Count       int           `json:"count"`
	Progress    int           `json:"progress"`
	Status      string        `json:"status"` // active, done or claimed
	Reward      MissionReward `json:"reward"`
	Line        string        `json:"line"`
	// Title is filled by the service ("Harvest 3 crystal").
	Title     string `json:"title"`
	FactionID int    `json:"-"`
}

// MissionsForDay lists the player's missions for a day, in the order they were given.
func MissionsForDay(ctx context.Context, q db.Querier, playerID int64, day time.Time) ([]PlayerMission, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.id, f.persona, f.code, f.color, m.audience, m.goal, COALESCE(m.goal_key, ''), m.count, m.progress,
		       m.status, m.reward, m.line, m.faction_id
		FROM td.player_missions m JOIN td.factions f ON f.id = m.faction_id
		WHERE m.player_id = $1 AND m.day = $2::date
		ORDER BY m.id`, playerID, day.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerMission, 0, 4)
	for rows.Next() {
		var m PlayerMission
		var reward []byte
		if err := rows.Scan(&m.ID, &m.Persona, &m.FactionCode, &m.Color, &m.Audience, &m.Goal, &m.GoalKey, &m.Count,
			&m.Progress, &m.Status, &reward, &m.Line, &m.FactionID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(reward, &m.Reward); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MissionTypeRow is a mission type that may be given.
type MissionTypeRow struct {
	ID        string
	FactionID int
	Weight    int
	Goal      string
	GoalKey   sql.NullString
	Count     int
	Line      string
}

// MissionCandidates lists the mission types for an audience at a level; factionID 0 means any.
func MissionCandidates(ctx context.Context, q db.Querier, audience string, factionID, level int) ([]MissionTypeRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, faction_id, weight, goal, goal_key, count, line FROM td.mission_types
		WHERE audience = $1 AND ($2::int = 0 OR faction_id = $2::int) AND $3::int BETWEEN min_level AND max_level
		ORDER BY id`, audience, factionID, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MissionTypeRow
	for rows.Next() {
		var t MissionTypeRow
		if err := rows.Scan(&t.ID, &t.FactionID, &t.Weight, &t.Goal, &t.GoalKey, &t.Count, &t.Line); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GiveMission records one mission for a player's day. A repeat of the same type that day is ignored.
func GiveMission(ctx context.Context, q db.Querier, playerID int64, day time.Time, audience string, t MissionTypeRow,
	reward MissionReward) error {
	encoded, err := json.Marshal(reward)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.player_missions (player_id, mission_type_id, faction_id, day, audience, goal, goal_key, count, reward, line)
		VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8, $9::jsonb, $10)
		ON CONFLICT (player_id, day, mission_type_id) DO NOTHING`,
		playerID, t.ID, t.FactionID, day.Format("2006-01-02"), audience, t.Goal, t.GoalKey, t.Count, string(encoded), t.Line)
	return err
}

// FinishedMission is a mission that an action just completed.
type FinishedMission struct {
	ID      int64
	Persona string
	Goal    string
	GoalKey string
	Count   int
}

// AdvanceMissions counts an action toward the player's active missions for the day that match its
// goal (and key, when the mission names one), and returns the ones it finished.
func AdvanceMissions(ctx context.Context, q db.Querier, playerID int64, day time.Time, goal, key string,
	amount int) ([]FinishedMission, error) {
	rows, err := q.QueryContext(ctx, `
		WITH moved AS (
		    UPDATE td.player_missions
		    SET progress = LEAST(count, progress + $5::int),
		        status = CASE WHEN progress + $5::int >= count THEN 'done' ELSE 'active' END,
		        done_at = CASE WHEN progress + $5::int >= count THEN now() ELSE done_at END
		    WHERE player_id = $1 AND day = $2::date AND status = 'active' AND goal = $3
		      AND (goal_key IS NULL OR goal_key = $4)
		    RETURNING id, faction_id, goal, COALESCE(goal_key, '') AS goal_key, count, status)
		SELECT moved.id, f.persona, moved.goal, moved.goal_key, moved.count
		FROM moved JOIN td.factions f ON f.id = moved.faction_id
		WHERE moved.status = 'done'`, playerID, day.Format("2006-01-02"), goal, key, amount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FinishedMission
	for rows.Next() {
		var m FinishedMission
		if err := rows.Scan(&m.ID, &m.Persona, &m.Goal, &m.GoalKey, &m.Count); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MissionForUpdate locks one of the player's missions.
func MissionForUpdate(ctx context.Context, q db.Querier, playerID, id int64) (PlayerMission, error) {
	var m PlayerMission
	var reward []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, status, reward, faction_id, goal, COALESCE(goal_key, ''), count, progress
		FROM td.player_missions WHERE id = $1 AND player_id = $2 FOR UPDATE`, id, playerID).
		Scan(&m.ID, &m.Status, &reward, &m.FactionID, &m.Goal, &m.GoalKey, &m.Count, &m.Progress)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(reward, &m.Reward)
}

func ClaimMission(ctx context.Context, q db.Querier, id int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE td.player_missions SET status = 'claimed', claimed_at = now() WHERE id = $1 AND status = 'done'`, id)
	return err
}

// FollowCounts counts the missions a player finished for each faction's lore power.
func FollowCounts(ctx context.Context, q db.Querier, playerID int64) (map[int]int, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT faction_id, count(*) FROM td.player_missions
		WHERE player_id = $1 AND status IN ('done', 'claimed') GROUP BY faction_id`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var faction, count int
		if err := rows.Scan(&faction, &count); err != nil {
			return nil, err
		}
		out[faction] = count
	}
	return out, rows.Err()
}

// Invite records the faction that invited a freelancer at the faction level (the power that
// speaks to them); a player is invited once. Read it back with InvitedFaction.
func Invite(ctx context.Context, q db.Querier, playerID int64, factionID int) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.player_invitations (player_id, faction_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		playerID, factionID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// BranchMember is one player in a faction's branch, named by share code and callsign.
type BranchMember struct {
	PlayerID  int64  `json:"-"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Level     int    `json:"level"`
	Rank      int    `json:"rank"`
	RankTitle string `json:"rankTitle"`
}

// BranchMembers lists a branch, highest rank first.
func BranchMembers(ctx context.Context, q db.Querier, branchID int64) ([]BranchMember, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, `+playerRefSQL("p")+`, p.level, m.rank
		FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.branch_id = $1 ORDER BY m.rank DESC, p.level DESC, p.id`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BranchMember, 0, 8)
	for rows.Next() {
		var m BranchMember
		if err := rows.Scan(&m.PlayerID, &m.Code, &m.Name, &m.Level, &m.Rank); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMemberRank changes a member's rank.
func SetMemberRank(ctx context.Context, q db.Querier, playerID int64, rank int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.faction_members SET rank = $2 WHERE player_id = $1`, playerID, rank)
	return err
}

// FinishedMissionPowers lists the powers whose freelancer mission the player finished on this
// mission day (done or claimed): each gives them its daily favour until the day ends.
func FinishedMissionPowers(ctx context.Context, q db.Querier, playerID int64, day time.Time) ([]int, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT faction_id FROM td.player_missions
		WHERE player_id = $1 AND day = $2::date AND audience = 'freelancer' AND status IN ('done', 'claimed')
		ORDER BY faction_id`, playerID, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var f int
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
