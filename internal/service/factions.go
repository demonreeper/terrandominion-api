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
	return store.ListFactions(ctx, s.db)
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

		ps := game.PlayerFactionState{Level: player.Level, InFaction: current != nil}
		fs := game.FactionState{Locked: faction.Status == "locked", HasLeader: faction.LeaderID.Valid}

		rank := game.RankMember
		if lead {
			if ruleErr := game.CanLead(ps, fs, settings); ruleErr != nil {
				return ruleErr
			}
			won, err := store.ClaimFaction(ctx, tx, faction.ID, playerID)
			if err != nil {
				return err
			}
			if !won {
				return apperr.FactionClaimed
			}
			rank = game.RankHead
		} else if ruleErr := game.CanJoin(ps, fs, settings); ruleErr != nil {
			return ruleErr
		}

		if err := store.AddMember(ctx, tx, playerID, faction.ID, rank); err != nil {
			return err
		}
		membership, err = store.MembershipOf(ctx, tx, playerID)
		return err
	})
	if err == nil && lead {
		slog.Info("faction claimed", "faction", code, "leader", playerID)
	}
	return membership, err
}
