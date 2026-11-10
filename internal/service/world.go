package service

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// NearbyNodes lists active nodes around the player, limited by their scanner's range: better
// scanners literally see further.
func (s *Service) NearbyNodes(ctx context.Context, playerID int64, lat, lon, radiusM float64) ([]store.Node, error) {
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
	// Cells this scan touches for the first time get their nodes now (drone strike plan, milestone 0).
	s.seedScan(ctx, settings, lat, lon, radiusM)
	nodes, err := store.NearbyNodes(ctx, s.db, playerID, lat, lon, radiusM, 100, settings.HarvestCooldownSec)
	if err != nil {
		return nil, err
	}
	return nodes, s.addContestInfo(ctx, playerID, nodes, settings)
}

type HarvestRequest struct {
	NodeID    int64
	Lat, Lon  float64
	AccuracyM float64
}

type HarvestOutcome struct {
	Resource string           `json:"resource"`
	Amount   int              `json:"amount"`
	Reserve  float64          `json:"nodeReserve"`
	Carried  map[string]int64 `json:"carried"`
	XP       int              `json:"xp"`
	Progress game.Progress    `json:"progress"`
	LevelUp  bool             `json:"levelUp"`
	// ReadyInSec is how long until this player may harvest the node again.
	ReadyInSec int `json:"readyInSec"`
}

// Harvest takes resources from a node. The node and the player are locked for the transaction,
// so two players draining one node, or one player double-tapping, both resolve correctly.
func (s *Service) Harvest(ctx context.Context, playerID int64, req HarvestRequest) (HarvestOutcome, error) {
	if !game.ValidCoordinate(req.Lat, req.Lon) {
		return HarvestOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out HarvestOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		node, err := store.NodeForUpdate(ctx, tx, req.NodeID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NodeInactive
		}
		if err != nil {
			return err
		}
		if node.Status != "active" {
			return apperr.NodeInactive
		}

		stats, err := store.Equipped(ctx, tx, playerID)
		if err != nil {
			return err
		}
		carried, err := store.Carried(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var carriedTotal int64
		for _, v := range carried {
			carriedTotal += v
		}
		lastAtNode, err := store.LastHarvestAtNode(ctx, tx, playerID, node.ID)
		if err != nil {
			return err
		}
		lastFix, err := store.LastHarvestFix(ctx, tx, playerID)
		if err != nil {
			return err
		}

		now := s.now()
		in := game.HarvestInput{
			Settings:           settings,
			Player:             game.Fix{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM, At: now},
			NodeLat:            node.Lat,
			NodeLon:            node.Lon,
			NodeYield:          node.Yield,
			NodeReserve:        node.Reserve,
			ExtractorYieldMult: stats.YieldMult,
			CarryCapacity:      stats.CarryCapacity,
			CarriedTotal:       carriedTotal,
			LastAtNode:         lastAtNode,
		}
		if lastFix != nil {
			in.LastAnyFix = &game.Fix{Lat: lastFix.Lat, Lon: lastFix.Lon, AccuracyM: lastFix.AccuracyM, At: lastFix.At}
		}

		result, ruleErr := game.EvaluateHarvest(in)
		if ruleErr != nil {
			return ruleErr
		}

		if err := checkNodeHold(ctx, tx, playerID, node.ID); err != nil {
			return err
		}

		reserve, err := store.DrainNode(ctx, tx, node.ID, result.ReserveCost)
		if err != nil {
			return err
		}
		harvestID, err := store.InsertHarvest(ctx, tx, playerID, node.ID, result.Amount,
			req.Lat, req.Lon, req.AccuracyM, result.DistanceM)
		if err != nil {
			return err
		}
		if err := store.AddCarried(ctx, tx, playerID, node.Resource, int64(result.Amount)); err != nil {
			return err
		}
		progress, levelUp, err := awardXP(ctx, tx, playerID, "harvest", settings.XpHarvest, harvestID, settings)
		if err != nil {
			return err
		}

		carried[node.Resource] += int64(result.Amount)
		out = HarvestOutcome{
			Resource: node.Resource, Amount: result.Amount, Reserve: reserve, Carried: carried,
			XP: settings.XpHarvest, Progress: progress, LevelUp: levelUp,
			ReadyInSec: game.HarvestReadyIn(settings, &now, now),
		}
		return nil
	})
	return out, err
}
