package game

import "testing"

func TestCheckCaptainMission(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		name    string
		post    CaptainMissionPost
		damaged bool
		want    string
	}{
		{"give", CaptainMissionPost{GoalBaseGive, "crystal", 300}, false, CaptainMissionOK},
		{"give at the cap", CaptainMissionPost{GoalBaseGive, "alloy", s.CaptainMissionGiveMax}, false, CaptainMissionOK},
		{"give over the cap", CaptainMissionPost{GoalBaseGive, "alloy", s.CaptainMissionGiveMax + 1}, false, CaptainMissionBadAmount},
		{"give nothing", CaptainMissionPost{GoalBaseGive, "data", 0}, false, CaptainMissionBadAmount},
		{"give an unknown resource", CaptainMissionPost{GoalBaseGive, "gold", 10}, false, CaptainMissionBadResource},
		{"give without a resource", CaptainMissionPost{GoalBaseGive, "", 10}, false, CaptainMissionBadResource},
		{"stand", CaptainMissionPost{GoalBaseStand, "", 10}, false, CaptainMissionOK},
		{"stand too long", CaptainMissionPost{GoalBaseStand, "", s.CaptainMissionStandMaxMin + 1}, false, CaptainMissionBadAmount},
		{"stand with a resource", CaptainMissionPost{GoalBaseStand, "alloy", 10}, false, CaptainMissionBadResource},
		{"repair a damaged base", CaptainMissionPost{GoalBaseRepair, "", 1}, true, CaptainMissionOK},
		{"repair a whole base", CaptainMissionPost{GoalBaseRepair, "", 1}, false, CaptainMissionNothingToFix},
		{"repair twice", CaptainMissionPost{GoalBaseRepair, "", 2}, true, CaptainMissionBadAmount},
		{"another kind", CaptainMissionPost{GoalHarvest, "crystal", 3}, false, CaptainMissionBadKind},
	}
	for _, c := range cases {
		if got := CheckCaptainMission(c.post, c.damaged, s); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCaptainMissionsLeft(t *testing.T) {
	s := DefaultSettings()
	if s.CaptainMissionsPerDay != 2 {
		t.Fatalf("CaptainMissionsPerDay default %d, want 2", s.CaptainMissionsPerDay)
	}
	for posted, want := range []int{2, 1, 0, 0} {
		if got := CaptainMissionsLeft(posted, s); got != want {
			t.Errorf("posted %d: left %d, want %d", posted, got, want)
		}
	}
}

func TestCaptainMissionTitles(t *testing.T) {
	cases := []struct {
		goal, key string
		count     int
		base      string
		want      string
	}{
		{GoalBaseGive, "crystal", 300, "Coral Spire", "Give 300 crystal at Coral Spire"},
		{GoalBaseStand, "", 10, "Coral Spire", "Stand at Coral Spire for 10 minutes"},
		{GoalBaseStand, "", 1, "Coral Spire", "Stand at Coral Spire for a minute"},
		{GoalBaseRepair, "", 1, "Coral Spire", "Repair a module at Coral Spire"},
		{GoalBaseGive, "alloy", 50, "", "Give 50 alloy at your base"},
	}
	for _, c := range cases {
		if got := CaptainMissionTitle(c.goal, c.key, c.count, c.base); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
	if got := NoticeCaptainMission("Reef-Runner", "Give 300 crystal at Coral Spire"); got !=
		"Reef-Runner posted a squad mission: Give 300 crystal at Coral Spire. It is in ORDERS." {
		t.Errorf("notice: %q", got)
	}
	if _, ok := PushRuleFor("captain_mission"); !ok {
		t.Error("captain_mission has no push rule, so it would not show in ALERTS")
	}
}
