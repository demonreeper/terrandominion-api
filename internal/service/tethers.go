package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// NearbyTethers lists active Tether Nodes around the player, within their scanner's range.
func (s *Service) NearbyTethers(ctx context.Context, playerID int64, lat, lon, radiusM float64) ([]store.Tether, error) {
	if !game.ValidCoordinate(lat, lon) {
		return nil, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	stats, err := store.Equipped(ctx, s.db, playerID)
	if err != nil {
		return nil, err
	}
	limit := settings.NearbyMaxRadiusM
	if stats.ScanRangeM > 0 {
		limit = math.Min(limit, stats.ScanRangeM)
	}
	if radiusM <= 0 || radiusM > limit {
		radiusM = limit
	}
	return store.NearbyTethers(ctx, s.db, playerID, lat, lon, radiusM, 100)
}

type BuildTetherRequest struct {
	Lat, Lon  float64
	AccuracyM float64
}

type BuildTetherOutcome struct {
	Tether   store.Tether     `json:"tether"`
	Carried  map[string]int64 `json:"carried"`
	Uploaded map[string]int64 `json:"uploaded"`
	XP       int              `json:"xp"`
	Progress game.Progress    `json:"progress"`
	LevelUp  bool             `json:"levelUp"`
}

// BuildTether builds a tier 1 Tether Node at the player's position. Locking the player row makes
// the per-player cap and the payment safe against a double tap.
func (s *Service) BuildTether(ctx context.Context, playerID int64, req BuildTetherRequest) (BuildTetherOutcome, error) {
	if !game.ValidCoordinate(req.Lat, req.Lon) {
		return BuildTetherOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out BuildTetherOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		owned, err := store.CountActiveTethers(ctx, tx, playerID)
		if err != nil {
			return err
		}
		carried, err := store.Carried(ctx, tx, playerID)
		if err != nil {
			return err
		}
		uploaded, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		lastFix, err := store.LastHarvestFix(ctx, tx, playerID)
		if err != nil {
			return err
		}

		now := s.now()
		in := game.TetherBuildInput{
			Settings:    settings,
			Player:      game.Fix{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM, At: now},
			ActiveOwned: owned,
			Carried:     carried,
			Uploaded:    uploaded,
		}
		if lastFix != nil {
			in.LastAnyFix = &game.Fix{Lat: lastFix.Lat, Lon: lastFix.Lon, AccuracyM: lastFix.AccuracyM, At: lastFix.At}
		}
		payment, ruleErr := game.EvaluateTetherBuild(in)
		if ruleErr != nil {
			return ruleErr
		}

		nodes, err := store.PickFreeNodes(ctx, tx, req.Lat, req.Lon, settings.TnLinkRadiusM,
			settings.NodeDepletedReserve, settings.TnLinksTier1)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			return apperr.TetherNoNode
		}

		for resource, amount := range payment.FromCarried {
			if err := store.AddCarried(ctx, tx, playerID, resource, -amount); err != nil {
				return err
			}
			carried[resource] -= amount
		}
		for resource, amount := range payment.FromUploaded {
			if err := store.AddUploaded(ctx, tx, playerID, resource, -amount); err != nil {
				return err
			}
			uploaded[resource] -= amount
		}

		membership, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var factionID *int
		if membership != nil {
			factionID = &membership.FactionID
		}
		lifetime := time.Duration(settings.TnLifetimeHours * float64(time.Hour))
		id, err := store.InsertTether(ctx, tx, store.NewTether{
			OwnerID: playerID, FactionID: factionID, Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM,
			Health: settings.TnMaxHealth, Spent: game.TetherBuildCost(settings), ExpiresAt: now.Add(lifetime),
		})
		if err != nil {
			return err
		}
		for slot, nodeID := range nodes {
			if err := store.InsertTetherLink(ctx, tx, id, slot, nodeID); err != nil {
				return err
			}
		}

		progress, levelUp, err := awardXP(ctx, tx, playerID, "tether_build", settings.XpTnBuild, id, settings)
		if err != nil {
			return err
		}
		tether, err := store.TetherByID(ctx, tx, playerID, id)
		if err != nil {
			return err
		}
		out = BuildTetherOutcome{
			Tether: tether, Carried: carried, Uploaded: uploaded,
			XP: settings.XpTnBuild, Progress: progress, LevelUp: levelUp,
		}
		return nil
	})
	return out, err
}

type DismantleTetherOutcome struct {
	Refund   map[string]int64 `json:"refund"`
	Uploaded map[string]int64 `json:"uploaded"`
}

// DismantleTether ends the owner's Tether Node and refunds part of its cost to their uploaded
// inventory, which has no carry limit, so a full player still gets the whole refund.
func (s *Service) DismantleTether(ctx context.Context, playerID, tetherID int64) (DismantleTetherOutcome, error) {
	settings := s.Settings(ctx)
	var out DismantleTetherOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		tether, err := store.TetherForUpdate(ctx, tx, tetherID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.TetherGone
		}
		if err != nil {
			return err
		}
		if tether.Status != "active" {
			return apperr.TetherGone
		}
		if tether.OwnerID != playerID {
			return apperr.TetherNotOwner
		}

		if err := store.EndTether(ctx, tx, tetherID, "dismantled"); err != nil {
			return err
		}
		refund := game.TetherRefund(tether.Spent, settings.TnDismantleRefund)
		for resource, amount := range refund {
			if err := store.AddUploaded(ctx, tx, playerID, resource, amount); err != nil {
				return err
			}
		}
		uploaded, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out = DismantleTetherOutcome{Refund: refund, Uploaded: uploaded}
		return nil
	})
	return out, err
}
