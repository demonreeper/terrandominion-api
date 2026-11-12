package game

import (
	"testing"
	"time"
)

func TestPoweredSlots(t *testing.T) {
	s := DefaultSettings()
	slots := []PowerSlot{
		{1, 0, "lattice", 300}, {1, 1, "shield", 300}, {1, 2, "vault", 300}, {1, 3, "uplink", 300},
		{1, 4, "collector", 300}, {1, 5, "shield", 0}, {2, 0, "vault", 300}, {2, 1, "", 0},
	}
	on := PoweredSlots(slots, s)
	for _, k := range [][2]int{{1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4}} {
		if !on[k] {
			t.Errorf("%v should be powered", k)
		}
	}
	if on[[2]int{1, 5}] || on[[2]int{2, 0}] {
		t.Error("a broken module and the fifth module stay dark")
	}
	slots[0].Health = 0
	if len(PoweredSlots(slots, s)) != 0 {
		t.Error("the Lattice down takes every module offline")
	}
}

func TestBaseShieldAndHack(t *testing.T) {
	s := DefaultSettings()
	if BaseShieldShare(0, 1, 6, s) != 0 || BaseShieldShare(2, 1, 1, s) != ShieldShare(s, 2, 1) ||
		BaseShieldShare(2, 1, 6, s) <= BaseShieldShare(2, 1, 1, s) {
		t.Fatal("shield matrix and the Bastion perk")
	}
	if SafeFloorPct(6, s) != 35 || SafeFloorPct(2, s) != 20 || HackPct(0, s) != 5 || HackPct(1, s) != 15 {
		t.Fatal("safe floor and hack share")
	}
	if HackSteal([4]int64{1000, 100, 0, 9}, 10, 20) != [4]int64{80, 8, 0, 0} {
		t.Fatalf("steal above the floor: %v", HackSteal([4]int64{1000, 100, 0, 9}, 10, 20))
	}
	now := time.Now()
	last := now.Add(-29 * time.Minute)
	if HackCooldownOver(&last, now, s) || !HackCooldownOver(nil, now, s) {
		t.Fatal("hack cooldown")
	}
	if PackedRefund("lattice", 2, s) != [4]int64{450, 300, 150, 75} || DestroyDrop([4]int64{100, 3, 0, 0}, s) != [4]int64{50, 1, 0, 0} {
		t.Fatal("refund and destroy drop")
	}
	if BaseRepairCost(11, s) != 6 || BaseAttackRange(25, s) != 65 {
		t.Fatal("repair and range")
	}
}
