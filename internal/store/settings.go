package store

import (
	"context"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type Setting struct {
	Key         string    `json:"key"`
	Value       float64   `json:"value"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
		SELECT key, value, description, is_public, updated_at FROM td.settings
		WHERE is_public OR NOT $1 ORDER BY key`, publicOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Setting, 0, 32)
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Description, &s.IsPublic, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
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

func LogAdminAction(ctx context.Context, q db.Querier, adminID int64, action, target string, detailJSON string) error {
	if detailJSON == "" {
		detailJSON = "{}"
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.admin_actions (admin_id, action, target, detail) VALUES ($1, $2, $3, $4::jsonb)`,
		adminID, action, target, detailJSON)
	return err
}
