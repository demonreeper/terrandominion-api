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

// DroneOutcome is a launch: where it flies from and to, and when it lands.
type DroneOutcome struct {
	ID         int64       `json:"id"`
	TetherID   int64       `json:"tnId"`
	Mk         int         `json:"mk"`
	LaunchedAt time.Time   `json:"launchedAt"`
	ArrivesAt  time.Time   `json:"arrivesAt"`
	DistanceM  float64     `json:"distanceM"`
	From       store.Point `json:"from"`
	To         store.Point `json:"to"`
	// Damage is what it will do if not intercepted, after the target's shield.
	Damage int64 `json:"damage"`
}

// healthLeft is health for a notice: rounded up, so a node still standing never reads 0.
func healthLeft(h float64) int64 { return int64(math.Ceil(math.Max(0, h))) }

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
		if err := advanceMissions(ctx, tx, playerID, game.GoalCraft, typeID, 1, settings, s.now()); err != nil {
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
		if err := spendCells(ctx, tx, attackerID, bay.Cells); err != nil {
			return err
		}
		if err := store.WearGear(ctx, tx, bay.GearID, settings.DroneLaunchWear); err != nil {
			return err
		}

		arrives := now.Add(game.DroneFlight(settings, distance))
		id, launched, err := store.LaunchDrone(ctx, tx, tetherID, attackerID, bay.Damage, bay.Mk, fix.Lat, fix.Lon, arrives)
		if err != nil {
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
		// What lands if nobody intercepts it: the shield is known now and lasts as long as the node.
		damage := game.Damage(game.HitDamage(bay.Damage, 0, game.ShieldShare(settings, tether.ShieldTier, bay.Mk)))
		flight := arrives.Sub(now)
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: target.OwnerID, TetherID: tetherID, ActorID: attackerID, Amount: amountOf(damage),
			Kind: "drone_inbound", Body: game.NoticeDroneInbound(target.AttackerName, place, flight, damage),
		}); err != nil {
			return err
		}
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: attackerID, TetherID: tetherID, ActorID: target.OwnerID, Amount: amountOf(damage),
			Kind: "drone_launched", Body: game.NoticeDroneLaunched(target.OwnerName, place, flight, damage),
		}); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, attackerID, game.GoalDrone, "", 1, settings, s.now()); err != nil {
			return err
		}
		out = DroneOutcome{
			ID: id, TetherID: tetherID, Mk: bay.Mk, LaunchedAt: launched, ArrivesAt: arrives, DistanceM: distance,
			From: store.Point{Lat: fix.Lat, Lon: fix.Lon}, To: store.Point{Lat: tether.Lat, Lon: tether.Lon}, Damage: damage,
		}
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
		// Both players: the launcher is paid XP when it lands.
		if err := lockPlayerPair(ctx, tx, drone.OwnerID, drone.LaunchedBy); err != nil {
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
		// Intercept and shield stack; a shield also stops one hit from destroying the node.
		hit := game.HitDamage(drone.Damage, drone.InterceptShare, game.ShieldShare(settings, tether.ShieldTier, drone.Mk))
		hit = game.CapShieldedHit(tether.Health, hit, tether.ShieldTier > 0)
		health, err := store.DamageTether(ctx, tx, drone.TetherID, hit)
		if err != nil {
			return err
		}
		if err := store.SetDroneLandedDamage(ctx, tx, drone.ID, hit); err != nil {
			return err
		}
		if err := store.MarkTetherAttacked(ctx, tx, drone.TetherID); err != nil {
			return err
		}
		damage := game.Damage(hit)
		destroyed := health <= 0
		toOwner := store.NewNotice{PlayerID: drone.OwnerID, TetherID: drone.TetherID, ActorID: drone.LaunchedBy,
			Amount: amountOf(damage), Kind: "attacked", Body: game.NoticeHit(launcher, "drone", place, damage, healthLeft(health))}
		if destroyed {
			if err := s.endTether(ctx, tx, drone.TetherID, tether, "destroyed", settings); err != nil {
				return err
			}
			if err := store.SetTetherDestroyer(ctx, tx, drone.TetherID, drone.LaunchedBy); err != nil {
				return err
			}
			toOwner.Kind, toOwner.Body = "destroyed", game.NoticeDestroyed(launcher, "drone", place)
		}
		if err := notify(ctx, tx, toOwner); err != nil {
			return err
		}
		// A drone shot down (nothing landed) pays nothing.
		if _, err := payAttackXP(ctx, tx, drone.LaunchedBy, drone.OwnerID, drone.TetherID, hit > 0, destroyed, settings, s.now()); err != nil {
			return err
		}
		return notify(ctx, tx, store.NewNotice{
			PlayerID: drone.LaunchedBy, TetherID: drone.TetherID, ActorID: drone.OwnerID, Amount: amountOf(damage),
			Kind: "drone_result", Body: game.NoticeResult("drone", owner, place, damage, healthLeft(health), destroyed),
		})
	})
}

// InterceptOutcome says how much of the drone's damage was stopped.
type InterceptOutcome struct {
	DroneID int64   `json:"droneId"`
	Share   float64 `json:"share"`
	// ShotDown is true when nothing of it will land.
	ShotDown bool `json:"shotDown"`
	CombatXP
}

// InterceptDrone lets the owner, at their Tether Node, spend energy cells to stop part of the
// soonest inbound drone. Locks: owner, node, then the strike row (as landing does), so a drone
// intercepted in the tick it lands is resolved once.
func (s *Service) InterceptDrone(ctx context.Context, playerID, tetherID int64, fix TetherFix) (InterceptOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return InterceptOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out InterceptOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		owner, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if _, err := s.ownTetherInRange(ctx, tx, playerID, tetherID, fix, settings); err != nil {
			return err
		}
		drone, ok, anyFlying, err := store.InboundDroneForUpdate(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		if !ok {
			if anyFlying {
				return apperr.AlreadyIntercepted
			}
			return apperr.NoDroneInbound
		}
		if err := spendCells(ctx, tx, playerID, settings.CellsIntercept); err != nil {
			return err
		}
		share := game.InterceptShare(settings, drone.Mk)
		if err := store.InterceptDrone(ctx, tx, drone.ID, share); err != nil {
			return err
		}
		place, err := store.TetherPlace(ctx, tx, tetherID, settings.TnLinkRadiusM)
		if err != nil {
			return err
		}
		attacker, err := store.GetPlayer(ctx, tx, drone.LaunchedBy)
		if err != nil {
			return err
		}
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: playerID, TetherID: tetherID, ActorID: drone.LaunchedBy,
			Kind: "drone_intercepted", Body: game.NoticeInterceptedDefender(attacker.Name, place, share),
		}); err != nil {
			return err
		}
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: drone.LaunchedBy, TetherID: tetherID, ActorID: playerID,
			Kind: "drone_intercepted", Body: game.NoticeInterceptedAttacker(owner.Name, place, share),
		}); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalIntercept, "", 1, settings, s.now()); err != nil {
			return err
		}
		out = InterceptOutcome{DroneID: drone.ID, Share: share, ShotDown: share >= 1}
		out.CombatXP, err = payInterceptXP(ctx, tx, playerID, drone.LaunchedBy, drone.ID, settings, s.now())
		return err
	})
	return out, err
}

// ownTetherInRange locks the player's own active Tether Node and checks they stand within weapon
// range of it (GPS quality and travel speed included). The player must already be locked.
func (s *Service) ownTetherInRange(ctx context.Context, tx *sql.Tx, playerID, tetherID int64, fix TetherFix,
	settings game.Settings) (store.TetherState, error) {
	tether, err := store.TetherForUpdate(ctx, tx, tetherID)
	if errors.Is(err, store.ErrNotFound) {
		return tether, apperr.TetherGone
	}
	if err != nil {
		return tether, err
	}
	if tether.Status != "active" {
		return tether, apperr.TetherGone
	}
	if tether.OwnerID != playerID {
		return tether, apperr.TetherNotOwner
	}
	last, err := lastFix(ctx, tx, playerID)
	if err != nil {
		return tether, err
	}
	if _, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: s.now()},
		last, tether.Lat, tether.Lon, settings.WeaponRangeM); ruleErr != nil {
		return tether, ruleErr
	}
	return tether, nil
}

// MyDrones lists the player's strikes in flight and those ended in the last DroneResultsMinutes.
func (s *Service) MyDrones(ctx context.Context, playerID int64) ([]store.PlayerDrone, error) {
	settings := s.Settings(ctx)
	since := s.now().Add(-time.Duration(settings.DroneResultsMinutes * float64(time.Minute)))
	return store.PlayerDrones(ctx, s.db, playerID, since)
}

// ShieldOutcome is the shield now on the Tether Node.
type ShieldOutcome struct {
	ShieldTier int `json:"shieldTier"`
}

// FitShield puts a crafted TN Shield on the player's own Tether Node, standing at it. A higher Mk
// replaces a lower one; the item is used up.
func (s *Service) FitShield(ctx context.Context, playerID, tetherID int64, item string, fix TetherFix) (ShieldOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return ShieldOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	mk, ok := game.ShieldMkOf(item)
	if !ok {
		return ShieldOutcome{}, apperr.ItemMissing.With("not a shield")
	}
	settings := s.Settings(ctx)
	var out ShieldOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		tether, err := s.ownTetherInRange(ctx, tx, playerID, tetherID, fix, settings)
		if err != nil {
			return err
		}
		if mk <= tether.ShieldTier {
			return apperr.ShieldNotHigher.WithData(map[string]any{"shieldTier": tether.ShieldTier})
		}
		spent, err := store.SpendItem(ctx, tx, playerID, item, 1)
		if err != nil {
			return err
		}
		if !spent {
			return game.ItemError(item, "TN Shield Mk "+game.RomanMk(mk))
		}
		if err := store.SetShieldTier(ctx, tx, tetherID, mk); err != nil {
			return err
		}
		place, err := store.TetherPlace(ctx, tx, tetherID, settings.TnLinkRadiusM)
		if err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalShield, "", 1, settings, s.now()); err != nil {
			return err
		}
		out.ShieldTier = mk
		return notify(ctx, tx, store.NewNotice{
			PlayerID: playerID, TetherID: tetherID, Kind: "shield_fitted", Body: game.NoticeShieldFitted(mk, place),
		})
	})
	return out, err
}
