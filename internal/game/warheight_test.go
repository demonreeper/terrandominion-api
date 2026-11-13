package game

import (
	"math"
	"testing"
)

func TestWarUpHigh(t *testing.T) {
	s := Settings{WarMaxHeightM: 10}
	cases := []struct {
		name     string
		height   float64
		accuracy float64
		want     bool
	}{
		{"balcony, good height", 72, 3, true},
		{"street level", 1.5, 4, false},
		{"just over", 10.5, 5, true},
		{"at the limit", 10, 5, false},
		{"unknown height", math.NaN(), 5, false},
		{"no accuracy", 72, 0, false},
		{"poor accuracy", 72, 25, false},
	}
	for _, c := range cases {
		if got := WarUpHigh(c.height, c.accuracy, s); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if WarUpHigh(72, 3, Settings{WarMaxHeightM: 0}) {
		t.Error("0 turns the rule off")
	}
}
