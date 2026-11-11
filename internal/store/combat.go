package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// SpendItem takes consumables, and reports false (taking nothing) if the player has too few.
func SpendItem(ctx context.Context, q db.Querier, playerID int64, item string, count int) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE td.player_items SET count = count - $3 WHERE player_id = $1 AND item = $2 AND count >= $3`,
		playerID, item, count)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// CombatState is what an attack needs to know about a Tether Node that TetherState leaves out.
type CombatState struct {
	JammedUntil sql.NullTime
	SpikeArmed  bool
}

func TetherCombatState(ctx context.Context, q db.Querier, id int64) (CombatState, error) {
	var out CombatState
	err := q.QueryRowContext(ctx, `
		SELECT t.jammed_until, EXISTS (SELECT 1 FROM td.tether_spikes s WHERE s.tn_id = t.id AND s.status = 'armed')
		FROM td.tether_nodes t WHERE t.id = $1`, id).Scan(&out.JammedUntil, &out.SpikeArmed)
	return out, err
}

func JamTether(ctx context.Context, q db.Querier, id int64, until time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.tether_nodes SET jammed_until = $2 WHERE id = $1`, id, until)
	return err
}

// MarkTetherAttacked stamps the moment of a hit, which the map pulses red for a while after.
func MarkTetherAttacked(ctx context.Context, q db.Querier, id int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.tether_nodes SET attacked_at = now() WHERE id = $1`, id)
	return err
}

func PlantSpike(ctx context.Context, q db.Querier, tetherID, playerID int64, firesAt time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.tether_spikes (tn_id, planted_by, fires_at) VALUES ($1, $2, $3)`, tetherID, playerID, firesAt)
	return err
}

// DefuseSpike disarms a spike that has not yet fired, returning who planted it, or ErrNotFound.
func DefuseSpike(ctx context.Context, q db.Querier, tetherID int64) (int64, error) {
	var planter int64
	err := q.QueryRowContext(ctx, `
		UPDATE td.tether_spikes SET status = 'defused'
		WHERE tn_id = $1 AND status = 'armed' AND fires_at > now()
		RETURNING planted_by`, tetherID).Scan(&planter)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return planter, err
}

// DueSpike is an armed spike whose fuse has run out.
type DueSpike struct {
	ID, TetherID, OwnerID, PlantedBy int64
}

func DueSpikes(ctx context.Context, q db.Querier) ([]DueSpike, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT s.id, s.tn_id, t.owner_id, s.planted_by
		FROM td.tether_spikes s JOIN td.tether_nodes t ON t.id = s.tn_id
		WHERE s.status = 'armed' AND s.fires_at <= now() ORDER BY s.fires_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DueSpike, 0, 4)
	for rows.Next() {
		var d DueSpike
		if err := rows.Scan(&d.ID, &d.TetherID, &d.OwnerID, &d.PlantedBy); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FireSpike marks a spike fired, and reports false if something else got to it first.
func FireSpike(ctx context.Context, q db.Querier, id int64) (bool, error) {
	res, err := q.ExecContext(ctx, `UPDATE td.tether_spikes SET status = 'fired' WHERE id = $1 AND status = 'armed'`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// TetherAttackRecord is the evidence kept for one weapon used.
type TetherAttackRecord struct {
	TetherID, AttackerID           int64
	Weapon                         string
	Lat, Lon, AccuracyM, DistanceM float64
}

func InsertTetherAttack(ctx context.Context, q db.Querier, a TetherAttackRecord) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.tether_attacks (tn_id, attacker_id, weapon, reported, accuracy_m, distance_m)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($5, $4), 4326)::geography, $6, $7)`,
		a.TetherID, a.AttackerID, a.Weapon, a.Lat, a.Lon, a.AccuracyM, a.DistanceM)
	return err
}

// tetherPlaceSQL names where Tether Node t stands: the node in its first link, or else the nearest
// node within $radius metres (an ended Tether Node has no links left), or empty. $radius is the
// query's own placeholder, spliced in by the caller.
const tetherPlaceSQL = `COALESCE(
	(SELECT pn.name FROM td.tether_links pl JOIN td.nodes pn ON pn.id = pl.node_id
	 WHERE pl.tn_id = t.id ORDER BY pl.slot LIMIT 1),
	(SELECT pn.name FROM td.nodes pn WHERE ST_DWithin(pn.location, t.location, %s)
	 ORDER BY pn.location <-> t.location LIMIT 1),
	'')`

// TetherPlace is the name of the node a Tether Node stands by ("Crystal Seam K-27"), for notices
// and lists, or "" when there is none within radiusM.
func TetherPlace(ctx context.Context, q db.Querier, id int64, radiusM float64) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `SELECT `+fmt.Sprintf(tetherPlaceSQL, "$2")+` FROM td.tether_nodes t WHERE t.id = $1`,
		id, radiusM).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// ---- contests --------------------------------------------------------------------------

// RecentHarvester is the last player other than the one given to harvest a node since a time.
func RecentHarvester(ctx context.Context, q db.Querier, nodeID, exceptPlayer int64, since time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		SELECT player_id FROM td.harvests
		WHERE node_id = $1 AND player_id <> $2 AND created_at >= $3
		ORDER BY created_at DESC LIMIT 1`, nodeID, exceptPlayer, since).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// NodeHold is a player's claim on a node's next harvests.
type NodeHold struct {
	ContestID int64
	WinnerID  int64
}

// ActiveHold returns the node's live hold, or ErrNotFound. A run-out one counts as no hold.
func ActiveHold(ctx context.Context, q db.Querier, nodeID int64) (NodeHold, error) {
	var out NodeHold
	err := q.QueryRowContext(ctx, `
		SELECT id, winner_id FROM td.node_contests
		WHERE node_id = $1 AND status = 'held' AND prize_left > 0 AND hold_until > now()
		FOR UPDATE`, nodeID).Scan(&out.ContestID, &out.WinnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

// UseHoldHarvest spends one of the winner's harvests, ending the hold on the last one.
func UseHoldHarvest(ctx context.Context, q db.Querier, contestID int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.node_contests
		SET prize_left = prize_left - 1, status = CASE WHEN prize_left <= 1 THEN 'done' ELSE status END
		WHERE id = $1`, contestID)
	return err
}

// HasOpenContest says whether a contest is running on a node.
func HasOpenContest(ctx context.Context, q db.Querier, nodeID int64) (bool, error) {
	var found bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM td.node_contests WHERE node_id = $1 AND status = 'open')`, nodeID).Scan(&found)
	return found, err
}

func InsertContest(ctx context.Context, q db.Querier, nodeID, challenger, defender int64, endsAt time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.node_contests (node_id, challenger_id, defender_id, ends_at)
		VALUES ($1, $2, $3, $4) RETURNING id`, nodeID, challenger, defender, endsAt).Scan(&id)
	return id, err
}

// Contest is a running contest being changed.
type Contest struct {
	ID, NodeID, ChallengerID, DefenderID int64
	Status                               string
	EndsAt                               time.Time
}

func ContestForUpdate(ctx context.Context, q db.Querier, id int64) (Contest, error) {
	var c Contest
	err := q.QueryRowContext(ctx, `
		SELECT id, node_id, challenger_id, defender_id, status, ends_at
		FROM td.node_contests WHERE id = $1 FOR UPDATE`, id).
		Scan(&c.ID, &c.NodeID, &c.ChallengerID, &c.DefenderID, &c.Status, &c.EndsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func JoinContest(ctx context.Context, q db.Querier, contestID, playerID int64, side string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.node_contest_joins (contest_id, player_id, side) VALUES ($1, $2, $3)
		ON CONFLICT (contest_id, player_id) DO NOTHING`, contestID, playerID, side)
	return err
}

// ContestSideCounts says how many allies joined each side and whether the defender showed up.
func ContestSideCounts(ctx context.Context, q db.Querier, c Contest) (challengerAllies, defenderAllies int, defenderShowed bool, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE side = 'challenger'),
		       count(*) FILTER (WHERE side = 'defender' AND player_id <> $2),
		       COALESCE(bool_or(player_id = $2), false)
		FROM td.node_contest_joins WHERE contest_id = $1`, c.ID, c.DefenderID).
		Scan(&challengerAllies, &defenderAllies, &defenderShowed)
	return
}

// WorkingGearTiers totals the tiers of a player's equipped gear that isn't worn out.
func WorkingGearTiers(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var tiers int
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(gt.tier), 0)::int
		FROM td.player_gear pg JOIN td.gear_types gt ON gt.id = pg.gear_type_id
		WHERE pg.player_id = $1 AND pg.equipped AND pg.durability > 0`, playerID).Scan(&tiers)
	return tiers, err
}

// ResolveContest records the result and gives the winner the node, ending any hold before it.
func ResolveContest(ctx context.Context, q db.Querier, c Contest, winner int64, challengerScore, defenderScore, prize int,
	holdUntil time.Time) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE td.node_contests SET status = 'done' WHERE node_id = $1 AND status = 'held'`, c.NodeID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.node_contests
		SET status = 'held', winner_id = $2, challenger_score = $3, defender_score = $4, prize_left = $5, hold_until = $6
		WHERE id = $1`, c.ID, winner, challengerScore, defenderScore, prize, holdUntil)
	return err
}

func DueContests(ctx context.Context, q db.Querier) ([]Contest, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, node_id, challenger_id, defender_id, status, ends_at FROM td.node_contests
		WHERE status = 'open' AND ends_at <= now() ORDER BY ends_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Contest, 0, 4)
	for rows.Next() {
		var c Contest
		if err := rows.Scan(&c.ID, &c.NodeID, &c.ChallengerID, &c.DefenderID, &c.Status, &c.EndsAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EndRunOutHolds closes holds whose time is up, so the unique index frees the node.
func EndRunOutHolds(ctx context.Context, q db.Querier) error {
	_, err := q.ExecContext(ctx, `UPDATE td.node_contests SET status = 'done' WHERE status = 'held' AND hold_until <= now()`)
	return err
}

// NodeContestState is what a player needs to know about one node's contest.
type NodeContestState struct {
	ContestID    int64
	EndsAt       *time.Time
	ChallengerID int64
	DefenderID   int64
	HolderID     int64
	PrizeLeft    int
}

// NodeContestStates reads the open contest and live hold of each node given, keyed by node.
func NodeContestStates(ctx context.Context, q db.Querier, nodeIDs []int64) (map[int64]NodeContestState, error) {
	out := map[int64]NodeContestState{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	parts := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		parts[i] = strconv.FormatInt(id, 10)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id, node_id, challenger_id, defender_id, status, ends_at, COALESCE(winner_id, 0), prize_left
		FROM td.node_contests
		WHERE node_id = ANY($1::bigint[])
		  AND (status = 'open' OR (status = 'held' AND prize_left > 0 AND hold_until > now()))`,
		"{"+strings.Join(parts, ",")+"}")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, node, challenger, defender, winner int64
		var status string
		var ends time.Time
		var prize int
		if err := rows.Scan(&id, &node, &challenger, &defender, &status, &ends, &winner, &prize); err != nil {
			return nil, err
		}
		state := out[node]
		if status == "open" {
			end := ends
			state.ContestID, state.EndsAt, state.ChallengerID, state.DefenderID = id, &end, challenger, defender
		} else {
			state.HolderID, state.PrizeLeft = winner, prize
		}
		out[node] = state
	}
	return out, rows.Err()
}

// RecentlyHarvestedByOthers lists which of the nodes another player harvested since a time.
func RecentlyHarvestedByOthers(ctx context.Context, q db.Querier, nodeIDs []int64, playerID int64, since time.Time) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	parts := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		parts[i] = strconv.FormatInt(id, 10)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT node_id FROM td.harvests
		WHERE node_id = ANY($1::bigint[]) AND player_id <> $2 AND created_at >= $3`,
		"{"+strings.Join(parts, ",")+"}", playerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ---- notices ---------------------------------------------------------------------------

// Notice is one thing that happened to a player. TetherID, Actor (the other player, as named
// at list time) and Amount (damage or resources) are null when they don't apply. Severity is
// filled by the service from the kind.
type Notice struct {
	ID        int64      `json:"id"`
	Kind      string     `json:"kind"`
	Severity  string     `json:"severity"`
	TetherID  *int64     `json:"tnId"`
	Actor     *PlayerRef `json:"actor"`
	Amount    *int64     `json:"amount"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	// Push: this kind is sent to the phone as a notification, so ALERTS lists it. Filled by the service.
	Push bool `json:"push"`
}

// NewNotice is a notice to write. TetherID and ActorID are 0 and Amount nil when they don't apply.
type NewNotice struct {
	PlayerID, TetherID, ActorID int64
	Kind, Body, PushState       string
	Amount                      *int64
}

func InsertNotice(ctx context.Context, q db.Querier, n NewNotice) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.notices (player_id, tn_id, actor_id, amount, kind, body, push_state)
		VALUES ($1, NULLIF($2::bigint, 0), NULLIF($3::bigint, 0), $4, $5, $6, $7)`,
		n.PlayerID, n.TetherID, n.ActorID, n.Amount, n.Kind, n.Body, n.PushState)
	return err
}

// Notices returns a player's notices after an id, newest first. After 0 means the latest ones.
func Notices(ctx context.Context, q db.Querier, playerID, after int64, limit int) ([]Notice, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id, n.kind, n.tn_id, a.id, `+playerRefSQL("a")+`, n.amount, n.body, n.created_at
		FROM td.notices n LEFT JOIN td.players a ON a.id = n.actor_id
		WHERE n.player_id = $1 AND n.id > $2 ORDER BY n.id DESC LIMIT $3`, playerID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Notice, 0, limit)
	for rows.Next() {
		var n Notice
		var tether, amount, actorID sql.NullInt64
		var actorCode, actorName sql.NullString
		if err := rows.Scan(&n.ID, &n.Kind, &tether, &actorID, &actorCode, &actorName, &amount, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		if tether.Valid {
			n.TetherID = &tether.Int64
		}
		if actorID.Valid {
			n.Actor = &PlayerRef{Code: actorCode.String, Name: actorName.String}
		}
		if amount.Valid {
			n.Amount = &amount.Int64
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func DeleteOldNotices(ctx context.Context, q db.Querier, before time.Time) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.notices WHERE created_at < $1`, before)
	return err
}
