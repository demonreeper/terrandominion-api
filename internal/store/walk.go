package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Walk together (docs/walk-together-plan.md): links, members, asks, the open panel, the trail for the
// walking rule and the daily cap. Every query here names players through PlayerRef; the device hash never
// leaves the server.

// SessionDevice is the device hash a session signed in with.
func SessionDevice(ctx context.Context, q db.Querier, sessionID int64) ([]byte, error) {
	var hash []byte
	err := q.QueryRowContext(ctx, `SELECT device_hash FROM td.sessions WHERE id = $1`, sessionID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return hash, err
}

// WalkPoint is a stored fix: where, how accurate, and when the phone took it.
type WalkPoint struct {
	Lat, Lon  float64
	AccuracyM float64
	FixAt     time.Time
}

// OpenWalkPanel records that the player has WALK TOGETHER open, with their fix.
func OpenWalkPanel(ctx context.Context, q db.Querier, playerID int64, device []byte, fix WalkPoint, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.walk_open (player_id, device_hash, opened_at, fix_at, lat, lng, acc)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (player_id) DO UPDATE SET device_hash = $2, opened_at = $3, fix_at = $4, lat = $5, lng = $6, acc = $7`,
		playerID, device, now, fix.FixAt, fix.Lat, fix.Lon, fix.AccuracyM)
	return err
}

func CloseWalkPanel(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.walk_open WHERE player_id = $1`, playerID)
	return err
}

// OpenPanel is another player's open WALK TOGETHER panel.
type OpenPanel struct {
	PlayerID int64
	Player   PlayerRef
	Device   []byte
	Fix      WalkPoint
}

// WalkPanelOf is one player's open panel opened since a time; ErrNotFound when it isn't open.
func WalkPanelOf(ctx context.Context, q db.Querier, playerID int64, since time.Time) (OpenPanel, error) {
	var p OpenPanel
	err := q.QueryRowContext(ctx, `
		SELECT o.player_id, `+playerRefSQL("p")+`, o.device_hash, o.lat, o.lng, o.acc, o.fix_at
		FROM td.walk_open o JOIN td.players p ON p.id = o.player_id
		WHERE o.player_id = $1 AND o.opened_at >= $2`, playerID, since).
		Scan(&p.PlayerID, &p.Player.Code, &p.Player.Name, &p.Device, &p.Fix.Lat, &p.Fix.Lon, &p.Fix.AccuracyM, &p.Fix.FixAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// FriendsWithPanelOpen lists the player's accepted friends whose panel has reported since a time and who are
// not in a link already.
func FriendsWithPanelOpen(ctx context.Context, q db.Querier, playerID int64, since time.Time) ([]OpenPanel, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT o.player_id, `+playerRefSQL("p")+`, o.device_hash, o.lat, o.lng, o.acc, o.fix_at
		FROM td.walk_open o
		JOIN td.players p ON p.id = o.player_id
		JOIN td.friendships f ON f.low_id = LEAST($1::bigint, o.player_id) AND f.high_id = GREATEST($1::bigint, o.player_id)
		WHERE f.status = 'accepted' AND o.player_id <> $1 AND o.opened_at >= $2 AND p.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM td.walk_link_members m WHERE m.player_id = o.player_id AND m.left_at IS NULL)`,
		playerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenPanel{}
	for rows.Next() {
		var p OpenPanel
		if err := rows.Scan(&p.PlayerID, &p.Player.Code, &p.Player.Name, &p.Device, &p.Fix.Lat, &p.Fix.Lon, &p.Fix.AccuracyM, &p.Fix.FixAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AreFriends says whether two players are accepted friends.
func AreFriends(ctx context.Context, q db.Querier, a, b int64) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.friendships WHERE low_id = LEAST($1::bigint, $2::bigint)
		               AND high_id = GREATEST($1::bigint, $2::bigint) AND status = 'accepted')`, a, b).Scan(&ok)
	return ok, err
}

// ---- asks -------------------------------------------------------------------------------

func InsertWalkAsk(ctx context.Context, q db.Querier, from, to int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.walk_link_asks (from_player, to_player, created_at) VALUES ($1, $2, $3)
		ON CONFLICT (from_player, to_player) DO UPDATE SET created_at = $3`, from, to, now)
	return err
}

func DeleteWalkAsk(ctx context.Context, q db.Querier, from, to int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.walk_link_asks WHERE from_player = $1 AND to_player = $2`, from, to)
	return err
}

// WalkAskExists says whether from asked to since a time.
func WalkAskExists(ctx context.Context, q db.Querier, from, to int64, since time.Time) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.walk_link_asks WHERE from_player = $1 AND to_player = $2 AND created_at >= $3)`,
		from, to, since).Scan(&ok)
	return ok, err
}

// WalkAsksTo lists who asked the player since a time, newest first.
func WalkAsksTo(ctx context.Context, q db.Querier, playerID int64, since time.Time) ([]PlayerRef, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+playerRefSQL("p")+`
		FROM td.walk_link_asks a JOIN td.players p ON p.id = a.from_player
		WHERE a.to_player = $1 AND a.created_at >= $2
		ORDER BY a.created_at DESC`, playerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerRef{}
	for rows.Next() {
		var r PlayerRef
		if err := rows.Scan(&r.Code, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- links and members ------------------------------------------------------------------

// LiveWalkLink is the link the player is in now, or 0.
func LiveWalkLink(ctx context.Context, q db.Querier, playerID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		SELECT m.link_id FROM td.walk_link_members m JOIN td.walk_links l ON l.id = m.link_id
		WHERE m.player_id = $1 AND m.left_at IS NULL AND l.ended_at IS NULL`, playerID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// LockWalkLink holds a link for this transaction, so two heartbeats decide it one at a time.
func LockWalkLink(ctx context.Context, q db.Querier, linkID int64) (bool, error) {
	var ended sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT ended_at FROM td.walk_links WHERE id = $1 FOR UPDATE`, linkID).Scan(&ended)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && !ended.Valid, err
}

func CreateWalkLink(ctx context.Context, q db.Querier, now time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO td.walk_links (created_at) VALUES ($1) RETURNING id`, now).Scan(&id)
	return id, err
}

func AddWalkMember(ctx context.Context, q db.Querier, linkID, playerID int64, device []byte, fix WalkPoint, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.walk_link_members (link_id, player_id, device_hash, joined_at, last_seen_at, last_fix_at, last_lat, last_lng, last_acc)
		VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8)`,
		linkID, playerID, device, now, fix.FixAt, fix.Lat, fix.Lon, fix.AccuracyM)
	return err
}

// WalkMember is a live member as the link checks and the phone see them.
type WalkMember struct {
	PlayerID     int64
	Player       PlayerRef
	Device       []byte
	JoinedAt     time.Time
	LastSeenAt   time.Time
	Fix          *WalkPoint
	OutSince     *time.Time
	BonusActive  bool
	PausedReason string
	BonusSeconds int
	WalkedM      float64

	// W2: Bluetooth proof (tokens rotate; Heard is what this phone reported hearing, comma-joined).
	BleOn          bool
	BleToken       string
	BleTokenPrev   string
	BleTokenAt     *time.Time
	BleHeard       string
	BleHeardAt     *time.Time
	BleConfirmedAt *time.Time
	TwinCount      int
	TogetherSince  *time.Time
	BleReported    bool
}

// WalkMembers lists a link's live members, earliest first.
func WalkMembers(ctx context.Context, q db.Querier, linkID int64) ([]WalkMember, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.player_id, `+playerRefSQL("p")+`, m.device_hash, m.joined_at, m.last_seen_at,
		       m.last_fix_at, m.last_lat, m.last_lng, m.last_acc, m.out_since, m.bonus_active,
		       COALESCE(m.paused_reason, ''), m.bonus_seconds, m.walked_m,
		       m.ble_on, COALESCE(m.ble_token, ''), COALESCE(m.ble_token_prev, ''), m.ble_token_at,
		       m.ble_heard, m.ble_heard_at, m.ble_confirmed_at, m.twin_count, m.together_since, m.ble_reported
		FROM td.walk_link_members m JOIN td.players p ON p.id = m.player_id
		WHERE m.link_id = $1 AND m.left_at IS NULL
		ORDER BY m.joined_at, m.player_id`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WalkMember{}
	for rows.Next() {
		var m WalkMember
		var fixAt, out2, tokenAt, heardAt, confirmedAt, together sql.NullTime
		var lat, lng, acc sql.NullFloat64
		if err := rows.Scan(&m.PlayerID, &m.Player.Code, &m.Player.Name, &m.Device, &m.JoinedAt, &m.LastSeenAt,
			&fixAt, &lat, &lng, &acc, &out2, &m.BonusActive, &m.PausedReason, &m.BonusSeconds, &m.WalkedM,
			&m.BleOn, &m.BleToken, &m.BleTokenPrev, &tokenAt, &m.BleHeard, &heardAt, &confirmedAt, &m.TwinCount,
			&together, &m.BleReported); err != nil {
			return nil, err
		}
		m.BleTokenAt, m.BleHeardAt, m.BleConfirmedAt, m.TogetherSince =
			timePtr(tokenAt), timePtr(heardAt), timePtr(confirmedAt), timePtr(together)
		if fixAt.Valid && lat.Valid && lng.Valid && acc.Valid {
			m.Fix = &WalkPoint{Lat: lat.Float64, Lon: lng.Float64, AccuracyM: acc.Float64, FixAt: fixAt.Time}
		}
		if out2.Valid {
			t := out2.Time
			m.OutSince = &t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// WalkMemberUpdate is what a heartbeat writes for one member.
type WalkMemberUpdate struct {
	Fix          *WalkPoint
	SeenAt       time.Time
	OutSince     *time.Time
	BonusActive  bool
	PausedReason string
	AddBonusSec  int
	AddWalkedM   float64
}

func UpdateWalkMember(ctx context.Context, q db.Querier, linkID, playerID int64, u WalkMemberUpdate) error {
	var fixAt *time.Time
	var lat, lng, acc *float64
	if u.Fix != nil {
		fixAt, lat, lng, acc = &u.Fix.FixAt, &u.Fix.Lat, &u.Fix.Lon, &u.Fix.AccuracyM
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.walk_link_members SET
		    last_seen_at = $3,
		    last_fix_at = COALESCE($4, last_fix_at), last_lat = COALESCE($5, last_lat),
		    last_lng = COALESCE($6, last_lng), last_acc = COALESCE($7, last_acc),
		    out_since = $8, bonus_active = $9, paused_reason = NULLIF($10, ''),
		    bonus_seconds = bonus_seconds + $11, walked_m = walked_m + $12
		WHERE link_id = $1 AND player_id = $2 AND left_at IS NULL`,
		linkID, playerID, u.SeenAt, fixAt, lat, lng, acc, u.OutSince, u.BonusActive, u.PausedReason, u.AddBonusSec, u.AddWalkedM)
	return err
}

// WalkBleUpdate is what a heartbeat writes for a member's Bluetooth proof and the L3 / L8 checks.
type WalkBleUpdate struct {
	BleOn         bool
	Token, Prev   string
	TokenAt       *time.Time
	Heard         string
	HeardAt       time.Time
	ConfirmedAt   *time.Time
	TwinCount     int
	TogetherSince *time.Time
	BleReported   bool
}

func UpdateWalkMemberBle(ctx context.Context, q db.Querier, linkID, playerID int64, u WalkBleUpdate) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.walk_link_members SET ble_on = $3, ble_token = NULLIF($4, ''), ble_token_prev = NULLIF($5, ''),
		       ble_token_at = $6, ble_heard = $7, ble_heard_at = $8, ble_confirmed_at = $9, twin_count = $10,
		       together_since = $11, ble_reported = $12
		WHERE link_id = $1 AND player_id = $2 AND left_at IS NULL`,
		linkID, playerID, u.BleOn, u.Token, u.Prev, u.TokenAt, u.Heard, u.HeardAt, u.ConfirmedAt, u.TwinCount,
		u.TogetherSince, u.BleReported)
	return err
}

// WalkLinkTwinFlagged says whether a link has had twin fixes (L3): no bonus for the rest of it.
func WalkLinkTwinFlagged(ctx context.Context, q db.Querier, linkID int64) (bool, error) {
	var flagged bool
	err := q.QueryRowContext(ctx, `SELECT twin_flagged FROM td.walk_links WHERE id = $1`, linkID).Scan(&flagged)
	return flagged, err
}

func FlagWalkLinkTwins(ctx context.Context, q db.Querier, linkID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.walk_links SET twin_flagged = true WHERE id = $1`, linkID)
	return err
}

// SetWalkMemberOut records (or clears) since when a member has been out of range, and stops their bonus.
func SetWalkMemberOut(ctx context.Context, q db.Querier, linkID, playerID int64, outSince *time.Time, paused string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.walk_link_members SET out_since = $3, bonus_active = false, paused_reason = NULLIF($4, '')
		WHERE link_id = $1 AND player_id = $2 AND left_at IS NULL`, linkID, playerID, outSince, paused)
	return err
}

// LeaveWalkLink takes a member out. Their fix is wiped (only who, when, how long and how far are kept).
func LeaveWalkLink(ctx context.Context, q db.Querier, linkID, playerID int64, reason string, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.walk_link_members SET left_at = $3, left_reason = $4, bonus_active = false,
		       last_lat = NULL, last_lng = NULL, last_acc = NULL, last_fix_at = NULL
		WHERE link_id = $1 AND player_id = $2 AND left_at IS NULL`, linkID, playerID, now, reason)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `DELETE FROM td.walk_link_trail WHERE player_id = $1`, playerID)
	return err
}

// EndWalkLink ends a link: every member still in it leaves with the reason.
func EndWalkLink(ctx context.Context, q db.Querier, linkID int64, reason string, now time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.walk_links SET ended_at = $2, end_reason = $3 WHERE id = $1 AND ended_at IS NULL`, linkID, now, reason)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		DELETE FROM td.walk_link_trail WHERE player_id IN
		    (SELECT player_id FROM td.walk_link_members WHERE link_id = $1 AND left_at IS NULL)`, linkID)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		UPDATE td.walk_link_members SET left_at = $2, left_reason = $3, bonus_active = false,
		       last_lat = NULL, last_lng = NULL, last_acc = NULL, last_fix_at = NULL
		WHERE link_id = $1 AND left_at IS NULL`, linkID, now, reason)
	return err
}

// WalkLiveCount is how many members a link has now.
func WalkLiveCount(ctx context.Context, q db.Querier, linkID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM td.walk_link_members WHERE link_id = $1 AND left_at IS NULL`, linkID).Scan(&n)
	return n, err
}

// WalkStateOf is the player's live link for the bonus at action time: whether linked, the group's size, the
// last heartbeat's decision and when it was.
func WalkStateOf(ctx context.Context, q db.Querier, playerID int64) (linked bool, size int, active bool, seenAt time.Time, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT m.bonus_active, m.last_seen_at,
		       (SELECT count(*) FROM td.walk_link_members o WHERE o.link_id = m.link_id AND o.left_at IS NULL)
		FROM td.walk_link_members m JOIN td.walk_links l ON l.id = m.link_id
		WHERE m.player_id = $1 AND m.left_at IS NULL AND l.ended_at IS NULL`, playerID).Scan(&active, &seenAt, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, false, time.Time{}, nil
	}
	return err == nil, size, active, seenAt, err
}

// SilentWalkMembers lists live members not heard from since a time, as (link, player).
func SilentWalkMembers(ctx context.Context, q db.Querier, before time.Time) ([][2]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.link_id, m.player_id FROM td.walk_link_members m JOIN td.walk_links l ON l.id = m.link_id
		WHERE m.left_at IS NULL AND l.ended_at IS NULL AND m.last_seen_at < $1`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][2]int64
	for rows.Next() {
		var pair [2]int64
		if err := rows.Scan(&pair[0], &pair[1]); err != nil {
			return nil, err
		}
		out = append(out, pair)
	}
	return out, rows.Err()
}

// PurgeWalkHistory deletes links that ended before a time (members go with them), old asks and stale panels.
func PurgeWalkHistory(ctx context.Context, q db.Querier, endedBefore, asksBefore time.Time) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM td.walk_links WHERE ended_at < $1`, endedBefore); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM td.walk_link_asks WHERE created_at < $1`, asksBefore); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `DELETE FROM td.walk_open WHERE opened_at < $1`, asksBefore)
	return err
}

// ---- the walking rule and the daily cap -------------------------------------------------

func AppendWalkTrail(ctx context.Context, q db.Querier, playerID int64, lat, lon float64, at, pruneBefore time.Time) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM td.walk_link_trail WHERE player_id = $1 AND at < $2`, playerID, pruneBefore); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `INSERT INTO td.walk_link_trail (player_id, at, lat, lng) VALUES ($1, $2, $3, $4)`,
		playerID, at, lat, lon)
	return err
}

// WalkTrail is the player's trail since a time, oldest first.
func WalkTrail(ctx context.Context, q db.Querier, playerID int64, since time.Time) ([]WalkPoint, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT lat, lng, at FROM td.walk_link_trail WHERE player_id = $1 AND at >= $2 ORDER BY at`, playerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WalkPoint{}
	for rows.Next() {
		var p WalkPoint
		if err := rows.Scan(&p.Lat, &p.Lon, &p.FixAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func WalkBonusUsed(ctx context.Context, q db.Querier, playerID int64, day time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COALESCE((SELECT seconds FROM td.walk_bonus_days WHERE player_id = $1 AND day = $2), 0)`,
		playerID, day).Scan(&n)
	return n, err
}

func AddWalkBonus(ctx context.Context, q db.Querier, playerID int64, day time.Time, seconds int, metres float64) error {
	if seconds > 0 {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO td.walk_bonus_days (player_id, day, seconds) VALUES ($1, $2, $3)
			ON CONFLICT (player_id, day) DO UPDATE SET seconds = td.walk_bonus_days.seconds + $3`,
			playerID, day, seconds); err != nil {
			return err
		}
	}
	if metres > 0 {
		if _, err := q.ExecContext(ctx, `UPDATE td.players SET walked_together_m = walked_together_m + $2 WHERE id = $1`,
			playerID, metres); err != nil {
			return err
		}
	}
	return nil
}

func WalkedTogether(ctx context.Context, q db.Querier, playerID int64) (float64, error) {
	var m float64
	err := q.QueryRowContext(ctx, `SELECT walked_together_m FROM td.players WHERE id = $1`, playerID).Scan(&m)
	return m, err
}

// ---- W2b extras -------------------------------------------------------------------------

// RecordWalkFinds notes what a linked member's scan found, for every member's map (X1).
func RecordWalkFinds(ctx context.Context, q db.Querier, linkID int64, kind string, ids []int64) error {
	for _, id := range ids {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO td.walk_link_finds (link_id, kind, ref_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			linkID, kind, id); err != nil {
			return err
		}
	}
	return nil
}

// WalkFindNodes is the link's shared nodes still on the map, with their distance from a point.
func WalkFindNodes(ctx context.Context, q db.Querier, linkID int64, lat, lon float64) ([]Node, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography AS g)
		SELECT n.id, n.name, n.resource, ST_Y(n.location::geometry), ST_X(n.location::geometry), n.reserve,
		       ST_Distance(n.location, here.g), n.source = 'rare', n.expires_at, COALESCE(n.group_size, 0)
		FROM td.walk_link_finds f JOIN td.nodes n ON n.id = f.ref_id CROSS JOIN here
		WHERE f.link_id = $1 AND f.kind = 'node' AND n.status = 'active'
		ORDER BY n.location <-> here.g LIMIT 100`, linkID, lat, lon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Resource, &n.Lat, &n.Lon, &n.Reserve, &n.DistanceM, &n.Rare, &n.ExpiresAt, &n.GroupSize); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// WalkFindDrops is the link's shared map drops still lying there.
func WalkFindDrops(ctx context.Context, q db.Querier, linkID int64, lat, lon float64) ([]Drop, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography AS g)
		SELECT d.id, ST_Y(d.location::geometry), ST_X(d.location::geometry), d.contents, d.expires_at,
		       ST_Distance(d.location, here.g)
		FROM td.walk_link_finds f JOIN td.map_drops d ON d.id = f.ref_id CROSS JOIN here
		WHERE f.link_id = $1 AND f.kind = 'drop' AND d.picked_at IS NULL AND d.expires_at > now()
		ORDER BY d.location <-> here.g LIMIT 50`, linkID, lat, lon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Drop{}
	for rows.Next() {
		var d Drop
		var contents []byte
		if err := rows.Scan(&d.ID, &d.Lat, &d.Lon, &contents, &d.ExpiresAt, &d.DistanceM); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(contents, &d.Contents); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GroupNodeHarvested says whether the player has taken their once from a group node (X2).
func GroupNodeHarvested(ctx context.Context, q db.Querier, nodeID, playerID int64) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM td.group_node_harvests WHERE node_id = $1 AND player_id = $2)`,
		nodeID, playerID).Scan(&ok)
	return ok, err
}

func InsertGroupNodeHarvest(ctx context.Context, q db.Querier, nodeID, playerID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO td.group_node_harvests (node_id, player_id) VALUES ($1, $2)`, nodeID, playerID)
	return err
}

// WalkLockstepReported says whether a member has had the L4 lockstep report in this link, and marks it.
func WalkLockstepReported(ctx context.Context, q db.Querier, linkID, playerID int64) (bool, error) {
	var done bool
	err := q.QueryRowContext(ctx, `SELECT lockstep_reported FROM td.walk_link_members WHERE link_id = $1 AND player_id = $2`,
		linkID, playerID).Scan(&done)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	return done, err
}

func MarkWalkLockstepReported(ctx context.Context, q db.Querier, linkID, playerID int64) error {
	_, err := q.ExecContext(ctx, `UPDATE td.walk_link_members SET lockstep_reported = true WHERE link_id = $1 AND player_id = $2`,
		linkID, playerID)
	return err
}
