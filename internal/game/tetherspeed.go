package game

import (
	"math"
	"time"
)

// Tether Node slowdown (owner, 2026-11-11). A Tether Node gathers at full speed for
// TnFullSpeedHours after it is built or its owner last collected at it, then TnSlowPctPerHour
// slower for each further hour, never below TnSlowFloorPct. Collecting at it restores full speed.

// TetherSpeed is how fast a Tether Node gathers, from the floor up to 1 (full speed), when its
// owner last collected (or built it) this long ago. It falls smoothly, not in hourly steps.
func TetherSpeed(s Settings, sinceCollect time.Duration) float64 {
	floor := math.Min(1, math.Max(0.01, s.TnSlowFloorPct/100))
	away := sinceCollect.Hours() - s.TnFullSpeedHours
	if away <= 0 || s.TnSlowPctPerHour <= 0 {
		return 1
	}
	return math.Max(floor, 1-away*s.TnSlowPctPerHour/100)
}

// TetherSpeedPct is TetherSpeed in whole percent, as the game shows it.
func TetherSpeedPct(s Settings, sinceCollect time.Duration) int {
	return int(math.Round(TetherSpeed(s, sinceCollect) * 100))
}

// TetherCooldownSec is how long a link waits before its next harvest at this speed: the usual
// harvest cooldown at full speed, five times as long at 20%.
func TetherCooldownSec(s Settings, speed float64) int {
	if speed <= 0 || speed > 1 {
		speed = 1
	}
	return int(math.Round(float64(s.HarvestCooldownSec) / speed))
}
