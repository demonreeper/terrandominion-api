package game

import "github.com/demonreeper/terrandominion/api/internal/apperr"

// Ranks are numbers (1 member up to 100). Rank 100 belongs to the shared lore AI (internal name
// signal; players see six separate faction commanders): no player heads a whole faction.
// Players command a region (owner, 2026-11-10).
const (
	RankMember            = 1
	RankWarden            = 50
	RankRegionalCommander = 90
	RankHead              = 100
)

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

// RankTitle names a rank for players.
func RankTitle(rank int) string {
	switch {
	case rank >= RankHead:
		return "Commander"
	case rank >= RankRegionalCommander:
		return "Regional Commander"
	case rank >= RankWarden:
		return "Warden"
	default:
		return "Member"
	}
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
