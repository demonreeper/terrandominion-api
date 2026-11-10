package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type DroneOutcome struct {
	ArrivesAt time.Time `json:"arrivesAt"`
	DistanceM float64   `json:"distanceM"`
}

// CraftGear makes a piece of gear from the Craft tab; it goes straight into its empty slot.
func (s *Service) CraftGear(ctx context.Context, playerID int64, typeID string) (CraftOutcome, error) {
	settings := s.Settings(ctx)
	var out CraftOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		available, err := store.CraftableGearFor(ctx, tx, playerID, game.CraftableGearIDs)
		if err != nil {
			return err
		}
		var gear *store.CraftableGear
		for i := range available {
			if available[i].ID == typeID {
				gear = &available[i]
			}
		}
		if gear == nil {
			return apperr.CraftUnknown
		}
		if player.Level < gear.MinLevel {
			return game.LevelError(gear.MinLevel, player.Level)
		}
		if err := s.payCost(ctx, tx, playerID, gear.Cost); err != nil {
			return err
		}
		if err := store.GrantCraftedGear(ctx, tx, playerID, typeID); err != nil {
			return err
		}
		progress, levelUp, err := awardXP(ctx, tx, playerID, "craft", settings.XpCraft, 0, settings)
		if err != nil {
			return err
		}
		out = CraftOutcome{XP: settings.XpCraft, Progress: progress, LevelUp: levelUp}
		return nil
	})
	return out, err
}

// LaunchDrone sends a strike at a Tether Node in range of the player's drone bay.
func (s *Service) LaunchDrone(ctx context.Context, attackerID, tetherID int64, fix TetherFix) (DroneOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return DroneOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out DroneOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		target, err := s.prepareAttack(ctx, tx, attackerID, tetherID, settings)
		if err != nil {
			return err
		}
		tether := target.Tether
		bay, err := store.EquippedDroneBay(ctx, tx, attackerID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NoDroneBay
		}
		if err != nil {
			return err
		}
		flying, err := store.DronesInFlight(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		if flying >= bay.Drones {
			return apperr.DroneBusy
		}

		last, err := lastFix(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		now := s.now()
		distance, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now},
			last, tether.Lat, tether.Lon, bay.RangeM)
		if ruleErr != nil {
			return ruleErr
		}
		if err := spendCells(ctx, tx, attackerID, settings.CellsDrone); err != nil {
			return err
		}

		arrives := now.Add(game.DroneFlight(settings, distance))
		if err := store.LaunchDrone(ctx, tx, tetherID, attackerID, bay.Damage, arrives); err != nil {
			return err
		}
		if target.LosesProtection {
			until := now.Add(time.Duration(settings.ProtectionLossDays * 24 * float64(time.Hour)))
			if err := store.LoseProtection(ctx, tx, attackerID, until); err != nil {
				return err
			}
		}
		if err := store.InsertTetherAttack(ctx, tx, store.TetherAttackRecord{
			TetherID: tetherID, AttackerID: attackerID, Weapon: "drone",
			Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, DistanceM: distance,
		}); err != nil {
			return err
		}
		place, err := store.TetherPlace(ctx, tx, tetherID, settings.TnLinkRadiusM)
		if err != nil {
			return err
		}
		damage := game.Damage(bay.Damage)
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: target.OwnerID, TetherID: tetherID, ActorID: attackerID, Amount: amountOf(damage),
			Kind: "drone_inbound", Body: game.NoticeDroneInbound(target.AttackerName, place, arrives.Sub(now), damage),
		}); err != nil {
			return err
		}
		out = DroneOutcome{ArrivesAt: arrives, DistanceM: distance}
		return nil
	})
	return out, err
}

func (s *Service) landDrones(ctx context.Context, settings game.Settings) {
	drones, err := store.DueDrones(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 drone tick failed", "err", err)
		}
		return
	}
	for _, drone := range drones {
		if ctx.Err() != nil {
			return
		}
		if err := s.landDrone(ctx, drone, settings); err != nil {
			slog.Error("E1900 drone failed", "drone", drone.ID, "err", err)
		}
	}
}

func (s *Service) landDrone(ctx context.Context, drone store.DueDrone, settings game.Settings) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, drone.OwnerID); err != nil {
			return err
		}
		tether, err := store.TetherForUpdate(ctx, tx, drone.TetherID)
		if err != nil {
			return err
		}
		landed, err := store.LandDrone(ctx, tx, drone.ID)
		if err != nil || !landed || tether.Status != "active" {
			return err
		}
		place, err := store.TetherPlace(ctx, tx, drone.TetherID, settings.TnLinkRadiusM)
		if err != nil {
			return err
		}
		owner, launcher, err := playerNames(ctx, tx, drone.OwnerID, drone.LaunchedBy)
		if err != nil {
			return err
		}
		health, err := store.DamageTether(ctx, tx, drone.TetherID, drone.Damage)
		if err != nil {
			return err
		}
		if err := store.MarkTetherAttacked(ctx, tx, drone.TetherID); err != nil {
			return err
		}
		damage := game.Damage(drone.Damage)
		destroyed := health <= 0
		toOwner := store.NewNotice{PlayerID: drone.OwnerID, TetherID: drone.TetherID, ActorID: drone.LaunchedBy,
			Amount: amountOf(damage), Kind: "attacked", Body: game.NoticeHit(launcher, "drone", place, damage)}
		if destroyed {
			if err := s.endTether(ctx, tx, drone.TetherID, tether, "destroyed", settings); err != nil {
				return err
			}
			toOwner.Kind, toOwner.Body = "destroyed", game.NoticeDestroyed(launcher, "drone", place)
		}
		if err := notify(ctx, tx, toOwner); err != nil {
			return err
		}
		return notify(ctx, tx, store.NewNotice{
			PlayerID: drone.LaunchedBy, TetherID: drone.TetherID, ActorID: drone.OwnerID, Amount: amountOf(damage),
			Kind: "drone_result", Body: game.NoticeResult("drone", owner, place, damage, destroyed),
		})
	})
}
