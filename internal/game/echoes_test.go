package game

import (
	"math"
	"testing"
)

func TestEchoSetsAreTheApprovedFirstSet(t *testing.T) {
	// Only the owner's approved lines (2026-11-11); a change here is a change to the story.
	if len(EchoSets) != 1 || len(EchoSets[0]) != 5 {
		t.Fatalf("want one set of five, got %d sets", len(EchoSets))
	}
	if text, ok := EchoLine(1, 3); !ok || text != "A signal folded here and never unfolded." {
		t.Fatalf("line 3: %q %v", text, ok)
	}
	if _, ok := EchoLine(1, 6); ok {
		t.Fatal("line 6 should not exist")
	}
	if _, ok := EchoLine(2, 1); ok {
		t.Fatal("set 2 should not exist")
	}
}

func TestPickEchoLineCoversEveryLine(t *testing.T) {
	seen := map[int]bool{}
	for i := 0; i < 50; i++ {
		set, line := PickEchoLine(float64(i) / 50)
		if set != 1 || line < 1 || line > 5 {
			t.Fatalf("r=%v gave %d/%d", float64(i)/50, set, line)
		}
		seen[line] = true
	}
	if len(seen) != 5 {
		t.Fatalf("saw lines %v", seen)
	}
	if set, line := PickEchoLine(0.999999); set != 1 || line != 5 {
		t.Fatalf("top end: %d/%d", set, line)
	}
}

func TestEchoOffsetStaysNearItsAnchor(t *testing.T) {
	s := DefaultSettings()
	for _, r := range []float64{0, 0.5, 0.99} {
		bearing, distance := EchoOffset(s, r, r)
		if distance < s.EchoOffsetMinM || distance > s.EchoOffsetMaxM {
			t.Fatalf("distance %v outside %v..%v", distance, s.EchoOffsetMinM, s.EchoOffsetMaxM)
		}
		lat, lon := OffsetPoint(4.1755, 73.5093, bearing, distance)
		if d := DistanceM(4.1755, 73.5093, lat, lon); math.Abs(d-distance) > 0.5 {
			t.Fatalf("offset point is %v m away, want %v", d, distance)
		}
	}
}

func TestEchoBadgeName(t *testing.T) {
	if EchoBadge(1) != "codex_set_1" {
		t.Fatal(EchoBadge(1))
	}
}
