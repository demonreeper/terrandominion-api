package game

import "github.com/demonreeper/terrandominion/api/internal/apperr"

const (
	RankMember = 1
	RankHead   = 100
)

type FactionState struct {
	Locked    bool
	HasLeader bool
}

type PlayerFactionState struct {
	Level     int
	InFaction bool
}

// CanLead: a player at the lead level who is not yet in a faction may claim a locked faction
// with no leader. The first to claim wins; the database enforces that with a conditional update.
func CanLead(p PlayerFactionState, f FactionState, s Settings) *apperr.Error {
	switch {
	case p.InFaction:
		return apperr.FactionMemberAlready
	case p.Level < s.LeadFactionLevel:
		return apperr.FactionLevelLow
	case f.HasLeader:
		return apperr.FactionClaimed
	case !f.Locked:
		return apperr.FactionNotLocked
	}
	return nil
}

// CanJoin: a player at the join level who is not in a faction may join an active one. Locked
// factions cannot be joined until a leader wakes them.
func CanJoin(p PlayerFactionState, f FactionState, s Settings) *apperr.Error {
	switch {
	case p.InFaction:
		return apperr.FactionMemberAlready
	case p.Level < s.JoinFactionLevel:
		return apperr.FactionLevelLow
	case f.Locked:
		return apperr.FactionLocked
	}
	return nil
}
