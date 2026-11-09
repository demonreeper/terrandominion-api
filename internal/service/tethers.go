package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"math/rand/v2"
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

		tooClose, err := store.TetherNearby(ctx, tx, req.Lat, req.Lon, settings.TnMinSpacingM)
		if err != nil {
			return err
		}
		if tooClose {
			return apperr.TetherTooClose
		}

		nodes, err := s.pickLinks(ctx, tx, req.Lat, req.Lon, nil, settings.TnLinksTier1, settings)
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
	Held     map[string]int64 `json:"held"`
	Uploaded map[string]int64 `json:"uploaded"`
}

// DismantleTether ends the owner's Tether Node. Part of its cost and everything it held go to
// their uploaded inventory, which has no carry limit, so a full player loses none of it.
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
		held, err := store.TakeTetherStock(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		for resource, amount := range held {
			if err := store.AddUploaded(ctx, tx, playerID, resource, amount); err != nil {
				return err
			}
		}
		uploaded, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out = DismantleTetherOutcome{Refund: refund, Held: held, Uploaded: uploaded}
		return nil
	})
	return out, err
}

// TetherFix is the position a player reports when acting at a Tether Node.
type TetherFix struct {
	Lat, Lon  float64
	AccuracyM float64
}

// checkAtTether confirms the player is standing at the Tether Node.
func (s *Service) checkAtTether(ctx context.Context, q db.Querier, playerID int64, tether store.TetherState,
	fix TetherFix, settings game.Settings) (float64, error) {
	lastFix, err := store.LastHarvestFix(ctx, q, playerID)
	if err != nil {
		return 0, err
	}
	var last *game.Fix
	if lastFix != nil {
		last = &game.Fix{Lat: lastFix.Lat, Lon: lastFix.Lon, AccuracyM: lastFix.AccuracyM, At: lastFix.At}
	}
	player := game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: s.now()}
	distance, ruleErr := game.CheckAtTether(settings, player, last, tether.Lat, tether.Lon)
	if ruleErr != nil {
		return 0, ruleErr
	}
	return distance, nil
}

type CollectTetherOutcome struct {
	Moved    map[string]int64 `json:"moved"`
	Uploaded map[string]int64 `json:"uploaded"`
}

// CollectTether moves the held half into the owner's uploaded inventory.
func (s *Service) CollectTether(ctx context.Context, playerID, tetherID int64, fix TetherFix) (CollectTetherOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return CollectTetherOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out CollectTetherOutcome

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
		distance, err := s.checkAtTether(ctx, tx, playerID, tether, fix, settings)
		if err != nil {
			return err
		}

		moved, err := store.TakeTetherStock(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		if len(moved) == 0 {
			return apperr.TetherNothing
		}
		for resource, amount := range moved {
			if err := store.AddUploaded(ctx, tx, playerID, resource, amount); err != nil {
				return err
			}
		}
		if err := store.InsertTetherTransfer(ctx, tx, store.TetherTransfer{
			TetherID: tetherID, PlayerID: playerID, Kind: "collect", Amounts: moved, Fees: map[string]int64{},
			Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, DistanceM: distance,
		}); err != nil {
			return err
		}
		uploaded, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out = CollectTetherOutcome{Moved: moved, Uploaded: uploaded}
		return nil
	})
	return out, err
}

type UploadTetherOutcome struct {
	Stored   map[string]int64 `json:"stored"`
	Fee      map[string]int64 `json:"fee"`
	Carried  map[string]int64 `json:"carried"`
	Uploaded map[string]int64 `json:"uploaded"`
}

// UploadTether moves everything the player carries into their uploaded inventory, less the
// owner's charge. Both players are locked lowest id first, and before the Tether Node, the same
// order the tick uses, so no two of these can wait on each other.
func (s *Service) UploadTether(ctx context.Context, playerID, tetherID int64, fix TetherFix) (UploadTetherOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return UploadTetherOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out UploadTetherOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ownerID, err := store.TetherOwner(ctx, tx, tetherID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.TetherGone
		}
		if err != nil {
			return err
		}
		first, second := min(playerID, ownerID), max(playerID, ownerID)
		if _, err := store.GetPlayerForUpdate(ctx, tx, first); err != nil {
			return err
		}
		if second != first {
			if _, err := store.GetPlayerForUpdate(ctx, tx, second); err != nil {
				return err
			}
		}
		tether, err := store.TetherForUpdate(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		if tether.Status != "active" {
			return apperr.TetherGone
		}

		membership, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var playerFaction *int
		if membership != nil {
			playerFaction = &membership.FactionID
		}
		if !game.CanUseTether(ownerID, tether.FactionID, playerID, playerFaction) {
			return apperr.TetherNoAccess
		}
		distance, err := s.checkAtTether(ctx, tx, playerID, tether, fix, settings)
		if err != nil {
			return err
		}

		carried, err := store.Carried(ctx, tx, playerID)
		if err != nil {
			return err
		}
		stored, fees := game.SplitUpload(carried, settings.TnUploadFee, playerID == ownerID)
		if len(stored) == 0 && len(fees) == 0 {
			return apperr.TetherNothing
		}
		for resource, amount := range stored {
			if err := store.AddCarried(ctx, tx, playerID, resource, -amount); err != nil {
				return err
			}
			if err := store.AddUploaded(ctx, tx, playerID, resource, amount); err != nil {
				return err
			}
			carried[resource] -= amount
		}
		for resource, amount := range fees {
			if err := store.AddCarried(ctx, tx, playerID, resource, -amount); err != nil {
				return err
			}
			if err := store.AddUploaded(ctx, tx, ownerID, resource, amount); err != nil {
				return err
			}
			carried[resource] -= amount
		}
		if err := store.InsertTetherTransfer(ctx, tx, store.TetherTransfer{
			TetherID: tetherID, PlayerID: playerID, Kind: "upload", Amounts: stored, Fees: fees,
			Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, DistanceM: distance,
		}); err != nil {
			return err
		}
		uploaded, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out = UploadTetherOutcome{Stored: stored, Fee: fees, Carried: carried, Uploaded: uploaded}
		return nil
	})
	return out, err
}

const (
	tetherTickInterval = time.Minute
	// Spike fuses and contests are shown to the second, so they resolve much sooner than the tick.
	combatTickInterval = 5 * time.Second
)

// RunTetherLoop harvests ready links once a minute until ctx ends. A tick that overruns simply
// delays the next one, because this is a single goroutine and the ticker drops what it misses.
func (s *Service) RunTetherLoop(ctx context.Context) {
	ticker := time.NewTicker(tetherTickInterval)
	defer ticker.Stop()
	combat := time.NewTicker(combatTickInterval)
	defer combat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tetherTick(ctx)
		case <-combat.C:
			s.resolveCombat(ctx, s.Settings(ctx))
		}
	}
}

// respawnDepletedNodes replaces every depleted node, each in its own transaction. Links still on
// a retired node move away on their next harvest.
func (s *Service) respawnDepletedNodes(ctx context.Context, settings game.Settings) {
	depleted, err := store.DepletedNodes(ctx, s.db, settings.NodeDepletedReserve)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 node respawn failed", "err", err)
		}
		return
	}
	for _, id := range depleted {
		if ctx.Err() != nil {
			return
		}
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			node, err := store.NodeForUpdate(ctx, tx, id)
			if err != nil {
				return err
			}
			// A player's harvest or the regrow job may have got there first.
			if node.Status != "active" || node.Reserve >= settings.NodeDepletedReserve {
				return nil
			}
			lat, lon := game.RespawnPoint(settings, node.Lat, node.Lon, rand.Float64(), rand.Float64())
			newID, err := store.ReplaceNode(ctx, tx, id, lat, lon)
			if err == nil {
				slog.Info("node respawned", "old", id, "new", newID, "resource", node.Resource)
			}
			return err
		})
		if err != nil {
			slog.Error("E1900 node respawn failed", "node", id, "err", err)
		}
	}
}

// tetherTick does the world's once-a-minute work: depleted nodes first, so links can move onto
// the new ones. Each Tether Node gets its own transaction, so one failure costs only that node.
func (s *Service) tetherTick(ctx context.Context) {
	s.respawnDepletedNodes(ctx, s.Settings(ctx))
	s.decayTethers(ctx, s.Settings(ctx))
	if err := store.DeleteOldNotices(ctx, s.db, s.now().AddDate(0, 0, -noticeKeepDays)); err != nil && ctx.Err() == nil {
		slog.Error("E1900 notice cleanup failed", "err", err)
	}

	due, err := store.DueTethers(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 tether tick failed", "err", err)
		}
		return
	}
	settings := s.Settings(ctx)
	for _, tether := range due {
		if ctx.Err() != nil {
			return
		}
		if err := s.harvestTether(ctx, tether, settings); err != nil {
			slog.Error("E1900 tether harvest failed", "tether", tether.ID, "err", err)
		}
	}
	if len(due) > 0 {
		slog.Debug("tether tick", "tethers", len(due))
	}
}

// harvestTether collects every ready link of one Tether Node. Locks are taken owner, node, links,
// resource nodes: the same order as the player actions.
func (s *Service) harvestTether(ctx context.Context, due store.DueTether, settings game.Settings) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, due.OwnerID); err != nil {
			return err
		}
		tether, err := store.TetherForUpdate(ctx, tx, due.ID)
		if err != nil {
			return err
		}
		if tether.Status != "active" {
			return nil
		}
		links, err := store.DueLinksForUpdate(ctx, tx, due.ID)
		if err != nil {
			return err
		}
		for _, link := range links {
			var gathered int64
			node, err := store.NodeForUpdate(ctx, tx, link.NodeID)
			if err != nil {
				return err
			}
			harvest, ok := game.TetherHarvest{}, false
			if node.Status == "active" {
				harvest, ok = game.EvaluateTetherHarvest(settings, node.Yield, node.Reserve)
			}
			if ok {
				if _, err := store.DrainNode(ctx, tx, node.ID, harvest.ReserveCost); err != nil {
					return err
				}
				if err := store.AddUploaded(ctx, tx, tether.OwnerID, node.Resource, harvest.ToOwner); err != nil {
					return err
				}
				if err := store.AddTetherStock(ctx, tx, due.ID, node.Resource, harvest.ToStock); err != nil {
					return err
				}
				gathered = harvest.Amount
			} else {
				// Depleted or gone: move to another free node, or stay idle if there is none.
				others, err := store.TetherLinkResources(ctx, tx, due.ID, link.Slot)
				if err != nil {
					return err
				}
				free, err := s.pickLinks(ctx, tx, tether.Lat, tether.Lon, others, 1, settings)
				if err != nil {
					return err
				}
				if len(free) > 0 {
					if err := store.MoveTetherLink(ctx, tx, due.ID, link.Slot, free[0]); err != nil {
						return err
					}
				}
			}
			if err := store.RecordLinkHarvest(ctx, tx, due.ID, link.Slot, gathered, settings.HarvestCooldownSec); err != nil {
				return err
			}
		}
		return s.fillTetherLinks(ctx, tx, due.ID, tether, settings)
	})
}

// fillTetherLinks links free nodes to any empty slots, so an upgrade made where few nodes were
// free picks up the rest as they become available.
func (s *Service) fillTetherLinks(ctx context.Context, tx *sql.Tx, id int64, tether store.TetherState, settings game.Settings) error {
	used, err := store.TetherLinkSlots(ctx, tx, id)
	if err != nil {
		return err
	}
	slots := game.FreeSlots(used, game.TetherLinks(settings, tether.Tier))
	if len(slots) == 0 {
		return nil
	}
	linked, err := store.TetherLinkResources(ctx, tx, id, -1)
	if err != nil {
		return err
	}
	nodes, err := s.pickLinks(ctx, tx, tether.Lat, tether.Lon, linked, len(slots), settings)
	if err != nil {
		return err
	}
	for i, nodeID := range nodes {
		if err := store.InsertTetherLink(ctx, tx, id, slots[i], nodeID); err != nil {
			return err
		}
	}
	return nil
}

// pickLinks chooses free nodes around a point, one of each resource first.
func (s *Service) pickLinks(ctx context.Context, tx *sql.Tx, lat, lon float64, linked []string, count int,
	settings game.Settings) ([]int64, error) {
	candidates, err := store.FreeNodes(ctx, tx, lat, lon, settings.TnLinkRadiusM, settings.NodeDepletedReserve)
	if err != nil {
		return nil, err
	}
	options := make([]game.LinkCandidate, len(candidates))
	for i, c := range candidates {
		options[i] = game.LinkCandidate{NodeID: c.NodeID, Resource: c.Resource}
	}
	return game.PickLinks(options, linked, count), nil
}

type UpgradeTetherOutcome struct {
	Tether   store.Tether     `json:"tether"`
	Carried  map[string]int64 `json:"carried"`
	Uploaded map[string]int64 `json:"uploaded"`
	XP       int              `json:"xp"`
	Progress game.Progress    `json:"progress"`
	LevelUp  bool             `json:"levelUp"`
}

// UpgradeTether raises the owner's Tether Node to tier 2 and links more nodes. It is refused
// when no extra node is free, so nobody pays for links that do not exist.
func (s *Service) UpgradeTether(ctx context.Context, playerID, tetherID int64, fix TetherFix) (UpgradeTetherOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return UpgradeTetherOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out UpgradeTetherOutcome

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
		if tether.Tier >= 2 {
			return apperr.TetherMaxTier
		}
		if _, err := s.checkAtTether(ctx, tx, playerID, tether, fix, settings); err != nil {
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
		cost := game.TetherUpgradeCost(settings)
		payment, ruleErr := game.PayCost(cost, carried, uploaded)
		if ruleErr != nil {
			return ruleErr
		}

		used, err := store.TetherLinkSlots(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		slots := game.FreeSlots(used, game.TetherLinks(settings, 2))
		linked, err := store.TetherLinkResources(ctx, tx, tetherID, -1)
		if err != nil {
			return err
		}
		nodes, err := s.pickLinks(ctx, tx, tether.Lat, tether.Lon, linked, len(slots), settings)
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
		if err := store.UpgradeTether(ctx, tx, tetherID, 2, game.AddCosts(tether.Spent, cost)); err != nil {
			return err
		}
		for i, nodeID := range nodes {
			if err := store.InsertTetherLink(ctx, tx, tetherID, slots[i], nodeID); err != nil {
				return err
			}
		}

		progress, levelUp, err := awardXP(ctx, tx, playerID, "tether_upgrade", settings.XpUpgrade, tetherID, settings)
		if err != nil {
			return err
		}
		view, err := store.TetherByID(ctx, tx, playerID, tetherID)
		if err != nil {
			return err
		}
		out = UpgradeTetherOutcome{
			Tether: view, Carried: carried, Uploaded: uploaded,
			XP: settings.XpUpgrade, Progress: progress, LevelUp: levelUp,
		}
		return nil
	})
	return out, err
}

// TetherDetail is one Tether Node. Per-link names and totals are only given to a player standing
// close to it.
type TetherDetail struct {
	Tether  store.Tether           `json:"tether"`
	InRange bool                   `json:"inRange"`
	Links   []store.TetherLinkInfo `json:"links"`
}

// GetTether reads one Tether Node for a player at the given position.
func (s *Service) GetTether(ctx context.Context, playerID, tetherID int64, fix TetherFix) (TetherDetail, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return TetherDetail{}, apperr.BadRequest.With("bad coordinate")
	}
	tether, err := store.TetherByID(ctx, s.db, playerID, tetherID)
	if errors.Is(err, store.ErrNotFound) {
		return TetherDetail{}, apperr.TetherGone
	}
	if err != nil {
		return TetherDetail{}, err
	}
	out := TetherDetail{Tether: tether, Links: []store.TetherLinkInfo{}}
	out.InRange = game.WithinTetherInfo(s.Settings(ctx), game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM},
		tether.Lat, tether.Lon)
	if out.InRange {
		if out.Links, err = store.TetherLinkInfos(ctx, s.db, tetherID); err != nil {
			return TetherDetail{}, err
		}
	}
	return out, nil
}
