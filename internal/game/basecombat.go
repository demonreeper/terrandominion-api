package game

import (
	"math"
	"sort"
	"time"
)

// Faction bases, B5 (base-building-plan.md 5, 6, 8): power from the Energy Lattice, the Shield Matrix, hacks on the
// vault, tier drops and destruction.

// factionBastion is Bastion Order (larger safe floor, stronger base shield).
const factionBastion = 6

// PowerSlot is a module slot for the power rule.
type PowerSlot struct {
	Ring, Index int
	Module      string
	Health      int
}

// PoweredSlots decides which modules are online: each Energy Lattice with health powers BaseModulesPerLattice other
// modules, nearest the tower first (ring, then index); a module at 0 health is offline until repaired; a Lattice with
// health is online itself. Knock the Lattice out and the modules it powered go offline: the siege target.
func PoweredSlots(slots []PowerSlot, s Settings) map[[2]int]bool {
	out := map[[2]int]bool{}
	lattices := 0
	var others []PowerSlot
	for _, sl := range slots {
		if sl.Module == "" {
			continue
		}
		if sl.Module == "lattice" {
			if sl.Health > 0 {
				lattices++
				out[[2]int{sl.Ring, sl.Index}] = true
			}
			continue
		}
		others = append(others, sl)
	}
	sort.Slice(others, func(i, j int) bool {
		if others[i].Ring != others[j].Ring {
			return others[i].Ring < others[j].Ring
		}
		return others[i].Index < others[j].Index
	})
	capacity := lattices * max(0, s.BaseModulesPerLattice)
	for _, sl := range others {
		if sl.Health > 0 && capacity > 0 {
			out[[2]int{sl.Ring, sl.Index}] = true
			capacity--
		}
	}
	return out
}

// BaseShieldShare is what the base's best online Shield Matrix stops of an attack of attackMk (the TN shield's rule by
// Mk), Bastion Order's a little more. No Shield Matrix (0) stops nothing.
func BaseShieldShare(shieldMk, attackMk, factionID int, s Settings) float64 {
	share := ShieldShare(s, shieldMk, attackMk)
	if shieldMk > 0 && factionID == factionBastion && s.BasePerkBastionShieldPct > 0 {
		share = math.Min(s.TnShieldMax+s.BasePerkBastionShieldPct/100, share+s.BasePerkBastionShieldPct/100)
	}
	return clamp01(share)
}

// SafeFloorPct is the share of a vault a hack never takes.
func SafeFloorPct(factionID int, s Settings) float64 {
	if factionID == factionBastion {
		return s.BasePerkBastionFloorPct
	}
	return s.BaseSafeFloorPct
}

// HackPct is the share a hack takes, between BaseHackMinPct and BaseHackMaxPct (roll is 0 to 1).
func HackPct(roll float64, s Settings) float64 {
	lo, hi := math.Min(s.BaseHackMinPct, s.BaseHackMaxPct), math.Max(s.BaseHackMinPct, s.BaseHackMaxPct)
	return lo + (hi-lo)*clamp01(roll)
}

// HackSteal is what a hack takes of each resource: pct of what lies above the safe floor, rounded down.
func HackSteal(vault [4]int64, pct, floorPct float64) [4]int64 {
	var out [4]int64
	for i, v := range vault {
		above := float64(v) * (1 - clamp01(floorPct/100))
		out[i] = int64(math.Floor(above * clamp01(pct/100)))
	}
	return out
}

// HackCooldownOver says whether a player may hack this base again.
func HackCooldownOver(last *time.Time, now time.Time, s Settings) bool {
	return last == nil || !now.Before(last.Add(time.Duration(s.BaseHackCooldownMin*float64(time.Minute))))
}

// PackedRefund is what a module packed away by a tier drop returns to the vault: BasePackedRefundPct of what it cost
// (each Mk's bill up to its own).
func PackedRefund(kind string, mk int, s Settings) [4]int64 {
	var total [4]int64
	for m := 1; m <= mk; m++ {
		bill := ModuleBill(kind, m, s)
		for i := range total {
			total[i] += int64(bill[i])
		}
	}
	for i := range total {
		total[i] = int64(math.Floor(float64(total[i]) * s.BasePackedRefundPct / 100))
	}
	return total
}

// DestroyDrop is what a destroyed Spire leaves on the map: BaseDestroyDropPct of its vault.
func DestroyDrop(vault [4]int64, s Settings) [4]int64 {
	var out [4]int64
	for i, v := range vault {
		out[i] = int64(math.Floor(float64(v) * s.BaseDestroyDropPct / 100))
	}
	return out
}

// BaseRepairCost is the alloy to repair this many points (the TN repair rule).
func BaseRepairCost(points int, s Settings) int64 {
	return int64(math.Ceil(float64(max(0, points)) * s.RepairAlloyPerPoint))
}

// BaseAttackRange is how far from a base's centre an attacker may stand: its zone plus the weapon range.
func BaseAttackRange(zoneM float64, s Settings) float64 { return zoneM + s.WeaponRangeM }

// Base combat notices. Country-neutral; the place is the base's name.

func NoticeBaseAttacked(base string) string {
	return base + " is under attack. Walk to it to defend it."
}

func NoticeBaseTierLost(base, tier string) string {
	return base + " fell back to a " + tier + ". Its outer modules were packed away."
}

func NoticeBaseDestroyed(base string) string {
	return base + " was destroyed. Half its vault fell on the map; the Undersignal will reclaim it."
}
