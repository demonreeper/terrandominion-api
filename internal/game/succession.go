package game

import (
	"sort"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// Leader succession (GAME_DESIGN "Leader succession", faction-ranks-plan.md): when a regional Commander steps down, is
// inactive for CommanderInactiveDays, is banned or deletes the account, the highest rank in that country takes the
// seat at once as acting Commander. For CommanderVoteDays after the seat changes any Member or higher there can start
// a vote, which runs CommanderVoteHours; a majority of the ballots cast wins. With no majority, or no vote started in
// the window, the acting Commander keeps the seat and stops being acting. Nobody left: the branch goes dormant.

// SeatCandidate is a member of a branch as succession weighs them.
type SeatCandidate struct {
	PlayerID int64
	Rank     int
	Nomad    bool
	Points   int64
	JoinedAt time.Time
	// LastSeen is the last sign-in or refresh (the account's creation when never seen).
	LastSeen time.Time
	// Active: the account is neither banned nor waiting to be deleted.
	Active bool
}

// CommanderInactive says whether a member last seen at lastSeen has been away CommanderInactiveDays (0: never).
func CommanderInactive(lastSeen, now time.Time, s Settings) bool {
	if s.CommanderInactiveDays <= 0 || lastSeen.IsZero() {
		return false
	}
	return !now.Before(lastSeen.Add(days(s.CommanderInactiveDays)))
}

// CanHoldSeat says whether a member may hold or stand for a regional seat: Member or higher (below the two global
// seats), not a Nomad, an active account, and not inactive long enough to lose the seat again at once.
func CanHoldSeat(c SeatCandidate, now time.Time, s Settings) bool {
	return c.Active && c.Rank < RankAssistantHead && Can(c.Rank, c.Nomad, PowerHoldSeat) &&
		!CommanderInactive(c.LastSeen, now, s)
}

// SuccessionOrder sorts the members who may hold the seat, next in line first: the highest rank, then the most
// contribution points, then the longest in the faction (earliest joined), then the lowest id. leaving is left out.
func SuccessionOrder(members []SeatCandidate, leaving int64, now time.Time, s Settings) []SeatCandidate {
	out := make([]SeatCandidate, 0, len(members))
	for _, m := range members {
		if m.PlayerID != leaving && CanHoldSeat(m, now, s) {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Rank != b.Rank:
			return a.Rank > b.Rank
		case a.Points != b.Points:
			return a.Points > b.Points
		case !a.JoinedAt.Equal(b.JoinedAt):
			return a.JoinedAt.Before(b.JoinedAt)
		}
		return a.PlayerID < b.PlayerID
	})
	return out
}

// NextCommander is who takes the seat as acting Commander (false: nobody left, the branch goes dormant).
func NextCommander(members []SeatCandidate, leaving int64, now time.Time, s Settings) (int64, bool) {
	order := SuccessionOrder(members, leaving, now, s)
	if len(order) == 0 {
		return 0, false
	}
	return order[0].PlayerID, true
}

func days(d float64) time.Duration { return time.Duration(d * float64(24*time.Hour)) }

// VoteWindowEnd is the last moment a vote can start after the seat changed at changedAt.
func VoteWindowEnd(changedAt time.Time, s Settings) time.Time {
	return changedAt.Add(days(s.CommanderVoteDays))
}

// VoteClosesAt is when a vote opened at openedAt closes.
func VoteClosesAt(openedAt time.Time, s Settings) time.Time {
	return openedAt.Add(time.Duration(s.CommanderVoteHours * float64(time.Hour)))
}

// SeatState is a branch's seat as the vote rules read it.
type SeatState struct {
	HasLeader bool
	Acting    bool
	ChangedAt time.Time
	// VoteOpen: a vote is running now.
	VoteOpen bool
}

// CanStartVote: a member who may vote (Member or higher, not a Nomad) starts a vote while the seat is held by an
// acting Commander, within CommanderVoteDays of the change, with no vote running.
func CanStartVote(rank int, nomad bool, seat SeatState, now time.Time, s Settings) *apperr.Error {
	switch {
	case !Can(rank, nomad, PowerVote):
		return apperr.VoteNotAllowed
	case seat.VoteOpen:
		return apperr.VoteRefused.Say("A vote is already open.")
	case !seat.HasLeader || !seat.Acting:
		return apperr.VoteRefused.Say("A vote can only start while an acting Commander holds the seat.")
	case !now.Before(VoteWindowEnd(seat.ChangedAt, s)):
		return apperr.VoteRefused.Say("The time to start a vote has passed.")
	}
	return nil
}

// ActingSettles says whether an acting Commander becomes the Commander because no vote was started in the window.
func ActingSettles(seat SeatState, now time.Time, s Settings) bool {
	return seat.HasLeader && seat.Acting && !seat.VoteOpen && !now.Before(VoteWindowEnd(seat.ChangedAt, s))
}

// VoteOutcome is a counted vote: Winner holds a majority of the ballots cast (more than half), else Majority is
// false. A tie at the top is never a majority.
type VoteOutcome struct {
	Winner   int64
	Majority bool
	Ballots  int
	Top      int
}

// CountVotes counts the ballots (one candidate id per ballot).
func CountVotes(ballots []int64) VoteOutcome {
	out := VoteOutcome{Ballots: len(ballots)}
	tally := map[int64]int{}
	for _, c := range ballots {
		tally[c]++
	}
	var lead int64
	for c, n := range tally {
		if n > out.Top || (n == out.Top && c < lead) {
			lead, out.Top = c, n
		}
	}
	if out.Top*2 > out.Ballots {
		out.Winner, out.Majority = lead, true
	}
	return out
}

// SeatAfterVote is who holds the seat when a vote closes: the winner with a majority (if they may still hold a seat),
// else whoever holds it now (0: the seat is empty). The seat is never acting after a vote.
func SeatAfterVote(holder int64, out VoteOutcome, winnerEligible bool) int64 {
	if out.Majority && winnerEligible {
		return out.Winner
	}
	return holder
}
