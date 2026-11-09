package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

const noticeKeepDays = 30

type AttackTetherOutcome struct {
	Health       float64    `json:"health"`
	Destroyed    bool       `json:"destroyed"`
	JammedUntil  *time.Time `json:"jammedUntil,omitempty"`
	SpikeFiresAt *time.Time `json:"spikeFiresAt,omitempty"`
}

// lastFix reads the player's previous position for the travel-speed check.
func lastFix(ctx context.Context, q db.Querier, playerID int64) (*game.Fix, error) {
	fix, err := store.LastHarvestFix(ctx, q, playerID)
	if err != nil || fix == nil {
		return nil, err
	}
	return &game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: fix.At}, nil
}

// spendWeaponCost takes the weapon and its energy cells, or fails with nothing taken.
func spendWeaponCost(ctx context.Context, tx *sql.Tx, playerID int64, weapon game.Weapon) error {
	ok, err := store.SpendItem(ctx, tx, playerID, weapon.ID, 1)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ItemMissing.With("%s", weapon.ID)
	}
	return spendCells(ctx, tx, playerID, weapon.Cells)
}

func spendCells(ctx context.Context, tx *sql.Tx, playerID int64, cells int) error {
	if cells <= 0 {
		return nil
	}
	ok, err := store.SpendItem(ctx, tx, playerID, game.ItemCell, cells)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NoCells
	}
	return nil
}

// prepareAttack locks both players (lowest id first) and then the Tether Node, the same order as
// the tick and uploads, and applies the attack rules.
func (s *Service) prepareAttack(ctx context.Context, tx *sql.Tx, attackerID, tetherID int64,
	settings game.Settings) (int64, store.TetherState, bool, error) {
	ownerID, err := store.TetherOwner(ctx, tx, tetherID)
	if errors.Is(err, store.ErrNotFound) {
		return 0, store.TetherState{}, false, apperr.TetherGone
	}
	if err != nil {
		return 0, store.TetherState{}, false, err
	}
	first, second := min(attackerID, ownerID), max(attackerID, ownerID)
	firstPlayer, err := store.GetPlayerForUpdate(ctx, tx, first)
	if err != nil {
		return 0, store.TetherState{}, false, err
	}
	secondPlayer := firstPlayer
	if second != first {
		if secondPlayer, err = store.GetPlayerForUpdate(ctx, tx, second); err != nil {
			return 0, store.TetherState{}, false, err
		}
	}
	attacker, owner := firstPlayer, secondPlayer
	if attackerID != first {
		attacker, owner = secondPlayer, firstPlayer
	}

	tether, err := store.TetherForUpdate(ctx, tx, tetherID)
	if err != nil {
		return 0, tether, false, err
	}
	if tether.Status != "active" {
		return 0, tether, false, apperr.TetherGone
	}
	membership, err := store.MembershipOf(ctx, tx, attackerID)
	if err != nil {
		return 0, tether, false, err
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
		return 0, tether, false, ruleErr
	}
	return ownerID, tether, losesProtection, nil
}

// AttackTether uses one weapon on a Tether Node. Players are locked lowest id first, then the node,
// the same order as the tick and uploads.
func (s *Service) AttackTether(ctx context.Context, attackerID, tetherID int64, weaponID string, fix TetherFix) (AttackTetherOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return AttackTetherOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	weapon, known := game.WeaponByID(settings, weaponID)
	if !known {
		return AttackTetherOutcome{}, apperr.ItemMissing.With("unknown weapon")
	}
	var out AttackTetherOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ownerID, tether, losesProtection, err := s.prepareAttack(ctx, tx, attackerID, tetherID, settings)
		if err != nil {
			return err
		}

		last, err := lastFix(ctx, tx, attackerID)
		if err != nil {
			return err
		}
		now := s.now()
		distance, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now},
			last, tether.Lat, tether.Lon, settings.WeaponRangeM)
		if ruleErr != nil {
			return ruleErr
		}

		combat, err := store.TetherCombatState(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		if weapon.ID == game.WeaponJammer && combat.JammedUntil.Valid && combat.JammedUntil.Time.After(now) {
			return apperr.AlreadyJammed
		}
		if weapon.ID == game.WeaponSpike && combat.SpikeArmed {
			return apperr.SpikeArmed
		}
		if err := spendWeaponCost(ctx, tx, attackerID, weapon); err != nil {
			return err
		}

		place, err := store.TetherPlace(ctx, tx, tetherID)
		if err != nil {
			return err
		}
		switch weapon.ID {
		case game.WeaponEmPulse:
			health, err := store.DamageTether(ctx, tx, tetherID, weapon.Damage)
			if err != nil {
				return err
			}
			out.Health, out.Destroyed = health, health <= 0
			if err := store.MarkTetherAttacked(ctx, tx, tetherID); err != nil {
				return err
			}
			if out.Destroyed {
				if err := s.endTether(ctx, tx, tetherID, tether, "destroyed", settings); err != nil {
					return err
				}
				if err := notify(ctx, tx, ownerID, tetherID, "destroyed", capitalize(place)+" was destroyed."); err != nil {
					return err
				}
			} else if err := notify(ctx, tx, ownerID, tetherID, "attacked", capitalize(place)+" is under attack."); err != nil {
				return err
			}
		case game.WeaponJammer:
			until := now.Add(weapon.Jam)
			out.JammedUntil = &until
			if err := store.JamTether(ctx, tx, tetherID, until); err != nil {
				return err
			}
			if err := store.MarkTetherAttacked(ctx, tx, tetherID); err != nil {
				return err
			}
			if err := notify(ctx, tx, ownerID, tetherID, "jammed", capitalize(place)+" was jammed."); err != nil {
				return err
			}
		case game.WeaponSpike:
			firesAt := now.Add(weapon.Fuse)
			out.SpikeFiresAt = &firesAt
			if err := store.PlantSpike(ctx, tx, tetherID, attackerID, firesAt); err != nil {
				return err
			}
			if err := store.MarkTetherAttacked(ctx, tx, tetherID); err != nil {
				return err
			}
			body := fmt.Sprintf("A spike was planted on %s. %d min to defuse.", place, int(weapon.Fuse.Minutes()))
			if err := notify(ctx, tx, ownerID, tetherID, "spike", body); err != nil {
				return err
			}
		}

		if losesProtection {
			until := now.Add(time.Duration(settings.ProtectionLossDays * 24 * float64(time.Hour)))
			if err := store.LoseProtection(ctx, tx, attackerID, until); err != nil {
				return err
			}
		}
		return store.InsertTetherAttack(ctx, tx, store.TetherAttackRecord{
			TetherID: tetherID, AttackerID: attackerID, Weapon: weapon.ID,
			Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, DistanceM: distance,
		})
	})
	return out, err
}

// DefuseSpike lets the owner, standing at their Tether Node, disarm a spike before it fires.
func (s *Service) DefuseSpike(ctx context.Context, playerID, tetherID int64, fix TetherFix) error {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
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
		last, err := lastFix(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if _, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: s.now()},
			last, tether.Lat, tether.Lon, settings.WeaponRangeM); ruleErr != nil {
			return ruleErr
		}
		planter, err := store.DefuseSpike(ctx, tx, tetherID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NoSpike
		}
		if err != nil {
			return err
		}
		return notify(ctx, tx, planter, tetherID, "defused", "Your spike was defused.")
	})
}

// resolveCombat fires due spikes and ends finished contests. Each gets its own transaction.
func (s *Service) resolveCombat(ctx context.Context, settings game.Settings) {
	spikes, err := store.DueSpikes(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 spike tick failed", "err", err)
		}
		return
	}
	for _, spike := range spikes {
		if ctx.Err() != nil {
			return
		}
		if err := s.fireSpike(ctx, spike, settings); err != nil {
			slog.Error("E1900 spike failed", "spike", spike.ID, "err", err)
		}
	}

	s.landDrones(ctx, settings)

	if err := store.EndRunOutHolds(ctx, s.db); err != nil && ctx.Err() == nil {
		slog.Error("E1900 hold expiry failed", "err", err)
	}
	contests, err := store.DueContests(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 contest tick failed", "err", err)
		}
		return
	}
	for _, contest := range contests {
		if ctx.Err() != nil {
			return
		}
		if err := s.finishContest(ctx, contest.ID, contest.NodeID, settings); err != nil {
			slog.Error("E1900 contest failed", "contest", contest.ID, "err", err)
		}
	}
}

func (s *Service) fireSpike(ctx context.Context, spike store.DueSpike, settings game.Settings) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, spike.OwnerID); err != nil {
			return err
		}
		tether, err := store.TetherForUpdate(ctx, tx, spike.TetherID)
		if err != nil {
			return err
		}
		fired, err := store.FireSpike(ctx, tx, spike.ID)
		if err != nil || !fired || tether.Status != "active" {
			return err
		}
		place, err := store.TetherPlace(ctx, tx, spike.TetherID)
		if err != nil {
			return err
		}
		health, err := store.DamageTether(ctx, tx, spike.TetherID, settings.SpikeDamage)
		if err != nil {
			return err
		}
		if err := store.MarkTetherAttacked(ctx, tx, spike.TetherID); err != nil {
			return err
		}
		if health <= 0 {
			if err := s.endTether(ctx, tx, spike.TetherID, tether, "destroyed", settings); err != nil {
				return err
			}
			if err := notify(ctx, tx, spike.OwnerID, spike.TetherID, "destroyed", capitalize(place)+" was destroyed by a spike."); err != nil {
				return err
			}
			return notify(ctx, tx, spike.PlantedBy, spike.TetherID, "spike_result", "Your spike destroyed the Tether Node.")
		}
		if err := notify(ctx, tx, spike.OwnerID, spike.TetherID, "spike_fired", "A spike went off on "+place+"."); err != nil {
			return err
		}
		return notify(ctx, tx, spike.PlantedBy, spike.TetherID, "spike_result", "Your spike went off.")
	})
}

func capitalize(text string) string {
	if text == "" || text[0] < 'a' || text[0] > 'z' {
		return text
	}
	return string(text[0]-'a'+'A') + text[1:]
}
