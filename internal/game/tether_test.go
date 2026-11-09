package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

func tetherBase() TetherBuildInput {
	return TetherBuildInput{
		Settings: DefaultSettings(),
		Player:   Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 10, At: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)},
		Carried:  map[string]int64{"crystal": 80, "alloy": 60, "data": 20},
		Uploaded: map[string]int64{},
	}
}

func TestTetherBuildPaysCarriedFirst(t *testing.T) {
	in := tetherBase()
	in.Carried = map[string]int64{"crystal": 50, "alloy": 60, "data": 20}
	in.Uploaded = map[string]int64{"crystal": 40, "alloy": 500}
	pay, err := EvaluateTetherBuild(in)
	if err != nil {
		t.Fatalf("should be able to build: %v", err)
	}
	if pay.FromCarried["crystal"] != 50 || pay.FromUploaded["crystal"] != 30 {
		t.Fatalf("crystal should be 50 carried + 30 uploaded, got %+v", pay)
	}
	if pay.FromCarried["alloy"] != 60 || pay.FromUploaded["alloy"] != 0 {
		t.Fatalf("alloy should come only from carried, got %+v", pay)
	}
}

func TestTetherBuildRules(t *testing.T) {
	if _, err := EvaluateTetherBuild(tetherBase()); err != nil {
		t.Fatalf("exact cost should be enough: %v", err)
	}

	short := tetherBase()
	short.Carried["data"] = 19
	if _, err := EvaluateTetherBuild(short); err == nil || err.Code != apperr.TetherCost.Code {
		t.Fatalf("expected not enough resources, got %v", err)
	}

	full := tetherBase()
	full.ActiveOwned = full.Settings.TnMaxPerPlayer
	if _, err := EvaluateTetherBuild(full); err != apperr.TetherCap {
		t.Fatalf("expected cap reached, got %v", err)
	}

	weak := tetherBase()
	weak.Player.AccuracyM = 120
	if _, err := EvaluateTetherBuild(weak); err != apperr.HarvestGPSWeak {
		t.Fatalf("expected weak GPS, got %v", err)
	}

	teleport := tetherBase()
	teleport.LastAnyFix = &Fix{Lat: 4.0, Lon: 73.0, AccuracyM: 10, At: teleport.Player.At.Add(-time.Minute)}
	if _, err := EvaluateTetherBuild(teleport); err == nil || err.Code != apperr.HarvestImplausible.Code {
		t.Fatalf("expected implausible travel, got %v", err)
	}
}

func TestTetherHarvestSplitsEverything(t *testing.T) {
	s := DefaultSettings()
	h, ok := EvaluateTetherHarvest(s, 21, 1)
	if !ok || h.Amount != 21 || h.ToOwner+h.ToStock != 21 || h.ReserveCost != s.HarvestReserveCost {
		t.Fatalf("unexpected harvest %+v", h)
	}
	if _, ok := EvaluateTetherHarvest(s, 20, s.NodeDepletedReserve-0.01); ok {
		t.Fatal("a depleted node should give nothing")
	}
}

func TestCanUseTether(t *testing.T) {
	one, two := 1, 2
	cases := []struct {
		name          string
		tetherFaction *int
		player        int64
		playerFaction *int
		want          bool
	}{
		{"owner of a faction node", &one, 10, nil, true},
		{"anyone at a freelancer node", nil, 20, &two, true},
		{"same faction", &one, 20, &one, true},
		{"other faction", &one, 20, &two, false},
		{"freelancer at a faction node", &one, 20, nil, false},
	}
	for _, c := range cases {
		if got := CanUseTether(10, c.tetherFaction, c.player, c.playerFaction); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestCheckAtTether(t *testing.T) {
	s := DefaultSettings()
	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	near := Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 10, At: at}
	if _, err := CheckAtTether(s, near, nil, 4.1755, 73.5094); err != nil {
		t.Fatalf("should be in range: %v", err)
	}
	if _, err := CheckAtTether(s, near, nil, 4.1755, 73.5110); err != apperr.TetherTooFar {
		t.Fatalf("expected too far, got %v", err)
	}
	weak := near
	weak.AccuracyM = 120
	if _, err := CheckAtTether(s, weak, nil, 4.1755, 73.5094); err != apperr.HarvestGPSWeak {
		t.Fatalf("expected weak GPS, got %v", err)
	}
}

func TestSplitUpload(t *testing.T) {
	carried := map[string]int64{"crystal": 100, "alloy": 0, "data": 5}
	kept, fees := SplitUpload(carried, 0.05, false)
	if kept["crystal"] != 95 || fees["crystal"] != 5 {
		t.Fatalf("crystal should split 95/5, got %v %v", kept, fees)
	}
	if _, ok := kept["alloy"]; ok {
		t.Fatalf("empty resources should be left out, got %v", kept)
	}
	if kept["data"]+fees["data"] != 5 {
		t.Fatalf("nothing should be lost to rounding, got %v %v", kept, fees)
	}
	kept, fees = SplitUpload(carried, 0.05, true)
	if kept["crystal"] != 100 || len(fees) != 0 {
		t.Fatalf("the owner should pay nothing, got %v %v", kept, fees)
	}
}

func TestUpgradeRefundCoversEverythingSpent(t *testing.T) {
	s := DefaultSettings()
	spent := AddCosts(TetherBuildCost(s), TetherUpgradeCost(s))
	got := TetherRefund(spent, s.TnDismantleRefund)
	if got["crystal"] != 40 || got["alloy"] != 90 || got["data"] != 40 || got["credit"] != 20 {
		t.Fatalf("refund should match the design table, got %+v", got)
	}
}

func TestFreeSlots(t *testing.T) {
	got := FreeSlots([]int{0, 2}, 4)
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("expected slots 1 and 3, got %v", got)
	}
	if got := FreeSlots([]int{0}, 1); len(got) != 0 {
		t.Fatalf("a full tier 1 node has no free slot, got %v", got)
	}
}

func TestWithinTetherInfo(t *testing.T) {
	s := DefaultSettings()
	near := Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 5}
	if !WithinTetherInfo(s, near, 4.1755, 73.5095) {
		t.Fatal("about 22 m should be close enough")
	}
	if WithinTetherInfo(s, near, 4.1755, 73.5110) {
		t.Fatal("about 190 m should be too far")
	}
}

func TestRespawnPointStaysInTheRing(t *testing.T) {
	s := DefaultSettings()
	const lat, lon = 4.1755, 73.5093
	for b := 0.0; b < 1; b += 0.125 {
		for _, r := range []float64{0, 0.5, 1} {
			newLat, newLon := RespawnPoint(s, lat, lon, b, r)
			d := DistanceM(lat, lon, newLat, newLon)
			if d < s.NodeRespawnMinM-1 || d > s.NodeRespawnMaxM+1 {
				t.Fatalf("bearing %.3f, spread %.1f landed %.1f m away", b, r, d)
			}
		}
	}
}

func TestDecayEmptiesOverTheLifetime(t *testing.T) {
	s := DefaultSettings()
	if got := TetherDecayPerHour(s) * s.TnLifetimeHours; got != s.TnMaxHealth {
		t.Fatalf("decay over a lifetime should equal max health, got %v", got)
	}
	s.TnLifetimeHours = 0
	if TetherDecayPerHour(s) != 0 {
		t.Fatal("no lifetime should mean no decay, not a division by zero")
	}
}

func TestTakeDrop(t *testing.T) {
	contents := map[string]int64{"crystal": 30, "alloy": 50}
	taken, left := TakeDrop(contents, 60)
	if taken["alloy"] != 50 || taken["crystal"] != 10 || left["crystal"] != 20 || len(left) != 1 {
		t.Fatalf("expected all alloy and 10 crystal, got %v left %v", taken, left)
	}
	taken, left = TakeDrop(contents, 0)
	if len(taken) != 0 || left["alloy"] != 50 || left["crystal"] != 30 {
		t.Fatalf("a full player should take nothing, got %v left %v", taken, left)
	}
}

func TestCanAttackTether(t *testing.T) {
	s := DefaultSettings()
	one := 1
	cases := []struct {
		name    string
		attack  TetherAttack
		allowed bool
		loses   bool
	}{
		{"own node", TetherAttack{AttackerID: 1, OwnerID: 1}, false, false},
		{"faction player, owner above join level", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerFaction: &one, OwnerLevel: 21}, true, false},
		{"faction player, owner at join level", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerFaction: &one, OwnerLevel: 20}, false, false},
		{"freelancer within band", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerLevel: 15, OwnerLevel: 10}, true, false},
		{"freelancer outside band", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerLevel: 16, OwnerLevel: 10}, false, false},
		{"freelancer outside band, lower", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerLevel: 4, OwnerLevel: 10}, false, false},
		{"freelancer attacks a faction node", TetherAttack{AttackerID: 2, OwnerID: 1, TetherFaction: &one}, true, true},
		{"faction player attacks a faction node", TetherAttack{AttackerID: 2, OwnerID: 1, AttackerFaction: &one, TetherFaction: &one}, true, false},
	}
	for _, c := range cases {
		loses, err := CanAttackTether(s, c.attack)
		if (err == nil) != c.allowed || loses != c.loses {
			t.Errorf("%s: got loses=%v err=%v", c.name, loses, err)
		}
	}
}

func TestPickLinksTakesOneOfEach(t *testing.T) {
	candidates := []LinkCandidate{
		{1, "crystal"}, {2, "crystal"}, {3, "alloy"}, {4, "crystal"}, {5, "data"}, {6, "credit"}, {7, "alloy"},
	}
	got := PickLinks(candidates, nil, 4)
	want := []int64{1, 3, 5, 6}
	if len(got) != 4 {
		t.Fatalf("expected 4 links, got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected one of each resource %v, got %v", want, got)
		}
	}
}

func TestPickLinksSkipsResourcesAlreadyLinked(t *testing.T) {
	candidates := []LinkCandidate{{1, "crystal"}, {2, "alloy"}, {3, "data"}, {4, "credit"}}
	got := PickLinks(candidates, []string{"crystal"}, 3)
	if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Fatalf("an upgrade should add the missing resources, got %v", got)
	}
}

func TestPickLinksFillsWithDuplicatesWhenATypeIsMissing(t *testing.T) {
	candidates := []LinkCandidate{{1, "crystal"}, {2, "crystal"}, {3, "alloy"}}
	got := PickLinks(candidates, nil, 3)
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 2 {
		t.Fatalf("expected distinct types first, then a duplicate, got %v", got)
	}
	if got := PickLinks(candidates, nil, 9); len(got) != 3 {
		t.Fatalf("can't link more than exist, got %v", got)
	}
}

func TestTetherRefund(t *testing.T) {
	got := TetherRefund(map[string]int64{"crystal": 80, "alloy": 60, "data": 21}, 0.5)
	if got["crystal"] != 40 || got["alloy"] != 30 || got["data"] != 10 {
		t.Fatalf("unexpected refund %+v", got)
	}
}
