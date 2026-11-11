package service

import (
	"context"
	"math"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction perks (docs/profile-plan.md, P1b) and the freelancer's daily favour
// (docs/freelancer-path-plan.md F4), applied here on the server. They are read at the moment of
// each action.

// factionOf is the id of the faction the player is in, or 0 for a freelancer.
func factionOf(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil || membership == nil {
		return 0, err
	}
	return membership.FactionID, nil
}

// perksOf is what works on the player right now, with its lines and tags: their faction's perks at
// full strength for a member; for a freelancer today's favour from each power whose mission they
// finished (freelancer-path-plan.md F4), until the mission day ends.
func (s *Service) perksOf(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (game.Perks, []game.PerkLine, error) {
	faction, err := factionOf(ctx, q, playerID)
	if err != nil {
		return game.NoPerks(), nil, err
	}
	if faction != 0 {
		return game.FactionPerks(settings, faction), game.FactionPerkLines(settings, faction), nil
	}
	day, _ := game.MissionDay(settings, s.now())
	powers, err := store.FinishedMissionPowers(ctx, q, playerID, day)
	if err != nil {
		return game.NoPerks(), nil, err
	}
	return game.FavourPerks(settings, powers), game.FavourPerkLines(settings, powers), nil
}

// equipped is the player's gear stats with their perks (or favour) on top: carry capacity and scan
// range. Every rule that reads either goes through here.
func (s *Service) equipped(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (store.EquippedStats, game.Perks, error) {
	stats, err := store.Equipped(ctx, q, playerID)
	if err != nil {
		return stats, game.NoPerks(), err
	}
	perks, _, err := s.perksOf(ctx, q, playerID, settings)
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
