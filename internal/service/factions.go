package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Factions lists the six factions, marking the one that invited this player at the faction level.
// A player already in a faction is not shown an invitation.
func (s *Service) Factions(ctx context.Context, playerID int64) ([]store.Faction, error) {
	factions, err := store.ListFactions(ctx, s.db, game.LaunchRegion)
	if err != nil {
		return nil, err
	}
	settings := s.Settings(ctx)
	membership, err := store.MembershipOf(ctx, s.db, playerID)
	for i := range factions {
		factions[i].Perks = perkLines(settings, factions[i].ID)
		factions[i].Mine = membership != nil && membership.FactionID == factions[i].ID
	}
	if err != nil || membership != nil {
		return factions, err
	}
	bound, err := store.BoundFaction(ctx, s.db, playerID)
	if err != nil {
		return nil, err
	}
	for i := range factions {
		factions[i].Bound = bound != "" && factions[i].Code == bound
	}
	invited, err := store.InvitedFaction(ctx, s.db, playerID)
	if err != nil {
		return nil, err
	}
	for i := range factions {
		factions[i].Invited = invited != 0 && factions[i].ID == invited
	}
	return factions, nil
}

// LeadFaction claims a dormant faction. Both the faction row and the player row are locked, and
// the claim itself is a conditional update, so of two players racing for the same faction exactly
// one wins and the other gets E1303.
func (s *Service) LeadFaction(ctx context.Context, playerID int64, code string) (*store.Membership, error) {
	return s.enterFaction(ctx, playerID, code, true)
}

// JoinFaction joins an active faction as a member.
func (s *Service) JoinFaction(ctx context.Context, playerID int64, code string) (*store.Membership, error) {
	return s.enterFaction(ctx, playerID, code, false)
}

func (s *Service) enterFaction(ctx context.Context, playerID int64, code string, lead bool) (*store.Membership, error) {
	// The primary admin plays as a freelancer: never a commander, never a member.
	if !game.MayEnterFaction(playerID) {
		return nil, apperr.PrimaryAdminFreelancer
	}
	settings := s.Settings(ctx)
	var membership *store.Membership

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		faction, err := store.FactionByCodeForUpdate(ctx, tx, code)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("faction %q", code)
		}
		if err != nil {
			return err
		}
		current, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		// A pledge made on this phone or sign-in before stands: only that faction is open.
		if current == nil {
			bound, err := store.BoundFaction(ctx, tx, playerID)
			if err != nil {
				return err
			}
			if !game.MayPledgeTo(bound, faction.Code) {
				appeal, err := s.boundAppeal(ctx, tx, playerID, bound)
				if err != nil {
					return err
				}
				return apperr.PledgeStands.WithData(map[string]any{
					"faction": bound, "appealCode": appeal, "appealUrl": game.AppealMailto(appeal),
				})
			}
		}

		// The player's region is their country; the Maldives is the only one at launch.
		region := game.LaunchRegion
		branch, err := store.BranchForUpdate(ctx, tx, faction.ID, region, faction.Name+" "+game.RegionName(region))
		if err != nil {
			return err
		}
		ps := game.PlayerFactionState{Level: player.Level, InFaction: current != nil}
		bs := game.BranchState{HasCommander: branch.LeaderID.Valid}

		rank := game.RankMember
		if lead {
			if ruleErr := game.CanClaimRegion(ps, bs, settings); ruleErr != nil {
				return ruleErr
			}
			won, err := store.ClaimBranch(ctx, tx, branch.ID, playerID)
			if err != nil {
				return err
			}
			if !won {
				return apperr.FactionClaimed
			}
			if err := store.WakeFaction(ctx, tx, faction.ID); err != nil {
				return err
			}
			// The first commander this branch has ever had founded it.
			if !branch.FoundedBy.Valid {
				if err := store.FoundBranch(ctx, tx, branch.ID, playerID, faction.ID, region); err != nil {
					return err
				}
			}
			rank = game.RankRegionalCommander
		} else if ruleErr := game.CanJoin(ps, bs, settings); ruleErr != nil {
			return ruleErr
		}

		if err := store.AddMember(ctx, tx, playerID, faction.ID, branch.ID, rank); err != nil {
			return err
		}
		// The pledge sticks to every phone and sign-in this player uses.
		if err := s.markPledge(ctx, tx, playerID, faction.Code); err != nil {
			return err
		}
		membership, err = store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		describeMembership(membership)
		// The chosen power speaks once, its short line (freelancer-path-plan.md C3).
		membership.PowerName, membership.PowerLine, err = store.FactionVoice(ctx, tx, faction.ID)
		return err
	})
	if err == nil && lead {
		slog.Info("regional command claimed", "faction", code, "region", game.LaunchRegion, "commander", playerID)
	}
	return membership, err
}

// describeMembership names the rank for the app ("Regional Commander").
func describeMembership(m *store.Membership) {
	if m != nil {
		m.RankTitle = game.RankTitle(m.Rank)
	}
}

// FactionInside is a faction as an admin looking inside sees it: every branch with its commander
// and members. The admin is not a member, is not listed and nobody is told (a "ghost" visit, for
// investigating); each look is written to the admin log.
type FactionInside struct {
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	Color    string                `json:"color"`
	Status   string                `json:"status"`
	Members  int                   `json:"members"`
	Branches []store.FactionBranch `json:"branches"`
}

// AdminFactions lists the six factions for an admin to look inside.
func (s *Service) AdminFactions(ctx context.Context) ([]store.Faction, error) {
	return store.ListFactions(ctx, s.db, game.LaunchRegion)
}

// AdminFaction looks inside one faction without joining it.
func (s *Service) AdminFaction(ctx context.Context, adminID int64, code string) (FactionInside, error) {
	var out FactionInside
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		faction, err := store.FactionByCodeForUpdate(ctx, tx, code)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("faction %q", code)
		}
		if err != nil {
			return err
		}
		out = FactionInside{Code: faction.Code, Name: faction.Name, Color: faction.Color, Status: faction.Status}
		if out.Branches, err = store.FactionBranches(ctx, tx, faction.ID); err != nil {
			return err
		}
		for i := range out.Branches {
			members, err := store.BranchMembers(ctx, tx, out.Branches[i].ID)
			if err != nil {
				return err
			}
			for j := range members {
				members[j].RankTitle = game.RankTitle(members[j].Rank)
			}
			out.Branches[i].Members = members
			out.Members += len(members)
		}
		return store.LogAdminAction(ctx, tx, adminID, "faction.inspect", faction.Code, "{}")
	})
	return out, err
}
