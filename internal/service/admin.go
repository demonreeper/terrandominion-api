package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

func (s *Service) PublicSettings(ctx context.Context) ([]store.Setting, error) {
	return store.ListSettings(ctx, s.db, true)
}

func (s *Service) AdminSettings(ctx context.Context) ([]store.Setting, error) {
	return store.ListSettings(ctx, s.db, false)
}

// AdminUpdateSetting changes one tunable value and records who changed it and from what.
func (s *Service) AdminUpdateSetting(ctx context.Context, adminID int64, key string, value float64) error {
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ok, err := store.UpdateSetting(ctx, tx, key, value, adminID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.NotFound.With("setting %q", key)
		}
		detail, _ := json.Marshal(map[string]any{"value": value})
		return store.LogAdminAction(ctx, tx, adminID, "setting.update", key, string(detail))
	})
	if err == nil {
		s.invalidateSettings()
	}
	return err
}

var resources = map[string]bool{"crystal": true, "alloy": true, "data": true, "credit": true}

// AdminCreateNode places a node by hand, the way places are seeded in MonoCity.
func (s *Service) AdminCreateNode(ctx context.Context, adminID int64, n store.NewNode) (int64, error) {
	n.Name = strings.TrimSpace(n.Name)
	switch {
	case n.Name == "" || len(n.Name) > 80:
		return 0, apperr.BadRequest.With("node name")
	case !resources[n.Resource]:
		return 0, apperr.BadRequest.With("resource %q", n.Resource)
	case !game.ValidCoordinate(n.Lat, n.Lon):
		return 0, apperr.BadRequest.With("coordinate")
	case n.Yield <= 0 || n.Yield > 1000:
		return 0, apperr.BadRequest.With("yield")
	}
	if n.RegenPerHour <= 0 {
		n.RegenPerHour = 0.05
	}

	var id int64
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		id, err = store.CreateNode(ctx, tx, n, adminID)
		if err != nil {
			return err
		}
		detail, _ := json.Marshal(n)
		return store.LogAdminAction(ctx, tx, adminID, "node.create", n.Name, string(detail))
	})
	return id, err
}
