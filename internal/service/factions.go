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

func (s *Service) Factions(ctx context.Context) ([]store.Faction, error) {
	return store.ListFactions(ctx, s.db, game.LaunchRegion)
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
		membership, err = store.MembershipOf(ctx, tx, playerID)
		describeMembership(membership)
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
