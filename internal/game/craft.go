package game

import "math"

// Recipe is one consumable a player can craft.
type Recipe struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	Cost map[string]int64 `json:"cost"`
}

// Recipes lists the consumables in display order, priced from the settings.
func Recipes(s Settings) []Recipe {
	return []Recipe{
		{ID: "energy_cell", Name: "Energy Cell", Cost: positive(map[string]int{
			"crystal": s.CraftCellCrystal, "data": s.CraftCellData,
		})},
		{ID: "em_pulse", Name: "EM Pulse", Cost: positive(map[string]int{
			"crystal": s.CraftPulseCrystal, "data": s.CraftPulseData,
		})},
		{ID: "signal_jammer", Name: "Signal Jammer", Cost: positive(map[string]int{
			"data": s.CraftJammerData, "alloy": s.CraftJammerAlloy,
		})},
		{ID: "overload_spike", Name: "Overload Spike", Cost: positive(map[string]int{
			"alloy": s.CraftSpikeAlloy, "crystal": s.CraftSpikeCrystal, "credit": s.CraftSpikeCredit,
		})},
	}
}

// RecipeByID finds a recipe; ok is false for an unknown id.
func RecipeByID(s Settings, id string) (Recipe, bool) {
	for _, recipe := range Recipes(s) {
		if recipe.ID == id {
			return recipe, true
		}
	}
	return Recipe{}, false
}

// RepairCost is the alloy to bring gear back to full, rounded up. Empty when nothing is missing.
func RepairCost(s Settings, durability, maxDurability int) map[string]int64 {
	missing := maxDurability - durability
	if missing <= 0 || s.RepairAlloyPerPoint <= 0 {
		return map[string]int64{}
	}
	return map[string]int64{"alloy": int64(math.Ceil(float64(missing) * s.RepairAlloyPerPoint))}
}

func positive(amounts map[string]int) map[string]int64 {
	cost := map[string]int64{}
	for resource, amount := range amounts {
		if amount > 0 {
			cost[resource] = int64(amount)
		}
	}
	return cost
}
