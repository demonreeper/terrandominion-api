package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

// Setting is one tunable number, with what the admin screen needs to show and change it safely:
// its group and name, the range it may take, the plus and minus step (1 = whole numbers) and
// whether it is locked.
type Setting struct {
	Key         string    `json:"key"`
	Value       float64   `json:"value"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Group       string    `json:"group"`
	Label       string    `json:"label"`
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Step        float64   `json:"step"`
	Locked      bool      `json:"locked"`
}

const settingColumns = `key, value, description, is_public, updated_at, grp, label, min_value, max_value, step, locked`

func scanSetting(row interface{ Scan(...any) error }) (Setting, error) {
	var s Setting
	err := row.Scan(&s.Key, &s.Value, &s.Description, &s.IsPublic, &s.UpdatedAt, &s.Group, &s.Label,
		&s.Min, &s.Max, &s.Step, &s.Locked)
	return s, err
}

func LoadSettings(ctx context.Context, q db.Querier) (map[string]float64, error) {
	rows, err := q.QueryContext(ctx, `SELECT key, value FROM td.settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func ListSettings(ctx context.Context, q db.Querier, publicOnly bool) ([]Setting, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+settingColumns+` FROM td.settings
		WHERE is_public OR NOT $1 ORDER BY label, key`, publicOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Setting, 0, 160)
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SettingForUpdate reads one setting and locks its row, so a change is checked against the range
// and the old value it really replaces.
func SettingForUpdate(ctx context.Context, q db.Querier, key string) (Setting, error) {
	s, err := scanSetting(q.QueryRowContext(ctx,
		`SELECT `+settingColumns+` FROM td.settings WHERE key = $1 FOR UPDATE`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// UpdateSetting changes an existing setting only; new keys arrive through migrations, so a typo
// in the admin tool cannot create a setting nothing reads.
func UpdateSetting(ctx context.Context, q db.Querier, key string, value float64, adminID int64) (bool, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE td.settings SET value = $2, updated_at = now(), updated_by = $3 WHERE key = $1`,
		key, value, adminID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SettingChange is one past change to a setting, for the screen's history.
type SettingChange struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Old   float64 `json:"old"`
	New   float64 `json:"new"`
	// By is the admin as players see them (their callsign).
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// RecentSettingChanges lists the latest changes, newest first. Changes logged before the old value
// was kept (0038) are left out: there is nothing to show them from.
func RecentSettingChanges(ctx context.Context, q db.Querier, limit int) ([]SettingChange, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT a.target, COALESCE(NULLIF(s.label, ''), a.target),
		       (a.detail->>'old')::double precision, (a.detail->>'value')::double precision,
		       COALESCE(td.player_name(p.callsign, p.public_code, p.status), ''), a.created_at
		FROM td.admin_actions a
		LEFT JOIN td.settings s ON s.key = a.target
		LEFT JOIN td.players p ON p.id = a.admin_id
		WHERE a.action = 'setting.update' AND a.detail ? 'old' AND a.detail ? 'value'
		ORDER BY a.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SettingChange, 0, limit)
	for rows.Next() {
		var c SettingChange
		if err := rows.Scan(&c.Key, &c.Label, &c.Old, &c.New, &c.By, &c.At); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func LogAdminAction(ctx context.Context, q db.Querier, adminID int64, action, target string, detailJSON string) error {
	if detailJSON == "" {
		detailJSON = "{}"
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.admin_actions (admin_id, action, target, detail) VALUES ($1, $2, $3, $4::jsonb)`,
		adminID, action, target, detailJSON)
	return err
}

// Announcement is the one line on everyone's main menu.
type Announcement struct {
	Text   string    `json:"text"`
	Kind   string    `json:"kind"`
	EndsAt time.Time `json:"endsAt"`
}

// CurrentAnnouncement is the announcement still running, or nil.
func CurrentAnnouncement(ctx context.Context, q db.Querier) (*Announcement, error) {
	var a Announcement
	err := q.QueryRowContext(ctx,
		`SELECT body, kind, ends_at FROM td.announcement WHERE id = 1 AND ends_at > now()`).
		Scan(&a.Text, &a.Kind, &a.EndsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SetAnnouncement posts or replaces the announcement.
func SetAnnouncement(ctx context.Context, q db.Querier, text, kind string, endsAt time.Time, adminID int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.announcement (id, body, kind, ends_at, posted_by) VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET body = $1, kind = $2, ends_at = $3, posted_by = $4, created_at = now()`,
		text, kind, endsAt, adminID)
	return err
}

// ClearAnnouncement removes the announcement.
func ClearAnnouncement(ctx context.Context, q db.Querier) error {
	_, err := q.ExecContext(ctx, `DELETE FROM td.announcement`)
	return err
}
