package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// CraftOutcome is the result of crafting, upgrading or repairing. The client reloads /me for the
// new balances, so this only reports what the action earned.
type CraftOutcome struct {
	XP       int           `json:"xp"`
	Progress game.Progress `json:"progress"`
	LevelUp  bool          `json:"levelUp"`
}

// Craft makes one consumable, paid from carried then uploaded.
func (s *Service) Craft(ctx context.Context, playerID int64, recipeID string) (CraftOutcome, error) {
	settings := s.Settings(ctx)
	recipe, known := game.RecipeByID(settings, recipeID)
	if !known {
		return CraftOutcome{}, apperr.CraftUnknown
	}

	var out CraftOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if player.Level < recipe.MinLevel {
			return game.LevelError(recipe.MinLevel, player.Level)
		}
		if err := s.payCost(ctx, tx, playerID, recipe.Cost); err != nil {
			return err
		}
		if err := store.AddItem(ctx, tx, playerID, recipe.ID, 1); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalCraft, recipe.ID, 1, settings, s.now()); err != nil {
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

// UpgradeGear turns a piece of gear into the next tier, at full durability, in the same slot.
func (s *Service) UpgradeGear(ctx context.Context, playerID, gearID int64) (CraftOutcome, error) {
	settings := s.Settings(ctx)
	var out CraftOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		gear, err := ownedGear(ctx, tx, playerID, gearID)
		if err != nil {
			return err
		}
		if gear.UpgradesTo == "" {
			return apperr.GearNoUpgrade
		}
		next, err := store.GearTypeByID(ctx, tx, gear.UpgradesTo)
		if err != nil {
			return err
		}
		if player.Level < next.MinLevel {
			return game.LevelError(next.MinLevel, player.Level)
		}
		if err := s.payCost(ctx, tx, playerID, next.Cost); err != nil {
			return err
		}
		if err := store.SetGearType(ctx, tx, gearID, next.ID, next.MaxDurability); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalUpgradeGear, "", 1, settings, s.now()); err != nil {
			return err
		}
		progress, levelUp, err := awardXP(ctx, tx, playerID, "gear_upgrade", settings.XpUpgrade, gearID, settings)
		if err != nil {
			return err
		}
		out = CraftOutcome{XP: settings.XpUpgrade, Progress: progress, LevelUp: levelUp}
		return nil
	})
	return out, err
}

// RepairGear brings a piece of gear back to full durability for alloy. It earns no XP, so repairing
// a scratch over and over gains nothing.
func (s *Service) RepairGear(ctx context.Context, playerID, gearID int64) error {
	settings := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		gear, err := ownedGear(ctx, tx, playerID, gearID)
		if err != nil {
			return err
		}
		if gear.Durability >= gear.MaxDurability {
			return apperr.GearNotWorn
		}
		if err := s.payCost(ctx, tx, playerID, game.RepairCost(settings, gear.Durability, gear.MaxDurability)); err != nil {
			return err
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalRepairGear, "", 1, settings, s.now()); err != nil {
			return err
		}
		return store.SetGearDurability(ctx, tx, gearID, gear.MaxDurability)
	})
}

func ownedGear(ctx context.Context, tx *sql.Tx, playerID, gearID int64) (store.OwnedGear, error) {
	gear, err := store.OwnedGearForUpdate(ctx, tx, playerID, gearID)
	if errors.Is(err, store.ErrNotFound) {
		return gear, apperr.GearMissing
	}
	return gear, err
}

// payCost takes a cost from carried then uploaded, or fails with nothing taken.
func (s *Service) payCost(ctx context.Context, tx *sql.Tx, playerID int64, cost map[string]int64) error {
	carried, err := store.Carried(ctx, tx, playerID)
	if err != nil {
		return err
	}
	uploaded, err := store.Uploaded(ctx, tx, playerID)
	if err != nil {
		return err
	}
	payment, ruleErr := game.PayCost(cost, carried, uploaded)
	if ruleErr != nil {
		return ruleErr
	}
	for resource, amount := range payment.FromCarried {
		if err := store.AddCarried(ctx, tx, playerID, resource, -amount); err != nil {
			return err
		}
	}
	for resource, amount := range payment.FromUploaded {
		if err := store.AddUploaded(ctx, tx, playerID, resource, -amount); err != nil {
			return err
		}
	}
	return nil
}
