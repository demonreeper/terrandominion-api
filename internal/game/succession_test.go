package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

var succNow = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func cand(id int64, rank int, points int64, joinedDaysAgo int) SeatCandidate {
	return SeatCandidate{PlayerID: id, Rank: rank, Points: points, JoinedAt: succNow.AddDate(0, 0, -joinedDaysAgo),
		LastSeen: succNow.Add(-time.Hour), Active: true}
}

func TestSuccessionSettingsDefaults(t *testing.T) {
	s := DefaultSettings()
	if s.CommanderInactiveDays != 14 || s.CommanderVoteDays != 7 || s.CommanderVoteHours != 48 {
		t.Fatalf("defaults %v %v %v", s.CommanderInactiveDays, s.CommanderVoteDays, s.CommanderVoteHours)
	}
	got := SettingsFrom(map[string]float64{"CommanderInactiveDays": 3, "CommanderVoteDays": 2, "CommanderVoteHours": 12})
	if got.CommanderInactiveDays != 3 || got.CommanderVoteDays != 2 || got.CommanderVoteHours != 12 {
		t.Fatalf("from settings %+v", got)
	}
}

// The order of succession: highest rank, then most points, then longest in the faction; the leaver, Prospects,
// Nomads, banned or inactive accounts and the global seats are skipped.
func TestSuccessionOrder(t *testing.T) {
	s := DefaultSettings()
	nomad := cand(20, RankLead, 9999, 400)
	nomad.Nomad = true
	banned := cand(21, RankLead, 9999, 400)
	banned.Active = false
	away := cand(22, RankLead, 9999, 400)
	away.LastSeen = succNow.AddDate(0, 0, -15)
	members := []SeatCandidate{
		cand(10, RankRegionalCommander, 0, 500), // leaving
		cand(11, RankMember, 5000, 300),
		cand(12, RankWarden, 100, 10),
		cand(13, RankWarden, 300, 5),
		cand(14, RankWarden, 300, 50),
		cand(15, RankProspect, 99999, 900),
		cand(16, RankHead, 0, 900),
		nomad, banned, away,
	}
	order := SuccessionOrder(members, 10, succNow, s)
	var ids []int64
	for _, c := range order {
		ids = append(ids, c.PlayerID)
	}
	want := []int64{14, 13, 12, 11}
	if len(ids) != len(want) {
		t.Fatalf("order %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order %v, want %v", ids, want)
		}
	}
	if next, ok := NextCommander(members, 10, succNow, s); !ok || next != 14 {
		t.Fatalf("next %d %v", next, ok)
	}
	// Nobody left who may hold the seat: dormant.
	if _, ok := NextCommander([]SeatCandidate{cand(10, RankRegionalCommander, 0, 1), cand(15, RankProspect, 0, 1), nomad}, 10, succNow, s); ok {
		t.Fatal("only a Prospect and a Nomad left: the branch should go dormant")
	}
	// Exactly equal: the lower id goes first, so the order never wobbles.
	if next, _ := NextCommander([]SeatCandidate{cand(31, RankMember, 5, 9), cand(30, RankMember, 5, 9)}, 0, succNow, s); next != 30 {
		t.Fatalf("full tie: %d", next)
	}
}

func TestCommanderInactive(t *testing.T) {
	s := DefaultSettings()
	if CommanderInactive(succNow.AddDate(0, 0, -13), succNow, s) {
		t.Fatal("13 days is not inactive")
	}
	if !CommanderInactive(succNow.AddDate(0, 0, -14), succNow, s) {
		t.Fatal("14 days is inactive")
	}
	s.CommanderInactiveDays = 0
	if CommanderInactive(succNow.AddDate(-1, 0, 0), succNow, s) {
		t.Fatal("0 turns inactivity off")
	}
}

// The 7-day window: a vote starts only while an acting Commander holds the seat, before the window ends, with none
// running; once the window passes with no vote the acting Commander settles in.
func TestVoteWindow(t *testing.T) {
	s := DefaultSettings()
	changed := succNow.AddDate(0, 0, -6)
	seat := SeatState{HasLeader: true, Acting: true, ChangedAt: changed}
	if err := CanStartVote(RankMember, false, seat, succNow, s); err != nil {
		t.Fatalf("day 6: %v", err)
	}
	if !errIs(CanStartVote(RankProspect, false, seat, succNow, s), apperr.VoteNotAllowed) ||
		!errIs(CanStartVote(RankWarden, true, seat, succNow, s), apperr.VoteNotAllowed) {
		t.Fatal("Prospects and Nomads can't start a vote")
	}
	open := seat
	open.VoteOpen = true
	if !errIs(CanStartVote(RankMember, false, open, succNow, s), apperr.VoteRefused) {
		t.Fatal("one vote at a time")
	}
	settled := seat
	settled.Acting = false
	if !errIs(CanStartVote(RankMember, false, settled, succNow, s), apperr.VoteRefused) {
		t.Fatal("no vote once the seat isn't acting")
	}
	day7 := VoteWindowEnd(changed, s)
	if !day7.Equal(changed.AddDate(0, 0, 7)) {
		t.Fatalf("window end %v", day7)
	}
	if !errIs(CanStartVote(RankMember, false, seat, day7, s), apperr.VoteRefused) {
		t.Fatal("the window is over at 7 days")
	}
	if ActingSettles(seat, day7.Add(-time.Second), s) || !ActingSettles(seat, day7, s) {
		t.Fatal("the acting Commander settles in at 7 days, not before")
	}
	if ActingSettles(open, day7.Add(time.Hour), s) {
		t.Fatal("an open vote holds the seat acting until it closes")
	}
	if !VoteClosesAt(succNow, s).Equal(succNow.Add(48 * time.Hour)) {
		t.Fatal("votes run 48 h")
	}
}

func errIs(err *apperr.Error, want *apperr.Error) bool { return err != nil && err.Code == want.Code }

// The vote result: more than half of the ballots cast wins; otherwise the holder stays.
func TestVoteResult(t *testing.T) {
	out := CountVotes([]int64{7, 7, 7, 8, 9})
	if !out.Majority || out.Winner != 7 || out.Ballots != 5 || out.Top != 3 {
		t.Fatalf("majority: %+v", out)
	}
	if got := SeatAfterVote(8, out, true); got != 7 {
		t.Fatalf("winner takes the seat: %d", got)
	}
	if got := SeatAfterVote(8, out, false); got != 8 {
		t.Fatalf("a winner who can no longer hold it: the holder stays, got %d", got)
	}
	// A plurality is not a majority.
	plural := CountVotes([]int64{7, 7, 8, 9})
	if plural.Majority {
		t.Fatalf("2 of 4 is not a majority: %+v", plural)
	}
	if got := SeatAfterVote(8, plural, true); got != 8 {
		t.Fatalf("no majority: the acting Commander stays, got %d", got)
	}
	if empty := CountVotes(nil); empty.Majority || SeatAfterVote(0, empty, true) != 0 {
		t.Fatalf("no ballots: %+v", empty)
	}
}

// A tie at the top is never a majority, and the acting Commander stays.
func TestVoteTie(t *testing.T) {
	out := CountVotes([]int64{7, 8, 8, 7})
	if out.Majority || out.Top != 2 {
		t.Fatalf("tie: %+v", out)
	}
	if got := SeatAfterVote(9, out, true); got != 9 {
		t.Fatalf("tie: holder stays, got %d", got)
	}
}

func TestSuccessionNotices(t *testing.T) {
	want := map[string]string{
		NoticeSeatActing("Aishath", 7):  "Aishath is acting Commander in your country. For 7 days any Member or higher can start a vote for the seat in FACTIONS.",
		NoticeSeatActingYou(1):          "You are acting Commander in your country. For 1 day any Member or higher can start a vote for the seat.",
		NoticeSeatSettled("Aishath"):    "No vote was started. Aishath is now Commander in your country.",
		NoticeVoteOpen(48):              "A vote for Commander in your country is open for 48 h. Cast yours in FACTIONS.",
		NoticeVoteWon("Aishath"):        "The vote has ended. Aishath is Commander in your country.",
		NoticeVoteNoMajority("Aishath"): "The vote has ended with no majority. Aishath stays Commander in your country.",
		NoticeVoteNoMajority(""):        "The vote has ended with no majority. The seat stays empty.",
	}
	for got, w := range want {
		if got != w {
			t.Errorf("got %q, want %q", got, w)
		}
	}
	for _, kind := range []string{"seat_changed", "vote_open", "vote_ended"} {
		if _, ok := PushRuleFor(kind); !ok {
			t.Errorf("%s should push (ALERTS lists it)", kind)
		}
	}
}
