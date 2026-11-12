package game

import (
	"testing"
	"time"
)

func TestRankLadder(t *testing.T) {
	if len(Ranks) != 8 {
		t.Fatalf("eight ranks, got %d", len(Ranks))
	}
	for i := 1; i < len(Ranks); i++ {
		if Ranks[i].Level >= Ranks[i-1].Level {
			t.Fatalf("ranks out of order at %s", Ranks[i].Key)
		}
	}
	// Each rank has every power of the rank below it, except the Assistant Global Head (a Warden's, everywhere).
	for i := 2; i < len(Ranks)-1; i++ {
		if Ranks[i].Key == "assistant_global_head" {
			continue
		}
		for _, p := range Ranks[i+1].Powers {
			if !Can(Ranks[i].Level, false, p) {
				t.Errorf("%s lacks %s that %s has", Ranks[i].Key, p, Ranks[i+1].Key)
			}
		}
	}
	cases := []struct {
		level int
		power Power
		want  bool
	}{
		{RankProspect, PowerGive, true}, {RankProspect, PowerVote, false}, {RankProspect, PowerVaultOnSite, false},
		{RankMember, PowerVote, true}, {RankMember, PowerPostJobs, false},
		{RankCaptain, PowerPostJobs, true}, {RankCaptain, PowerSquadLead, true}, {RankCaptain, PowerVaultAnywhere, false},
		{RankWarden, PowerProposeBase, true}, {RankWarden, PowerApproveNomads, true}, {RankWarden, PowerStartT4, false},
		{RankLead, PowerStartT4, true}, {RankLead, PowerApproveBase, false},
		{RankRegionalCommander, PowerApproveBase, true}, {RankRegionalCommander, PowerEveryRegion, false},
		{RankAssistantHead, PowerEveryRegion, true}, {RankAssistantHead, PowerApproveBase, false},
		{RankAssistantHead, PowerProposeBase, true}, {RankHead, PowerApproveBase, true},
	}
	for _, c := range cases {
		if got := Can(c.level, false, c.power); got != c.want {
			t.Errorf("%s can %s: %v, want %v", RankAt(c.level).Key, c.power, got, c.want)
		}
	}
	// A Nomad has a Member's powers at most, and no vote or seat.
	if Can(RankWarden, true, PowerProposeBase) || Can(RankMember, true, PowerVote) || !Can(RankMember, true, PowerGive) {
		t.Error("nomad powers")
	}
	if RankAt(1).Key != "prospect" || RankAt(60).Key != "warden" {
		t.Error("RankAt")
	}
}

func TestCanAppoint(t *testing.T) {
	ok := []struct{ by, target, to int }{
		{RankRegionalCommander, RankMember, RankLead}, {RankRegionalCommander, RankCaptain, RankWarden},
		{RankLead, RankMember, RankWarden}, {RankLead, RankMember, RankCaptain}, {RankWarden, RankMember, RankCaptain},
	}
	for _, c := range ok {
		if !CanAppoint(c.by, false, c.target, c.to) {
			t.Errorf("%d should appoint %d to %d", c.by, c.target, c.to)
		}
	}
	no := []struct{ by, target, to int }{
		{RankWarden, RankMember, RankWarden}, {RankLead, RankMember, RankLead}, {RankCaptain, RankMember, RankCaptain},
		{RankRegionalCommander, RankWarden, RankCaptain}, {RankRegionalCommander, RankProspect, RankCaptain},
		{RankRegionalCommander, RankMember, RankRegionalCommander},
	}
	for _, c := range no {
		if CanAppoint(c.by, false, c.target, c.to) {
			t.Errorf("%d must not appoint %d to %d", c.by, c.target, c.to)
		}
	}
	if CanAppoint(RankRegionalCommander, true, RankMember, RankCaptain) {
		t.Error("a Nomad appoints nobody")
	}
	if !ActsAsGlobalHead(RankAssistantHead, true) || ActsAsGlobalHead(RankAssistantHead, false) || ActsAsGlobalHead(RankWarden, true) {
		t.Error("acting Global Head")
	}
}

func TestPromotions(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	if ProspectDone(now.Add(-6*24*time.Hour), 0, now, s) || !ProspectDone(now.Add(-7*24*time.Hour), 0, now, s) ||
		!ProspectDone(now, 100, now, s) {
		t.Error("prospect: 7 days or 100 points")
	}
	if NextPromotion(RankMember, false, 999, s) != 0 || NextPromotion(RankMember, false, 1000, s) != RankCaptain {
		t.Error("captain at 1000")
	}
	if NextPromotion(RankMember, false, 9999, s) != RankCaptain || NextPromotion(RankCaptain, false, 9999, s) != RankWarden ||
		NextPromotion(RankWarden, false, 9999, s) != RankLead || NextPromotion(RankLead, false, 1e9, s) != 0 {
		t.Error("one step at a time, never past Lead")
	}
	if NextPromotion(RankMember, true, 9999, s) != 0 || NextPromotion(RankProspect, false, 9999, s) != 0 {
		t.Error("no climbing for Nomads or Prospects by points")
	}
	if RankSlots(RankWarden, s) != 6 || RankSlots(RankMember, s) != 0 {
		t.Error("slots")
	}
	if !NomadMoveAt(time.Time{}, s).IsZero() || !NomadMoveAt(now, s).Equal(now.Add(30*24*time.Hour)) {
		t.Error("nomad cooldown")
	}
	if !NomadAskExpiry(now, s).Equal(now.Add(48 * time.Hour)) {
		t.Error("ask lasts 48 h")
	}
}
