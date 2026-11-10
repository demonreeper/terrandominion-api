package game

import (
	"testing"
	"time"
)

func TestMissionDayRefreshesAtMaldivesMidnight(t *testing.T) {
	s := DefaultSettings()
	// 18:59 UTC is 23:59 in the Maldives: still the 10th.
	day, next := MissionDay(s, time.Date(2026, 11, 10, 18, 59, 0, 0, time.UTC))
	if day.Format("2006-01-02") != "2026-11-10" {
		t.Fatalf("expected the 10th, got %v", day)
	}
	if !next.Equal(time.Date(2026, 11, 10, 19, 0, 0, 0, time.UTC)) {
		t.Fatalf("next day should start at 19:00 UTC, got %v", next)
	}
	// 19:00 UTC is midnight in the Maldives: the 11th.
	day, _ = MissionDay(s, time.Date(2026, 11, 10, 19, 0, 0, 0, time.UTC))
	if day.Format("2006-01-02") != "2026-11-11" {
		t.Fatalf("expected the 11th, got %v", day)
	}
}

func TestMissionRewardsGrow(t *testing.T) {
	s := DefaultSettings()
	low := MissionRewardFor(s, 2, false, 0.9)
	high := MissionRewardFor(s, 18, false, 0.9)
	if !(high.XP > low.XP && high.Credits > low.Credits) {
		t.Fatalf("rewards should grow with level: %+v then %+v", low, high)
	}
	if low.Item != "" {
		t.Fatal("no item above the chance")
	}
	if got := MissionRewardFor(s, 2, false, 0.1); got.Item != ItemCell || got.ItemCount != 1 {
		t.Fatalf("a low-level player gets an energy cell, got %+v", got)
	}
	if got := MissionRewardFor(s, 18, false, 0.1); got.Item != WeaponSpike {
		t.Fatalf("level 18 gets a spike, got %+v", got)
	}
	command := MissionRewardFor(s, 20, true, 0.9)
	member := MissionRewardFor(s, 20, false, 0.9)
	if command.XP != member.XP*2 {
		t.Fatalf("command orders pay double: %+v vs %+v", command, member)
	}
}

func TestPickMissionsOneFromEachPower(t *testing.T) {
	var cands []MissionCandidate
	for faction := 1; faction <= 6; faction++ {
		for i := 0; i < 3; i++ {
			cands = append(cands, MissionCandidate{ID: string(rune('a'+faction)) + string(rune('0'+i)), FactionID: faction, Weight: 1})
		}
	}
	seed := 7
	rnd := func(k int) int { seed = (seed*1103515245 + 12345) & 0x7fffffff; return seed % k }
	got := PickMissions(cands, 4, true, rnd)
	if len(got) != 4 {
		t.Fatalf("expected 4, got %d", len(got))
	}
	seen := map[int]bool{}
	for _, m := range got {
		if seen[m.FactionID] {
			t.Fatalf("two missions from faction %d: %v", m.FactionID, got)
		}
		seen[m.FactionID] = true
	}
	// A member's pool is one faction: four different missions from it, no repeats.
	one := cands[:3]
	if got := PickMissions(one, 4, false, rnd); len(got) != 3 {
		t.Fatalf("can't give more missions than exist, got %d", len(got))
	}
}

func TestMostFollowed(t *testing.T) {
	if f, ok := MostFollowed(map[int]int{2: 3, 5: 7, 1: 2}); !ok || f != 5 {
		t.Fatalf("expected 5, got %d %v", f, ok)
	}
	if f, _ := MostFollowed(map[int]int{4: 3, 2: 3}); f != 2 {
		t.Fatalf("a tie goes to the lowest id, got %d", f)
	}
	if _, ok := MostFollowed(map[int]int{}); ok {
		t.Fatal("nothing finished, nobody to follow")
	}
}

func TestMissionTitles(t *testing.T) {
	cases := map[string]string{
		MissionTitle(GoalHarvest, "crystal", 3):          "Harvest 3 crystal",
		MissionTitle(GoalHarvest, "", 1):                 "Harvest a node",
		MissionTitle(GoalCraft, ItemCell, 2):             "Craft 2 Energy Cells",
		MissionTitle(GoalCraft, WeaponEmPulse, 1):        "Craft an EM Pulse",
		MissionTitle(GoalCraft, WeaponJammer, 1):         "Craft a Signal Jammer",
		MissionTitle(GoalBuildTether, "", 1):             "Build a Tether Node",
		MissionTitle(GoalAppointWarden, "", 1):           "Appoint a Warden",
		NoticeMissionDone("Signum", "Harvest 3 crystal"): "Signum: Harvest 3 crystal. Done. Claim your reward in ORDERS.",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
