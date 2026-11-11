package game

import (
	"fmt"
	"math"
)

// Faction perks (docs/profile-plan.md, P1b). Each faction gives its members one or two perks that
// fit systems the game already has. They apply only while the player is a member and stop the
// moment they leave. Every number is a setting (Perk<Name>), tuned from the admin settings screen.

// Perk keys, as the app knows them: where a perk changes a number the player sees, the app shows
// that perk's tag beside it.
const (
	PerkCrystal       = "crystal"        // more crystal from harvests and Tether Node collects
	PerkUpgradeCost   = "upgrade_cost"   // cheaper Tether Node upgrades
	PerkScan          = "scan"           // longer scan range
	PerkRare          = "rare"           // rare roaming nodes seen from further
	PerkBuildCost     = "build_cost"     // cheaper Tether Node builds
	PerkMissionCredit = "mission_credit" // more credit from missions
	PerkDroneSpeed    = "drone_speed"    // faster drones
	PerkCarry         = "carry"          // more carry capacity
	PerkDamage        = "damage"         // own Tether Nodes take less damage
	PerkIntercept     = "intercept"      // intercepts stop more
)

// Perks is what a player's faction changes for them. The zero-perk value (NoPerks) changes nothing:
// multipliers are 1 and the bonus is 0.
type Perks struct {
	CrystalMult       float64 // crystal from a harvest or a Tether Node collect
	UpgradeCostMult   float64 // Tether Node upgrade cost
	ScanRangeMult     float64 // scan range
	RareRangeMult     float64 // how far rare roaming nodes are seen, as a multiple of the unperked scan range
	BuildCostMult     float64 // Tether Node build cost
	MissionCreditMult float64 // credit a mission pays
	DroneSpeedMult    float64 // drone flight speed
	CarryMult         float64 // carry capacity
	DamageTakenMult   float64 // damage the player's own Tether Nodes take
	InterceptBonus    float64 // added to the share of a drone's damage an intercept stops
}

// NoPerks is a freelancer's, or a member of a faction with no perk in a given system.
func NoPerks() Perks {
	return Perks{CrystalMult: 1, UpgradeCostMult: 1, ScanRangeMult: 1, RareRangeMult: 1, BuildCostMult: 1,
		MissionCreditMult: 1, DroneSpeedMult: 1, CarryMult: 1, DamageTakenMult: 1}
}

func more(pct int) float64 { return 1 + float64(max(0, pct))/100 }
func less(pct int) float64 { return 1 - float64(min(100, max(0, pct)))/100 }

// FactionPerks is what membership of the faction with this id gives (0 = freelancer: nothing).
// The ids are the powers' (PowerSignum and so on).
func FactionPerks(s Settings, factionID int) Perks {
	p := NoPerks()
	switch factionID {
	case PowerSignum:
		p.CrystalMult = more(s.PerkSignumCrystalPct)
	case PowerShingo:
		p.UpgradeCostMult = less(s.PerkShingoUpgradePct)
	case PowerSema:
		p.ScanRangeMult = more(s.PerkSemaScanPct)
		// Rare nodes are never seen from nearer than everything else.
		p.RareRangeMult = math.Max(p.ScanRangeMult, more(s.PerkSemaRarePct))
	case PowerIshara:
		p.BuildCostMult = less(s.PerkIsharaBuildPct)
		p.MissionCreditMult = more(s.PerkIsharaMissionCreditPct)
	case PowerTohu:
		p.DroneSpeedMult = more(s.PerkTohuDroneSpeedPct)
		p.CarryMult = more(s.PerkTohuCarryPct)
	case PowerMerki:
		p.DamageTakenMult = less(s.PerkMerkiDamagePct)
		p.InterceptBonus = float64(min(100, max(0, s.PerkMerkiInterceptPct))) / 100
	}
	return p
}

// PerkLine is one perk as players read it: Line on the FACTIONS screen and a profile
// ("+25% CRYSTAL FROM HARVESTS AND TETHER NODES"), Tag beside a number it changed in play
// ("+25% SIGNUM"). Key is one of the Perk constants.
type PerkLine struct {
	Key  string `json:"key"`
	Line string `json:"line"`
	Tag  string `json:"tag"`
}

// perkTagName is the short name on a perk's tag: the power whose faction gives it. One name per
// tag, so nothing links the six.
var perkTagName = map[int]string{
	PowerSignum: "SIGNUM", PowerShingo: "SHINGŌ", PowerSema: "SĒMA",
	PowerIshara: "ISHĀRA", PowerTohu: "TOHU", PowerMerki: "MERKI",
}

// FactionPerkLines lists a faction's perks with their numbers, for the FACTIONS screen, /v1/me and
// profiles. A perk tuned to 0 is left out. Unknown factions have none.
func FactionPerkLines(s Settings, factionID int) []PerkLine {
	name := perkTagName[factionID]
	lines := []PerkLine{}
	add := func(key string, pct int, sign, text string) {
		if pct <= 0 {
			return
		}
		lines = append(lines, PerkLine{
			Key:  key,
			Line: fmt.Sprintf("%s%d%% %s", sign, pct, text),
			Tag:  fmt.Sprintf("%s%d%% %s", sign, pct, name),
		})
	}
	switch factionID {
	case PowerSignum:
		add(PerkCrystal, s.PerkSignumCrystalPct, "+", "CRYSTAL FROM HARVESTS AND TETHER NODES")
	case PowerShingo:
		add(PerkUpgradeCost, s.PerkShingoUpgradePct, "-", "TETHER NODE UPGRADE COST")
	case PowerSema:
		add(PerkScan, s.PerkSemaScanPct, "+", "SCAN RANGE")
		add(PerkRare, s.PerkSemaRarePct, "+", "RANGE TO SEE RARE NODES")
	case PowerIshara:
		add(PerkBuildCost, s.PerkIsharaBuildPct, "-", "TETHER NODE BUILD COST")
		add(PerkMissionCredit, s.PerkIsharaMissionCreditPct, "+", "CREDIT FROM MISSIONS")
	case PowerTohu:
		add(PerkDroneSpeed, s.PerkTohuDroneSpeedPct, "+", "DRONE SPEED")
		add(PerkCarry, s.PerkTohuCarryPct, "+", "CARRY CAPACITY")
	case PowerMerki:
		add(PerkDamage, s.PerkMerkiDamagePct, "-", "DAMAGE TO YOUR TETHER NODES")
		add(PerkIntercept, s.PerkMerkiInterceptPct, "+", "DRONE DAMAGE STOPPED BY AN INTERCEPT")
	}
	return lines
}

// PerkTag is the tag of one of a faction's perks ("+25% SIGNUM"), or "" when the faction has no
// such perk: what the server puts on an outcome whose number a perk changed.
func PerkTag(s Settings, factionID int, key string) string {
	for _, line := range FactionPerkLines(s, factionID) {
		if line.Key == key {
			return line.Tag
		}
	}
	return ""
}

// PerkAmount applies a "more" multiplier to a whole amount, rounding to the nearest; an amount of
// at least 1 never drops below 1.
func PerkAmount(amount int64, mult float64) int64 {
	if mult == 1 || amount <= 0 {
		return amount
	}
	return max(1, int64(math.Round(float64(amount)*mult)))
}

// PerkCost applies a "less" multiplier to a cost, rounding up so a discount never makes something
// free unless the perk is 100%.
func PerkCost(cost map[string]int64, mult float64) map[string]int64 {
	if mult == 1 {
		return cost
	}
	out := make(map[string]int64, len(cost))
	for resource, amount := range cost {
		if discounted := int64(math.Ceil(float64(amount) * mult)); discounted > 0 {
			out[resource] = discounted
		}
	}
	return out
}

// PerkInterceptShare is the share of a drone's damage an intercept stops, with a perk's bonus,
// never above everything.
func PerkInterceptShare(share, bonus float64) float64 {
	return math.Min(1, share+bonus)
}

// SettingsWithPerks is the rules as they stand for one player: the costs and the drone speed their
// faction changes are folded in, so the rule functions need no second argument. Everything else is
// untouched.
func SettingsWithPerks(s Settings, p Perks) Settings {
	discount := func(amount int, mult float64) int {
		if mult == 1 || amount <= 0 {
			return amount
		}
		return int(math.Ceil(float64(amount) * mult))
	}
	s.TnBuildCrystal = discount(s.TnBuildCrystal, p.BuildCostMult)
	s.TnBuildAlloy = discount(s.TnBuildAlloy, p.BuildCostMult)
	s.TnBuildData = discount(s.TnBuildData, p.BuildCostMult)
	s.TnUpgradeAlloy = discount(s.TnUpgradeAlloy, p.UpgradeCostMult)
	s.TnUpgradeData = discount(s.TnUpgradeData, p.UpgradeCostMult)
	s.TnUpgradeCredit = discount(s.TnUpgradeCredit, p.UpgradeCostMult)
	s.DroneSpeedMps *= p.DroneSpeedMult
	return s
}
