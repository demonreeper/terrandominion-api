package game

import (
	"math"
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// metresNorth moves a latitude north by m metres (near the equator, close enough for these tests).
func metresNorth(lat, m float64) float64 { return lat + m/111195.0 }

func goodSpot() BaseSpot {
	return BaseSpot{
		Lat: 4.175, Lon: 73.51, FactionID: 1,
		Map: SpotMap{OnLand: true, NearWalkway: true},
		Cap: 2,
	}
}

func TestBaseSlotLayout(t *testing.T) {
	s := DefaultSettings()
	want := map[int][3]int{1: {4, 0, 0}, 2: {8, 0, 0}, 3: {8, 4, 0}, 4: {8, 4, 6}, 5: {8, 4, 12}}
	for tier, rings := range want {
		got := [3]int{}
		seen := map[BaseSlot]bool{}
		for _, sl := range BaseSlotLayout(tier, s) {
			if seen[sl] {
				t.Fatalf("tier %d: slot %+v twice", tier, sl)
			}
			seen[sl] = true
			got[sl.Ring-1]++
		}
		if got != rings {
			t.Errorf("tier %d: rings %v, want %v", tier, got, rings)
		}
	}
	// A higher tier keeps every slot of the lower one.
	low := BaseSlotLayout(3, s)
	high := BaseSlotLayout(4, s)
	for i := range low {
		if low[i] != high[i] {
			t.Fatalf("slot %d moved from %+v to %+v", i, low[i], high[i])
		}
	}
}

func TestBaseTierNumbers(t *testing.T) {
	s := DefaultSettings()
	if BaseZoneM(1, s) != 25 || BaseZoneM(5, s) != 100 || BaseCoreHealth(3, s) != 3500 || BaseSlotCount(4, s) != 18 {
		t.Fatal("tier table differs from the plan")
	}
	if BaseTierName(2) != "Outpost" || ValidBaseTier(0) || ValidBaseTier(6) {
		t.Fatal("tier names")
	}
}

func TestBaseCap(t *testing.T) {
	s := DefaultSettings()
	for _, c := range []struct{ members, want int }{{0, 2}, {49, 2}, {50, 3}, {150, 4}, {300, 5}, {10000, 6}} {
		if got := BaseCap(c.members, s); got != c.want {
			t.Errorf("BaseCap(%d) = %d, want %d", c.members, got, c.want)
		}
	}
	s.BaseCapStep2Players = 0
	if BaseCap(10000, s) != 5 {
		t.Error("a step of 0 should be off")
	}
}

func TestCanProposeBase(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	ok := Proposer{InFaction: true, Rank: RankWarden, HomeRegion: "MV", HereRegion: "MV", AccuracyM: 8, BasesEnabled: true}
	if err := CanProposeBase(ok, now, s); err != nil {
		t.Fatalf("a Warden at home: %v", err)
	}
	cases := []struct {
		name string
		edit func(*Proposer)
		want *apperr.Error
	}{
		{"off", func(p *Proposer) { p.BasesEnabled = false }, apperr.BasesOff},
		{"freelancer", func(p *Proposer) { p.InFaction = false }, apperr.NotAMember},
		{"member", func(p *Proposer) { p.Rank = RankMember }, apperr.BaseRankTooLow},
		{"abroad", func(p *Proposer) { p.HereRegion = "XA" }, apperr.BaseNotHomeRegion},
		{"proposals off", func(p *Proposer) { p.ProposalsOff = true }, apperr.BaseProposalsOff},
		{"poor fix", func(p *Proposer) { p.AccuracyM = 25 }, apperr.BaseAccuracy},
		{"open flag", func(p *Proposer) { p.HasOpenProposal = true }, apperr.BaseProposalOpen},
		{"cooldown", func(p *Proposer) { p.CooldownUntil = now.Add(time.Hour) }, apperr.BaseProposalCooldown},
	}
	for _, c := range cases {
		p := ok
		c.edit(&p)
		if err := CanProposeBase(p, now, s); err == nil || err.Code != c.want.Code {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	lead := ok
	lead.Rank = RankLead
	if CanProposeBase(lead, now, s) != nil || !BaseCanApprove(RankRegionalCommander) || BaseCanApprove(RankLead) {
		t.Error("lead proposes, only the Commander approves")
	}
	if !BaseCooldownUntil(now, s).Equal(now.Add(24 * time.Hour)) {
		t.Error("cooldown is 24 h")
	}
}

func TestCheckBaseSpot(t *testing.T) {
	s := DefaultSettings()
	if e := CheckBaseSpot(goodSpot(), s); e != nil {
		t.Fatalf("empty ground: %v", e.Err)
	}
	near := func(faction int, m, zone float64) NearBase {
		sp := goodSpot()
		return NearBase{ID: 9, FactionID: faction, Name: "IRONROOT", Lat: metresNorth(sp.Lat, m), Lon: sp.Lon, ZoneM: zone}
	}
	cases := []struct {
		name string
		edit func(*BaseSpot)
		want *apperr.Error
	}{
		{"water", func(sp *BaseSpot) { sp.Map.OnLand = false }, apperr.BaseSpotTerrain},
		{"building", func(sp *BaseSpot) { sp.Map.InBlocked = true }, apperr.BaseSpotTerrain},
		{"no walkway", func(sp *BaseSpot) { sp.Map.NearWalkway = false }, apperr.BaseSpotTerrain},
		{"school", func(sp *BaseSpot) { sp.Map.NearSensitive = true }, apperr.BaseSpotSensitive},
		{"own base 2 km", func(sp *BaseSpot) { sp.Bases = []NearBase{near(1, 2000, 25)} }, apperr.BaseSpacingSame},
		{"other base 140 m", func(sp *BaseSpot) { sp.Bases = []NearBase{near(2, 140, 25)} }, apperr.BaseSpacingOther},
		// 160 m clears the 150 m centre rule, but a 110 m zone + 30 m gap + the Spire's 25 m is 165 m.
		{"big zone", func(sp *BaseSpot) { sp.Bases = []NearBase{near(2, 160, 110)} }, apperr.BaseZoneTooClose},
		{"other flag", func(sp *BaseSpot) {
			sp.Flags = []NearFlag{{ID: 3, FactionID: 2, Lat: metresNorth(sp.Lat, 70), Lon: sp.Lon}}
		}, apperr.BaseFlagHeld},
		{"cap", func(sp *BaseSpot) { sp.Held = 2 }, apperr.BaseCapReached},
	}
	for _, c := range cases {
		sp := goodSpot()
		c.edit(&sp)
		e := CheckBaseSpot(sp, s)
		if e == nil || e.Err.Code != c.want.Code {
			t.Errorf("%s: got %v, want %v", c.name, e, c.want)
		}
	}

	// Things that don't block: own faction's flag, other base far enough, itself, the flag being approved.
	sp := goodSpot()
	sp.Flags = []NearFlag{{ID: 3, FactionID: 1, Lat: metresNorth(sp.Lat, 10), Lon: sp.Lon},
		{ID: 4, FactionID: 2, Lat: metresNorth(sp.Lat, 81), Lon: sp.Lon}}
	sp.Bases = []NearBase{near(2, 200, 40), near(1, 3100, 25)}
	if e := CheckBaseSpot(sp, s); e != nil {
		t.Fatalf("clear ground refused: %v", e.Err)
	}
	sp = goodSpot()
	sp.SelfFlagID = 4
	sp.Flags = []NearFlag{{ID: 4, FactionID: 2, Lat: sp.Lat, Lon: sp.Lon}}
	if e := CheckBaseSpot(sp, s); e != nil {
		t.Fatalf("a flag blocks its own approval: %v", e.Err)
	}
}

func TestCheckBaseGrowth(t *testing.T) {
	s := DefaultSettings()
	sp := goodSpot()
	sp.SelfID = 1
	// A neighbour 140 m away with a 40 m zone: room for 140 - 40 - 30 = 70 m.
	sp.Bases = []NearBase{{ID: 1, FactionID: 1, Lat: sp.Lat, Lon: sp.Lon, ZoneM: 40},
		{ID: 2, FactionID: 2, Name: "GLASS MERIDIAN", Lat: metresNorth(sp.Lat, 140), Lon: sp.Lon, ZoneM: 40}}
	if e := CheckBaseGrowth(sp, 3, s); e != nil {
		t.Fatalf("60 m zone fits: %v", e.Err)
	}
	e := CheckBaseGrowth(sp, 4, s)
	if e == nil || e.Err.Code != apperr.BaseZoneTooClose.Code || e.Near != "GLASS MERIDIAN" {
		t.Fatalf("80 m zone should stop at the border: %+v", e)
	}
	if room := BaseZoneRoomM(sp, s); math.Abs(room-70) > 0.5 {
		t.Errorf("room %.1f, want 70", room)
	}
	if BaseSearchM(s) < s.BaseSpacingSameFactionM {
		t.Error("search must cover the own-faction spacing")
	}
}

func TestBaseNames(t *testing.T) {
	seq := 0
	intn := func(n int) int { seq++; return seq % n }
	for faction := 1; faction <= 6; faction++ {
		name := MakeBaseName(faction, intn)
		if !IsBaseName(faction, name) {
			t.Fatalf("faction %d: %q not accepted", faction, name)
		}
		if IsBaseName(faction%6+1, name) && faction%6+1 != faction {
			// Words belong to one faction; a clash would let one faction take another's style.
			t.Errorf("%q is also a name of faction %d", name, faction%6+1)
		}
	}
	for _, bad := range []string{"", "GLASS", "glass watch", "GLASS  WATCH", "GLASS WATCH X", "IRON WATCH"} {
		if IsBaseName(1, bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if MakeBaseName(7, intn) != "" || IsBaseName(0, "GLASS WATCH") {
		t.Error("unknown faction")
	}
	taken := map[string]bool{}
	opts := BaseNameOptions(2, BaseNameOptionCount, intn, func(n string) bool { return taken[n] })
	if len(opts) != BaseNameOptionCount {
		t.Fatalf("got %d names", len(opts))
	}
	for _, n := range opts {
		if taken[n] {
			t.Fatalf("%q twice", n)
		}
		taken[n] = true
	}
}

func TestBaseBill(t *testing.T) {
	s := DefaultSettings()
	if BaseBill(1, s) != [4]int{400, 400, 150, 50} || BaseBill(5, s) != [4]int{15000, 8000, 6000, 3000} {
		t.Fatal("bills differ from the plan")
	}
	if BaseAssembly(1, s) != 15*time.Minute || BaseAssembly(5, s) != 72*time.Hour || BaseAssembly(9, s) != 0 {
		t.Fatal("assembly times differ from the plan")
	}
	if !BaseProposalExpiry(time.Unix(0, 0), s).Equal(time.Unix(0, 0).Add(48 * time.Hour)) {
		t.Fatal("flags stand 48 h")
	}
}

func TestBaseProjects(t *testing.T) {
	s := DefaultSettings()
	if ModuleBill("lattice", 1, s) != [4]int{300, 200, 100, 50} || ModuleBill("lattice", 3, s) != [4]int{900, 600, 300, 150} {
		t.Fatal("module bills")
	}
	if ModuleBill("hangar", 1, s) != [4]int{} || ValidModule("hangar") || !ValidModule("uplink") || !ValidModule("collector") {
		t.Fatal("the first five modules (Collector Array with B4)")
	}
	if ModuleMinTier("collector") != 2 || ModuleMinTier("vault") != 1 || CollectorLinks(1, s) != 2 || CollectorLinks(4, s) != 4 {
		t.Fatal("collector from the Outpost, 2 to 4 nodes")
	}
	if CollectorAmount(10, "crystal", 1, s) != 13 || CollectorAmount(10, "alloy", 1, s) != 10 || CollectorAmount(10, "crystal", 2, s) != 10 {
		t.Fatal("Lumen Collective gets more crystal")
	}
	if ModuleAssembly("vault", 2, s) != 60*time.Minute || ModuleHealth(3, s) != 900 {
		t.Fatal("assembly and health by Mk")
	}
	if ProjectBill([4]int{100, 0, 0, 7}, 3, s) != [4]int{85, 0, 0, 6} || ProjectBill([4]int{100, 0, 0, 0}, 1, s) != [4]int{100, 0, 0, 0} {
		t.Fatal("Verdant Exchange pays 15% less")
	}
	if AssemblyRate(1, 0, s) != 1 || AssemblyRate(1, 3, s) != 1.3 || AssemblyRate(1, 50, s) != 2 || AssemblyRate(2, 0, s) != 1.25 {
		t.Fatal("presence and Forge Meridian")
	}
	got := Give([4]int64{500, 10, 0, 99}, [4]int64{300, 50, 9, 99}, [4]int64{400, 400, 150, 50}, [4]int64{0, 395, 0, 0})
	if got != [4]int64{300, 5, 0, 50} {
		t.Fatalf("give capped by holding and need: %v", got)
	}
	if !Funded([4]int64{1, 2, 0, 0}, [4]int64{1, 2, 0, 0}) || Funded([4]int64{1, 2, 0, 0}, [4]int64{1, 1, 0, 0}) {
		t.Fatal("funded")
	}
	if GivePoints([4]int64{100, 50, 0, 0}, s) != 150 || GiveXP([4]int64{100, 100, 0, 0}, s) != 10 {
		t.Fatal("points and XP")
	}
	if !CanCancelProject(true, RankWarden, RankLead) || CanCancelProject(false, RankWarden, RankWarden) || !CanCancelProject(false, RankLead, RankWarden) {
		t.Fatal("cancel")
	}
	if StartPower("module", 0) != PowerStartProjects || StartPower("tier", 4) != PowerStartT4 || StartPower("tier", 5) != PowerApproveBase {
		t.Fatal("start power")
	}
}
