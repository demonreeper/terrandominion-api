package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Captain squad missions (faction-ranks-plan.md, Squads): a Captain's posts and the missions they give the squad.

// SquadBase is one of the faction's standing bases in the region, as a Captain picks it.
type SquadBase struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Tier    int    `json:"tier"`
	Damaged bool   `json:"damaged"`
}

// StandingBases lists a faction's standing (active) bases in a region, by name, each saying whether a module there is
// below full health.
func StandingBases(ctx context.Context, q db.Querier, factionID int, region string) ([]SquadBase, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT b.id, b.name, b.tier,
		       EXISTS (SELECT 1 FROM td.base_slots s WHERE s.base_id = b.id AND s.module IS NOT NULL AND s.health < s.max_health)
		FROM td.bases b
		WHERE b.faction_id = $1 AND b.region = $2 AND b.status = 'active'
		ORDER BY b.name, b.id`, factionID, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SquadBase{}
	for rows.Next() {
		var b SquadBase
		if err := rows.Scan(&b.ID, &b.Name, &b.Tier, &b.Damaged); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// SquadMember is a member of a Captain's squad as a post reaches them.
type SquadMember struct {
	PlayerID  int64
	Level     int
	FactionID int
}

// SquadMembersOf lists a Captain's squad.
func SquadMembersOf(ctx context.Context, q db.Querier, captainID int64) ([]SquadMember, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT s.player_id, p.level, m.faction_id
		FROM td.squad_members s JOIN td.players p ON p.id = s.player_id
		JOIN td.faction_members m ON m.player_id = s.player_id
		WHERE s.captain_id = $1 ORDER BY s.player_id`, captainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SquadMember
	for rows.Next() {
		var m SquadMember
		if err := rows.Scan(&m.PlayerID, &m.Level, &m.FactionID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CaptainPost is one of a Captain's posts.
type CaptainPost struct {
	ID       int64     `json:"id"`
	BaseID   int64     `json:"baseId"`
	BaseName string    `json:"baseName"`
	Goal     string    `json:"goal"`
	GoalKey  string    `json:"goalKey,omitempty"`
	Count    int       `json:"count"`
	Members  int       `json:"members"`
	At       time.Time `json:"at"`
	// Title is filled by the service ("Give 300 crystal at Coral Spire").
	Title string `json:"title"`
}

// InsertCaptainPost records a post and returns its id.
func InsertCaptainPost(ctx context.Context, q db.Querier, captainID int64, factionID int, region string, baseID int64,
	day time.Time, goal, key string, count int) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.captain_missions (captain_id, faction_id, region, base_id, day, goal, goal_key, count)
		VALUES ($1, $2, $3, $4, $5::date, $6, NULLIF($7, ''), $8) RETURNING id`,
		captainID, factionID, region, baseID, day.Format("2006-01-02"), goal, key, count).Scan(&id)
	return id, err
}

// SetCaptainPostMembers records how many squad members a post went to.
func SetCaptainPostMembers(ctx context.Context, q db.Querier, id int64, members int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.captain_missions SET members = $2 WHERE id = $1`, id, members)
	return err
}

// CaptainPostsOn lists a Captain's posts on a mission day, oldest first.
func CaptainPostsOn(ctx context.Context, q db.Querier, captainID int64, day time.Time) ([]CaptainPost, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.base_id, COALESCE(b.name, ''), c.goal, COALESCE(c.goal_key, ''), c.count, c.members, c.created_at
		FROM td.captain_missions c LEFT JOIN td.bases b ON b.id = c.base_id
		WHERE c.captain_id = $1 AND c.day = $2::date ORDER BY c.id`, captainID, day.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CaptainPost{}
	for rows.Next() {
		var p CaptainPost
		if err := rows.Scan(&p.ID, &p.BaseID, &p.BaseName, &p.Goal, &p.GoalKey, &p.Count, &p.Members, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CaptainMissionType is the mission type a power says a Captain's mission of this goal in.
func CaptainMissionType(ctx context.Context, q db.Querier, factionID int, goal string) (MissionTypeRow, error) {
	var t MissionTypeRow
	err := q.QueryRowContext(ctx, `
		SELECT id, faction_id, weight, goal, goal_key, count, line FROM td.mission_types
		WHERE audience = 'member' AND faction_id = $1 AND goal = $2 AND captain_mission ORDER BY id LIMIT 1`,
		factionID, goal).Scan(&t.ID, &t.FactionID, &t.Weight, &t.Goal, &t.GoalKey, &t.Count, &t.Line)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// GiveCaptainMission gives a squad member a Captain's post as a mission for the day: the post's goal, resource, amount
// and base, in the member's power's words (t). A second give of the same post is ignored.
func GiveCaptainMission(ctx context.Context, q db.Querier, playerID int64, day time.Time, t MissionTypeRow, postID,
	baseID int64, goal, key string, count int, reward MissionReward) error {
	encoded, err := json.Marshal(reward)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.player_missions (player_id, mission_type_id, faction_id, day, audience, goal, goal_key, count, reward,
		                                line, captain_mission_id, base_id)
		VALUES ($1, $2, $3, $4::date, 'member', $5, NULLIF($6, ''), $7, $8::jsonb, $9, $10, $11)
		ON CONFLICT (player_id, captain_mission_id) WHERE captain_mission_id IS NOT NULL DO NOTHING`,
		playerID, t.ID, t.FactionID, day.Format("2006-01-02"), goal, key, count, string(encoded), t.Line, postID, baseID)
	return err
}

// DrawnMissionsGiven says whether the player has had the day's drawn missions (any mission not from a Captain), so a
// Captain's post arriving first doesn't stand in for them.
func DrawnMissionsGiven(ctx context.Context, q db.Querier, playerID int64, day time.Time) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.player_missions WHERE player_id = $1 AND day = $2::date AND captain_mission_id IS NULL)`,
		playerID, day.Format("2006-01-02")).Scan(&ok)
	return ok, err
}
