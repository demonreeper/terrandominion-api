package game

import "testing"

func TestFavourPerks(t *testing.T) {
	s := DefaultSettings() // FavourPerkPct 40
	if got := FavourPerks(s, nil); got != NoPerks() {
		t.Errorf("no missions finished: %+v", got)
	}
	// Signum's +25% crystal at 40% is +10%; Tohu's +30% drones and +25% carry are +12% and +10%.
	p := FavourPerks(s, []int{PowerSignum, PowerTohu})
	if p.CrystalMult != 1.10 || p.DroneSpeedMult != 1.12 || p.CarryMult != 1.10 {
		t.Errorf("favour %+v", p)
	}
	if p.UpgradeCostMult != 1 || p.ScanRangeMult != 1 {
		t.Errorf("a power whose mission was not finished gives nothing: %+v", p)
	}
	full := FactionPerks(s, PowerSignum)
	if p.CrystalMult >= full.CrystalMult {
		t.Error("the favour must be weaker than the member's perk")
	}
	s.FavourPerkPct = 0
	if got := FavourPerks(s, []int{PowerSignum}); got != NoPerks() {
		t.Errorf("favour switched off: %+v", got)
	}
}

func TestFavourPerkLines(t *testing.T) {
	s := DefaultSettings()
	lines := FavourPerkLines(s, []int{PowerSignum, PowerSignum, PowerIshara})
	if len(lines) != 3 { // Signum once, Ishāra's two
		t.Fatalf("lines %+v", lines)
	}
	if lines[0].Tag != "+10% SIGNUM" || PerkTagIn(lines, PerkCrystal) != "+10% SIGNUM" {
		t.Errorf("tag %q", lines[0].Tag)
	}
	if PerkTagIn(lines, PerkDroneSpeed) != "" {
		t.Error("no tag for a perk not in the favour")
	}
	// A small perk never rounds away to nothing.
	s.PerkSignumCrystalPct = 1
	if got := FavourPerkLines(s, []int{PowerSignum}); len(got) != 1 || got[0].Tag != "+1% SIGNUM" {
		t.Errorf("small perk %+v", got)
	}
}

func TestMissionCounts(t *testing.T) {
	s := DefaultSettings() // MissionsPerDay 6, MemberMissionsPerDay 1
	if c, own := MissionCounts(s, false, 0); c != 0 || own != 6 {
		t.Errorf("freelancer %d+%d", c, own)
	}
	if c, own := MissionCounts(s, true, RankMember); c != 0 || own != 1 {
		t.Errorf("member %d+%d", c, own)
	}
	if c, own := MissionCounts(s, true, RankWarden); c != 1 || own != 1 {
		t.Errorf("warden %d+%d", c, own)
	}
}
