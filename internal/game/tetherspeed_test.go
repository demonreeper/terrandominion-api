package game

import (
	"testing"
	"time"
)

func TestTetherSpeed(t *testing.T) {
	s := DefaultSettings() // full speed 2 h, 10% an hour, floor 20%
	hours := func(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
	for _, c := range []struct {
		away float64
		want int
	}{
		{0, 100}, {1.9, 100}, {2, 100},
		{3, 90}, {4.5, 75}, {7, 50},
		{10, 20}, {11, 20}, {48, 20},
		{-1, 100}, // a clock a little ahead never speeds it up past full
	} {
		if got := TetherSpeedPct(s, hours(c.away)); got != c.want {
			t.Errorf("after %v h: %d%%, want %d%%", c.away, got, c.want)
		}
	}
}

func TestTetherSpeed_SettingsBoundIt(t *testing.T) {
	s := DefaultSettings()
	day := 24 * time.Hour

	s.TnSlowPctPerHour = 0 // switched off
	if got := TetherSpeed(s, day); got != 1 {
		t.Errorf("slowdown off: %v", got)
	}
	s.TnSlowPctPerHour = 100
	s.TnSlowFloorPct = 0 // a floor of nothing would stop it for good: never below 1%
	if got := TetherSpeed(s, day); got != 0.01 {
		t.Errorf("zero floor: %v", got)
	}
	s.TnSlowFloorPct = 250 // a floor over 100% is full speed
	if got := TetherSpeed(s, day); got != 1 {
		t.Errorf("floor over 100: %v", got)
	}
}

func TestTetherCooldownSec(t *testing.T) {
	s := DefaultSettings()
	s.HarvestCooldownSec = 300
	for speed, want := range map[float64]int{1: 300, 0.5: 600, 0.2: 1500, 0: 300, 2: 300} {
		if got := TetherCooldownSec(s, speed); got != want {
			t.Errorf("cooldown at %v: %d, want %d", speed, got, want)
		}
	}
}
