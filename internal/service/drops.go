package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// NearbyDrops lists map drops around the player, within their scanner's range.
func (s *Service) NearbyDrops(ctx context.Context, playerID int64, lat, lon float64) ([]store.Drop, error) {
	if !game.ValidCoordinate(lat, lon) {
		return nil, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	stats, _, err := s.equipped(ctx, s.db, playerID, settings)
	if err != nil {
		return nil, err
	}
	radius := settings.NearbyMaxRadiusM
	if stats.ScanRangeM > 0 {
		radius = math.Min(radius, stats.ScanRangeM)
	}
	return store.NearbyDrops(ctx, s.db, lat, lon, radius, 100)
}

type PickDropOutcome struct {
	Taken map[string]int64 `json:"taken"`
	// LinkBonus is the walk-together percent added to what was taken (0 when not linked); the extra is
	// in Taken and Carried, never taken from the drop.
	LinkBonus int              `json:"linkBonus,omitempty"`
	Left      map[string]int64 `json:"left"`
	Carried   map[string]int64 `json:"carried"`
}

// PickDrop moves what fits of a drop into the player's carried resources. Anything that does not
// fit stays on the map for them or anyone else.
func (s *Service) PickDrop(ctx context.Context, playerID, dropID int64, fix TetherFix) (PickDropOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return PickDropOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out PickDropOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		drop, err := store.DropForUpdate(ctx, tx, dropID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.DropGone
		}
		if err != nil {
			return err
		}

		lastFix, err := store.LastHarvestFix(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var last *game.Fix
		if lastFix != nil {
			last = &game.Fix{Lat: lastFix.Lat, Lon: lastFix.Lon, AccuracyM: lastFix.AccuracyM, At: lastFix.At}
		}
		player := game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: s.now()}
		if _, ruleErr := game.CheckAtDrop(settings, player, last, drop.Lat, drop.Lon); ruleErr != nil {
			return ruleErr
		}

		stats, _, err := s.equipped(ctx, tx, playerID, settings)
		if err != nil {
			return err
		}
		carried, err := store.Carried(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var total int64
		for _, amount := range carried {
			total += amount
		}
		taken, left := game.TakeDrop(drop.Contents, stats.CarryCapacity-total)
		if len(taken) == 0 {
			return apperr.HarvestCarryFull
		}
		// Walk together: the link adds its percent on top of what was taken, as far as the rig can carry it.
		// It comes from the link, not the drop, so the drop keeps what it had left.
		linkPct := s.walkBonus(ctx, tx, playerID, settings).Yield
		room := stats.CarryCapacity - total
		for _, amount := range taken {
			room -= amount
		}
		got := map[string]int64{}
		for resource, amount := range taken {
			got[resource] = amount
			if linkPct > 0 && room > 0 {
				extra := min(game.WalkAmount(amount, 1, linkPct)-amount, room)
				got[resource] += extra
				room -= extra
			}
		}
		for resource, amount := range got {
			if err := store.AddCarried(ctx, tx, playerID, resource, amount); err != nil {
				return err
			}
			carried[resource] += amount
		}
		if len(left) == 0 {
			err = store.MarkDropPicked(ctx, tx, dropID, playerID)
		} else {
			err = store.SetDropContents(ctx, tx, dropID, left)
		}
		if err != nil {
			return err
		}
		out = PickDropOutcome{Taken: got, Left: left, Carried: carried}
		if linkPct > 0 {
			out.LinkBonus = linkPct
		}
		return nil
	})
	return out, err
}

// endTether closes a Tether Node with a final status and drops what it held on the map.
func (s *Service) endTether(ctx context.Context, tx *sql.Tx, id int64, tether store.TetherState, status string,
	settings game.Settings) error {
	if err := store.EndTether(ctx, tx, id, status); err != nil {
		return err
	}
	held, err := store.TakeTetherStock(ctx, tx, id)
	if err != nil || len(held) == 0 {
		return err
	}
	lifetime := time.Duration(settings.DropLifetimeHours * float64(time.Hour))
	return store.InsertDrop(ctx, tx, id, tether.Lat, tether.Lon, held, s.now().Add(lifetime))
}

// decayTethers wears every Tether Node down, ends the ones that are spent, and clears old drops.
func (s *Service) decayTethers(ctx context.Context, settings game.Settings) {
	if err := store.DecayTethers(ctx, s.db, game.TetherDecayPerHour(settings)); err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 tether decay failed", "err", err)
		}
		return
	}
	spent, err := store.SpentTethers(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 tether decay failed", "err", err)
		}
		return
	}
	for _, id := range spent {
		if ctx.Err() != nil {
			return
		}
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			tether, err := store.TetherForUpdate(ctx, tx, id)
			if err != nil || tether.Status != "active" {
				return err
			}
			return s.endTether(ctx, tx, id, tether, "decayed", settings)
		})
		if err != nil {
			slog.Error("E1900 tether decay failed", "tether", id, "err", err)
		}
	}
	// What nobody picked up, the Undersignal takes back; each owner is told once per drop.
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		reclaimed, err := store.ReclaimExpiredDrops(ctx, tx)
		if err != nil {
			return err
		}
		for _, drop := range reclaimed {
			if err := notify(ctx, tx, store.NewNotice{
				PlayerID: drop.OwnerID, Kind: "drop_reclaimed", Body: game.NoticeDropReclaimed(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("E1900 drop expiry failed", "err", err)
	}
}
