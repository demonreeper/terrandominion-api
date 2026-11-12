package game

import "github.com/demonreeper/terrandominion/api/internal/apperr"

// Ranks are data: game/ranks.go (faction-ranks-plan.md). Rank 100 belongs to the shared lore AI (internal name
// signal; players see six separate faction commanders): no player heads a whole faction yet.

// LaunchRegion is the one region at launch. A region is a country (owner, 2026-11-10), so a
// faction has at most one Regional Commander per country; MV is the Maldives.
const LaunchRegion = "MV"

// RegionName names a region for players.
func RegionName(code string) string {
	if code == "MV" {
		return "Maldives"
	}
	return code
}

// BranchState is a faction's branch in one region: whether anyone commands it yet.
type BranchState struct {
	HasCommander bool
}

type PlayerFactionState struct {
	Level     int
	InFaction bool
}

// CanClaimRegion: a player at the lead level who is in no faction may take regional command of a
// faction in their region when nobody holds it. The first to claim wins; the database enforces
// that with a conditional update.
func CanClaimRegion(p PlayerFactionState, b BranchState, s Settings) *apperr.Error {
	switch {
	case p.InFaction:
		return apperr.FactionMemberAlready
	case p.Level < s.LeadFactionLevel:
		return LevelError(s.LeadFactionLevel, p.Level)
	case b.HasCommander:
		return apperr.FactionClaimed
	}
	return nil
}

// CanJoin: a player at the join level who is in no faction may join a faction's branch in their
// region, once it has a Regional Commander.
func CanJoin(p PlayerFactionState, b BranchState, s Settings) *apperr.Error {
	switch {
	case p.InFaction:
		return apperr.FactionMemberAlready
	case p.Level < s.JoinFactionLevel:
		return LevelError(s.JoinFactionLevel, p.Level)
	case !b.HasCommander:
		return apperr.FactionLocked
	}
	return nil
}
