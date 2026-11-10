package game

import (
	"fmt"
	"math"
	"time"
)

// Drone strike plan, milestones 2-7: intercepts, Tether Node shields and what reaches a node.

// WeaponMk is the Mk EM Pulse, Signal Jammer and Overload Spike count as against a shield, until
// they get tiers of their own.
const WeaponMk = 1

// MaxMk is the highest Drone Bay and shield Mk.
const MaxMk = 4

// InterceptShare is the share of a drone's damage an intercept stops, by the drone's Mk: the higher
// the Mk, the less it stops. 1 means the drone is shot down.
func InterceptShare(s Settings, droneMk int) float64 {
	shares := []float64{s.InterceptShareMk1, s.InterceptShareMk2, s.InterceptShareMk3, s.InterceptShareMk4}
	mk := min(max(droneMk, 1), MaxMk)
	return clamp01(shares[mk-1])
}

// ShieldShare is the share of an attack a Tether Node's shield stops: TnShieldBase when the Mks
// match, TnShieldStep more for each Mk the shield is above the attack and less for each below,
// between TnShieldMin and TnShieldMax. No shield (Mk 0) stops nothing.
func ShieldShare(s Settings, shieldMk, attackMk int) float64 {
	if shieldMk <= 0 {
		return 0
	}
	share := s.TnShieldBase + s.TnShieldStep*float64(shieldMk-max(attackMk, 1))
	return clamp01(math.Min(s.TnShieldMax, math.Max(s.TnShieldMin, share)))
}

// HitDamage is what reaches a Tether Node: damage x (1 - intercept share) x (1 - shield share),
// rounded up.
func HitDamage(damage, interceptShare, shieldShare float64) float64 {
	d := damage * (1 - clamp01(interceptShare)) * (1 - clamp01(shieldShare))
	if d <= 0 {
		return 0
	}
	// The small allowance keeps 70.0000001 (floating point) from becoming 71.
	return math.Ceil(d - 1e-9)
}

// CapShieldedHit stops one hit taking a shielded Tether Node below 1 health, even if decay has
// already lowered it. A node already at 1 or less can be destroyed by the next hit.
func CapShieldedHit(health, damage float64, shielded bool) float64 {
	if shielded && health > 1 && damage > health-1 {
		return health - 1
	}
	return damage
}

// ShieldedJam cuts a Signal Jammer's time by the shield's share, to the second.
func ShieldedJam(jam time.Duration, shieldShare float64) time.Duration {
	return (time.Duration(float64(jam) * (1 - clamp01(shieldShare)))).Round(time.Second)
}

func clamp01(v float64) float64 { return math.Min(1, math.Max(0, v)) }

// RomanMk writes a Mk for players: 1 -> "I".
func RomanMk(mk int) string {
	switch mk {
	case 1:
		return "I"
	case 2:
		return "II"
	case 3:
		return "III"
	case 4:
		return "IV"
	}
	return fmt.Sprint(mk)
}

// ShieldItemID is the item (and recipe) id of a TN Shield: "tn_shield_mk2".
func ShieldItemID(mk int) string { return fmt.Sprintf("tn_shield_mk%d", mk) }

// ShieldMkOf reads the Mk from a shield item id; ok is false for anything else.
func ShieldMkOf(itemID string) (int, bool) {
	for mk := 1; mk <= MaxMk; mk++ {
		if ShieldItemID(mk) == itemID {
			return mk, true
		}
	}
	return 0, false
}

// shieldRecipes are the four TN Shields, crafted on the Craft tab as items.
func shieldRecipes(s Settings) []Recipe {
	levels := []int{s.ShieldLevelMk1, s.ShieldLevelMk2, s.ShieldLevelMk3, s.ShieldLevelMk4}
	costs := []map[string]int{
		{"crystal": s.CraftShield1Crystal, "alloy": s.CraftShield1Alloy},
		{"alloy": s.CraftShield2Alloy, "data": s.CraftShield2Data},
		{"alloy": s.CraftShield3Alloy, "data": s.CraftShield3Data, "credit": s.CraftShield3Credit},
		{"alloy": s.CraftShield4Alloy, "data": s.CraftShield4Data, "credit": s.CraftShield4Credit},
	}
	out := make([]Recipe, 0, MaxMk)
	for mk := 1; mk <= MaxMk; mk++ {
		out = append(out, Recipe{
			ID: ShieldItemID(mk), Name: "TN Shield Mk " + RomanMk(mk), Cost: positive(costs[mk-1]),
			MinLevel: levels[mk-1],
		})
	}
	return out
}
