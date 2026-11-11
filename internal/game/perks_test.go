package game

import (
	"reflect"
	"testing"
	"time"
)

func TestFreelancerHasNoPerks(t *testing.T) {
	s := DefaultSettings()
	if got := FactionPerks(s, 0); got != NoPerks() {
		t.Errorf("freelancer perks = %+v, want none", got)
	}
	if got := FactionPerks(s, 99); got != NoPerks() {
		t.Errorf("unknown faction perks = %+v, want none", got)
	}
	if lines := FactionPerkLines(s, 0); len(lines) != 0 {
		t.Errorf("freelancer perk lines = %v", lines)
	}
}

// Each faction changes its own systems and nothing else, with the numbers the plan confirmed.
func TestFactionPerks(t *testing.T) {
	s := DefaultSettings()
	none := NoPerks()
	cases := []struct {
		faction int
		want    Perks
	}{
		{PowerSignum, func() Perks { p := none; p.CrystalMult = 1.25; return p }()},
		{PowerShingo, func() Perks { p := none; p.UpgradeCostMult = 0.75; return p }()},
		{PowerSema, func() Perks { p := none; p.ScanRangeMult = 2; p.RareRangeMult = 2; return p }()},
		{PowerIshara, func() Perks { p := none; p.BuildCostMult = 0.8; p.MissionCreditMult = 1.25; return p }()},
		{PowerTohu, func() Perks { p := none; p.DroneSpeedMult = 1.3; p.CarryMult = 1.25; return p }()},
		{PowerMerki, func() Perks { p := none; p.DamageTakenMult = 0.85; p.InterceptBonus = 0.10; return p }()},
	}
	for _, c := range cases {
		got := FactionPerks(s, c.faction)
		if !perksClose(got, c.want) {
			t.Errorf("faction %d: perks = %+v, want %+v", c.faction, got, c.want)
		}
	}
}

func perksClose(a, b Perks) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for i := 0; i < va.NumField(); i++ {
		if d := va.Field(i).Float() - vb.Field(i).Float(); d > 1e-9 || d < -1e-9 {
			return false
		}
	}
	return true
}

// The perk stops the moment the player leaves: the same action with no faction has no bonus.
func TestPerksApplyToMembersOnly(t *testing.T) {
	s := DefaultSettings()

	// Signum: +25% crystal.
	if got := PerkAmount(12, FactionPerks(s, PowerSignum).CrystalMult); got != 15 {
		t.Errorf("member crystal = %d, want 15", got)
	}
	if got := PerkAmount(12, FactionPerks(s, 0).CrystalMult); got != 12 {
		t.Errorf("freelancer crystal = %d, want 12", got)
	}

	// Shingō: Tether Node upgrades cost 25% less. Ishāra: builds cost 20% less.
	upgrade := TetherUpgradeCost(SettingsWithPerks(s, FactionPerks(s, PowerShingo)))
	if upgrade["alloy"] != 90 || upgrade["data"] != 45 || upgrade["credit"] != 30 {
		t.Errorf("member upgrade cost = %v", upgrade)
	}
	if got := TetherUpgradeCost(SettingsWithPerks(s, FactionPerks(s, 0))); !reflect.DeepEqual(got, TetherUpgradeCost(s)) {
		t.Errorf("freelancer upgrade cost = %v", got)
	}
	build := TetherBuildCost(SettingsWithPerks(s, FactionPerks(s, PowerIshara)))
	if build["crystal"] != 64 || build["alloy"] != 48 || build["data"] != 16 {
		t.Errorf("member build cost = %v", build)
	}
	// A build discount is not an upgrade discount, and the other way round.
	if got := TetherUpgradeCost(SettingsWithPerks(s, FactionPerks(s, PowerIshara))); !reflect.DeepEqual(got, TetherUpgradeCost(s)) {
		t.Errorf("Ishāra should not discount upgrades: %v", got)
	}
	if got := TetherBuildCost(SettingsWithPerks(s, FactionPerks(s, PowerShingo))); !reflect.DeepEqual(got, TetherBuildCost(s)) {
		t.Errorf("Shingō should not discount builds: %v", got)
	}

	// Ishāra: +25% mission credit.
	if got := PerkAmount(8, FactionPerks(s, PowerIshara).MissionCreditMult); got != 10 {
		t.Errorf("member mission credit = %d, want 10", got)
	}

	// Tohu: drones fly 30% faster (a 1300 m flight takes 1000 m's time), carry +25%.
	member := DroneFlight(SettingsWithPerks(s, FactionPerks(s, PowerTohu)), 1300)
	freelancer := DroneFlight(s, 1000)
	if d := member - freelancer; d > time.Millisecond || d < -time.Millisecond {
		t.Errorf("member flight over 1300 m = %v, freelancer over 1000 m = %v", member, freelancer)
	}
	if DroneFlight(SettingsWithPerks(s, FactionPerks(s, 0)), 1300) != DroneFlight(s, 1300) {
		t.Error("a freelancer's drone should fly at the plain speed")
	}

	// Merki: own Tether Nodes take 15% less damage, intercepts stop 10% more.
	merki := FactionPerks(s, PowerMerki)
	if got := HitDamage(40*merki.DamageTakenMult, 0, 0); got < 33.99 || got > 34.01 {
		t.Errorf("member damage taken = %v, want 34", got)
	}
	if got := PerkInterceptShare(0.75, merki.InterceptBonus); got < 0.849 || got > 0.851 {
		t.Errorf("member intercept share = %v, want 0.85", got)
	}
	if got := PerkInterceptShare(1.0, merki.InterceptBonus); got != 1 {
		t.Errorf("an intercept never stops more than everything: %v", got)
	}
	if got := PerkInterceptShare(0.75, FactionPerks(s, 0).InterceptBonus); got != 0.75 {
		t.Errorf("freelancer intercept share = %v", got)
	}
}

func TestPerkNumbersAreSettings(t *testing.T) {
	s := SettingsFrom(map[string]float64{"PerkSignumCrystalPct": 50, "PerkTohuCarryPct": 0, "PerkSemaScanPct": 50, "PerkSemaRarePct": 0})
	if got := FactionPerks(s, PowerSignum).CrystalMult; got != 1.5 {
		t.Errorf("tuned crystal mult = %v", got)
	}
	// A perk tuned to 0 is off, and is not listed.
	if got := FactionPerks(s, PowerTohu).CarryMult; got != 1 {
		t.Errorf("carry perk tuned to 0 = %v", got)
	}
	for _, line := range FactionPerkLines(s, PowerTohu) {
		if line.Key == PerkCarry {
			t.Errorf("a perk tuned to 0 should not be listed: %+v", line)
		}
	}
	// Rare nodes are never seen from nearer than everything else.
	sema := FactionPerks(s, PowerSema)
	if sema.ScanRangeMult != 1.5 || sema.RareRangeMult != 1.5 {
		t.Errorf("Sēma perks = %+v", sema)
	}
}

func TestFactionPerkLines(t *testing.T) {
	s := DefaultSettings()
	lines := FactionPerkLines(s, PowerSignum)
	if len(lines) != 1 || lines[0].Key != PerkCrystal ||
		lines[0].Line != "+25% CRYSTAL FROM HARVESTS AND TETHER NODES" || lines[0].Tag != "+25% SIGNUM" {
		t.Errorf("Signum lines = %+v", lines)
	}
	if got := PerkTag(s, PowerIshara, PerkBuildCost); got != "-20% ISHĀRA" {
		t.Errorf("Ishāra build tag = %q", got)
	}
	if got := PerkTag(s, PowerIshara, PerkCrystal); got != "" {
		t.Errorf("Ishāra has no crystal perk, got %q", got)
	}
	for _, faction := range Powers {
		if len(FactionPerkLines(s, faction)) == 0 {
			t.Errorf("faction %d has no perk lines", faction)
		}
	}
}

func TestPerkCostRoundsUp(t *testing.T) {
	got := PerkCost(map[string]int64{"alloy": 5, "data": 1}, 0.75)
	if got["alloy"] != 4 || got["data"] != 1 {
		t.Errorf("PerkCost = %v", got)
	}
	same := map[string]int64{"alloy": 5}
	if got := PerkCost(same, 1); !reflect.DeepEqual(got, same) {
		t.Errorf("no perk should leave the cost alone: %v", got)
	}
}
