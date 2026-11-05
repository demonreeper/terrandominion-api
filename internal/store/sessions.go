package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type Session struct {
	ID        int64
	PlayerID  int64
	ExpiresAt time.Time
	RevokedAt sql.NullTime
}

func CreateSession(ctx context.Context, q db.Querier, playerID int64, refreshHash, deviceHash []byte,
	platform, appVersion string, expires time.Time) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO td.sessions (player_id, refresh_hash, device_hash, platform, app_version, expires_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6) RETURNING id`,
		playerID, refreshHash, deviceHash, platform, appVersion, expires).Scan(&id)
	return id, err
}

// SessionByRefreshForUpdate locks the session so a refresh token can only be spent once, even if
// the client retries.
func SessionByRefreshForUpdate(ctx context.Context, q db.Querier, refreshHash []byte) (Session, error) {
	var s Session
	err := q.QueryRowContext(ctx, `
		SELECT id, player_id, expires_at, revoked_at FROM td.sessions
		WHERE refresh_hash = $1 FOR UPDATE`, refreshHash).
		Scan(&s.ID, &s.PlayerID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// RotateSession swaps in a new refresh hash and slides the expiry forward.
func RotateSession(ctx context.Context, q db.Querier, id int64, refreshHash []byte, appVersion string, expires time.Time) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.sessions
		SET refresh_hash = $2, expires_at = $3, last_used_at = now(),
		    app_version = COALESCE(NULLIF($4, ''), app_version)
		WHERE id = $1`, id, refreshHash, expires, appVersion)
	return err
}

func RevokeSession(ctx context.Context, q db.Querier, id, playerID int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE td.sessions SET revoked_at = now() WHERE id = $1 AND player_id = $2 AND revoked_at IS NULL`,
		id, playerID)
	return err
}

func RevokeAllSessions(ctx context.Context, q db.Querier, playerID int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE td.sessions SET revoked_at = now() WHERE player_id = $1 AND revoked_at IS NULL`, playerID)
	return err
}

// PruneSessions deletes sessions that expired or were revoked more than a month ago.
func PruneSessions(ctx context.Context, q db.Querier) (int64, error) {
	res, err := q.ExecContext(ctx, `
		DELETE FROM td.sessions
		WHERE expires_at < now() - interval '30 days'
		   OR revoked_at < now() - interval '30 days'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RegisterDevice records a sign-in from a device and reports whether this device has already
// received the welcome grant. The first account on a phone gets it; later ones do not.
func RegisterDevice(ctx context.Context, q db.Querier, deviceHash []byte, playerID int64, newAccount bool) (alreadyGranted bool, err error) {
	increment := 0
	if newAccount {
		increment = 1
	}
	err = q.QueryRowContext(ctx, `
		INSERT INTO td.devices (device_hash, first_player_id, accounts)
		VALUES ($1, $2, $3)
		ON CONFLICT (device_hash) DO UPDATE
		SET last_seen_at = now(), accounts = td.devices.accounts + $3
		RETURNING welcome_granted`, deviceHash, playerID, increment).Scan(&alreadyGranted)
	return alreadyGranted, err
}

func MarkWelcomeGranted(ctx context.Context, q db.Querier, deviceHash []byte) error {
	_, err := q.ExecContext(ctx, `UPDATE td.devices SET welcome_granted = true WHERE device_hash = $1`, deviceHash)
	return err
}
