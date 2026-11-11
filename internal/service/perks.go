package service

import (
	"context"
	"math"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction perks (docs/profile-plan.md, P1b): applied here, on the server, to members only. A
// player's perks are read from their membership at the moment of the action, so they stop the
// moment the player leaves.

// factionOf is the id of the faction the player is in, or 0 for a freelancer.
func factionOf(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil || membership == nil {
		return 0, err
	}
	return membership.FactionID, nil
}

// perksOf is what the player's faction gives them right now (nothing for a freelancer).
func perksOf(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (game.Perks, int, error) {
	faction, err := factionOf(ctx, q, playerID)
	if err != nil {
		return game.NoPerks(), 0, err
	}
	return game.FactionPerks(settings, faction), faction, nil
}

// equipped is the player's gear stats with their faction's perks on top: carry capacity and scan
// range. Every rule that reads either goes through here.
func equipped(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (store.EquippedStats, game.Perks, error) {
	stats, err := store.Equipped(ctx, q, playerID)
	if err != nil {
		return stats, game.NoPerks(), err
	}
	perks, _, err := perksOf(ctx, q, playerID, settings)
	if err != nil {
		return stats, perks, err
	}
	return applyPerks(stats, perks), perks, nil
}

// applyPerks folds a player's perks into their gear stats.
func applyPerks(stats store.EquippedStats, perks game.Perks) store.EquippedStats {
	stats.CarryCapacity = int64(math.Round(float64(stats.CarryCapacity) * perks.CarryMult))
	stats.ScanRangeM *= perks.ScanRangeMult
	return stats
}

// perkLines is a faction's perks in the shape the store's views carry.
func perkLines(settings game.Settings, factionID int) []store.PerkLine {
	lines := game.FactionPerkLines(settings, factionID)
	out := make([]store.PerkLine, 0, len(lines))
	for _, line := range lines {
		out = append(out, store.PerkLine{Key: line.Key, Line: line.Line, Tag: line.Tag})
	}
	return out
}
