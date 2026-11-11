package game

import (
	"fmt"
	"math"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// Signal echoes (docs/no-ar-support-plan.md, "AR sees more", milestone 7): faint traces in the
// Undersignal, seen and read only in the AR view. Looks and story only: reading one gives no
// resources, no XP and no combat information. The CODEX on a player's own profile lists the lines
// found, in sets of five; a finished set gives a profile badge.

// EchoSets are the lines, set by set. Only lines the owner approved go here (first set approved
// 2026-11-11); never add others. Lore rules: LORE.md and the screen rules, and never a hint of a
// seventh voice.
var EchoSets = [][]string{
	{
		"Before the first map, this street was already listening.",
		"Something counted your steps. It did not lose count.",
		"A signal folded here and never unfolded.",
		"Every stake driven into the lattice is felt.",
		"What decays is not lost. It is taken back.",
	},
}

// EchoLine is one line's text; ok is false for a set or line that doesn't exist. Sets and lines count from 1.
func EchoLine(set, line int) (string, bool) {
	if set < 1 || set > len(EchoSets) || line < 1 || line > len(EchoSets[set-1]) {
		return "", false
	}
	return EchoSets[set-1][line-1], true
}

// EchoSetSize is how many lines a set has (0 for an unknown set).
func EchoSetSize(set int) int {
	if set < 1 || set > len(EchoSets) {
		return 0
	}
	return len(EchoSets[set-1])
}

// EchoBadge is the profile badge a finished set gives.
func EchoBadge(set int) string { return fmt.Sprintf("codex_set_%d", set) }

// PickEchoLine chooses which line a new echo carries: r is a random number in [0, 1).
func PickEchoLine(r float64) (set, line int) {
	total := 0
	for _, lines := range EchoSets {
		total += len(lines)
	}
	if total == 0 {
		return 0, 0
	}
	n := int(math.Floor(r * float64(total)))
	if n >= total {
		n = total - 1
	}
	for s, lines := range EchoSets {
		if n < len(lines) {
			return s + 1, n + 1
		}
		n -= len(lines)
	}
	return 0, 0
}

// EchoOffset is where a new echo stands from the node or Tether Node it is placed beside: a bearing
// and a distance from two random numbers in [0, 1). Near, not on top of it.
func EchoOffset(s Settings, r1, r2 float64) (bearingDeg, distanceM float64) {
	lo, hi := s.EchoOffsetMinM, s.EchoOffsetMaxM
	if hi < lo {
		hi = lo
	}
	return r1 * 360, lo + r2*(hi-lo)
}

// OffsetPoint is the point a distance away along a bearing (great circle; plenty for tens of metres).
func OffsetPoint(lat, lon, bearingDeg, distanceM float64) (float64, float64) {
	const earthM = 6371000.0
	d := distanceM / earthM
	b := bearingDeg * math.Pi / 180
	p1 := lat * math.Pi / 180
	l1 := lon * math.Pi / 180
	p2 := math.Asin(math.Sin(p1)*math.Cos(d) + math.Cos(p1)*math.Sin(d)*math.Cos(b))
	l2 := l1 + math.Atan2(math.Sin(b)*math.Sin(d)*math.Cos(p1), math.Cos(d)-math.Sin(p1)*math.Sin(p2))
	return p2 * 180 / math.Pi, l2 * 180 / math.Pi
}

// CheckAtEcho confirms the player is close enough to read an echo: the same range, accuracy and
// travel checks as a node.
func CheckAtEcho(s Settings, player Fix, last *Fix, lat, lon float64) (float64, *apperr.Error) {
	return checkAtSpot(s, player, last, lat, lon, apperr.EchoTooFar, "the echo")
}
