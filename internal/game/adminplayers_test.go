package game

import (
	"strings"
	"testing"
)

func TestCheckAdjust(t *testing.T) {
	carried := map[string]int64{"alloy": 40, "crystal": 0}
	stored := map[string]int64{"alloy": 500, "crystal": 10}
	cases := []struct {
		name string
		a    Adjustment
		want string
	}{
		{"xp up", Adjustment{XP: 250, Reason: "Lost to a bug."}, AdjustOK},
		{"xp down to zero", Adjustment{XP: -1000, Reason: "Reverting a bug."}, AdjustOK},
		{"xp below zero", Adjustment{XP: -1001, Reason: "Reverting a bug."}, AdjustBelow},
		{"no reason", Adjustment{XP: 5}, AdjustReason},
		{"short reason", Adjustment{XP: 5, Reason: " x "}, AdjustReason},
		{"long reason", Adjustment{XP: 5, Reason: strings.Repeat("x", 501)}, AdjustReason},
		{"nothing", Adjustment{Reason: "Nothing."}, AdjustNothing},
		{"zeros only", Adjustment{Carried: map[string]int64{"alloy": 0}, Reason: "Nothing."}, AdjustNothing},
		{"carried down", Adjustment{Carried: map[string]int64{"alloy": -40}, Reason: "Duplicated."}, AdjustOK},
		{"carried too far down", Adjustment{Carried: map[string]int64{"alloy": -41}, Reason: "Duplicated."}, AdjustBelow},
		{"stored up", Adjustment{Stored: map[string]int64{"crystal": 100}, Reason: "Compensation."}, AdjustOK},
		{"stored below", Adjustment{Stored: map[string]int64{"crystal": -11}, Reason: "Duplicated."}, AdjustBelow},
		{"unknown resource", Adjustment{Stored: map[string]int64{"gold": 1}, Reason: "Compensation."}, AdjustResource},
		{"too much", Adjustment{XP: AdjustMax + 1, Reason: "Compensation."}, AdjustTooMuch},
		{"too much of a resource", Adjustment{Carried: map[string]int64{"alloy": AdjustMax + 1}, Reason: "Compensation."}, AdjustTooMuch},
	}
	for _, c := range cases {
		if got := CheckAdjust(c.a, 1000, carried, stored); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRetentionPct(t *testing.T) {
	for _, c := range []struct{ retained, of, want int }{{0, 0, 0}, {0, 10, 0}, {3, 10, 30}, {1, 3, 33}, {2, 3, 67}, {10, 10, 100}} {
		if got := RetentionPct(c.retained, c.of); got != c.want {
			t.Errorf("RetentionPct(%d, %d) = %d, want %d", c.retained, c.of, got, c.want)
		}
	}
}

func TestCleanPlayerSearch(t *testing.T) {
	for _, c := range []struct {
		typed, want string
		ok          bool
	}{
		{"  Night  ", "Night", true},
		{"operative-3NDGRH1M", "3NDGRH1M", true},
		{"x", "", false},
		{"", "", false},
		{"OPERATIVE-", "", false},
		{strings.Repeat("a", 81), "", false},
	} {
		got, ok := CleanPlayerSearch(c.typed)
		if got != c.want || ok != c.ok {
			t.Errorf("CleanPlayerSearch(%q) = %q, %v; want %q, %v", c.typed, got, ok, c.want, c.ok)
		}
	}
}
