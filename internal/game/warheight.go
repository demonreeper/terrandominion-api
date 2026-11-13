package game

import "math"

// WarHeightAccuracyM is the worst height accuracy (68 %) the war rule trusts; a poorer height never blocks.
const WarHeightAccuracyM = 12

// WarUpHigh reports whether a war action (attack, drone, hack, contest, intercept, defuse, repair) is refused because
// the phone is well above the ground (owner, 2026-11-13: no war from a balcony or a tower flat). heightM is the app's
// height above ground and accuracyM its accuracy; NaN, a missing accuracy or a poor one never blocks, nor does a
// WarMaxHeightM of 0. A general direction, not strict: the height comes from the phone.
func WarUpHigh(heightM, accuracyM float64, s Settings) bool {
	if s.WarMaxHeightM <= 0 || math.IsNaN(heightM) || math.IsInf(heightM, 0) {
		return false
	}
	if !(accuracyM > 0 && accuracyM <= WarHeightAccuracyM) {
		return false
	}
	return heightM > s.WarMaxHeightM
}
