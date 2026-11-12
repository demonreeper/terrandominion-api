package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
)

// Leader succession (migration 0072): the seat on td.faction_branches (leader_player_id, leader_acting,
// acting_prev_rank, seat_changed_at), td.commander_votes and td.commander_ballots.

// Seat is a branch's Commander seat.
type Seat struct {
	BranchID  int64
	FactionID int
	Region    string
	LeaderID  sql.NullInt64
	Acting    bool
	PrevRank  sql.NullInt64
	ChangedAt sql.NullTime
}

const seatCols = `id, faction_id, country_code, leader_player_id, leader_acting, acting_prev_rank, seat_changed_at`

func scanSeat(row *sql.Row) (Seat, error) {
	var s Seat
	err := row.Scan(&s.BranchID, &s.FactionID, &s.Region, &s.LeaderID, &s.Acting, &s.PrevRank, &s.ChangedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// SeatForUpdate locks a branch's seat.
func SeatForUpdate(ctx context.Context, q db.Querier, branchID int64) (Seat, error) {
	return scanSeat(q.QueryRowContext(ctx, `SELECT `+seatCols+` FROM td.faction_branches WHERE id = $1 FOR UPDATE`, branchID))
}

// SeatOf reads a branch's seat without a lock.
func SeatOf(ctx context.Context, q db.Querier, branchID int64) (Seat, error) {
	return scanSeat(q.QueryRowContext(ctx, `SELECT `+seatCols+` FROM td.faction_branches WHERE id = $1`, branchID))
}

// SeatHeldBy finds and locks the seat a player holds (ErrNotFound: none).
func SeatHeldBy(ctx context.Context, q db.Querier, playerID int64) (Seat, error) {
	return scanSeat(q.QueryRowContext(ctx,
		`SELECT `+seatCols+` FROM td.faction_branches WHERE leader_player_id = $1 ORDER BY id LIMIT 1 FOR UPDATE`, playerID))
}

// SetSeat writes a branch's seat: leader 0 empties it. prevRank 0 clears it; changedAt nil keeps the old time.
func SetSeat(ctx context.Context, q db.Querier, branchID, leader int64, acting bool, prevRank int, changedAt *time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.faction_branches SET leader_player_id = NULLIF($2, 0), leader_acting = $3,
		       acting_prev_rank = NULLIF($4, 0)::smallint, seat_changed_at = COALESCE($5, seat_changed_at)
		WHERE id = $1`, branchID, leader, acting, prevRank, changedAt)
	return err
}

// SeatCandidates lists a branch's players as succession weighs them.
func SeatCandidates(ctx context.Context, q db.Querier, branchID int64) ([]game.SeatCandidate, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id, m.rank, m.nomad, p.contribution_points, m.joined_at,
		       COALESCE(p.last_seen_at, p.created_at), p.status = 'active'
		FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.branch_id = $1 AND p.type = 'player'`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.SeatCandidate
	for rows.Next() {
		var c game.SeatCandidate
		if err := rows.Scan(&c.PlayerID, &c.Rank, &c.Nomad, &c.Points, &c.JoinedAt, &c.LastSeen, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SeatCandidateIn reads one member of a branch as succession weighs them (ErrNotFound: not in that branch).
func SeatCandidateIn(ctx context.Context, q db.Querier, branchID, playerID int64) (game.SeatCandidate, error) {
	var c game.SeatCandidate
	err := q.QueryRowContext(ctx, `
		SELECT m.player_id, m.rank, m.nomad, p.contribution_points, m.joined_at,
		       COALESCE(p.last_seen_at, p.created_at), p.status = 'active'
		FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.branch_id = $1 AND m.player_id = $2 AND p.type = 'player'`, branchID, playerID).
		Scan(&c.PlayerID, &c.Rank, &c.Nomad, &c.Points, &c.JoinedAt, &c.LastSeen, &c.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// BranchPlayers lists the players of a branch who hear about its seat (accounts not banned or deleted).
func BranchPlayers(ctx context.Context, q db.Querier, branchID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id FROM td.faction_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.branch_id = $1 AND p.type = 'player' AND p.status NOT IN ('banned', 'deleted')
		ORDER BY m.player_id`, branchID)
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

// SeatsToVacate lists the branches whose Commander must give up the seat: banned, waiting to be deleted, deleted, or
// not seen since `inactiveBefore` (zero time: inactivity is off).
func SeatsToVacate(ctx context.Context, q db.Querier, inactiveBefore time.Time, limit int) ([]Seat, error) {
	return querySeats(ctx, q, `
		SELECT b.id, b.faction_id, b.country_code, b.leader_player_id, b.leader_acting, b.acting_prev_rank, b.seat_changed_at
		FROM td.faction_branches b JOIN td.players p ON p.id = b.leader_player_id
		WHERE p.type = 'player' AND (p.status <> 'active'
		   OR ($1::timestamptz IS NOT NULL AND COALESCE(p.last_seen_at, p.created_at) < $1::timestamptz))
		ORDER BY b.id LIMIT $2`, nullTime(inactiveBefore), limit)
}

// ActingSeatsDue lists the branches with an acting Commander whose vote window started before `changedBefore` and
// that have no open vote.
func ActingSeatsDue(ctx context.Context, q db.Querier, changedBefore time.Time, limit int) ([]Seat, error) {
	return querySeats(ctx, q, `
		SELECT b.id, b.faction_id, b.country_code, b.leader_player_id, b.leader_acting, b.acting_prev_rank, b.seat_changed_at
		FROM td.faction_branches b
		WHERE b.leader_acting AND b.leader_player_id IS NOT NULL AND b.seat_changed_at <= $1
		  AND NOT EXISTS (SELECT 1 FROM td.commander_votes v WHERE v.branch_id = b.id AND v.status = 'open')
		ORDER BY b.id LIMIT $2`, changedBefore, limit)
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func querySeats(ctx context.Context, q db.Querier, query string, args ...any) ([]Seat, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Seat
	for rows.Next() {
		var s Seat
		if err := rows.Scan(&s.BranchID, &s.FactionID, &s.Region, &s.LeaderID, &s.Acting, &s.PrevRank, &s.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- votes -------------------------------------------------------------------------------------

// CommanderVote is a vote for a branch's Commander.
type CommanderVote struct {
	ID        int64
	BranchID  int64
	StartedBy sql.NullInt64
	OpenedAt  time.Time
	ClosesAt  time.Time
	Status    string
	WinnerID  sql.NullInt64
	Ballots   int
}

const voteCols = `id, branch_id, started_by, opened_at, closes_at, status, winner_id,
	(SELECT count(*) FROM td.commander_ballots c WHERE c.vote_id = v.id)`

func scanVote(row *sql.Row) (CommanderVote, error) {
	var v CommanderVote
	err := row.Scan(&v.ID, &v.BranchID, &v.StartedBy, &v.OpenedAt, &v.ClosesAt, &v.Status, &v.WinnerID, &v.Ballots)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// OpenVote is a branch's open vote (nil: none).
func OpenVote(ctx context.Context, q db.Querier, branchID int64) (*CommanderVote, error) {
	v, err := scanVote(q.QueryRowContext(ctx,
		`SELECT `+voteCols+` FROM td.commander_votes v WHERE v.branch_id = $1 AND v.status = 'open'`, branchID))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// VoteForUpdate locks a vote.
func VoteForUpdate(ctx context.Context, q db.Querier, id int64) (CommanderVote, error) {
	return scanVote(q.QueryRowContext(ctx, `SELECT `+voteCols+` FROM td.commander_votes v WHERE v.id = $1 FOR UPDATE`, id))
}

// InsertVote opens a vote.
func InsertVote(ctx context.Context, q db.Querier, branchID, startedBy int64, opened, closes time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.commander_votes (branch_id, started_by, opened_at, closes_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		branchID, startedBy, opened, closes).Scan(&id)
	return id, err
}

// CastBallot records a voter's ballot; false when they have voted in this vote already.
func CastBallot(ctx context.Context, q db.Querier, voteID, voterID, candidateID int64, now time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.commander_ballots (vote_id, voter_id, candidate_id, cast_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, voteID, voterID, candidateID, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// MyBallot is who a voter picked in a vote (nil: no ballot yet).
func MyBallot(ctx context.Context, q db.Querier, voteID, voterID int64) (*PlayerRef, error) {
	var ref PlayerRef
	err := q.QueryRowContext(ctx, `
		SELECT `+playerRefSQL("p")+` FROM td.commander_ballots c JOIN td.players p ON p.id = c.candidate_id
		WHERE c.vote_id = $1 AND c.voter_id = $2`, voteID, voterID).Scan(&ref.Code, &ref.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

// VoteBallots lists a vote's ballots (the candidate of each).
func VoteBallots(ctx context.Context, q db.Querier, voteID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT candidate_id FROM td.commander_ballots WHERE vote_id = $1 ORDER BY voter_id`, voteID)
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

// CloseVote records a vote's result. endedBy 0: it closed on time.
func CloseVote(ctx context.Context, q db.Querier, id int64, status string, winner int64, ballots int, now time.Time, endedBy int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.commander_votes SET status = $2, winner_id = NULLIF($3, 0), ballots = $4, ended_at = $5,
		       ended_by = NULLIF($6, 0)
		WHERE id = $1 AND status = 'open'`, id, status, winner, ballots, now, endedBy)
	return err
}

// DueVotes lists open votes whose time is up.
func DueVotes(ctx context.Context, q db.Querier, now time.Time, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.commander_votes WHERE status = 'open' AND closes_at <= $1 ORDER BY closes_at LIMIT $2`, now, limit)
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

// PlayerRefByID names a player for the app.
func PlayerRefByID(ctx context.Context, q db.Querier, id int64) (PlayerRef, error) {
	var ref PlayerRef
	err := q.QueryRowContext(ctx, `SELECT `+playerRefSQL("p")+` FROM td.players p WHERE p.id = $1`, id).Scan(&ref.Code, &ref.Name)
	return ref, err
}
