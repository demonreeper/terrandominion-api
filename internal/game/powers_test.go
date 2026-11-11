package game

import (
	"strings"
	"testing"
	"time"
)

var personaNames = map[int]string{
	PowerSignum: "Signum", PowerShingo: "Shingō", PowerIshara: "Ishāra",
	PowerSema: "Sēma", PowerTohu: "Tohu", PowerMerki: "Merki",
}

func TestPowerLinesAreComplete(t *testing.T) {
	kinds := []string{CallLevel10, CallLevel19, CallLevel19Repeat, MessageLevel19}
	for _, power := range Powers {
		if PowerVoice(power) == "" {
			t.Errorf("power %d has no voice folder", power)
		}
		for _, kind := range kinds {
			if PowerLine(power, kind) == "" {
				t.Errorf("power %d has no %s line", power, kind)
			}
		}
	}
	for _, kind := range []string{CallLevel10, CallLevel19, CallLevel19Repeat} {
		if CallAudioKey(kind) == "" {
			t.Errorf("%s has no audio key", kind)
		}
	}
	if CallAudioKey(MessageLevel19) != "" {
		t.Error("the written message has no audio")
	}
}

// Lore rules: no power says its own (or another's) name, nothing says "AI", and no two powers
// share a line, which would link them.
func TestPowerLinesKeepTheSecret(t *testing.T) {
	seen := map[string]int{}
	for _, power := range Powers {
		for kind, line := range powerLines[power] {
			for _, name := range personaNames {
				if strings.Contains(strings.ToLower(line), strings.ToLower(name)) {
					t.Errorf("power %d %s names %s: %q", power, kind, name, line)
				}
			}
			for _, word := range strings.FieldsFunc(line, func(r rune) bool { return !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z') }) {
				if word == "AI" {
					t.Errorf("power %d %s says AI: %q", power, kind, line)
				}
			}
			if other, dup := seen[line]; dup {
				t.Errorf("powers %d and %d share a line: %q", other, power, line)
			}
			seen[line] = power
		}
	}
}

// The day 3 message says the choice is final (freelancer-path-plan.md C4), and each power says it in
// its own words: no closing sentence is shared, so the six stay unlinked.
func TestDay3MessageSaysTheChoiceIsFinal(t *testing.T) {
	finality := []string{"cannot", "once", "no exit", "no turning back", "never given back"}
	seen := map[string]int{}
	for _, power := range Powers {
		line := PowerLine(power, MessageLevel19)
		if !strings.Contains(line, "Choose") {
			t.Errorf("power %d day 3 message lost its choice: %q", power, line)
		}
		final := false
		for _, word := range finality {
			if strings.Contains(strings.ToLower(line), word) {
				final = true
			}
		}
		if !final {
			t.Errorf("power %d day 3 message doesn't say the choice is final: %q", power, line)
		}
		sentences := strings.Split(line, ". ")
		last := sentences[len(sentences)-1]
		if other, dup := seen[last]; dup {
			t.Errorf("powers %d and %d end the day 3 message the same way: %q", other, power, last)
		}
		seen[last] = power
	}
}

func TestPlaystylePoints(t *testing.T) {
	cases := []struct {
		action, key string
		power       int
	}{
		{GoalHarvest, "crystal", PowerSignum},
		{GoalHarvest, "alloy", PowerSignum},
		{GoalHarvest, "data", PowerSema},
		{GoalHarvest, "credit", PowerIshara},
		{GoalBuildTether, "", PowerShingo},
		{GoalUpgradeTether, "", PowerShingo},
		{GoalUpgradeGear, "", PowerShingo},
		{GoalRepairGear, "", PowerShingo},
		{GoalCraft, ItemCell, PowerShingo},
		{GoalCraft, WeaponJammer, PowerSema},
		{GoalCraft, WeaponEmPulse, PowerTohu},
		{GoalCollect, "", PowerIshara},
		{GoalUpload, "", PowerIshara},
		{GoalAttack, WeaponEmPulse, PowerTohu},
		{GoalAttack, WeaponSpike, PowerSema},
		{GoalAttack, WeaponJammer, PowerSema},
		{GoalDrone, "", PowerTohu},
		{GoalIntercept, "", PowerMerki},
		{GoalShield, "", PowerMerki},
		{PlayDefuse, "", PowerMerki},
	}
	for _, c := range cases {
		points := PlaystylePoints(c.action, c.key)
		top, topPoints := 0, 0
		for power, p := range points {
			if p > topPoints {
				top, topPoints = power, p
			}
		}
		if top != c.power {
			t.Errorf("%s %q: top power %d, want %d (%v)", c.action, c.key, top, c.power, points)
		}
	}
	if PlaystylePoints(GoalAppointWarden, "") != nil {
		t.Error("appointing a Warden is not a playstyle")
	}
	// One build outweighs a couple of harvests: rare actions must be able to win.
	if PlaystylePoints(GoalBuildTether, "")[PowerShingo] <= 2*PlaystylePoints(GoalHarvest, "crystal")[PowerSignum] {
		t.Error("a build should outweigh two crystal harvests")
	}
}

func TestDistancePoints(t *testing.T) {
	for metres, want := range map[float64]int{0: 0, 99: 0, 100: 1, 250: 2, 499: 4, 500: 5, 50000: 5} {
		if got := DistancePoints(metres); got != want {
			t.Errorf("%v m: got %d, want %d", metres, got, want)
		}
	}
}

func TestCallingPower(t *testing.T) {
	if _, ok := CallingPower(nil, nil); ok {
		t.Fatal("nothing to go on")
	}
	if p, ok := CallingPower(map[int]int{PowerTohu: 40, PowerSignum: 90, PowerMerki: 12}, nil); !ok || p != PowerSignum {
		t.Fatalf("highest score: got %d", p)
	}
	// A tie goes to the power whose daily missions the player finished most.
	scores := map[int]int{PowerSema: 50, PowerIshara: 50}
	if p, _ := CallingPower(scores, map[int]int{PowerSema: 2, PowerIshara: 5}); p != PowerIshara {
		t.Fatalf("tie by missions: got %d", p)
	}
	// Still tied: the lowest id, so the answer never flips between two requests.
	if p, _ := CallingPower(scores, nil); p != PowerIshara {
		t.Fatalf("stable tie: got %d", p)
	}
	// No actions scored yet: the power they followed most.
	if p, ok := CallingPower(nil, map[int]int{PowerMerki: 3, PowerTohu: 1}); !ok || p != PowerMerki {
		t.Fatalf("missions only: got %d", p)
	}
}

func TestCallsDue(t *testing.T) {
	s := DefaultSettings()
	mid10 := CallMidpointXP(10, s)
	if mid10 <= XPForLevel(10, s) || mid10 >= XPForLevel(11, s) {
		t.Fatalf("midpoint %d outside level 10 (%d..%d)", mid10, XPForLevel(10, s), XPForLevel(11, s))
	}
	if path, offer := CallsDue(mid10-5, mid10-1, s); path || offer {
		t.Fatal("not there yet")
	}
	if path, offer := CallsDue(mid10-1, mid10, s); !path || offer {
		t.Fatal("crossing half of level 10 sets off the path call only")
	}
	if path, _ := CallsDue(mid10, mid10+50, s); path {
		t.Fatal("the path call is set off once")
	}

	mid19 := CallMidpointXP(19, s)
	if _, offer := CallsDue(mid19-1, mid19+10, s); !offer {
		t.Fatal("crossing half of level 19 sets off the offer")
	}
	// Jumping straight to level 20 skips the offer: FACTIONS takes over.
	if _, offer := CallsDue(mid19-1, XPForLevel(20, s), s); offer {
		t.Fatal("no offer once level 20 is reached")
	}
	// One big gain can set off the path call on its way.
	if path, offer := CallsDue(0, mid19, s); !path || !offer {
		t.Fatal("a gain past both midpoints sets off both")
	}
}

func TestOfferSchedule(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	repeat, message := OfferSchedule(now, s)
	if !repeat.Equal(now.Add(24*time.Hour)) || !message.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("got %v and %v", repeat, message)
	}
	if CallMissed(now, now.Add(30*time.Second), s) {
		t.Fatal("still ringing at 30 s")
	}
	if !CallMissed(now, now.Add(2*time.Minute), s) {
		t.Fatal("rung out after 2 minutes")
	}
}

func TestCallNotices(t *testing.T) {
	if got := NoticeCall("Sēma"); got != "Incoming call: Sēma" {
		t.Fatalf("got %q", got)
	}
	if rule, ok := PushRuleFor("call"); !ok || !rule.Always {
		t.Fatal("a call pushes whatever the player's switches say")
	}
	for _, name := range personaNames {
		if strings.Contains(NoticeInvited(), name) {
			t.Fatalf("the level 20 text is plain game text: %q", NoticeInvited())
		}
	}
}
