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

type Player struct {
	ID          int64
	DisplayName string
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
	photo_file, delete_after, created_at`

func scanPlayer(row interface{ Scan(...any) error }) (Player, error) {
	var p Player
	err := row.Scan(&p.ID, &p.DisplayName, &p.Type, &p.Status, &p.IsAdmin, &p.IsMod, &p.Level, &p.XP,
		&p.CountryCode, &p.PhotoFile, &p.DeleteAfter, &p.CreatedAt)
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
func AddXP(ctx context.Context, q db.Querier, id int64, action string, xp int, refID int64) (int64, error) {
	if _, err := q.ExecContext(ctx,
		`INSERT INTO td.xp_events (player_id, action, xp, ref_id) VALUES ($1, $2, $3, NULLIF($4, 0))`,
		id, action, xp, refID); err != nil {
		return 0, err
	}
	var total int64
	err := q.QueryRowContext(ctx,
		`UPDATE td.players SET xp = xp + $2, updated_at = now() WHERE id = $1 RETURNING xp`, id, xp).Scan(&total)
	return total, err
}

func SetLevel(ctx context.Context, q db.Querier, id int64, level int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET level = $2 WHERE id = $1 AND level <> $2`, id, level)
	return err
}
