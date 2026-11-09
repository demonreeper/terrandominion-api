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
	stats, err := store.Equipped(ctx, s.db, playerID)
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
	Taken   map[string]int64 `json:"taken"`
	Left    map[string]int64 `json:"left"`
	Carried map[string]int64 `json:"carried"`
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

		stats, err := store.Equipped(ctx, tx, playerID)
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
		for resource, amount := range taken {
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
		out = PickDropOutcome{Taken: taken, Left: left, Carried: carried}
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
	if _, err := store.DeleteExpiredDrops(ctx, s.db); err != nil && ctx.Err() == nil {
		slog.Error("E1900 drop expiry failed", "err", err)
	}
}

type AttackTetherOutcome struct {
	Health    float64 `json:"health"`
	Destroyed bool    `json:"destroyed"`
}

// AttackTether applies damage under the attack rules. No endpoint calls it yet: how much a weapon
// does, and what it needs in range and cost, waits on the weapons design.
func (s *Service) AttackTether(ctx context.Context, attackerID, tetherID int64, damage float64) (AttackTetherOutcome, error) {
	if damage <= 0 {
		return AttackTetherOutcome{}, apperr.BadRequest.With("damage")
	}
	settings := s.Settings(ctx)
	var out AttackTetherOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ownerID, err := store.TetherOwner(ctx, tx, tetherID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.TetherGone
		}
		if err != nil {
			return err
		}
		first, second := min(attackerID, ownerID), max(attackerID, ownerID)
		firstPlayer, err := store.GetPlayerForUpdate(ctx, tx, first)
		if err != nil {
			return err
		}
		secondPlayer := firstPlayer
		if second != first {
			if secondPlayer, err = store.GetPlayerForUpdate(ctx, tx, second); err != nil {
				return err
			}
		}
		attacker, owner := firstPlayer, secondPlayer
		if attackerID != first {
			attacker, owner = secondPlayer, firstPlayer
		}

		tether, err := store.TetherForUpdate(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		if tether.Status != "active" {
			return apperr.TetherGone
		}
		membership, err := store.MembershipOf(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		var attackerFaction *int
		if membership != nil {
			attackerFaction = &membership.FactionID
		}
		losesProtection, ruleErr := game.CanAttackTether(settings, game.TetherAttack{
			AttackerID: attackerID, AttackerFaction: attackerFaction, AttackerLevel: attacker.Level,
			OwnerID: ownerID, OwnerLevel: owner.Level, TetherFaction: tether.FactionID,
		})
		if ruleErr != nil {
			return ruleErr
		}

		health, err := store.DamageTether(ctx, tx, tetherID, damage)
		if err != nil {
			return err
		}
		if losesProtection {
			until := s.now().Add(time.Duration(settings.ProtectionLossDays * 24 * float64(time.Hour)))
			if err := store.LoseProtection(ctx, tx, attackerID, until); err != nil {
				return err
			}
		}
		out = AttackTetherOutcome{Health: health, Destroyed: health <= 0}
		if out.Destroyed {
			return s.endTether(ctx, tx, tetherID, tether, "destroyed", settings)
		}
		return nil
	})
	return out, err
}
