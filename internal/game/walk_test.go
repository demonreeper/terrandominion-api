package game

import (
	"testing"
	"time"
)

// Walk together (docs/walk-together-plan.md): the rules the plan's tests name, at the rule level. The
// service flow (friends only, devices, attack, lock) is checked end to end on dev.

var walkNow = time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)

func walkAt(lat, lon, acc float64, ago time.Duration) WalkFix {
	return WalkFix{Lat: lat, Lon: lon, AccuracyM: acc, FixAt: walkNow.Add(-ago)}
}

// metres north of a point in Malé, near enough for these tests.
func north(m float64) float64 { return 4.1755 + m/111320 }

func TestWalkLinkPctsIsTheConfirmedTable(t *testing.T) {
	s := DefaultSettings()
	cases := map[int]WalkPcts{1: {}, 2: {25, 15, 10}, 3: {40, 25, 20}, 4: {50, 30, 30}, 5: {50, 30, 30}}
	for size, want := range cases {
		if got := WalkLinkPcts(s, size); got != want {
			t.Errorf("size %d: %+v, want %+v", size, got, want)
		}
	}
}

func TestWalkFixUsable(t *testing.T) {
	s := DefaultSettings()
	if !WalkFixUsable(s, walkAt(north(0), 73.5, 8, 5*time.Second), walkNow, false) {
		t.Error("a fresh accurate fix should count")
	}
	if WalkFixUsable(s, walkAt(north(0), 73.5, 30, 0), walkNow, false) {
		t.Error("accuracy past 25 m should not count")
	}
	if WalkFixUsable(s, walkAt(north(0), 73.5, 8, 25*time.Second), walkNow, false) {
		t.Error("a fix older than 20 s should not count")
	}
	mock := walkAt(north(0), 73.5, 8, 0)
	mock.Mock = true
	if WalkFixUsable(s, mock, walkNow, false) {
		t.Error("a fake-location fix should not count")
	}
	// Spoof-exempt testers skip the fake flag (L1 to L5), so the owner can test with a fake location.
	if !WalkFixUsable(s, mock, walkNow, true) {
		t.Error("an exempt tester's fake fix should count")
	}
}

func TestWalkTogetherRangeAfterAccuracy(t *testing.T) {
	s := DefaultSettings()
	a := walkAt(north(0), 73.5, 5, 0)
	if !WalkTogether(s, a, walkAt(north(28), 73.5, 5, 0), s.WalkLinkFormM) {
		t.Error("28 m apart should form within 30 m")
	}
	// 45 m apart, 10 m + 10 m accuracy covers 20 m: 25 m counts.
	if !WalkTogether(s, walkAt(north(0), 73.5, 10, 0), walkAt(north(45), 73.5, 10, 0), s.WalkLinkFormM) {
		t.Error("accuracy should be subtracted")
	}
	// Two poor fixes cover at most 25 m together: 80 m apart is 55 m, too far.
	if WalkTogether(s, walkAt(north(0), 73.5, 25, 0), walkAt(north(80), 73.5, 25, 0), s.WalkLinkFormM) {
		t.Error("two poor fixes must not cover a long gap")
	}
	// Fixes more than 20 s apart in time don't count together.
	if WalkTogether(s, walkAt(north(0), 73.5, 5, 0), walkAt(north(5), 73.5, 5, 25*time.Second), s.WalkLinkFormM) {
		t.Error("fixes taken far apart in time must not count")
	}
}

func TestWalkInRangeHoldsThroughAChain(t *testing.T) {
	s := DefaultSettings()
	a := walkAt(north(0), 73.5, 5, 0)
	b := walkAt(north(55), 73.5, 5, 0)
	c := walkAt(north(110), 73.5, 5, 0)
	far := walkAt(north(400), 73.5, 5, 0)
	got := WalkInRange(s, []*WalkFix{&a, &b, &c, &far, nil})
	want := []bool{true, true, true, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("member %d in range = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestWalkBonusPausesStandingStill(t *testing.T) {
	s := DefaultSettings()
	trail := []WalkTrailPoint{{Lat: north(0), Lon: 73.5, At: walkNow.Add(-170 * time.Second)}}
	if WalkMoved(s, trail, north(30), 73.5, walkNow) {
		t.Error("30 m in the window is not walking")
	}
	if !WalkMoved(s, trail, north(60), 73.5, walkNow) {
		t.Error("60 m in the window is walking")
	}
	// Ground covered before the window doesn't count.
	old := []WalkTrailPoint{{Lat: north(0), Lon: 73.5, At: walkNow.Add(-200 * time.Second)}}
	if WalkMoved(s, old, north(60), 73.5, walkNow) {
		t.Error("movement before the window should not count")
	}
	if got := WalkPaused(true, true, false, 600); got != WalkPausedKeepWalking {
		t.Errorf("standing still: %q", got)
	}
	if got := WalkPaused(true, true, true, 600); got != "" {
		t.Errorf("walking: %q", got)
	}
}

func TestWalkDailyCap(t *testing.T) {
	s := DefaultSettings()
	if left := WalkCapLeft(s, 120*60); left != 0 {
		t.Errorf("120 minutes used: %d left", left)
	}
	if got := WalkPaused(true, true, true, 0); got != WalkPausedCapSpent {
		t.Errorf("cap spent: %q", got)
	}
	// A beat counts its time, never more than two beats, never past the cap.
	if got := WalkBonusSeconds(s, 15*time.Second, 600); got != 15 {
		t.Errorf("one beat: %d", got)
	}
	if got := WalkBonusSeconds(s, 10*time.Minute, 600); got != 30 {
		t.Errorf("a gap: %d", got)
	}
	if got := WalkBonusSeconds(s, 15*time.Second, 4); got != 4 {
		t.Errorf("near the cap: %d", got)
	}
}

func TestWalkLinkBonusNeedsALiveActiveLink(t *testing.T) {
	s := DefaultSettings()
	live := WalkMemberState{Linked: true, GroupSize: 2, BonusActive: true, LastSeenAt: walkNow.Add(-10 * time.Second)}
	if got := WalkLinkBonus(s, live, walkNow); got != (WalkPcts{25, 15, 10}) {
		t.Errorf("live pair: %+v", got)
	}
	paused := live
	paused.BonusActive = false
	if got := WalkLinkBonus(s, paused, walkNow); got != (WalkPcts{}) {
		t.Errorf("paused: %+v", got)
	}
	silent := live
	silent.LastSeenAt = walkNow.Add(-2 * time.Minute)
	if got := WalkLinkBonus(s, silent, walkNow); got != (WalkPcts{}) {
		t.Errorf("silent: %+v", got)
	}
	off := s
	off.WalkLinkEnabled = 0
	if got := WalkLinkBonus(off, live, walkNow); got != (WalkPcts{}) {
		t.Errorf("disabled: %+v", got)
	}
}

func TestWalkBonusAddsToPerksNeverMultiplies(t *testing.T) {
	// Signum's +25% crystal and a 2-link's +15% make +40%, not 1.25 x 1.15 = +43.75%.
	if got := WalkAmount(100, 1.25, 15); got != 140 {
		t.Errorf("perk and link: %d, want 140", got)
	}
	if got := WalkAmount(100, 1, 15); got != 115 {
		t.Errorf("link alone: %d", got)
	}
	if got := WalkAmount(100, 1, 0); got != 100 {
		t.Errorf("nothing: %d", got)
	}
	// Sēma's scan x2 (+100%) plus a 2-link's +25% is +125% on the base.
	if got := WalkScanRange(300, 2, 25); got != 675 {
		t.Errorf("scan: %v, want 675", got)
	}
}
