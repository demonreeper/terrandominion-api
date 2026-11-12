package service

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction bases, B6 (base-building-plan.md 7): the upkeep tick. Runs with the minute's economy tick, after the visit
// tick has marked every base a member stood at (which ends a fade).
func (s *Service) upkeepTick(ctx context.Context, st game.Settings, now time.Time) {
	bases, err := store.UpkeepBases(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 upkeep tick failed", "err", err)
		}
		return
	}
	for _, u := range bases {
		step := game.BaseUpkeep(u.Tier, u.LastVisit, u.FadedAt, u.DroppedAt, now, st)
		if step == game.UpkeepNone {
			continue
		}
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			b, err := store.BaseCombatForUpdate(ctx, tx, u.ID)
			if err != nil {
				return err
			}
			members, err := store.FactionMembersInRegion(ctx, tx, b.FactionID, b.Region)
			if err != nil {
				return err
			}
			switch step {
			case game.UpkeepFade:
				if err := store.SetBaseFaded(ctx, tx, b.ID, now); err != nil {
					return err
				}
				for _, id := range members {
					if err := notify(ctx, tx, store.NewNotice{PlayerID: id, Kind: "base_fading", Body: game.NoticeBaseFading(b.Name)}); err != nil {
						return err
					}
				}
				return nil
			case game.UpkeepDropTier:
				if err := s.dropTier(ctx, tx, b, false, st, now); err != nil {
					return err
				}
				return store.SetFadeDropped(ctx, tx, b.ID, now)
			case game.UpkeepCollapse:
				// The members hear it from destroyBase (half the vault drops on the map, as for a destroyed Spire).
				return s.destroyBase(ctx, tx, b, st, now)
			}
			return nil
		})
		if err != nil {
			slog.Error("E1900 upkeep failed", "base", u.ID, "err", err)
		}
	}
}
