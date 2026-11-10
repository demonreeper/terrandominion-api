package game

import (
	"testing"
	"time"
)

func TestShieldShareTable(t *testing.T) {
	s := DefaultSettings()
	// The plan's table: rows are shield Mk I-IV, columns attack Mk I-IV.
	want := [4][4]float64{
		{0.30, 0.15, 0.05, 0.05},
		{0.45, 0.30, 0.15, 0.05},
		{0.60, 0.45, 0.30, 0.15},
		{0.75, 0.60, 0.45, 0.30},
	}
	for shield := 1; shield <= 4; shield++ {
		for attack := 1; attack <= 4; attack++ {
			if got := ShieldShare(s, shield, attack); got < want[shield-1][attack-1]-1e-9 || got > want[shield-1][attack-1]+1e-9 {
				t.Errorf("shield Mk %d vs attack Mk %d: got %.2f, want %.2f", shield, attack, got, want[shield-1][attack-1])
			}
		}
	}
	if ShieldShare(s, 0, 1) != 0 {
		t.Error("no shield should stop nothing")
	}
}

func TestInterceptShares(t *testing.T) {
	s := DefaultSettings()
	for mk, want := range map[int]float64{1: 1, 2: 0.75, 3: 0.5, 4: 0.25} {
		if got := InterceptShare(s, mk); got != want {
			t.Errorf("Mk %d: got %v, want %v", mk, got, want)
		}
	}
}

func TestHitDamagePlanExamples(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		name               string
		damage             float64
		attackMk, shieldMk int
		intercepted        bool
		want               float64
	}{
		{"Mk IV drone on a Mk IV shield", 100, 4, 4, false, 70},
		{"Mk IV drone on a Mk IV shield, intercepted", 100, 4, 4, true, 53},
		{"Mk IV drone on a Mk I shield", 100, 4, 1, false, 95},
		{"Mk IV drone on a Mk I shield, intercepted", 100, 4, 1, true, 72},
		{"EM Pulse on a Mk II shield", 25, WeaponMk, 2, false, 14},
		{"Overload Spike on a Mk IV shield", 40, WeaponMk, 4, false, 10},
		{"Mk I drone on a Mk I shield", 15, 1, 1, false, 11},
		{"Mk I drone, intercepted, is shot down", 15, 1, 0, true, 0},
		{"no shield, no intercept", 20, 2, 0, false, 20},
	}
	for _, c := range cases {
		intercept := 0.0
		if c.intercepted {
			intercept = InterceptShare(s, c.attackMk)
		}
		got := HitDamage(c.damage, intercept, ShieldShare(s, c.shieldMk, c.attackMk))
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNoOneHitKillOnAShield(t *testing.T) {
	if got := CapShieldedHit(100, 100, true); got != 99 {
		t.Fatalf("a full shielded node should keep 1 health, took %v", got)
	}
	if got := CapShieldedHit(30, 70, true); got != 29 {
		t.Fatalf("decay doesn't let one hit through, took %v", got)
	}
	if got := CapShieldedHit(1, 70, true); got != 70 {
		t.Fatalf("a second hit can destroy it, took %v", got)
	}
	if got := CapShieldedHit(30, 70, false); got != 70 {
		t.Fatalf("no shield, no floor, took %v", got)
	}
}

func TestShieldedJam(t *testing.T) {
	s := DefaultSettings()
	got := ShieldedJam(30*time.Minute, ShieldShare(s, 2, WeaponMk))
	if got.Round(time.Minute) != 17*time.Minute {
		t.Fatalf("a 30 min jam on a Mk II shield should last about 17 min, got %v", got)
	}
}

func TestShieldItems(t *testing.T) {
	s := DefaultSettings()
	for mk := 1; mk <= MaxMk; mk++ {
		id := ShieldItemID(mk)
		if got, ok := ShieldMkOf(id); !ok || got != mk {
			t.Fatalf("%s should be Mk %d", id, mk)
		}
		recipe, ok := RecipeByID(s, id)
		if !ok || recipe.MinLevel != []int{5, 10, 15, 20}[mk-1] {
			t.Fatalf("%s recipe wrong: %+v", id, recipe)
		}
	}
	if _, ok := ShieldMkOf("em_pulse"); ok {
		t.Fatal("a weapon is not a shield")
	}
}
