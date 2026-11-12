package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Leader succession (GAME_DESIGN "Leader succession", faction-ranks-plan.md; rules in game/succession.go). The seat
// is freed by STEP DOWN, a ban, a deletion request, an admin taking the rank or the pledge away, becoming a Nomad, or
// the sweep (inactive, banned or deleted Commanders). The next in line takes it at once as acting Commander; the
// branch hears it. A vote can then start for CommanderVoteDays; the sweep closes votes and settles a window that ran
// out with no vote.

// ---- filling the seat --------------------------------------------------------------------------

// leaveSeat frees the seat playerID holds, if any, and fills it with the next in line. demoteTo > 0 also sets the
// leaver's rank (to Member for STEP DOWN, inactivity, a ban or a deletion); 0 leaves it to the caller.
func (s *Service) leaveSeat(ctx context.Context, tx *sql.Tx, playerID int64, demoteTo int, why string) error {
	seat, err := store.SeatHeldBy(ctx, tx, playerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if demoteTo > 0 {
		m, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if m != nil && m.Rank >= game.RankRegionalCommander && m.Rank < game.RankAssistantHead {
			if err := setRank(ctx, tx, playerID, demoteTo, m.Nomad); err != nil {
				return err
			}
		}
	}
	slog.Info("commander seat freed", "branch", seat.BranchID, "commander", playerID, "why", why)
	return s.fillSeat(ctx, tx, seat.BranchID, playerID)
}

// fillSeat gives a branch's seat, held by `leaving` or empty, to the next in line as acting Commander, or leaves it
// empty (the branch goes dormant) when nobody may hold it. Every player of the branch is told.
func (s *Service) fillSeat(ctx context.Context, tx *sql.Tx, branchID, leaving int64) error {
	st := s.Settings(ctx)
	now := s.now()
	seat, err := store.SeatForUpdate(ctx, tx, branchID)
	if err != nil {
		return err
	}
	if seat.LeaderID.Valid && seat.LeaderID.Int64 != leaving {
		return nil
	}
	candidates, err := store.SeatCandidates(ctx, tx, branchID)
	if err != nil {
		return err
	}
	players, err := store.BranchPlayers(ctx, tx, branchID)
	if err != nil {
		return err
	}
	next, ok := game.NextCommander(candidates, leaving, now, st)
	if !ok {
		if err := store.SetSeat(ctx, tx, branchID, 0, false, 0, &now); err != nil {
			return err
		}
		slog.Info("commander seat empty", "branch", branchID)
		return notifyAll(ctx, tx, players, leaving, "seat_changed", 0, func(int64) string { return game.NoticeSeatEmpty() })
	}
	prev := game.RankMember
	for _, c := range candidates {
		if c.PlayerID == next {
			prev = c.Rank
		}
	}
	if err := setRank(ctx, tx, next, game.RankRegionalCommander, false); err != nil {
		return err
	}
	if err := store.SetSeat(ctx, tx, branchID, next, true, prev, &now); err != nil {
		return err
	}
	ref, err := store.PlayerRefByID(ctx, tx, next)
	if err != nil {
		return err
	}
	slog.Info("acting commander", "branch", branchID, "commander", next)
	return notifyAll(ctx, tx, players, leaving, "seat_changed", next, func(p int64) string {
		if p == next {
			return game.NoticeSeatActingYou(st.CommanderVoteDays)
		}
		return game.NoticeSeatActing(ref.Name, st.CommanderVoteDays)
	})
}

// notifyAll tells each player but `skip` one notice kind; actor is the player it is about (0: none).
func notifyAll(ctx context.Context, tx *sql.Tx, players []int64, skip int64, kind string, actor int64, body func(int64) string) error {
	for _, p := range players {
		if p == skip {
			continue
		}
		n := store.NewNotice{PlayerID: p, Kind: kind, Body: body(p)}
		if actor != 0 && actor != p {
			n.ActorID = actor
		}
		if err := notify(ctx, tx, n); err != nil {
			return err
		}
	}
	return nil
}

// ---- the player's side -------------------------------------------------------------------------

// SeatView is GET /v1/factions/seat (and BranchView.seat): the Commander's seat in the player's home country.
type SeatView struct {
	Commander *store.PlayerRef `json:"commander"`
	// Acting: the seat is held by an acting Commander. VoteUntil is the last moment to start a vote (acting only).
	Acting    bool       `json:"acting"`
	ChangedAt *time.Time `json:"changedAt,omitempty"`
	VoteUntil *time.Time `json:"voteUntil,omitempty"`
	// Mine: the viewer holds the seat (STEP DOWN). CanVote: the viewer may start a vote and cast a ballot (Member or
	// higher, not a Nomad); CanStartVote: and one can start now.
	Mine         bool      `json:"mine"`
	CanStepDown  bool      `json:"canStepDown"`
	CanVote      bool      `json:"canVote"`
	CanStartVote bool      `json:"canStartVote"`
	Vote         *VoteView `json:"vote,omitempty"`
}

// VoteView is an open vote. Ballots is how many are cast (secret: no tally until it ends); MyVote is the viewer's
// pick. Candidates (GET /v1/factions/seat only) are the members who may hold the seat.
type VoteView struct {
	ID         int64                `json:"id"`
	OpenedAt   time.Time            `json:"openedAt"`
	ClosesAt   time.Time            `json:"closesAt"`
	Ballots    int                  `json:"ballots"`
	StartedBy  *store.PlayerRef     `json:"startedBy,omitempty"`
	MyVote     *store.PlayerRef     `json:"myVote,omitempty"`
	Candidates []store.BranchMember `json:"candidates,omitempty"`
}

// Seat is GET /v1/factions/seat.
func (s *Service) Seat(ctx context.Context, playerID int64) (SeatView, error) {
	m, err := memberFor(ctx, s.db, playerID)
	if err != nil {
		return SeatView{}, err
	}
	if m.BranchID == nil {
		return SeatView{}, apperr.NotInBranch
	}
	return s.seatView(ctx, s.db, playerID, m, true)
}

func (s *Service) seatView(ctx context.Context, q db.Querier, playerID int64, m *store.Membership, candidates bool) (SeatView, error) {
	st := s.Settings(ctx)
	now := s.now()
	var out SeatView
	seat, err := store.SeatOf(ctx, q, *m.BranchID)
	if err != nil {
		return out, err
	}
	if seat.LeaderID.Valid {
		ref, err := store.PlayerRefByID(ctx, q, seat.LeaderID.Int64)
		if err != nil {
			return out, err
		}
		out.Commander = &ref
		out.Mine = seat.LeaderID.Int64 == playerID
		out.CanStepDown = out.Mine
	}
	out.Acting = seat.Acting && seat.LeaderID.Valid
	if seat.ChangedAt.Valid {
		at := seat.ChangedAt.Time
		out.ChangedAt = &at
		if out.Acting {
			until := game.VoteWindowEnd(at, st)
			out.VoteUntil = &until
		}
	}
	vote, err := store.OpenVote(ctx, q, *m.BranchID)
	if err != nil {
		return out, err
	}
	out.CanVote = game.Can(m.Rank, m.Nomad, game.PowerVote)
	state := game.SeatState{HasLeader: seat.LeaderID.Valid, Acting: seat.Acting, ChangedAt: seat.ChangedAt.Time, VoteOpen: vote != nil}
	out.CanStartVote = game.CanStartVote(m.Rank, m.Nomad, state, now, st) == nil
	if vote == nil {
		return out, nil
	}
	v := &VoteView{ID: vote.ID, OpenedAt: vote.OpenedAt, ClosesAt: vote.ClosesAt, Ballots: vote.Ballots}
	if vote.StartedBy.Valid {
		ref, err := store.PlayerRefByID(ctx, q, vote.StartedBy.Int64)
		if err != nil {
			return out, err
		}
		v.StartedBy = &ref
	}
	if v.MyVote, err = store.MyBallot(ctx, q, vote.ID, playerID); err != nil {
		return out, err
	}
	if candidates {
		if v.Candidates, err = seatCandidates(ctx, q, *m.BranchID, seat, now, st); err != nil {
			return out, err
		}
	}
	out.Vote = v
	return out, nil
}

// seatCandidates lists the branch members who may stand for the seat, in the order of succession.
func seatCandidates(ctx context.Context, q db.Querier, branchID int64, seat store.Seat, now time.Time, st game.Settings) ([]store.BranchMember, error) {
	weighed, err := store.SeatCandidates(ctx, q, branchID)
	if err != nil {
		return nil, err
	}
	members, err := store.BranchMembers(ctx, q, branchID)
	if err != nil {
		return nil, err
	}
	byID := map[int64]store.BranchMember{}
	for _, b := range members {
		byID[b.PlayerID] = b
	}
	out := []store.BranchMember{}
	for _, c := range game.SuccessionOrder(weighed, 0, now, st) {
		b, ok := byID[c.PlayerID]
		if !ok {
			continue
		}
		describeBranchMember(&b, seat)
		out = append(out, b)
	}
	return out, nil
}

// describeBranchMember names a roster row's rank: Acting Commander for the seat's acting holder, Nomad for a Nomad.
func describeBranchMember(b *store.BranchMember, seat store.Seat) {
	b.RankTitle = game.RankTitle(b.Rank)
	if b.Nomad {
		b.RankTitle = "Nomad"
	}
	b.RankKey = game.RankAt(b.Rank).Key
	if seat.Acting && seat.LeaderID.Valid && seat.LeaderID.Int64 == b.PlayerID {
		b.Acting = true
		b.RankTitle = game.ActingCommanderTitle
	}
}

// StepDown: the Commander (acting or not) gives up the seat and becomes a Member; the next in line takes it at once
// as acting Commander, or the branch goes dormant if nobody may hold it.
func (s *Service) StepDown(ctx context.Context, playerID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		if _, err := store.SeatHeldBy(ctx, tx, playerID); errors.Is(err, store.ErrNotFound) {
			return apperr.NotSeatHolder
		} else if err != nil {
			return err
		}
		return s.leaveSeat(ctx, tx, playerID, game.RankMember, "stepped down")
	})
}

// StartVote: a Member or higher opens a vote for the seat of their home country while an acting Commander holds it,
// within CommanderVoteDays of the change. Everyone in the branch is told.
func (s *Service) StartVote(ctx context.Context, playerID int64) (SeatView, error) {
	st := s.Settings(ctx)
	now := s.now()
	var out SeatView
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if m.BranchID == nil {
			return apperr.NotInBranch
		}
		seat, err := store.SeatForUpdate(ctx, tx, *m.BranchID)
		if err != nil {
			return err
		}
		open, err := store.OpenVote(ctx, tx, *m.BranchID)
		if err != nil {
			return err
		}
		state := game.SeatState{HasLeader: seat.LeaderID.Valid, Acting: seat.Acting, ChangedAt: seat.ChangedAt.Time, VoteOpen: open != nil}
		if ruleErr := game.CanStartVote(m.Rank, m.Nomad, state, now, st); ruleErr != nil {
			if ruleErr.Code == apperr.VoteRefused.Code && seat.ChangedAt.Valid {
				return ruleErr.WithData(map[string]any{"until": game.VoteWindowEnd(seat.ChangedAt.Time, st).UTC().Format(time.RFC3339)})
			}
			return ruleErr
		}
		if _, err := store.InsertVote(ctx, tx, *m.BranchID, playerID, now, game.VoteClosesAt(now, st)); err != nil {
			return err
		}
		players, err := store.BranchPlayers(ctx, tx, *m.BranchID)
		if err != nil {
			return err
		}
		if err := notifyAll(ctx, tx, players, 0, "vote_open", playerID, func(int64) string {
			return game.NoticeVoteOpen(st.CommanderVoteHours)
		}); err != nil {
			return err
		}
		out, err = s.seatView(ctx, tx, playerID, m, true)
		return err
	})
	if err == nil {
		slog.Info("commander vote opened", "by", playerID)
	}
	return out, err
}

// CastVote: one ballot per voter for a member who may hold the seat, while the vote is open.
func (s *Service) CastVote(ctx context.Context, playerID, candidateID int64) (SeatView, error) {
	st := s.Settings(ctx)
	now := s.now()
	var out SeatView
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if m.BranchID == nil {
			return apperr.NotInBranch
		}
		if !game.Can(m.Rank, m.Nomad, game.PowerVote) {
			return apperr.VoteNotAllowed
		}
		open, err := store.OpenVote(ctx, tx, *m.BranchID)
		if err != nil {
			return err
		}
		if open == nil {
			return apperr.VoteGone
		}
		vote, err := store.VoteForUpdate(ctx, tx, open.ID)
		if err != nil {
			return err
		}
		if vote.Status != "open" || !now.Before(vote.ClosesAt) {
			return apperr.VoteGone
		}
		c, err := store.SeatCandidateIn(ctx, tx, *m.BranchID, candidateID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.VoteCandidateBad
		}
		if err != nil {
			return err
		}
		if !game.CanHoldSeat(c, now, st) {
			return apperr.VoteCandidateBad
		}
		cast, err := store.CastBallot(ctx, tx, vote.ID, playerID, candidateID, now)
		if err != nil {
			return err
		}
		if !cast {
			return apperr.VoteAlreadyCast
		}
		out, err = s.seatView(ctx, tx, playerID, m, true)
		return err
	})
	return out, err
}

// closeVote counts a vote and settles the seat: a majority of the ballots cast puts the winner in it (the acting
// Commander they replace goes back to their old rank); with none, whoever holds it stays and the seat stops being
// acting. Everyone in the branch is told. endedBy is the admin of an END VOTE (0: it closed on time).
func (s *Service) closeVote(ctx context.Context, tx *sql.Tx, voteID, endedBy int64) error {
	st := s.Settings(ctx)
	now := s.now()
	vote, err := store.VoteForUpdate(ctx, tx, voteID)
	if errors.Is(err, store.ErrNotFound) {
		return apperr.VoteGone
	}
	if err != nil {
		return err
	}
	if vote.Status != "open" {
		return apperr.VoteGone
	}
	seat, err := store.SeatForUpdate(ctx, tx, vote.BranchID)
	if err != nil {
		return err
	}
	ballots, err := store.VoteBallots(ctx, tx, voteID)
	if err != nil {
		return err
	}
	result := game.CountVotes(ballots)
	eligible := false
	if result.Majority {
		c, err := store.SeatCandidateIn(ctx, tx, vote.BranchID, result.Winner)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		eligible = err == nil && game.CanHoldSeat(c, now, st)
	}
	holder := int64(0)
	if seat.LeaderID.Valid {
		holder = seat.LeaderID.Int64
	}
	next := game.SeatAfterVote(holder, result, eligible)
	changed := &now
	if next == holder {
		changed = nil
	} else {
		if holder != 0 {
			back := game.RankMember
			if seat.Acting && seat.PrevRank.Valid {
				back = int(seat.PrevRank.Int64)
			}
			hm, err := store.MembershipOf(ctx, tx, holder)
			if err != nil {
				return err
			}
			if hm != nil && hm.BranchID != nil && *hm.BranchID == vote.BranchID &&
				hm.Rank >= game.RankRegionalCommander && hm.Rank < game.RankAssistantHead {
				if err := setRank(ctx, tx, holder, back, hm.Nomad); err != nil {
					return err
				}
			}
		}
		if err := setRank(ctx, tx, next, game.RankRegionalCommander, false); err != nil {
			return err
		}
	}
	if err := store.SetSeat(ctx, tx, vote.BranchID, next, false, 0, changed); err != nil {
		return err
	}
	status, winner := "no_majority", int64(0)
	if result.Majority && eligible {
		status, winner = "won", result.Winner
	}
	if err := store.CloseVote(ctx, tx, voteID, status, winner, result.Ballots, now, endedBy); err != nil {
		return err
	}
	name := ""
	if next != 0 {
		ref, err := store.PlayerRefByID(ctx, tx, next)
		if err != nil {
			return err
		}
		name = ref.Name
	}
	players, err := store.BranchPlayers(ctx, tx, vote.BranchID)
	if err != nil {
		return err
	}
	slog.Info("commander vote closed", "vote", voteID, "status", status, "commander", next, "ballots", result.Ballots)
	return notifyAll(ctx, tx, players, 0, "vote_ended", next, func(int64) string {
		if status == "won" {
			return game.NoticeVoteWon(name)
		}
		return game.NoticeVoteNoMajority(name)
	})
}

// ---- the sweep ---------------------------------------------------------------------------------

var lastSuccessionTick time.Time

// successionTick runs with the base tick, at most once a minute: frees the seats of Commanders who are banned,
// deleting their account or inactive for CommanderInactiveDays; closes votes whose time is up; and makes an acting
// Commander the Commander when CommanderVoteDays pass with no vote started.
func (s *Service) successionTick(ctx context.Context) {
	now := s.now()
	if now.Sub(lastSuccessionTick) < time.Minute {
		return
	}
	lastSuccessionTick = now
	st := s.Settings(ctx)
	fail := func(what string, err error, args ...any) {
		if err != nil && ctx.Err() == nil {
			slog.Error("E1900 "+what, append(args, "err", err)...)
		}
	}

	var inactiveBefore time.Time
	if st.CommanderInactiveDays > 0 {
		inactiveBefore = now.Add(-time.Duration(st.CommanderInactiveDays * float64(24*time.Hour)))
	}
	seats, err := store.SeatsToVacate(ctx, s.db, inactiveBefore, 50)
	fail("succession sweep failed", err)
	for _, seat := range seats {
		leader := seat.LeaderID.Int64
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			locked, err := store.SeatForUpdate(ctx, tx, seat.BranchID)
			if err != nil || !locked.LeaderID.Valid || locked.LeaderID.Int64 != leader {
				return err
			}
			// Check again under the lock: a sign-in since the query keeps the seat.
			c, err := store.SeatCandidateIn(ctx, tx, seat.BranchID, leader)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if err == nil && c.Active && !game.CommanderInactive(c.LastSeen, now, st) {
				return nil
			}
			why := "inactive"
			if err == nil && !c.Active {
				why = "banned or deleting"
			}
			return s.leaveSeat(ctx, tx, leader, game.RankMember, why)
		})
		fail("seat vacate failed", err, "branch", seat.BranchID)
	}

	due, err := store.DueVotes(ctx, s.db, now, 50)
	fail("vote sweep failed", err)
	for _, id := range due {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if err := s.closeVote(ctx, tx, id, 0); err != nil && !errors.Is(err, apperr.VoteGone) {
				return err
			}
			return nil
		})
		fail("vote close failed", err, "vote", id)
	}

	settle := now.Add(-time.Duration(st.CommanderVoteDays * float64(24*time.Hour)))
	acting, err := store.ActingSeatsDue(ctx, s.db, settle, 50)
	fail("acting sweep failed", err)
	for _, seat := range acting {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			locked, err := store.SeatForUpdate(ctx, tx, seat.BranchID)
			if err != nil {
				return err
			}
			open, err := store.OpenVote(ctx, tx, seat.BranchID)
			if err != nil {
				return err
			}
			state := game.SeatState{HasLeader: locked.LeaderID.Valid, Acting: locked.Acting, ChangedAt: locked.ChangedAt.Time, VoteOpen: open != nil}
			if !game.ActingSettles(state, now, st) {
				return nil
			}
			leader := locked.LeaderID.Int64
			if err := store.SetSeat(ctx, tx, seat.BranchID, leader, false, 0, nil); err != nil {
				return err
			}
			ref, err := store.PlayerRefByID(ctx, tx, leader)
			if err != nil {
				return err
			}
			players, err := store.BranchPlayers(ctx, tx, seat.BranchID)
			if err != nil {
				return err
			}
			slog.Info("acting commander settled", "branch", seat.BranchID, "commander", leader)
			return notifyAll(ctx, tx, players, 0, "seat_changed", leader, func(int64) string {
				return game.NoticeSeatSettled(ref.Name)
			})
		})
		fail("acting settle failed", err, "branch", seat.BranchID)
	}
}

// ---- admin -------------------------------------------------------------------------------------

// AdminEndVote closes a stuck vote now, counting the ballots cast so far as the sweep would.
func (s *Service) AdminEndVote(ctx context.Context, adminID, voteID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.closeVote(ctx, tx, voteID, adminID); err != nil {
			return err
		}
		return store.LogAdminAction(ctx, tx, adminID, "vote.end", fmt.Sprint(voteID), "{}")
	})
}
