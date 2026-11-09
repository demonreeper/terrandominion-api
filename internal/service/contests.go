package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type ContestOutcome struct {
	ID     int64     `json:"id"`
	EndsAt time.Time `json:"endsAt"`
}

// StartContest challenges for a node another player just harvested, or holds. The node is locked
// first, as in Harvest and when a contest resolves.
func (s *Service) StartContest(ctx context.Context, playerID, nodeID int64, fix TetherFix) (ContestOutcome, error) {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return ContestOutcome{}, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	var out ContestOutcome

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		node, err := store.NodeForUpdate(ctx, tx, nodeID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NodeInactive
		}
		if err != nil {
			return err
		}
		if node.Status != "active" {
			return apperr.NodeInactive
		}

		last, err := lastFix(ctx, tx, playerID)
		if err != nil {
			return err
		}
		now := s.now()
		if _, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now},
			last, node.Lat, node.Lon, settings.ContestRangeM); ruleErr != nil {
			return ruleErr
		}

		running, err := store.HasOpenContest(ctx, tx, nodeID)
		if err != nil {
			return err
		}
		if running {
			return apperr.ContestRunning
		}
		defender, err := s.contestDefender(ctx, tx, playerID, nodeID, now, settings)
		if err != nil {
			return err
		}
		if err := spendCells(ctx, tx, playerID, settings.CellsContest); err != nil {
			return err
		}

		endsAt := now.Add(time.Duration(settings.ContestSeconds) * time.Second)
		id, err := store.InsertContest(ctx, tx, nodeID, playerID, defender, endsAt)
		if err != nil {
			return err
		}
		body := fmt.Sprintf("%s is being contested. Be within %.0f m in the next minute to defend it.", node.Name, settings.ContestRangeM)
		if err := notify(ctx, tx, defender, 0, "contest", body); err != nil {
			return err
		}
		out = ContestOutcome{ID: id, EndsAt: endsAt}
		return nil
	})
	return out, err
}

// contestDefender is whoever holds the node's next harvests, or else its last harvester.
func (s *Service) contestDefender(ctx context.Context, tx *sql.Tx, playerID, nodeID int64, now time.Time, settings game.Settings) (int64, error) {
	hold, err := store.ActiveHold(ctx, tx, nodeID)
	if err == nil {
		if hold.WinnerID == playerID {
			return 0, apperr.ContestNobody.With("you already hold this node")
		}
		return hold.WinnerID, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 0, err
	}
	since := now.Add(-time.Duration(settings.ContestRecentMinutes * float64(time.Minute)))
	defender, err := store.RecentHarvester(ctx, tx, nodeID, playerID, since)
	if errors.Is(err, store.ErrNotFound) {
		return 0, apperr.ContestNobody
	}
	return defender, err
}

// JoinContest puts a player on a side: the defender themselves, or a faction ally of either side.
func (s *Service) JoinContest(ctx context.Context, playerID, contestID int64, fix TetherFix) error {
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		contest, err := store.ContestForUpdate(ctx, tx, contestID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.ContestOver
		}
		if err != nil {
			return err
		}
		now := s.now()
		if contest.Status != "open" || !contest.EndsAt.After(now) {
			return apperr.ContestOver
		}
		side, err := s.contestSide(ctx, tx, playerID, contest)
		if err != nil {
			return err
		}
		node, err := store.NodeByID(ctx, tx, contest.NodeID)
		if err != nil {
			return err
		}
		last, err := lastFix(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if _, ruleErr := game.CheckInRange(settings, game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now},
			last, node.Lat, node.Lon, settings.ContestRangeM); ruleErr != nil {
			return ruleErr
		}
		return store.JoinContest(ctx, tx, contestID, playerID, side)
	})
}

func (s *Service) contestSide(ctx context.Context, q db.Querier, playerID int64, contest store.Contest) (string, error) {
	if playerID == contest.ChallengerID {
		return "", apperr.ContestNotYours.With("you started this contest")
	}
	if playerID == contest.DefenderID {
		return "defender", nil
	}
	mine, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return "", err
	}
	if mine == nil {
		return "", apperr.ContestNotYours
	}
	defender, err := store.MembershipOf(ctx, q, contest.DefenderID)
	if err != nil {
		return "", err
	}
	if defender != nil && defender.FactionID == mine.FactionID {
		return "defender", nil
	}
	challenger, err := store.MembershipOf(ctx, q, contest.ChallengerID)
	if err != nil {
		return "", err
	}
	if challenger != nil && challenger.FactionID == mine.FactionID {
		return "challenger", nil
	}
	return "", apperr.ContestNotYours
}

// finishContest scores a contest and gives the winner the node's next harvests.
func (s *Service) finishContest(ctx context.Context, contestID, nodeID int64, settings game.Settings) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		node, err := store.NodeForUpdate(ctx, tx, nodeID)
		if err != nil {
			return err
		}
		contest, err := store.ContestForUpdate(ctx, tx, contestID)
		if err != nil || contest.Status != "open" {
			return err
		}
		challengerAllies, defenderAllies, showed, err := store.ContestSideCounts(ctx, tx, contest)
		if err != nil {
			return err
		}
		challenger, err := s.contestSideStats(ctx, tx, contest.ChallengerID, challengerAllies, false)
		if err != nil {
			return err
		}
		defender, err := s.contestSideStats(ctx, tx, contest.DefenderID, defenderAllies, showed)
		if err != nil {
			return err
		}

		challengerScore, defenderScore := game.ContestScore(settings, challenger), game.ContestScore(settings, defender)
		winner, loser := contest.DefenderID, contest.ChallengerID
		if game.ChallengerWins(challengerScore, defenderScore) {
			winner, loser = contest.ChallengerID, contest.DefenderID
		}
		holdUntil := s.now().Add(time.Duration(settings.ContestHoldMinutes * float64(time.Minute)))
		if err := store.ResolveContest(ctx, tx, contest, winner, challengerScore, defenderScore,
			settings.ContestPrizeHarvests, holdUntil); err != nil {
			return err
		}

		score := fmt.Sprintf("%d to %d", max(challengerScore, defenderScore), min(challengerScore, defenderScore))
		won := fmt.Sprintf("You won the contest for %s, %s. Its next %d harvests are yours.", node.Name, score, settings.ContestPrizeHarvests)
		if err := notify(ctx, tx, winner, 0, "contest_won", won); err != nil {
			return err
		}
		return notify(ctx, tx, loser, 0, "contest_lost", fmt.Sprintf("You lost the contest for %s, %s.", node.Name, score))
	})
}

func (s *Service) contestSideStats(ctx context.Context, q db.Querier, playerID int64, allies int, showed bool) (game.ContestSide, error) {
	player, err := store.GetPlayer(ctx, q, playerID)
	if err != nil {
		return game.ContestSide{}, err
	}
	tiers, err := store.WorkingGearTiers(ctx, q, playerID)
	if err != nil {
		return game.ContestSide{}, err
	}
	return game.ContestSide{Level: player.Level, GearTiers: tiers, Allies: allies, Showed: showed}, nil
}

// checkNodeHold refuses a harvest of a node someone else holds, and spends the holder's own
// harvest. Call it only once the harvest has passed every other rule.
func checkNodeHold(ctx context.Context, tx *sql.Tx, playerID, nodeID int64) error {
	hold, err := store.ActiveHold(ctx, tx, nodeID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if hold.WinnerID != playerID {
		return apperr.NodeContested
	}
	return store.UseHoldHarvest(ctx, tx, hold.ContestID)
}

// addContestInfo tells each node's viewer about its contest, so the app can offer the right button.
func (s *Service) addContestInfo(ctx context.Context, playerID int64, nodes []store.Node, settings game.Settings) error {
	ids := make([]int64, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	states, err := store.NodeContestStates(ctx, s.db, ids)
	if err != nil {
		return err
	}
	since := s.now().Add(-time.Duration(settings.ContestRecentMinutes * float64(time.Minute)))
	recent, err := store.RecentlyHarvestedByOthers(ctx, s.db, ids, playerID, since)
	if err != nil {
		return err
	}

	for i := range nodes {
		node := &nodes[i]
		state := states[node.ID]
		node.HeldByMe = state.HolderID == playerID
		node.HeldByOther = state.HolderID != 0 && !node.HeldByMe
		if state.ContestID != 0 {
			node.ContestID = state.ContestID
			node.ContestEndsAt = state.EndsAt
			contest := store.Contest{ChallengerID: state.ChallengerID, DefenderID: state.DefenderID}
			if playerID == state.ChallengerID {
				node.ContestRole = "challenger"
			} else if side, err := s.contestSide(ctx, s.db, playerID, contest); err == nil {
				node.ContestRole = side
			}
			continue
		}
		node.CanContest = node.HeldByOther || (recent[node.ID] && !node.HeldByMe)
	}
	return nil
}
