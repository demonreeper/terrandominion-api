package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// The JOBS board (docs/freelancer-path-plan.md F6).

// Job is one posted job, as the board shows it. Target names what to hit, scout or guard; the
// poster is named by faction only, never by player.
type Job struct {
	ID           int64            `json:"id"`
	Kind         string           `json:"kind"`
	FactionCode  string           `json:"factionCode"`
	FactionName  string           `json:"factionName"`
	FactionColor string           `json:"factionColor"`
	Target       string           `json:"target"`
	TetherID     int64            `json:"tetherId,omitempty"`
	NodeID       int64            `json:"nodeId,omitempty"`
	Lat          float64          `json:"lat"`
	Lon          float64          `json:"lon"`
	RadiusM      float64          `json:"radiusM"`
	Reward       map[string]int64 `json:"reward"`
	Slots        int              `json:"slots"`
	Taken        int              `json:"taken"`
	Done         int              `json:"done"`
	Status       string           `json:"status"`
	Deadline     time.Time        `json:"deadline"`
	CreatedAt    time.Time        `json:"createdAt"`
	// Mine: the asking freelancer's take on it (taken, done, failed), with guard progress.
	MyStatus     string `json:"myStatus,omitempty"`
	GuardSeconds int    `json:"guardSeconds,omitempty"`

	PosterID  int64      `json:"-"`
	FactionID int        `json:"-"`
	TetherOwn int64      `json:"-"`
	LastFixAt *time.Time `json:"-"`
	TakeID    int64      `json:"-"`
}

const jobColumns = `
	j.id, j.kind, f.code, f.name, f.color,
	CASE j.kind
	    WHEN 'hit' THEN COALESCE((SELECT td.player_name(p.callsign, p.public_code, p.status) || '''s Tether Node'
	                              FROM td.tether_nodes t JOIN td.players p ON p.id = t.owner_id WHERE t.id = j.tn_id), 'A Tether Node')
	    WHEN 'guard' THEN COALESCE((SELECT n.name FROM td.nodes n WHERE n.id = j.node_id), 'A node')
	    ELSE 'An area' END,
	COALESCE(j.tn_id, 0), COALESCE(j.node_id, 0), j.lat, j.lon, j.radius_m, j.reward, j.slots,
	(SELECT count(*) FROM td.job_takes k WHERE k.job_id = j.id AND k.status <> 'failed'),
	j.done, j.status, j.deadline, j.created_at, COALESCE(j.poster_id, 0), j.faction_id,
	COALESCE((SELECT t.owner_id FROM td.tether_nodes t WHERE t.id = j.tn_id), 0)`

func scanJob(row interface{ Scan(...any) error }, extra ...any) (Job, error) {
	var j Job
	var reward []byte
	dest := []any{&j.ID, &j.Kind, &j.FactionCode, &j.FactionName, &j.FactionColor, &j.Target, &j.TetherID, &j.NodeID,
		&j.Lat, &j.Lon, &j.RadiusM, &reward, &j.Slots, &j.Taken, &j.Done, &j.Status, &j.Deadline, &j.CreatedAt,
		&j.PosterID, &j.FactionID, &j.TetherOwn}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return j, err
	}
	return j, json.Unmarshal(reward, &j.Reward)
}

// NewJob is a job being posted.
type NewJob struct {
	PosterID  int64
	FactionID int
	Kind      string
	TetherID  int64
	NodeID    int64
	Lat, Lon  float64
	RadiusM   float64
	Reward    map[string]int64
	Slots     int
	Deadline  time.Time
}

func InsertJob(ctx context.Context, q db.Querier, j NewJob) (int64, error) {
	reward, err := json.Marshal(j.Reward)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `
		INSERT INTO td.jobs (poster_id, faction_id, kind, tn_id, node_id, lat, lon, radius_m, reward, slots, deadline)
		VALUES ($1, $2, $3, NULLIF($4::bigint, 0), NULLIF($5::bigint, 0), $6, $7, $8, $9::jsonb, $10, $11) RETURNING id`,
		j.PosterID, j.FactionID, j.Kind, j.TetherID, j.NodeID, j.Lat, j.Lon, j.RadiusM, string(reward), j.Slots, j.Deadline).Scan(&id)
	return id, err
}

// OpenJobsOfFaction counts a faction's open jobs; JobsPostedSince a poster's jobs since a time.
func OpenJobsOfFaction(ctx context.Context, q db.Querier, factionID int) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.jobs WHERE faction_id = $1 AND status = 'open'`, factionID).Scan(&n)
	return n, err
}

func JobsPostedSince(ctx context.Context, q db.Querier, posterID int64, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.jobs WHERE poster_id = $1 AND created_at >= $2`, posterID, since).Scan(&n)
	return n, err
}

// OpenJobs lists the board for a freelancer: every open job with slots, soonest deadline first,
// with that freelancer's own take marked.
func OpenJobs(ctx context.Context, q db.Querier, playerID int64, limit int) ([]Job, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+jobColumns+`
		FROM td.jobs j JOIN td.factions f ON f.id = j.faction_id
		LEFT JOIN td.job_takes k ON k.job_id = j.id AND k.taker_id = $1
		WHERE (j.status = 'open' AND j.deadline > now() AND (k.id IS NOT NULL OR
		       (SELECT count(*) FROM td.job_takes t WHERE t.job_id = j.id AND t.status <> 'failed') < j.slots))
		   OR (k.id IS NOT NULL AND j.created_at > now() - interval '3 days')
		ORDER BY (k.id IS NULL), j.deadline LIMIT $2`, playerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	// The freelancer's own take on each, read in a second pass so scanJob stays shared.
	for i := range out {
		if err := q.QueryRowContext(ctx, `SELECT status, guard_seconds FROM td.job_takes WHERE job_id = $1 AND taker_id = $2`,
			out[i].ID, playerID).Scan(&out[i].MyStatus, &out[i].GuardSeconds); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return out, nil
}

// PostedJobs lists the jobs a faction posted lately, for its Wardens and above.
func PostedJobs(ctx context.Context, q db.Querier, factionID int, limit int) ([]Job, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+jobColumns+`
		FROM td.jobs j JOIN td.factions f ON f.id = j.faction_id
		WHERE j.faction_id = $1 AND (j.status = 'open' OR j.created_at > now() - interval '3 days')
		ORDER BY j.status = 'open' DESC, j.created_at DESC LIMIT $2`, factionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// JobForUpdate reads and locks one job.
func JobForUpdate(ctx context.Context, q db.Querier, id int64) (Job, error) {
	j, err := scanJob(q.QueryRowContext(ctx, `
		SELECT `+jobColumns+` FROM td.jobs j JOIN td.factions f ON f.id = j.faction_id WHERE j.id = $1 FOR UPDATE OF j`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// HasTake says whether a freelancer already took a job; TakenCount how many they have open.
func HasTake(ctx context.Context, q db.Querier, jobID, playerID int64) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM td.job_takes WHERE job_id = $1 AND taker_id = $2)`, jobID, playerID).Scan(&ok)
	return ok, err
}

func TakenCount(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.job_takes k JOIN td.jobs j ON j.id = k.job_id
		WHERE k.taker_id = $1 AND k.status = 'taken' AND j.status = 'open'`, playerID).Scan(&n)
	return n, err
}

func InsertTake(ctx context.Context, q db.Querier, jobID, playerID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO td.job_takes (job_id, taker_id) VALUES ($1, $2)`, jobID, playerID)
	return err
}

// OpenTakesOf lists a freelancer's taken jobs of one kind that are still open, locked, for the
// completion checks.
func OpenTakesOf(ctx context.Context, q db.Querier, playerID int64, kind string) ([]Job, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+jobColumns+`, k.id, k.guard_seconds, k.last_fix_at
		FROM td.job_takes k JOIN td.jobs j ON j.id = k.job_id JOIN td.factions f ON f.id = j.faction_id
		WHERE k.taker_id = $1 AND k.status = 'taken' AND j.status = 'open' AND j.kind = $2 AND j.deadline > now()
		ORDER BY j.id FOR UPDATE OF k, j`, playerID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var last sql.NullTime
		var take int64
		var guard int
		j, err := scanJob(rows, &take, &guard, &last)
		if err != nil {
			return nil, err
		}
		j.TakeID, j.GuardSeconds, j.LastFixAt = take, guard, timePtr(last)
		out = append(out, j)
	}
	return out, rows.Err()
}

// RecordGuard adds guard time to a take and notes the fix.
func RecordGuard(ctx context.Context, q db.Querier, takeID int64, seconds int, at time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.job_takes SET guard_seconds = guard_seconds + $2, last_fix_at = $3 WHERE id = $1`,
		takeID, seconds, at)
	return err
}

// FinishTake marks a take done and counts it on the job, closing the job when every slot is paid.
func FinishTake(ctx context.Context, q db.Querier, takeID, jobID int64, at time.Time) error {
	if _, err := q.ExecContext(ctx, `UPDATE td.job_takes SET status = 'done', done_at = $2 WHERE id = $1`, takeID, at); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.jobs SET done = done + 1, status = CASE WHEN done + 1 >= slots THEN 'closed' ELSE status END WHERE id = $1`, jobID)
	return err
}

// DueJobs lists open jobs past their deadline, locked, to end them.
func DueJobs(ctx context.Context, q db.Querier, now time.Time, limit int) ([]Job, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+jobColumns+` FROM td.jobs j JOIN td.factions f ON f.id = j.faction_id
		WHERE j.status = 'open' AND j.deadline <= $1 ORDER BY j.deadline LIMIT $2 FOR UPDATE OF j SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// EndJob closes a job that ran out: unfinished takes fail, and the refund is recorded.
func EndJob(ctx context.Context, q db.Querier, jobID int64, refund map[string]int64) error {
	encoded, err := json.Marshal(refund)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE td.job_takes SET status = 'failed' WHERE job_id = $1 AND status = 'taken'`, jobID); err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE td.jobs SET status = 'expired', refunded = $2::jsonb WHERE id = $1`, jobID, string(encoded))
	return err
}

// ShareAPhone says whether two players have signed in on the same phone.
func ShareAPhone(ctx context.Context, q db.Querier, a, b int64) (bool, error) {
	var shared bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.sessions x JOIN td.sessions y ON y.device_hash = x.device_hash
		               WHERE x.player_id = $1 AND y.player_id = $2)`, a, b).Scan(&shared)
	return shared, err
}

// TetherBrief is what posting a hit job needs to know about its target.
type TetherBrief struct {
	OwnerID   int64
	FactionID *int
	Lat, Lon  float64
	Status    string
}

func TetherBriefOf(ctx context.Context, q db.Querier, id int64) (TetherBrief, error) {
	var t TetherBrief
	var faction sql.NullInt32
	err := q.QueryRowContext(ctx, `
		SELECT owner_id, faction_id, ST_Y(location::geometry), ST_X(location::geometry), status FROM td.tether_nodes WHERE id = $1`, id).
		Scan(&t.OwnerID, &faction, &t.Lat, &t.Lon, &t.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if faction.Valid {
		f := int(faction.Int32)
		t.FactionID = &f
	}
	return t, err
}

// NodeBrief is where a node is, for a guard job.
func NodeBrief(ctx context.Context, q db.Querier, id int64) (lat, lon float64, active bool, err error) {
	var status string
	err = q.QueryRowContext(ctx, `SELECT ST_Y(location::geometry), ST_X(location::geometry), status FROM td.nodes WHERE id = $1`, id).
		Scan(&lat, &lon, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, ErrNotFound
	}
	return lat, lon, status == "active", err
}
