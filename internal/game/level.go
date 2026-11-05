package game

import "math"

const MaxLevel = 200

// XPForLevel is the total XP needed to reach level n (level 1 needs 0). Each step costs
// LevelGrowth times the one before it, starting from LevelBaseXp.
func XPForLevel(n int, s Settings) int64 {
	if n <= 1 {
		return 0
	}
	growth := s.LevelGrowth
	if growth <= 1 {
		return int64(s.LevelBaseXp * float64(n-1))
	}
	total := s.LevelBaseXp * (math.Pow(growth, float64(n-1)) - 1) / (growth - 1)
	return int64(math.Round(total))
}

// LevelForXP is the level a player with this much XP has reached.
func LevelForXP(xp int64, s Settings) int {
	level := 1
	for level < MaxLevel && XPForLevel(level+1, s) <= xp {
		level++
	}
	return level
}

// Progress is what the client shows: current level, XP into it, and XP the level needs.
type Progress struct {
	Level     int   `json:"level"`
	XP        int64 `json:"xp"`
	LevelXP   int64 `json:"levelXp"`
	LevelNeed int64 `json:"levelNeed"`
}

func ProgressFor(xp int64, s Settings) Progress {
	level := LevelForXP(xp, s)
	floor := XPForLevel(level, s)
	next := XPForLevel(level+1, s)
	return Progress{Level: level, XP: xp, LevelXP: xp - floor, LevelNeed: next - floor}
}
