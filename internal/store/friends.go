package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Friends (owner, 2026-11-11). td.friendships keeps one row per pair, low id first.

// FriendRow is one entry of a player's friend list or pending requests, from their side.
type FriendRow struct {
	Player     PlayerRef `json:"player"`
	Level      int       `json:"level"`
	Faction    string    `json:"faction,omitempty"`
	FriendName string    `json:"friendName,omitempty"` // the name they show friends; only once accepted
	Status     string    `json:"status"`               // friend, incoming, outgoing
	Since      time.Time `json:"since"`
}

// FriendSettings is a player's own friends block: the name they show friends and who may find them.
type FriendSettings struct {
	FriendName      string `json:"friendName"`
	FindableByEmail bool   `json:"findableByEmail"`
	AcceptFriends   bool   `json:"acceptFriends"`
}

func FriendSettingsOf(ctx context.Context, q db.Querier, playerID int64) (FriendSettings, error) {
	var f FriendSettings
	var name sql.NullString
	err := q.QueryRowContext(ctx, `SELECT friend_name, findable_by_email, accept_friends FROM td.players WHERE id = $1`, playerID).
		Scan(&name, &f.FindableByEmail, &f.AcceptFriends)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	f.FriendName = name.String
	return f, err
}

func SetFriendSettings(ctx context.Context, q db.Querier, playerID int64, f FriendSettings) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.players SET friend_name = NULLIF($2, ''), findable_by_email = $3, accept_friends = $4 WHERE id = $1`,
		playerID, f.FriendName, f.FindableByEmail, f.AcceptFriends)
	return err
}

// Friends lists a player's friends and pending requests both ways. The friend name is only read for
// accepted friends.
func Friends(ctx context.Context, q db.Querier, playerID int64) ([]FriendRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+playerRefSQL("p")+`, p.level, COALESCE(f.name, ''),
		       CASE WHEN fr.status = 'accepted' THEN COALESCE(p.friend_name, '') ELSE '' END,
		       CASE WHEN fr.status = 'accepted' THEN 'friend'
		            WHEN fr.requested_by = $1 THEN 'outgoing' ELSE 'incoming' END,
		       COALESCE(fr.accepted_at, fr.created_at)
		FROM td.friendships fr
		JOIN td.players p ON p.id = CASE WHEN fr.low_id = $1 THEN fr.high_id ELSE fr.low_id END
		LEFT JOIN td.faction_members m ON m.player_id = p.id
		LEFT JOIN td.factions f ON f.id = m.faction_id
		WHERE (fr.low_id = $1 OR fr.high_id = $1) AND p.status <> 'deleted'
		ORDER BY td.player_name(p.callsign, p.public_code, p.status)`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FriendRow{}
	for rows.Next() {
		var r FriendRow
		if err := rows.Scan(&r.Player.Code, &r.Player.Name, &r.Level, &r.Faction, &r.FriendName, &r.Status, &r.Since); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Friendship is the pair's row, if any.
type Friendship struct {
	RequestedBy int64
	Status      string
}

func FriendshipForUpdate(ctx context.Context, q db.Querier, low, high int64) (Friendship, error) {
	var f Friendship
	err := q.QueryRowContext(ctx, `
		SELECT requested_by, status FROM td.friendships WHERE low_id = $1 AND high_id = $2 FOR UPDATE`, low, high).
		Scan(&f.RequestedBy, &f.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func InsertFriendRequest(ctx context.Context, q db.Querier, low, high, by int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO td.friendships (low_id, high_id, requested_by) VALUES ($1, $2, $3)`, low, high, by)
	return err
}

func AcceptFriendship(ctx context.Context, q db.Querier, low, high int64, at time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.friendships SET status = 'accepted', accepted_at = $3 WHERE low_id = $1 AND high_id = $2`, low, high, at)
	return err
}

func DeleteFriendship(ctx context.Context, q db.Querier, low, high int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.friendships WHERE low_id = $1 AND high_id = $2`, low, high)
	return err
}

// FriendCount is a player's friends and pending requests together.
func FriendCount(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.friendships WHERE low_id = $1 OR high_id = $1`, playerID).Scan(&n)
	return n, err
}

// FriendRequestsSince counts the requests a player sent from a time (still pending or accepted).
func FriendRequestsSince(ctx context.Context, q db.Querier, playerID int64, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.friendships WHERE requested_by = $1 AND created_at >= $2`, playerID, since).Scan(&n)
	return n, err
}

// FriendStatus is how a viewer stands with another player: "", friend, incoming or outgoing.
func FriendStatus(ctx context.Context, q db.Querier, viewerID, otherID int64) (string, error) {
	low, high := viewerID, otherID
	if low > high {
		low, high = high, low
	}
	var f Friendship
	err := q.QueryRowContext(ctx, `SELECT requested_by, status FROM td.friendships WHERE low_id = $1 AND high_id = $2`, low, high).
		Scan(&f.RequestedBy, &f.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	switch {
	case f.Status == "accepted":
		return "friend", nil
	case f.RequestedBy == viewerID:
		return "outgoing", nil
	}
	return "incoming", nil
}

// FoundFriend is a search result for players: the callsign, level and faction, how the searcher
// stands with them, and whether they take requests. Never an email or a real name.
type FoundFriend struct {
	Player  PlayerRef `json:"player"`
	Level   int       `json:"level"`
	Faction string    `json:"faction,omitempty"`
	Status  string    `json:"status"` // "", friend, incoming, outgoing
	Accepts bool      `json:"accepts"`
	ID      int64     `json:"-"`
}

var foundFriendSQL = `
	SELECT p.id, ` + playerRefSQL("p") + `, p.level, COALESCE(f.name, ''), p.accept_friends,
	       COALESCE((SELECT CASE WHEN fr.status = 'accepted' THEN 'friend'
	                             WHEN fr.requested_by = $1 THEN 'outgoing' ELSE 'incoming' END
	                 FROM td.friendships fr
	                 WHERE fr.low_id = LEAST($1::bigint, p.id) AND fr.high_id = GREATEST($1::bigint, p.id)), '')
	FROM td.players p
	LEFT JOIN td.faction_members m ON m.player_id = p.id
	LEFT JOIN td.factions f ON f.id = m.faction_id
	WHERE p.status = 'active' AND p.type = 'player' AND p.id <> $1`

func scanFound(rows *sql.Rows) ([]FoundFriend, error) {
	defer rows.Close()
	out := []FoundFriend{}
	for rows.Next() {
		var f FoundFriend
		if err := rows.Scan(&f.ID, &f.Player.Code, &f.Player.Name, &f.Level, &f.Faction, &f.Accepts, &f.Status); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindPlayersByCallsign finds players by a piece of their callsign or their whole share code.
func FindPlayersByCallsign(ctx context.Context, q db.Querier, searcherID int64, typed string, limit int) ([]FoundFriend, error) {
	rows, err := q.QueryContext(ctx, foundFriendSQL+`
		  AND p.callsign IS NOT NULL AND (p.callsign ILIKE $2 OR p.public_code = upper($3))
		ORDER BY p.last_seen_at DESC NULLS LAST LIMIT $4`, searcherID, likePattern(typed), typed, limit)
	if err != nil {
		return nil, err
	}
	return scanFound(rows)
}

// FindPlayerByEmail finds the one player with exactly this sign-in email who allows it.
func FindPlayerByEmail(ctx context.Context, q db.Querier, searcherID int64, email string) ([]FoundFriend, error) {
	rows, err := q.QueryContext(ctx, foundFriendSQL+`
		  AND p.findable_by_email
		  AND EXISTS (SELECT 1 FROM td.player_identities i WHERE i.player_id = p.id AND lower(i.email) = $2)
		LIMIT 1`, searcherID, email)
	if err != nil {
		return nil, err
	}
	return scanFound(rows)
}

func RecordEmailSearch(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO td.email_searches (player_id) VALUES ($1)`, playerID)
	return err
}

func EmailSearchesSince(ctx context.Context, q db.Querier, playerID int64, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM td.email_searches WHERE player_id = $1 AND searched_at >= $2`, playerID, since).Scan(&n)
	return n, err
}

// FriendTarget is what a friend request needs to know about the other player.
type FriendTarget struct {
	ID            int64
	IsPlayer      bool
	Active        bool
	AcceptFriends bool
}

func FriendTargetByCode(ctx context.Context, q db.Querier, code string) (FriendTarget, error) {
	var t FriendTarget
	err := q.QueryRowContext(ctx, `
		SELECT id, type = 'player', status = 'active', accept_friends FROM td.players WHERE public_code = $1`, code).
		Scan(&t.ID, &t.IsPlayer, &t.Active, &t.AcceptFriends)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}
