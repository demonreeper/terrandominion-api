package game

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// isErr says whether a rule returned (a copy of) the numbered error.
func isErr(err, want *apperr.Error) bool {
	return err != nil && err.Code == want.Code
}

func dataJSON(t *testing.T, e *apperr.Error) string {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestShortError(t *testing.T) {
	e := ShortError(map[string]int64{"data": 10, "alloy": 30})
	if e.Code != apperr.TetherCost.Code {
		t.Fatalf("code %d", e.Code)
	}
	if e.Message != "You need 30 more alloy and 10 more data." {
		t.Fatalf("message %q", e.Message)
	}
	want := `{"code":1602,"message":"You need 30 more alloy and 10 more data.","data":{"short":{"alloy":30,"data":10}}}`
	if got := dataJSON(t, e); got != want {
		t.Fatalf("json %s", got)
	}
	if apperr.TetherCost.Data != nil {
		t.Fatal("the shared error was changed")
	}
}

func TestPayCostShort(t *testing.T) {
	cost := map[string]int64{"alloy": 100, "data": 20, "crystal": 5}
	_, err := PayCost(cost, map[string]int64{"alloy": 50, "crystal": 5}, map[string]int64{"alloy": 20, "data": 15})
	if !isErr(err, apperr.TetherCost) {
		t.Fatalf("got %v", err)
	}
	short := err.Data["short"].(map[string]int64)
	if len(short) != 2 || short["alloy"] != 30 || short["data"] != 5 {
		t.Fatalf("short %v", short)
	}
	if err.Message != "You need 30 more alloy and 5 more data." {
		t.Fatalf("message %q", err.Message)
	}
	if _, err := PayCost(cost, map[string]int64{"alloy": 100, "crystal": 5}, map[string]int64{"data": 20}); err != nil {
		t.Fatalf("affordable cost refused: %v", err)
	}
}

func TestRangeError(t *testing.T) {
	e := RangeError(apperr.OutOfRange, 61.2, 40, "")
	if e.Message != "Move within 40 m. You are 62 m away." {
		t.Fatalf("message %q", e.Message)
	}
	if e.Data["distanceM"] != 62 || e.Data["rangeM"] != 40 {
		t.Fatalf("data %v", e.Data)
	}
	if !errors.Is(e, apperr.OutOfRange) || errors.Is(e, apperr.TetherTooFar) {
		t.Fatal("errors.Is should match on the code")
	}
	far := RangeError(apperr.TetherTooFar, 90, 40, "the Tether Node")
	if far.Message != "Move within 40 m of the Tether Node. You are 90 m away." {
		t.Fatalf("message %q", far.Message)
	}

	s := DefaultSettings()
	_, ruleErr := CheckInRange(s, Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 5}, nil, 4.1755, 73.5093+0.00055, 40)
	if !isErr(ruleErr, apperr.OutOfRange) || ruleErr.Data["rangeM"] != 40 {
		t.Fatalf("got %v", ruleErr)
	}
}

func TestSmallErrors(t *testing.T) {
	cells := CellsError(2, 1)
	if cells.Message != "You need 2 energy cells. You have 1." || cells.Data["need"] != 2 || cells.Data["have"] != 1 {
		t.Fatalf("cells %q %v", cells.Message, cells.Data)
	}
	if one := CellsError(1, 0); one.Message != "You need 1 energy cell. You have 0." {
		t.Fatalf("one cell %q", one.Message)
	}
	level := LevelError(5, 3)
	if level.Message != "You need level 5 for that. You are level 3." || level.Data["needLevel"] != 5 || level.Data["level"] != 3 {
		t.Fatalf("level %q %v", level.Message, level.Data)
	}
	cool := CooldownError(250)
	if cool.Message != "This node is recharging for you. Ready in 4 min 10 s." || cool.Data["readyInSec"] != 250 {
		t.Fatalf("cooldown %q %v", cool.Message, cool.Data)
	}
	if item := ItemError(WeaponEmPulse, "EM Pulse"); item.Message != "You need an EM Pulse. Craft one in Inventory." {
		t.Fatalf("item %q", item.Message)
	}
	if cap := CapError(3); cap.Message != "You already own 3 Tether Nodes, the most you can." || cap.Data["max"] != 3 {
		t.Fatalf("cap %q", cap.Message)
	}
	if gps := GPSWeakError(DefaultSettings(), 62.3); gps.Data["accuracyM"] != 63 || gps.Data["maxAccuracyM"] != 50 {
		t.Fatalf("gps %v", gps.Data)
	}
	if spacing := SpacingError(10); spacing.Message != "Too close to another Tether Node. Build at least 10 m from it." {
		t.Fatalf("spacing %q", spacing.Message)
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                              "0 s",
		1500 * time.Millisecond:        "2 s",
		45 * time.Second:               "45 s",
		10 * time.Minute:               "10 min",
		250 * time.Second:              "4 min 10 s",
		2 * time.Hour:                  "2 h",
		2*time.Hour + 5*time.Minute:    "2 h 5 min",
		2*time.Hour + 5*time.Second:    "2 h",
		-3 * time.Second:               "0 s",
		time.Hour + 59*time.Minute + 1: "1 h 59 min",
	} {
		if got := FormatDuration(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}

func TestHarvestReadyIn(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if got := HarvestReadyIn(s, nil, now); got != 0 {
		t.Fatalf("never harvested: %d", got)
	}
	if got := HarvestReadyIn(s, &now, now); got != s.HarvestCooldownSec {
		t.Fatalf("just harvested: %d", got)
	}
	last := now.Add(-100*time.Second - 300*time.Millisecond)
	if got := HarvestReadyIn(s, &last, now); got != 200 {
		t.Fatalf("100.3 s ago: %d", got)
	}
	old := now.Add(-time.Hour)
	if got := HarvestReadyIn(s, &old, now); got != 0 {
		t.Fatalf("long ago: %d", got)
	}

	in := harvestBase()
	recent := in.Player.At.Add(-60 * time.Second)
	in.LastAtNode = &recent
	_, err := EvaluateHarvest(in)
	if !isErr(err, apperr.HarvestCooldown) || err.Data["readyInSec"] != 240 {
		t.Fatalf("got %v %v", err, err.Data)
	}
}

func TestNoticeSeverity(t *testing.T) {
	want := map[string]string{
		"attacked": SeverityDanger, "destroyed": SeverityDanger, "spike": SeverityDanger,
		"drone_inbound": SeverityDanger, "spike_fired": SeverityDanger,
		"jammed": SeverityWarning, "contest": SeverityWarning, "contest_lost": SeverityWarning,
		"drone_intercepted": SeverityWarning,
		"spike_result":      SeverityInfo, "drone_result": SeverityInfo, "defused": SeverityInfo, "contest_won": SeverityInfo,
		"drone_launched": SeverityInfo, "shield_fitted": SeverityInfo,
		"mission_done": SeverityInfo, "invitation": SeverityInfo, "warden": SeverityInfo, "call": SeverityInfo,
		"drop_reclaimed": SeverityInfo, "report_reviewed": SeverityInfo,
		"job_done": SeverityInfo, "job_paid": SeverityInfo, "job_ended": SeverityInfo,
		"friend_request": SeverityInfo, "friend_accepted": SeverityInfo,
		"base_proposal": SeverityWarning, "base_approved": SeverityInfo, "base_declined": SeverityWarning,
		"base_expired":  SeverityWarning,
		"base_attacked": SeverityDanger, "base_tier_lost": SeverityDanger, "base_destroyed": SeverityDanger,
		"base_fading": SeverityWarning, "base_raised": SeverityInfo, "base_tier_up": SeverityInfo,
		"rank_up": SeverityInfo, "nomad_ask": SeverityWarning, "nomad_moved": SeverityInfo,
		"nomad_declined": SeverityWarning, "nomad_expired": SeverityWarning,
		"captain_mission": SeverityInfo,
		"seat_changed":    SeverityWarning, "vote_open": SeverityWarning, "vote_ended": SeverityInfo,
	}
	kinds := NoticeKinds()
	sort.Strings(kinds)
	if len(kinds) != len(want) {
		t.Fatalf("kinds %v", kinds)
	}
	for kind, severity := range want {
		if got := NoticeSeverity(kind); got != severity {
			t.Errorf("%s: got %s, want %s", kind, got, severity)
		}
	}
	if NoticeSeverity("something_new") != SeverityInfo {
		t.Fatal("unknown kinds should be info")
	}
	// Every kind that can push has a severity.
	for kind := range pushRules {
		if _, ok := noticeSeverity[kind]; !ok {
			t.Errorf("%s pushes but has no severity", kind)
		}
	}
}

func TestNoticeBodies(t *testing.T) {
	cases := map[string]string{
		NoticeHit("Aishath", "EM Pulse", "Crystal Seam K-27", 25, 75):  "Aishath's EM Pulse hit your Tether Node near Crystal Seam K-27 for 25 damage. 75 health left.",
		NoticeHit("Aishath", "drone", "", 15, 40):                      "Aishath's drone hit your Tether Node for 15 damage. 40 health left.",
		NoticeDestroyed("Aishath", "spike", "Alloy Cache B-02"):        "Aishath's spike destroyed your Tether Node near Alloy Cache B-02.",
		NoticeJammed("Aishath", "Data Relay C-11", 30*time.Minute):     "Aishath jammed your Tether Node near Data Relay C-11. It collects nothing for 30 min.",
		NoticeSpikePlanted("Aishath", "X", 10*time.Minute, 40):         "Aishath planted a spike on your Tether Node near X. Defuse it within 10 min or it deals 40 damage.",
		NoticeDroneInbound("Aishath", "X", 45*time.Second, 15):         "Aishath's drone is inbound to your Tether Node near X. It lands in 45 s for 15 damage.",
		NoticeResult("drone", "Aishath", "", 15, 40, false):            "Your drone hit Aishath's Tether Node for 15 damage. 40 health left.",
		NoticeResult("spike", "Aishath", "X", 40, 0, true):             "Your spike destroyed Aishath's Tether Node near X.",
		NoticeDroneLaunched("Aishath", "X", 20*time.Second, 15):        "Your drone is away to Aishath's Tether Node near X. It lands in 20 s for 15 damage.",
		NoticeInterceptedDefender("Aishath", "X", 0.75):                "You intercepted Aishath's drone at your Tether Node near X. 75% of its damage was stopped.",
		NoticeInterceptedDefender("Aishath", "", 1):                    "You shot down Aishath's drone at your Tether Node.",
		NoticeInterceptedAttacker("Aishath", "X", 0.5):                 "Your drone was intercepted at Aishath's Tether Node near X. 50% of its damage was stopped.",
		NoticeInterceptedAttacker("Aishath", "X", 1):                   "Your drone was shot down at Aishath's Tether Node near X.",
		NoticeShieldFitted(2, "X"):                                     "Shield Mk II fitted to your Tether Node near X.",
		NoticeDefused("Aishath", "X"):                                  "Aishath defused your spike on their Tether Node near X.",
		NoticeContest("Aishath", "Crystal Seam K-27", 40, time.Minute): "Aishath is contesting Crystal Seam K-27. Be within 40 m in the next 1 min to defend it.",
		NoticeContestWon("X", "Aishath", 12, 8, 3):                     "You won the contest for X against Aishath, 12 to 8. Its next 3 harvests are yours.",
		NoticeContestWon("X", "Aishath", 12, 8, 1):                     "You won the contest for X against Aishath, 12 to 8. Its next 1 harvest is yours.",
		NoticeContestLost("X", "Aishath", 8, 12):                       "You lost the contest for X to Aishath, 8 to 12.",
		NoticeHit("", "EM Pulse", "", 25, 0):                           "Someone's EM Pulse hit your Tether Node for 25 damage. 0 health left.",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	}
}

func TestMoreUrgent(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	items := []struct {
		name string
		u    TetherUrgency
	}{
		{"ended-old", TetherUrgency{Ended: true, EndedAt: now.Add(-50 * time.Minute)}},
		{"quiet-late", TetherUrgency{ExpiresAt: now.Add(40 * time.Hour)}},
		{"jammed", TetherUrgency{Jammed: true, ExpiresAt: now.Add(30 * time.Hour)}},
		{"ended-new", TetherUrgency{Ended: true, EndedAt: now.Add(-5 * time.Minute)}},
		{"spike", TetherUrgency{Threatened: true, Jammed: true, ExpiresAt: now.Add(45 * time.Hour)}},
		{"quiet-soon", TetherUrgency{ExpiresAt: now.Add(2 * time.Hour)}},
		{"attacked", TetherUrgency{UnderAttack: true, ExpiresAt: now.Add(47 * time.Hour)}},
	}
	sort.SliceStable(items, func(i, j int) bool { return MoreUrgent(items[i].u, items[j].u) })
	want := []string{"attacked", "spike", "jammed", "quiet-soon", "quiet-late", "ended-new", "ended-old"}
	for i, name := range want {
		if items[i].name != name {
			t.Fatalf("position %d: got %s, want %s", i, items[i].name, name)
		}
	}
}
