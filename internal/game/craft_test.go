package game

import "testing"

func TestRecipesMatchThePlan(t *testing.T) {
	s := DefaultSettings()
	want := map[string]map[string]int64{
		"energy_cell":    {"crystal": 10, "data": 5},
		"em_pulse":       {"crystal": 20, "data": 10},
		"signal_jammer":  {"data": 15, "alloy": 10},
		"overload_spike": {"alloy": 30, "crystal": 20, "credit": 10},
		"tn_shield_mk1":  {"crystal": 30, "alloy": 20},
		"tn_shield_mk2":  {"alloy": 80, "data": 40},
		"tn_shield_mk3":  {"alloy": 160, "data": 80, "credit": 40},
		"tn_shield_mk4":  {"alloy": 800, "data": 400, "credit": 200},
	}
	if got := len(Recipes(s)); got != len(want) {
		t.Fatalf("got %d recipes, want %d", got, len(want))
	}
	for id, cost := range want {
		recipe, ok := RecipeByID(s, id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		for resource, amount := range cost {
			if recipe.Cost[resource] != amount {
				t.Errorf("%s %s = %d, want %d", id, resource, recipe.Cost[resource], amount)
			}
		}
	}
	if _, ok := RecipeByID(s, "nope"); ok {
		t.Error("unknown recipe was found")
	}
}

func TestRecipeDropsZeroCosts(t *testing.T) {
	s := DefaultSettings()
	s.CraftCellData = 0
	recipe, _ := RecipeByID(s, "energy_cell")
	if _, ok := recipe.Cost["data"]; ok {
		t.Error("a zero cost should not be listed")
	}
}

func TestRepairCost(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		durability, max int
		alloy           int64
	}{
		{0, 100, 50},
		{99, 100, 1},  // half a point rounds up
		{97, 100, 2},  // 1.5 rounds up
		{100, 100, 0}, // nothing to repair
	}
	for _, c := range cases {
		if got := RepairCost(s, c.durability, c.max)["alloy"]; got != c.alloy {
			t.Errorf("RepairCost(%d/%d) = %d, want %d", c.durability, c.max, got, c.alloy)
		}
	}
}
