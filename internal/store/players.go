// Package store is the SQL layer: one function per query, no game rules. Every function takes a
// db.Querier, so the service layer decides which calls share a transaction.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

var ErrNotFound = errors.New("not found")

// Player is a td.players row. DisplayName is the Google or Apple account's real name, kept for
// support and admin use only: it never goes to another player. Name is what players see.
type Player struct {
	ID          int64
	DisplayName string
	// Code is the 8-character share code. Callsign is null until the player picks one.
	Code              string
	Callsign          sql.NullString
	CallsignChangedAt sql.NullTime
	// Name is the player-facing name: the callsign, "OPERATIVE-<code>" until one is picked, or
	// "Deleted player" for a deleted or banned account.
	Name        string
	Type        string
	Status      string
	IsAdmin     bool
	IsMod       bool
	Level       int
	XP          int64
	CountryCode string
	PhotoFile   sql.NullString
	DeleteAfter sql.NullTime
	CreatedAt   time.Time
}

const playerColumns = `id, display_name, type, status, is_admin, is_mod, level, xp, country_code,
	photo_file, delete_after, created_at, public_code, callsign, callsign_changed_at,
	td.player_name(callsign, public_code, status)`

// PlayerRef is how one player is named to another: the share code that opens their profile and
// the name to show. Code is empty for a deleted or banned account, whose name is not a link.
type PlayerRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Ref is the player as other players see them.
func (p Player) Ref() PlayerRef {
	if p.Status == "deleted" || p.Status == "banned" {
		return PlayerRef{Name: p.Name}
	}
	return PlayerRef{Code: p.Code, Name: p.Name}
}

// playerRefP is playerRefSQL("p") as a constant, for column lists that are constants themselves.
const playerRefP = `td.player_code(p.public_code, p.status), td.player_name(p.callsign, p.public_code, p.status)`

// playerRefSQL is the two columns of a PlayerRef (code, name) for the td.players row aliased by
// alias, in that order. Every player-facing query names players through it.
func playerRefSQL(alias string) string {
	return `td.player_code(` + alias + `.public_code, ` + alias + `.status), ` +
		`td.player_name(` + alias + `.callsign, ` + alias + `.public_code, ` + alias + `.status)`
}

func scanPlayer(row interface{ Scan(...any) error }) (Player, error) {
	var p Player
	err := row.Scan(&p.ID, &p.DisplayName, &p.Type, &p.Status, &p.IsAdmin, &p.IsMod, &p.Level, &p.XP,
		&p.CountryCode, &p.PhotoFile, &p.DeleteAfter, &p.CreatedAt, &p.Code, &p.Callsign, &p.CallsignChangedAt, &p.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func GetPlayer(ctx context.Context, q db.Querier, id int64) (Player, error) {
	return scanPlayer(q.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM td.players WHERE id = $1`, id))
}

// GetPlayerForUpdate locks the player row for the rest of the transaction, so concurrent actions
// by the same player (two taps, two phones) apply one after the other.
func GetPlayerForUpdate(ctx context.Context, q db.Querier, id int64) (Player, error) {
	return scanPlayer(q.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM td.players WHERE id = $1 FOR UPDATE`, id))
}

// PlayerIDByCode finds a player by share code (upper case). Deleted and banned accounts are not
// found: to everyone else they no longer exist.
func PlayerIDByCode(ctx context.Context, q db.Querier, code string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx,
		`SELECT id FROM td.players WHERE public_code = $1 AND status NOT IN ('deleted', 'banned') AND type <> 'system'`, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// CallsignTaken says whether another player already has this callsign, in any letter case.
func CallsignTaken(ctx context.Context, q db.Querier, callsign string, exceptPlayerID int64) (bool, error) {
	var taken bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM td.players WHERE lower(callsign) = lower($1) AND id <> $2)`,
		callsign, exceptPlayerID).Scan(&taken)
	return taken, err
}

// SetCallsign stores a player's callsign and when it changed.
func SetCallsign(ctx context.Context, q db.Querier, playerID int64, callsign string, at time.Time) error {
	_, err := q.ExecContext(ctx,
		`UPDATE td.players SET callsign = $2, callsign_changed_at = $3, updated_at = now() WHERE id = $1`,
		playerID, callsign, at)
	return err
}

// BlockedCallsignWords lists the words a callsign may not contain.
func BlockedCallsignWords(ctx context.Context, q db.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT word FROM td.callsign_blocked`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 32)
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, err
		}
		out = append(out, word)
	}
	return out, rows.Err()
}

// ProtectionLostUntil is when a freelancer's neutral standing returns (nil: they never lost it).
func ProtectionLostUntil(ctx context.Context, q db.Querier, id int64) (*time.Time, error) {
	var until sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT protection_lost_until FROM td.players WHERE id = $1`, id).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil || !until.Valid {
		return nil, err
	}
	return &until.Time, nil
}

// LoseProtection marks a freelancer as unprotected until the given time.
func LoseProtection(ctx context.Context, q db.Querier, id int64, until time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET protection_lost_until = $2 WHERE id = $1`, id, until)
	return err
}

func FindIdentity(ctx context.Context, q db.Querier, provider, subject string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx,
		`SELECT player_id FROM td.player_identities WHERE provider = $1 AND subject = $2`,
		provider, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// CreatePrimaryAdmin makes player 1, the owner (an admin), if there is no player 1 yet. It reports
// false when the id was already taken, and the caller makes an ordinary account.
func CreatePrimaryAdmin(ctx context.Context, q db.Querier, id int64, displayName, countryCode string) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO td.players (id, display_name, country_code, is_admin) OVERRIDING SYSTEM VALUE
		VALUES ($1, $2, $3, true) ON CONFLICT (id) DO NOTHING`, id, displayName, countryCode)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func CreatePlayer(ctx context.Context, q db.Querier, displayName, countryCode string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx,
		`INSERT INTO td.players (display_name, country_code) VALUES ($1, $2) RETURNING id`,
		displayName, countryCode).Scan(&id)
	return id, err
}

func LinkIdentity(ctx context.Context, q db.Querier, provider, subject string, playerID int64, email string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.player_identities (provider, subject, player_id, email) VALUES ($1, $2, $3, NULLIF($4, ''))`,
		provider, subject, playerID, email)
	return err
}

// TouchPlayer records the app version and last-seen time, and cancels a pending self-deletion:
// signing in during the grace period reactivates the account.
// PlayerIdentities lists the providers a player can sign in with ("apple", "dev", "google"), sorted.
func PlayerIdentities(ctx context.Context, q db.Querier, playerID int64) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT provider FROM td.player_identities WHERE player_id = $1 ORDER BY provider`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// UnlinkIdentity removes a player's sign-in of one provider.
func UnlinkIdentity(ctx context.Context, q db.Querier, playerID int64, provider string) error {
	_, err := q.ExecContext(ctx,
		`DELETE FROM td.player_identities WHERE player_id = $1 AND provider = $2`, playerID, provider)
	return err
}

func TouchPlayer(ctx context.Context, q db.Querier, id int64, appVersion string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.players
		SET last_seen_at = now(),
		    last_app_version = COALESCE(NULLIF($2, ''), last_app_version),
		    status = CASE WHEN status = 'pending_delete' THEN 'active' ELSE status END,
		    delete_after = CASE WHEN status = 'pending_delete' THEN NULL ELSE delete_after END,
		    updated_at = now()
		WHERE id = $1`, id, appVersion)
	return err
}

func ScheduleDeletion(ctx context.Context, q db.Querier, id int64, after time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.players SET status = 'pending_delete', delete_after = $2, updated_at = now()
		WHERE id = $1 AND status = 'active'`, id, after)
	return err
}

// AddXP adds XP and stores the level the caller computed from the new total.
// AddXP records XP in the ledger and adds it to the player. targetID is the other player in a fight
// (0 otherwise), which the daily cap on fighting XP counts.
func AddXP(ctx context.Context, q db.Querier, id int64, action string, xp int, refID, targetID int64) (int64, error) {
	if _, err := q.ExecContext(ctx,
		`INSERT INTO td.xp_events (player_id, action, xp, ref_id, target_player_id)
		 VALUES ($1, $2, $3, NULLIF($4::bigint, 0), NULLIF($5::bigint, 0))`,
		id, action, xp, refID, targetID); err != nil {
		return 0, err
	}
	var total int64
	err := q.QueryRowContext(ctx,
		`UPDATE td.players SET xp = xp + $2, updated_at = now() WHERE id = $1 RETURNING xp`, id, xp).Scan(&total)
	return total, err
}

// PaidXPAgainst counts the ledger rows of this action that paid the player XP against targetID since
// the given time (the fighting cap).
func PaidXPAgainst(ctx context.Context, q db.Querier, id, targetID int64, action string, since time.Time) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM td.xp_events
		 WHERE player_id = $1 AND target_player_id = $2 AND action = $3 AND xp > 0 AND created_at >= $4`,
		id, targetID, action, since).Scan(&n)
	return n, err
}

func SetLevel(ctx context.Context, q db.Querier, id int64, level int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET level = $2 WHERE id = $1 AND level <> $2`, id, level)
	return err
}
