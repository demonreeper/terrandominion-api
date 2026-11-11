package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Admin players and stats (docs/profile-plan.md, A2).

// FoundPlayer is one row of the staff player search. The name is the callsign whatever the status.
type FoundPlayer struct {
	Player   PlayerRef  `json:"player"`
	Level    int        `json:"level"`
	Status   string     `json:"status"`
	Faction  string     `json:"faction,omitempty"`
	IsAdmin  bool       `json:"isAdmin"`
	IsMod    bool       `json:"isMod"`
	LastSeen *time.Time `json:"lastSeen,omitempty"`
}

// likePattern makes typed text safe inside a LIKE pattern, with % on both sides.
func likePattern(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

// SearchPlayers finds players by a piece of their callsign or share code; with private set (admins)
// also by their id or a piece of an email. Deleted accounts are left out.
func SearchPlayers(ctx context.Context, q db.Querier, typed string, private bool, limit int) ([]FoundPlayer, error) {
	id, _ := strconv.ParseInt(typed, 10, 64)
	rows, err := q.QueryContext(ctx, `
		SELECT p.public_code, COALESCE(p.callsign, 'OPERATIVE-' || p.public_code), p.level, p.status::text,
		       COALESCE(f.name, ''), p.is_admin, p.is_mod, p.last_seen_at
		FROM td.players p
		LEFT JOIN td.faction_members m ON m.player_id = p.id
		LEFT JOIN td.factions f ON f.id = m.faction_id
		WHERE p.status <> 'deleted' AND p.type = 'player'
		  AND (p.callsign ILIKE $1 OR p.public_code ILIKE $1
		       OR ($2 AND (p.id = $3 OR EXISTS (
		            SELECT 1 FROM td.player_identities i WHERE i.player_id = p.id AND i.email ILIKE $1))))
		ORDER BY p.last_seen_at DESC NULLS LAST, p.id DESC LIMIT $4`,
		likePattern(typed), private, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FoundPlayer{}
	for rows.Next() {
		var f FoundPlayer
		var seen sql.NullTime
		if err := rows.Scan(&f.Player.Code, &f.Player.Name, &f.Level, &f.Status, &f.Faction, &f.IsAdmin, &f.IsMod, &seen); err != nil {
			return nil, err
		}
		f.LastSeen = timePtr(seen)
		out = append(out, f)
	}
	return out, rows.Err()
}

// PlayerActivity is what staff see of a player beyond the profile: when they joined and were last
// seen, and how many reports there are by and against them.
type PlayerActivity struct {
	Registered     time.Time
	LastSeen       *time.Time
	SuspendedUntil *time.Time
	TethersActive  int
	ReportsAgainst int
	ReportsBy      int
}

func PlayerActivityOf(ctx context.Context, q db.Querier, playerID int64) (PlayerActivity, error) {
	var a PlayerActivity
	var seen, until sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT p.created_at, p.last_seen_at, p.suspended_until,
		       (SELECT count(*) FROM td.tether_nodes t WHERE t.owner_id = p.id AND t.status = 'active'),
		       (SELECT count(*) FROM td.player_reports r WHERE r.target_id = p.id),
		       (SELECT count(*) FROM td.player_reports r WHERE r.reporter_id = p.id)
		FROM td.players p WHERE p.id = $1`, playerID).
		Scan(&a.Registered, &seen, &until, &a.TethersActive, &a.ReportsAgainst, &a.ReportsBy)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	a.LastSeen, a.SuspendedUntil = timePtr(seen), timePtr(until)
	return a, err
}

// SignIn is one way a player signs in, with the email that provider gave.
type SignIn struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
}

func PlayerSignIns(ctx context.Context, q db.Querier, playerID int64) ([]SignIn, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT provider, COALESCE(email, '') FROM td.player_identities WHERE player_id = $1 ORDER BY created_at`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SignIn{}
	for rows.Next() {
		var s SignIn
		if err := rows.Scan(&s.Provider, &s.Email); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LastSession is the phone a player last used: its platform and app version ("" when never).
func LastSession(ctx context.Context, q db.Querier, playerID int64) (platform, appVersion string, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT platform, COALESCE(app_version, '') FROM td.sessions
		WHERE player_id = $1 ORDER BY last_used_at DESC LIMIT 1`, playerID).Scan(&platform, &appVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return platform, appVersion, err
}

// SharedPhone lists the other accounts that have signed in on a phone this player signed in on.
func SharedPhone(ctx context.Context, q db.Querier, playerID int64) ([]PlayerRef, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.public_code, COALESCE(p.callsign, 'OPERATIVE-' || p.public_code)
		FROM td.sessions mine
		JOIN td.sessions other ON other.device_hash = mine.device_hash AND other.player_id <> mine.player_id
		JOIN td.players p ON p.id = other.player_id AND p.status <> 'deleted'
		WHERE mine.player_id = $1 ORDER BY p.id LIMIT 20`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerRef{}
	for rows.Next() {
		var id int64
		var ref PlayerRef
		if err := rows.Scan(&id, &ref.Code, &ref.Name); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// InsertInfringement stores a failed location check, unless one of the same kind was stored for
// this player inside the gap. Reports whether it was stored.
func InsertInfringement(ctx context.Context, q db.Querier, playerID int64, kind, detail, path string, gap time.Duration) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.infringements (player_id, kind, detail, path)
		SELECT $1, $2, $3, $4
		WHERE NOT EXISTS (
			SELECT 1 FROM td.infringements
			WHERE player_id = $1 AND kind = $2 AND created_at > now() - make_interval(secs => $5))`,
		playerID, kind, detail, path, gap.Seconds())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// InfringementKind is how often one kind of check failed for a player, and when last.
type InfringementKind struct {
	Kind  string    `json:"kind"`
	Count int       `json:"count"`
	Last  time.Time `json:"last"`
}

// Infringement is one stored failed check.
type Infringement struct {
	Kind   string    `json:"kind"`
	Detail string    `json:"detail"`
	Path   string    `json:"path"`
	At     time.Time `json:"at"`
	// Hard: it started (or would have started) a signal lock. Evidence holds the two fixes of a
	// spoofing incident; it is only in the admin-only private block.
	Hard      bool            `json:"hard,omitempty"`
	Dismissed bool            `json:"dismissed,omitempty"`
	Evidence  json.RawMessage `json:"evidence,omitempty"`
}

func InfringementKinds(ctx context.Context, q db.Querier, playerID int64) ([]InfringementKind, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT kind, count(*), max(created_at) FROM td.infringements
		WHERE player_id = $1 GROUP BY kind ORDER BY kind`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InfringementKind{}
	for rows.Next() {
		var k InfringementKind
		if err := rows.Scan(&k.Kind, &k.Count, &k.Last); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func RecentInfringements(ctx context.Context, q db.Querier, playerID int64, limit int) ([]Infringement, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT kind, detail, path, created_at, hard, dismissed, evidence FROM td.infringements
		WHERE player_id = $1 ORDER BY created_at DESC LIMIT $2`, playerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Infringement{}
	for rows.Next() {
		var i Infringement
		var evidence []byte
		if err := rows.Scan(&i.Kind, &i.Detail, &i.Path, &i.At, &i.Hard, &i.Dismissed, &evidence); err != nil {
			return nil, err
		}
		if len(evidence) > 2 {
			i.Evidence = json.RawMessage(evidence)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// PlayerAction is one admin action on a player, from td.admin_actions: an adjustment, a role
// change or a callsign reset. Detail is what was stored with it.
type PlayerAction struct {
	Action string          `json:"action"`
	By     string          `json:"by"`
	At     time.Time       `json:"at"`
	Detail json.RawMessage `json:"detail"`
}

// PlayerActions lists the admin actions on a player whose name starts with "player.", newest first.
func PlayerActions(ctx context.Context, q db.Querier, playerID int64, limit int) ([]PlayerAction, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT a.action, COALESCE(b.callsign, 'OPERATIVE-' || b.public_code, ''), a.created_at, a.detail
		FROM td.admin_actions a LEFT JOIN td.players b ON b.id = a.admin_id
		WHERE a.target = $1 AND a.action LIKE 'player.%' ORDER BY a.id DESC LIMIT $2`,
		strconv.FormatInt(playerID, 10), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerAction{}
	for rows.Next() {
		var a PlayerAction
		var raw []byte
		if err := rows.Scan(&a.Action, &a.By, &a.At, &raw); err != nil {
			return nil, err
		}
		a.Detail = json.RawMessage(raw)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ClearCallsign removes a player's callsign, so they are OPERATIVE-<code> until they pick again;
// the wait between changes starts over, so the new pick is free.
func ClearCallsign(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.players SET callsign = NULL, callsign_changed_at = NULL, updated_at = now() WHERE id = $1`, playerID)
	return err
}

func SetMod(ctx context.Context, q db.Querier, playerID int64, isMod bool) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET is_mod = $2, updated_at = now() WHERE id = $1`, playerID, isMod)
	return err
}

// ---- game stats ---------------------------------------------------------------------------------

// FactionCount is how many members a faction has.
type FactionCount struct {
	Name    string `json:"name"`
	Color   string `json:"color"`
	Members int    `json:"members"`
}

// DayActive is how many players did something on one day (the game's own time zone).
type DayActive struct {
	Day    string `json:"day"` // 2026-11-11
	Active int    `json:"active"`
}

// GameStats are the game's totals: counts only, no names, nothing with a place in it.
type GameStats struct {
	PlayersTotal  int `json:"playersTotal"`
	NewWeek       int `json:"newWeek"`
	Active24h     int `json:"active24h"`
	Active7d      int `json:"active7d"`
	Active30d     int `json:"active30d"`
	HarvestsMonth int `json:"harvestsMonth"`
	TethersActive int `json:"tethersActive"`
	FightsWeek    int `json:"fightsWeek"`
	// Retained of RetainedOf: the players who joined 7 to 14 days ago, and those of them seen
	// again a week or more after joining.
	Retained   int            `json:"retained"`
	RetainedOf int            `json:"retainedOf"`
	Factions   []FactionCount `json:"factions"`
	Days       []DayActive    `json:"days"`
}

// statsZone is the time zone a "day" and a "month" of the stats are counted in (launch is the Maldives only).
const statsZone = "Indian/Maldives"

// LoadGameStats counts the game's totals at now. Only real player accounts count (no bots, no
// deleted accounts).
func LoadGameStats(ctx context.Context, q db.Querier, now time.Time) (GameStats, error) {
	var s GameStats
	const real = `p.type = 'player' AND p.status <> 'deleted'`
	err := q.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM td.players p WHERE `+real+`),
		       (SELECT count(*) FROM td.players p WHERE `+real+` AND p.created_at > $1::timestamptz - interval '7 days'),
		       (SELECT count(*) FROM td.players p WHERE `+real+` AND p.last_seen_at > $1::timestamptz - interval '24 hours'),
		       (SELECT count(*) FROM td.players p WHERE `+real+` AND p.last_seen_at > $1::timestamptz - interval '7 days'),
		       (SELECT count(*) FROM td.players p WHERE `+real+` AND p.last_seen_at > $1::timestamptz - interval '30 days'),
		       (SELECT count(*) FROM td.harvests h
		         WHERE h.created_at >= date_trunc('month', $1::timestamptz AT TIME ZONE $2) AT TIME ZONE $2),
		       (SELECT count(*) FROM td.tether_nodes t WHERE t.status = 'active'),
		       (SELECT count(*) FROM td.tether_attacks a WHERE a.created_at > $1::timestamptz - interval '7 days')
		         + (SELECT count(*) FROM td.drone_strikes d WHERE d.created_at > $1::timestamptz - interval '7 days')
		         + (SELECT count(*) FROM td.node_contests c WHERE c.created_at > $1::timestamptz - interval '7 days'),
		       (SELECT count(*) FROM td.players p WHERE `+real+`
		          AND p.created_at > $1::timestamptz - interval '14 days' AND p.created_at <= $1::timestamptz - interval '7 days'
		          AND p.last_seen_at >= p.created_at + interval '7 days'),
		       (SELECT count(*) FROM td.players p WHERE `+real+`
		          AND p.created_at > $1::timestamptz - interval '14 days' AND p.created_at <= $1::timestamptz - interval '7 days')`,
		now, statsZone).
		Scan(&s.PlayersTotal, &s.NewWeek, &s.Active24h, &s.Active7d, &s.Active30d, &s.HarvestsMonth,
			&s.TethersActive, &s.FightsWeek, &s.Retained, &s.RetainedOf)
	if err != nil {
		return s, err
	}

	rows, err := q.QueryContext(ctx, `
		SELECT f.name, f.color, count(m.player_id)
		FROM td.factions f LEFT JOIN td.faction_members m ON m.faction_id = f.id
		GROUP BY f.id ORDER BY f.sort`)
	if err != nil {
		return s, err
	}
	s.Factions = []FactionCount{}
	for rows.Next() {
		var f FactionCount
		if err := rows.Scan(&f.Name, &f.Color, &f.Members); err != nil {
			rows.Close()
			return s, err
		}
		s.Factions = append(s.Factions, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return s, err
	}

	// A player is active on a day they earned XP: every action pays some, and the ledger keeps the day.
	rows, err = q.QueryContext(ctx, `
		SELECT to_char(d.day, 'YYYY-MM-DD'), count(DISTINCT x.player_id)
		FROM generate_series(
		       date_trunc('day', $1::timestamptz AT TIME ZONE $2) - interval '13 days',
		       date_trunc('day', $1::timestamptz AT TIME ZONE $2), interval '1 day') AS d(day)
		LEFT JOIN td.xp_events x
		       ON x.created_at >= d.day AT TIME ZONE $2 AND x.created_at < (d.day + interval '1 day') AT TIME ZONE $2
		      AND x.action NOT LIKE 'admin%'
		GROUP BY d.day ORDER BY d.day`, now, statsZone)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	s.Days = []DayActive{}
	for rows.Next() {
		var d DayActive
		if err := rows.Scan(&d.Day, &d.Active); err != nil {
			return s, err
		}
		s.Days = append(s.Days, d)
	}
	return s, rows.Err()
}
