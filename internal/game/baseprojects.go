package game

import (
	"math"
	"time"
)

// Faction bases, B2 (base-building-plan.md 4 and 5): projects with a bill and an assembly time, giving on site,
// the presence speed-up, contribution points and the first four modules.

// Modules buildable now (section 5): Energy Lattice, Shield Matrix, Vault and Uplink from tier 1 (B2), the Collector
// Array from tier 2 (B4). Sensor Net, Drone Hangar, Fabricator and Command Relay come with B8.
var BaseModules = []string{"lattice", "shield", "vault", "uplink", "collector"}

// ModuleMinTier is the tier a module can first be built at.
func ModuleMinTier(kind string) int {
	if kind == "collector" {
		return 2
	}
	return 1
}

// CollectorLinks is how many free nodes inside the zone one Collector Array tethers at a Mk.
func CollectorLinks(mk int, s Settings) int {
	if mk < 1 || mk > MaxModuleMk {
		return 0
	}
	return s.BaseCollectorLinks[mk]
}

// CollectorAmount is what one collector harvest puts into the vault: the node's TN harvest, more crystal for Lumen
// Collective (faction 1).
func CollectorAmount(amount int64, resource string, factionID int, s Settings) int64 {
	if factionID == 1 && resource == "crystal" && s.BasePerkLumenCrystalPct > 0 {
		return int64(math.Round(float64(amount) * (1 + s.BasePerkLumenCrystalPct/100)))
	}
	return amount
}

// ModuleName names a module for players.
func ModuleName(kind string) string {
	switch kind {
	case "lattice":
		return "Energy Lattice"
	case "shield":
		return "Shield Matrix"
	case "vault":
		return "Vault"
	case "uplink":
		return "Uplink"
	case "collector":
		return "Collector Array"
	}
	return ""
}

// ValidModule says whether a module can be built now.
func ValidModule(kind string) bool {
	for _, m := range BaseModules {
		if m == kind {
			return true
		}
	}
	return false
}

// MaxModuleMk is the top Mk (I to IV, like TN gear).
const MaxModuleMk = 4

// ModuleBill is the bill to build a module at Mk 1, or to upgrade it to mk: the Mk I bill times mk.
func ModuleBill(kind string, mk int, s Settings) [4]int {
	base, ok := s.BaseModuleCost[kind]
	if !ok || mk < 1 {
		return [4]int{}
	}
	var out [4]int
	for i := range base {
		out[i] = base[i] * mk
	}
	return out
}

// ModuleAssembly is a module's assembly at a Mk: the Mk I time times mk.
func ModuleAssembly(kind string, mk int, s Settings) time.Duration {
	return time.Duration(s.BaseModuleMin[kind] * float64(mk) * float64(time.Minute))
}

// ModuleHealth is a module's full health at a Mk.
func ModuleHealth(mk int, s Settings) int { return s.BaseModuleHealth * mk }

// Faction perks on bases (section 5, every number a setting). Faction ids: 2 Forge Meridian (assembly faster),
// 3 Verdant Exchange (projects cheaper).
const (
	factionForge   = 2
	factionVerdant = 3
)

// ProjectBill is a bill after the faction's perk (Verdant Exchange pays less).
func ProjectBill(bill [4]int, factionID int, s Settings) [4]int {
	if factionID != factionVerdant || s.BasePerkVerdantCostPct <= 0 {
		return bill
	}
	var out [4]int
	for i, v := range bill {
		out[i] = int(math.Ceil(float64(v) * (100 - s.BasePerkVerdantCostPct) / 100))
	}
	return out
}

// AssemblyRate is how many seconds of assembly pass per second: 1, plus BasePresenceStepPct per member on site, up to
// BasePresenceMaxMult; Forge Meridian builds faster on top.
func AssemblyRate(factionID, present int, s Settings) float64 {
	rate := 1 + float64(max(0, present))*s.BasePresenceStepPct/100
	if s.BasePresenceMaxMult > 0 {
		rate = math.Min(rate, s.BasePresenceMaxMult)
	}
	if factionID == factionForge && s.BasePerkForgeAssemblyPct > 0 {
		rate *= 1 + s.BasePerkForgeAssemblyPct/100
	}
	return rate
}

// Give caps what a player offers to what the bill still needs and what they hold, per resource (alloy, crystal,
// data, credit).
func Give(offer, have, bill, given [4]int64) [4]int64 {
	var out [4]int64
	for i := range offer {
		need := bill[i] - given[i]
		out[i] = max(0, min(offer[i], have[i], need))
	}
	return out
}

// Total adds up four amounts.
func Total(a [4]int64) int64 { return a[0] + a[1] + a[2] + a[3] }

// Funded says whether a bill is fully given.
func Funded(bill, given [4]int64) bool {
	for i := range bill {
		if given[i] < bill[i] {
			return false
		}
	}
	return true
}

// GivePoints is what giving earns (resources from the player's own storage; vault payments earn none).
func GivePoints(given [4]int64, s Settings) int64 {
	return int64(math.Floor(float64(Total(given)) * s.BasePointsPerUnit))
}

// GiveXP is the little XP giving earns.
func GiveXP(given [4]int64, s Settings) int {
	return int(Total(given) * int64(s.BaseGiveXpPer100) / 100)
}

// CanCancelProject: whoever started it, or a higher rank.
func CanCancelProject(isStarter bool, rank, starterRank int) bool {
	return isStarter || rank > starterRank
}

// StartPower is the power a project needs to start: a Citadel (tier 4) needs a lead, a Capital the Commander.
func StartPower(kind string, toTier int) Power {
	if kind == "tier" && toTier >= BaseTopTier {
		return PowerApproveBase
	}
	if kind == "tier" && toTier == BaseTopTierNonCapital {
		return PowerStartT4
	}
	return PowerStartProjects
}
