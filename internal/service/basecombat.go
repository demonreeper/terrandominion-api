package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction bases, B5 (base-building-plan.md 8): asynchronous, location-gated attacks on a base. EM Pulse, Overload Spike
// and drones pick a module or the core; the Shield Matrix cuts every hit; a module at 0 goes offline until repaired and
// the Energy Lattice powers the rest (knock it out and they go dark); the core at 0 drops a tier, or destroys a Spire.
// Hacks steal from the vault above its safe floor. New bases and bases that just dropped a tier are protected.

// BaseTarget is what an attack aims at: a module's slot, or the core.
type BaseTarget struct {
	Core  bool `json:"core"`
	Ring  int  `json:"ring"`
	Index int  `json:"index"`
}

// BaseAttackRequest is POST /v1/bases/{id}/attack, /drone: the fix, the weapon (attack only) and the target.
type BaseAttackRequest struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AccuracyM float64 `json:"accuracyM"`
	Weapon    string  `json:"weapon"`
	BaseTarget
}

// BaseHitResult is what an attack did.
type BaseHitResult struct {
	Damage     int        `json:"damage"`
	Health     int        `json:"health"`
	CoreHealth int        `json:"coreHealth"`
	TierLost   bool       `json:"tierLost"`
	Destroyed  bool       `json:"destroyed"`
	FiresAt    *time.Time `json:"firesAt,omitempty"`
	ArrivesAt  *time.Time `json:"arrivesAt,omitempty"`
}

// recomputePower writes which modules the Lattices power now.
func recomputePower(ctx context.Context, tx *sql.Tx, baseID int64, st game.Settings) error {
	slots, _, err := store.PowerSlots(ctx, tx, baseID)
	if err != nil {
		return err
	}
	return store.SetPowered(ctx, tx, baseID, game.PoweredSlots(slots, st))
}

// prepareBaseAttack checks an attacker may hit this base: not its own faction's, not protected, in range. It ends a
// walk link with one of its members, and a freelancer loses neutral standing.
func (s *Service) prepareBaseAttack(ctx context.Context, tx *sql.Tx, attackerID, baseID int64, req BaseAttackRequest,
	rangeM float64, st game.Settings, now time.Time) (store.BaseCombat, float64, error) {
	if _, err := store.GetPlayerForUpdate(ctx, tx, attackerID); err != nil {
		return store.BaseCombat{}, 0, err
	}
	b, err := store.BaseCombatForUpdate(ctx, tx, baseID)
	if errors.Is(err, store.ErrNotFound) {
		return b, 0, apperr.BaseGone
	}
	if err != nil {
		return b, 0, err
	}
	m, err := store.MembershipOf(ctx, tx, attackerID)
	if err != nil {
		return b, 0, err
	}
	if m != nil && m.FactionID == b.FactionID {
		return b, 0, apperr.BaseCombatRefused.Say("That is your own faction's base.")
	}
	if b.ProtectedUntil != nil && b.ProtectedUntil.After(now) {
		return b, 0, apperr.BaseProtected.WithData(map[string]any{"until": b.ProtectedUntil.UTC().Format(time.RFC3339)})
	}
	if !req.Core {
		if _, _, module, err := store.SlotHealth(ctx, tx, baseID, req.Ring, req.Index); err != nil || module == "" {
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return b, 0, err
			}
			return b, 0, apperr.BaseCombatRefused.Say("No module stands on that pad. Aim at a module or the core.")
		}
	}
	last, err := lastFix(ctx, tx, attackerID)
	if err != nil {
		return b, 0, err
	}
	distance, ruleErr := game.CheckInRange(st, game.Fix{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM, At: now},
		last, b.Lat, b.Lon, rangeM)
	if ruleErr != nil {
		return b, 0, ruleErr
	}
	// Attacking a base ends a walk-together link that has one of its members (section 8).
	if link, err := store.LiveWalkLink(ctx, tx, attackerID); err != nil {
		return b, 0, err
	} else if link != 0 {
		has, err := store.WalkLinkHasFaction(ctx, tx, link, b.FactionID)
		if err != nil {
			return b, 0, err
		}
		if has {
			if err := store.EndWalkLink(ctx, tx, link, game.WalkEndAttack, now); err != nil {
				return b, 0, err
			}
		}
	}
	// A freelancer who attacks a faction loses neutral standing.
	if m == nil {
		until := now.Add(time.Duration(st.ProtectionLossDays * 24 * float64(time.Hour)))
		if err := store.LoseProtection(ctx, tx, attackerID, until); err != nil {
			return b, 0, err
		}
	}
	return b, distance, nil
}

// applyBaseHit takes damage (after the shield and an intercept) off the target and follows through: module offline and
// power, core to 0 and a tier lost (or the Spire destroyed), the push, XP.
func (s *Service) applyBaseHit(ctx context.Context, tx *sql.Tx, b store.BaseCombat, target BaseTarget, damage float64,
	attackMk int, intercept float64, attackerID int64, st game.Settings, now time.Time) (BaseHitResult, error) {
	var out BaseHitResult
	shieldMk, err := store.BestShieldMk(ctx, tx, b.ID)
	if err != nil {
		return out, err
	}
	hit := game.HitDamage(damage, intercept, game.BaseShieldShare(shieldMk, attackMk, b.FactionID, st))
	out.Damage = int(game.Damage(hit))
	if target.Core {
		health, err := store.DamageCore(ctx, tx, b.ID, hit)
		if err != nil {
			return out, err
		}
		out.Health, out.CoreHealth = health, health
		if health <= 0 {
			if b.Tier <= 1 {
				out.Destroyed = true
				if err := s.destroyBase(ctx, tx, b, st, now); err != nil {
					return out, err
				}
			} else {
				out.TierLost = true
				if err := s.dropTier(ctx, tx, b, true, st, now); err != nil {
					return out, err
				}
			}
		}
	} else {
		health, module, err := store.DamageSlot(ctx, tx, b.ID, target.Ring, target.Index, hit)
		if err != nil {
			return out, err
		}
		out.Health, out.CoreHealth = health, b.CoreHealth
		if module != "" && health <= 0 {
			if err := recomputePower(ctx, tx, b.ID, st); err != nil {
				return out, err
			}
		}
	}
	if hit > 0 {
		if _, _, err := awardXP(ctx, tx, attackerID, "base_attack", st.XpAttack, b.ID, st); err != nil {
			return out, err
		}
	}
	if out.Destroyed || out.TierLost {
		if _, _, err := awardXP(ctx, tx, attackerID, "base_break", st.XpDestroy, b.ID, st); err != nil {
			return out, err
		}
	}
	return out, s.pushBaseAttacked(ctx, tx, b, st, now)
}

// pushBaseAttacked tells the faction's members in the region, once per base per BaseAttackPushMin.
func (s *Service) pushBaseAttacked(ctx context.Context, tx *sql.Tx, b store.BaseCombat, st game.Settings, now time.Time) error {
	if b.AttackPushedAt != nil && now.Sub(*b.AttackPushedAt) < time.Duration(st.BaseAttackPushMin*float64(time.Minute)) {
		return nil
	}
	members, err := store.FactionMembersInRegion(ctx, tx, b.FactionID, b.Region)
	if err != nil {
		return err
	}
	for _, id := range members {
		if err := notify(ctx, tx, store.NewNotice{PlayerID: id, Kind: "base_attacked", Body: game.NoticeBaseAttacked(b.Name)}); err != nil {
			return err
		}
	}
	return store.SetAttackPushed(ctx, tx, b.ID, now)
}

// dropTier takes a base down a tier: the zone shrinks, core health is the lower tier's full, the outer slots go and
// their modules are packed away (part of their cost to the vault). After a fight it is protected a while.
func (s *Service) dropTier(ctx context.Context, tx *sql.Tx, b store.BaseCombat, fight bool, st game.Settings, now time.Time) error {
	to := b.Tier - 1
	slots, mks, err := store.PowerSlots(ctx, tx, b.ID)
	if err != nil {
		return err
	}
	keep := map[[2]int]bool{}
	for _, sl := range game.BaseSlotLayout(to, st) {
		keep[[2]int{sl.Ring, sl.Index}] = true
	}
	var refund [4]int64
	for _, sl := range slots {
		if sl.Module == "" || keep[[2]int{sl.Ring, sl.Index}] {
			continue
		}
		r := game.PackedRefund(sl.Module, mks[[2]int{sl.Ring, sl.Index}], st)
		for i := range refund {
			refund[i] += r[i]
		}
	}
	if err := store.SetBaseTier(ctx, tx, b.ID, to, game.BaseCoreHealth(to, st)); err != nil {
		return err
	}
	if err := store.SetBaseZone(ctx, tx, b.ID, game.BaseZoneM(to, st)); err != nil {
		return err
	}
	if _, err := store.SetBaseSlots(ctx, tx, b.ID, game.BaseSlotLayout(to, st)); err != nil {
		return err
	}
	if err := store.AddVault(ctx, tx, b.ID, 0, 0, "packed", refund, now); err != nil {
		return err
	}
	if err := recomputePower(ctx, tx, b.ID, st); err != nil {
		return err
	}
	if fight {
		if err := store.SetBaseProtected(ctx, tx, b.ID, now.Add(time.Duration(st.BaseProtectDropHours*float64(time.Hour)))); err != nil {
			return err
		}
	}
	members, err := store.FactionMembersInRegion(ctx, tx, b.FactionID, b.Region)
	if err != nil {
		return err
	}
	for _, id := range members {
		if err := notify(ctx, tx, store.NewNotice{PlayerID: id, Kind: "base_tier_lost",
			Body: game.NoticeBaseTierLost(b.Name, game.BaseTierName(to))}); err != nil {
			return err
		}
	}
	return nil
}

// destroyBase ends a Spire: half its vault drops on the map (the usual drop lifetime, then the Undersignal reclaims
// it), the rest is lost.
func (s *Service) destroyBase(ctx context.Context, tx *sql.Tx, b store.BaseCombat, st game.Settings, now time.Time) error {
	vault, err := store.VaultForUpdate(ctx, tx, b.ID)
	if err != nil {
		return err
	}
	drop := game.DestroyDrop(vault, st)
	contents := map[string]int64{}
	for i, r := range resourceOrder {
		if drop[i] > 0 {
			contents[r] = drop[i]
		}
	}
	if len(contents) > 0 {
		expires := now.Add(time.Duration(st.DropLifetimeHours * float64(time.Hour)))
		if err := store.InsertBaseDrop(ctx, tx, b.ID, b.Lat, b.Lon, contents, expires); err != nil {
			return err
		}
	}
	if err := store.DestroyBase(ctx, tx, b.ID, now); err != nil {
		return err
	}
	members, err := store.FactionMembersInRegion(ctx, tx, b.FactionID, b.Region)
	if err != nil {
		return err
	}
	for _, id := range members {
		if err := notify(ctx, tx, store.NewNotice{PlayerID: id, Kind: "base_destroyed", Body: game.NoticeBaseDestroyed(b.Name)}); err != nil {
			return err
		}
	}
	slog.Info("base destroyed", "base", b.ID, "name", b.Name)
	return nil
}

// AttackBase: an EM Pulse hits at once; an Overload Spike is armed and fires after its fuse (members on site can defuse
// it). From within the zone plus weapon range.
func (s *Service) AttackBase(ctx context.Context, attackerID, baseID int64, req BaseAttackRequest) (BaseHitResult, error) {
	var out BaseHitResult
	st := s.Settings(ctx)
	weapon, known := game.WeaponByID(st, req.Weapon)
	if !known || (weapon.ID != game.WeaponEmPulse && weapon.ID != game.WeaponSpike) {
		return out, apperr.BadRequest.With("weapon: em_pulse or overload_spike")
	}
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		bref, err := store.BaseRefForUpdate(ctx, tx, baseID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.BaseGone
		}
		if err != nil {
			return err
		}
		b, _, err := s.prepareBaseAttack(ctx, tx, attackerID, baseID, req, game.BaseAttackRange(bref.ZoneM, st), st, now)
		if err != nil {
			return err
		}
		if err := spendWeaponCost(ctx, tx, attackerID, weapon); err != nil {
			return err
		}
		if weapon.ID == game.WeaponSpike {
			fires := now.Add(weapon.Fuse)
			x := store.BaseStrike{BaseID: baseID, Kind: "spike", LaunchedBy: attackerID, Damage: weapon.Damage,
				Mk: game.WeaponMk, DueAt: fires}
			if !req.Core {
				x.Ring, x.Index = &req.Ring, &req.Index
			}
			if _, err := store.InsertStrike(ctx, tx, x); err != nil {
				return err
			}
			out.FiresAt = &fires
			if err := s.pushBaseAttacked(ctx, tx, b, st, now); err != nil {
				return err
			}
		} else if out, err = s.applyBaseHit(ctx, tx, b, req.BaseTarget, weapon.Damage, game.WeaponMk, 0, attackerID, st, now); err != nil {
			return err
		}
		return advanceMissions(ctx, tx, attackerID, game.GoalAttack, weapon.ID, 1, st, now)
	})
	return out, err
}

// LaunchBaseDrone sends a drone from the attacker's Drone Bay at a module or the core; it lands on the tick.
func (s *Service) LaunchBaseDrone(ctx context.Context, attackerID, baseID int64, req BaseAttackRequest) (BaseHitResult, error) {
	var out BaseHitResult
	st := s.Settings(ctx)
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		bay, err := store.EquippedDroneBay(ctx, tx, attackerID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NoDroneBay
		}
		if err != nil {
			return err
		}
		b, distance, err := s.prepareBaseAttack(ctx, tx, attackerID, baseID, req, bay.RangeM, st, now)
		if err != nil {
			return err
		}
		flyingTN, err := store.DronesInFlight(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		flyingBase, err := store.BaseDronesInFlight(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		if flyingTN+flyingBase >= bay.Drones {
			return apperr.DroneBusy
		}
		if err := spendCells(ctx, tx, attackerID, bay.Cells); err != nil {
			return err
		}
		if err := store.WearGear(ctx, tx, bay.GearID, st.DroneLaunchWear); err != nil {
			return err
		}
		perks, _, err := s.perksOf(ctx, tx, attackerID, st)
		if err != nil {
			return err
		}
		arrives := now.Add(game.DroneFlight(game.SettingsWithPerks(st, perks), distance))
		x := store.BaseStrike{BaseID: baseID, Kind: "drone", LaunchedBy: attackerID, Damage: bay.Damage, Mk: bay.Mk,
			FromLat: &req.Lat, FromLon: &req.Lon, DueAt: arrives}
		if !req.Core {
			x.Ring, x.Index = &req.Ring, &req.Index
		}
		if _, err := store.InsertStrike(ctx, tx, x); err != nil {
			return err
		}
		out.ArrivesAt = &arrives
		if err := s.pushBaseAttacked(ctx, tx, b, st, now); err != nil {
			return err
		}
		return advanceMissions(ctx, tx, attackerID, game.GoalDrone, "", 1, st, now)
	})
	return out, err
}

// strikeTick lands drones and fires spikes that are due.
func (s *Service) strikeTick(ctx context.Context) {
	st := s.Settings(ctx)
	now := s.now()
	ids, err := store.DueStrikeIDs(ctx, s.db, now, 100)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 strike tick failed", "err", err)
		}
		return
	}
	for _, id := range ids {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			x, err := store.StrikeForUpdate(ctx, tx, id)
			if err != nil || x.Status != "pending" {
				return err
			}
			b, err := store.BaseCombatForUpdate(ctx, tx, x.BaseID)
			if errors.Is(err, store.ErrNotFound) {
				return store.SetStrikeStatus(ctx, tx, id, "void")
			}
			if err != nil {
				return err
			}
			target := BaseTarget{Core: x.Ring == nil}
			if x.Ring != nil && x.Index != nil {
				target.Ring, target.Index = *x.Ring, *x.Index
				if _, _, module, err := store.SlotHealth(ctx, tx, b.ID, target.Ring, target.Index); err != nil || module == "" {
					// The module is gone (a tier drop packed it away): the hit goes to the core.
					target = BaseTarget{Core: true}
				}
			}
			if _, err := s.applyBaseHit(ctx, tx, b, target, x.Damage, x.Mk, x.Intercept, x.LaunchedBy, st, now); err != nil {
				return err
			}
			return store.SetStrikeStatus(ctx, tx, id, "hit")
		})
		if err != nil {
			slog.Error("E1900 base strike failed", "strike", id, "err", err)
		}
	}
}

// DefendRequest is a member's fix at their base (intercept, defuse, repair).
type DefendRequest struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AccuracyM float64 `json:"accuracyM"`
	BaseTarget
}

// InterceptBaseDrone: a member on site spends energy cells to stop part of the soonest drone flying at the base.
func (s *Service) InterceptBaseDrone(ctx context.Context, playerID, baseID int64, req DefendRequest) (InterceptOutcome, error) {
	var out InterceptOutcome
	st := s.Settings(ctx)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		if _, _, err := s.onSiteMember(ctx, tx, playerID, baseID, AtBaseRequest{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM}, st); err != nil {
			return err
		}
		pending, err := store.PendingStrikes(ctx, tx, baseID)
		if err != nil {
			return err
		}
		var drone *store.BaseStrike
		for i := range pending {
			if pending[i].Kind == "drone" && pending[i].Intercept == 0 {
				drone = &pending[i]
				break
			}
		}
		if drone == nil {
			return apperr.NoDroneInbound
		}
		if err := spendCells(ctx, tx, playerID, st.CellsIntercept); err != nil {
			return err
		}
		perks, _, err := s.perksOf(ctx, tx, playerID, st)
		if err != nil {
			return err
		}
		share := game.PerkInterceptShare(game.InterceptShare(st, drone.Mk), perks.InterceptBonus)
		if err := store.SetStrikeIntercept(ctx, tx, drone.ID, share); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalIntercept, "", 1, st, s.now()); err != nil {
			return err
		}
		out = InterceptOutcome{DroneID: drone.ID, Share: share, ShotDown: share >= 1}
		return nil
	})
	return out, err
}

// DefuseBaseSpike: a member on site disarms the soonest Overload Spike armed on the base.
func (s *Service) DefuseBaseSpike(ctx context.Context, playerID, baseID int64, req DefendRequest) error {
	st := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, _, err := s.onSiteMember(ctx, tx, playerID, baseID, AtBaseRequest{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM}, st); err != nil {
			return err
		}
		pending, err := store.PendingStrikes(ctx, tx, baseID)
		if err != nil {
			return err
		}
		for _, x := range pending {
			if x.Kind == "spike" {
				return store.SetStrikeStatus(ctx, tx, x.ID, "defused")
			}
		}
		return apperr.NoSpike
	})
}

// RepairResult is what a repair mended and cost.
type RepairResult struct {
	Points int   `json:"points"`
	Alloy  int64 `json:"alloy"`
	Health int   `json:"health"`
}

// RepairBase: a member on site (Member and up) mends a module or the core with alloy from storage, per point (the TN
// repair rule); as much as the alloy pays for. A mended module comes back online (power permitting).
func (s *Service) RepairBase(ctx context.Context, playerID, baseID int64, req DefendRequest) (RepairResult, error) {
	var out RepairResult
	st := s.Settings(ctx)
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, b, err := s.onSiteMember(ctx, tx, playerID, baseID, AtBaseRequest{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM}, st)
		if err != nil {
			return err
		}
		if !game.Can(m.Rank, m.Nomad, game.PowerUpgrade) {
			return apperr.ProjectRefused.Say("Prospects can't repair yet.")
		}
		var health, full int
		if req.Core {
			c, err := store.BaseCombatForUpdate(ctx, tx, b.ID)
			if err != nil {
				return err
			}
			health, full = c.CoreHealth, game.BaseCoreHealth(c.Tier, st)
		} else {
			var module string
			health, full, module, err = store.SlotHealth(ctx, tx, b.ID, req.Ring, req.Index)
			if errors.Is(err, store.ErrNotFound) || module == "" {
				return apperr.ProjectRefused.Say("No module on that pad.")
			}
			if err != nil {
				return err
			}
		}
		missing := full - health
		if missing <= 0 {
			return apperr.ProjectRefused.Say("Nothing to repair there.")
		}
		stored, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		points := missing
		if per := st.RepairAlloyPerPoint; per > 0 {
			points = min(missing, int(float64(stored["alloy"])/per))
		}
		if points <= 0 {
			return apperr.NothingToGive.Say("You need alloy in storage to repair.")
		}
		cost := game.BaseRepairCost(points, st)
		if err := store.AddUploaded(ctx, tx, playerID, "alloy", -cost); err != nil {
			return err
		}
		out = RepairResult{Points: points, Alloy: cost, Health: health + points}
		if req.Core {
			if err := store.SetCoreHealth(ctx, tx, b.ID, out.Health); err != nil {
				return err
			}
		} else {
			if err := store.SetSlotHealth(ctx, tx, b.ID, req.Ring, req.Index, out.Health); err != nil {
				return err
			}
			if health <= 0 {
				if err := recomputePower(ctx, tx, b.ID, st); err != nil {
					return err
				}
			}
			if err := advanceMissions(ctx, tx, playerID, game.GoalBaseRepair, "", 1, st, now); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// HackResult is what a hack took.
type HackResult struct {
	Stolen map[string]int64 `json:"stolen"`
	Pct    float64          `json:"pct"`
}

// HackBase: within BaseHackRangeM, for BaseHackData, once per BaseHackCooldownMin per player per base: steals
// BaseHackMinPct to BaseHackMaxPct of the vault above its safe floor (larger for Bastion Order) into the hacker's
// storage. The hex-path puzzle is played on the phone.
func (s *Service) HackBase(ctx context.Context, playerID, baseID int64, req BaseAttackRequest) (HackResult, error) {
	var out HackResult
	st := s.Settings(ctx)
	now := s.now()
	req.Core = true
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		b, _, err := s.prepareBaseAttack(ctx, tx, playerID, baseID, req, st.BaseHackRangeM, st, now)
		if err != nil {
			return err
		}
		last, err := store.LastHack(ctx, tx, playerID, baseID)
		if err != nil {
			return err
		}
		if !game.HackCooldownOver(last, now, st) {
			return apperr.BaseCombatRefused.Say("You hacked this base a moment ago. Try again later.")
		}
		if st.BaseHackData > 0 {
			stored, err := store.Uploaded(ctx, tx, playerID)
			if err != nil {
				return err
			}
			if stored["data"] < int64(st.BaseHackData) {
				return apperr.NothingToGive.Say("A hack needs data in your storage.")
			}
			if err := store.AddUploaded(ctx, tx, playerID, "data", -int64(st.BaseHackData)); err != nil {
				return err
			}
		}
		vault, err := store.VaultForUpdate(ctx, tx, baseID)
		if err != nil {
			return err
		}
		out.Pct = game.HackPct(rand.Float64(), st)
		stolen := game.HackSteal(vault, out.Pct, game.SafeFloorPct(b.FactionID, st))
		out.Stolen = map[string]int64{}
		for i, r := range resourceOrder {
			if stolen[i] > 0 {
				out.Stolen[r] = stolen[i]
				if err := store.AddUploaded(ctx, tx, playerID, r, stolen[i]); err != nil {
					return err
				}
			}
		}
		if err := store.AddVault(ctx, tx, baseID, playerID, 0, "hack",
			[4]int64{-stolen[0], -stolen[1], -stolen[2], -stolen[3]}, now); err != nil {
			return err
		}
		if err := store.InsertHack(ctx, tx, baseID, playerID, out.Stolen, now); err != nil {
			return err
		}
		return s.pushBaseAttacked(ctx, tx, b, st, now)
	})
	return out, err
}

// BaseThreats is what flies at or is armed on a base (the card shows its own members the threats).
func (s *Service) baseThreats(ctx context.Context, q db.Querier, baseID int64) ([]store.BaseStrike, error) {
	return store.PendingStrikes(ctx, q, baseID)
}
