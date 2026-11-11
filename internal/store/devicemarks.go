package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// The device record (docs/freelancer-path-plan.md phase 2): salted hashes of devices and sign-ins
// with the faction pledged on them and any ban. Never joined to a player row, so it outlives
// account deletion.

// DeviceMark is one row of td.device_marks.
type DeviceMark struct {
	ID            int64     `json:"-"`
	Kind          string    `json:"kind"`
	Hash          []byte    `json:"-"`
	FactionCode   string    `json:"factionCode,omitempty"`
	FactionName   string    `json:"factionName,omitempty"`
	Banned        bool      `json:"banned"`
	BanSanctionID int64     `json:"-"`
	AppealCode    string    `json:"appealCode"`
	CreatedAt     time.Time `json:"createdAt"`
}

const markColumns = `m.id, m.kind, m.hash, COALESCE(m.faction_code, ''), COALESCE(f.name, ''), m.banned,
	COALESCE(m.ban_sanction_id, 0), m.appeal_code, m.created_at`

func scanMarks(rows *sql.Rows, err error) ([]DeviceMark, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceMark{}
	for rows.Next() {
		var m DeviceMark
		if err := rows.Scan(&m.ID, &m.Kind, &m.Hash, &m.FactionCode, &m.FactionName, &m.Banned, &m.BanSanctionID,
			&m.AppealCode, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarksFor reads the marks on a device and a sign-in (either may be nil).
func MarksFor(ctx context.Context, q db.Querier, deviceHash, signinHash []byte) ([]DeviceMark, error) {
	return scanMarks(q.QueryContext(ctx, `
		SELECT `+markColumns+`
		FROM td.device_marks m LEFT JOIN td.factions f ON f.code = m.faction_code
		WHERE (m.kind = 'device' AND m.hash = $1) OR (m.kind = 'signin' AND m.hash = $2)
		ORDER BY m.id`, deviceHash, signinHash))
}

// MarkPledge records that a pledge to a faction was made on this device or sign-in.
func MarkPledge(ctx context.Context, q db.Querier, kind string, hash []byte, factionCode, appealCode string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.device_marks (kind, hash, faction_code, appeal_code) VALUES ($1, $2, $3, $4)
		ON CONFLICT (kind, hash) DO UPDATE SET faction_code = EXCLUDED.faction_code, updated_at = now()`,
		kind, hash, factionCode, appealCode)
	return err
}

// MarkBan records that a ban covers this device or sign-in.
func MarkBan(ctx context.Context, q db.Querier, kind string, hash []byte, sanctionID int64, appealCode string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.device_marks (kind, hash, banned, ban_sanction_id, appeal_code) VALUES ($1, $2, true, $3, $4)
		ON CONFLICT (kind, hash) DO UPDATE SET banned = true, ban_sanction_id = EXCLUDED.ban_sanction_id, updated_at = now()`,
		kind, hash, sanctionID, appealCode)
	return err
}

// dropEmptyMarks removes marks that no longer hold a pledge or a ban.
func dropEmptyMarks(ctx context.Context, q db.Querier) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.device_marks WHERE faction_code IS NULL AND NOT banned`)
	return err
}

// LiftBanMarks clears the marks a lifted ban made.
func LiftBanMarks(ctx context.Context, q db.Querier, sanctionID int64) error {
	if _, err := q.ExecContext(ctx, `
		UPDATE td.device_marks SET banned = false, ban_sanction_id = NULL, updated_at = now()
		WHERE ban_sanction_id = $1`, sanctionID); err != nil {
		return err
	}
	return dropEmptyMarks(ctx, q)
}

// ForgetPledgeMarks clears the faction from marks on these devices and sign-ins (RELEASE PLEDGE).
func ForgetPledgeMarks(ctx context.Context, q db.Querier, deviceHashes, signinHashes [][]byte) error {
	for kind, hashes := range map[string][][]byte{"device": deviceHashes, "signin": signinHashes} {
		for _, hash := range hashes {
			if _, err := q.ExecContext(ctx, `
				UPDATE td.device_marks SET faction_code = NULL, updated_at = now() WHERE kind = $1 AND hash = $2`,
				kind, hash); err != nil {
				return err
			}
		}
	}
	return dropEmptyMarks(ctx, q)
}

// MarksOf lists the marks on the devices and sign-ins a player has used, for admins.
func MarksOf(ctx context.Context, q db.Querier, deviceHashes, signinHashes [][]byte) ([]DeviceMark, error) {
	out := []DeviceMark{}
	for kind, hashes := range map[string][][]byte{"device": deviceHashes, "signin": signinHashes} {
		for _, hash := range hashes {
			marks, err := scanMarks(q.QueryContext(ctx, `
				SELECT `+markColumns+`
				FROM td.device_marks m LEFT JOIN td.factions f ON f.code = m.faction_code
				WHERE m.kind = $1 AND m.hash = $2`, kind, hash))
			if err != nil {
				return nil, err
			}
			out = append(out, marks...)
		}
	}
	return out, nil
}

// MarkByAppeal finds a mark by its appeal code.
func MarkByAppeal(ctx context.Context, q db.Querier, code string) (DeviceMark, error) {
	marks, err := scanMarks(q.QueryContext(ctx, `
		SELECT `+markColumns+`
		FROM td.device_marks m LEFT JOIN td.factions f ON f.code = m.faction_code
		WHERE m.appeal_code = $1`, code))
	if err != nil {
		return DeviceMark{}, err
	}
	if len(marks) == 0 {
		return DeviceMark{}, ErrNotFound
	}
	return marks[0], nil
}

// DeleteMark removes a whole mark (an admin, after an appeal).
func DeleteMark(ctx context.Context, q db.Querier, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.device_marks WHERE id = $1`, id)
	return err
}

// PlayerDeviceHashes lists the device hashes of every session a player ever had.
func PlayerDeviceHashes(ctx context.Context, q db.Querier, playerID int64) ([][]byte, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT device_hash FROM td.sessions WHERE player_id = $1`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var h []byte
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SignInKey is one way a player signs in, as the device record hashes it.
type SignInKey struct {
	Provider string
	Subject  string
}

// PlayerSignInKeys lists a player's sign-ins (provider and subject).
func PlayerSignInKeys(ctx context.Context, q db.Querier, playerID int64) ([]SignInKey, error) {
	rows, err := q.QueryContext(ctx, `SELECT provider, subject FROM td.player_identities WHERE player_id = $1`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SignInKey
	for rows.Next() {
		var k SignInKey
		if err := rows.Scan(&k.Provider, &k.Subject); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// BoundFaction is the faction the device record binds a player to ("" when none).
func BoundFaction(ctx context.Context, q db.Querier, playerID int64) (string, error) {
	var code sql.NullString
	err := q.QueryRowContext(ctx, `SELECT bound_faction_code FROM td.players WHERE id = $1`, playerID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return code.String, err
}

// SetBoundFaction binds a player to a faction ("" clears it).
func SetBoundFaction(ctx context.Context, q db.Querier, playerID int64, code string) error {
	_, err := q.ExecContext(ctx, `UPDATE td.players SET bound_faction_code = NULLIF($2, '') WHERE id = $1`, playerID, code)
	return err
}
