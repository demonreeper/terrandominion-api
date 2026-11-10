package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

func TestLevels(t *testing.T) {
	s := DefaultSettings()
	if XPForLevel(1, s) != 0 || XPForLevel(2, s) != 100 {
		t.Fatalf("level 2 should need 100 XP, got %d", XPForLevel(2, s))
	}
	if got := LevelForXP(99, s); got != 1 {
		t.Fatalf("99 XP should be level 1, got %d", got)
	}
	if got := LevelForXP(100, s); got != 2 {
		t.Fatalf("100 XP should be level 2, got %d", got)
	}
	p := ProgressFor(150, s)
	if p.Level != 2 || p.LevelXP != 50 || p.LevelNeed != 125 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func harvestBase() HarvestInput {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	return HarvestInput{
		Settings:           DefaultSettings(),
		Player:             Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 10, At: now},
		NodeLat:            4.1755,
		NodeLon:            73.5095,
		NodeYield:          20,
		NodeReserve:        1,
		ExtractorYieldMult: 1,
		CarryCapacity:      200,
	}
}

func TestHarvestRules(t *testing.T) {
	in := harvestBase()
	res, err := EvaluateHarvest(in)
	if err != nil || res.Amount != 20 {
		t.Fatalf("expected 20, got %d %v", res.Amount, err)
	}

	far := harvestBase()
	far.NodeLon = 73.5110 // ~190 m east
	if _, err := EvaluateHarvest(far); !isErr(err, apperr.HarvestTooFar) {
		t.Fatalf("expected too far, got %v", err)
	}

	weak := harvestBase()
	weak.Player.AccuracyM = 120
	if _, err := EvaluateHarvest(weak); !isErr(err, apperr.HarvestGPSWeak) {
		t.Fatalf("expected weak GPS, got %v", err)
	}

	cool := harvestBase()
	last := cool.Player.At.Add(-time.Minute)
	cool.LastAtNode = &last
	if _, err := EvaluateHarvest(cool); !isErr(err, apperr.HarvestCooldown) {
		t.Fatalf("expected cooldown, got %v", err)
	}

	teleport := harvestBase()
	teleport.LastAnyFix = &Fix{Lat: 4.0, Lon: 73.0, AccuracyM: 10, At: teleport.Player.At.Add(-time.Minute)}
	if _, err := EvaluateHarvest(teleport); err == nil || err.Code != apperr.HarvestImplausible.Code {
		t.Fatalf("expected implausible travel, got %v", err)
	}

	full := harvestBase()
	full.CarriedTotal = 195
	res, err = EvaluateHarvest(full)
	if err != nil || res.Amount != 5 {
		t.Fatalf("expected carry cap of 5, got %d %v", res.Amount, err)
	}
}

func TestFactionRules(t *testing.T) {
	s := DefaultSettings()
	ready := PlayerFactionState{Level: 20}
	if err := CanLead(ready, FactionState{Locked: true}, s); err != nil {
		t.Fatalf("should be able to lead: %v", err)
	}
	if err := CanLead(PlayerFactionState{Level: 5}, FactionState{Locked: true}, s); !isErr(err, apperr.FactionLevelLow) {
		t.Fatalf("expected level too low, got %v", err)
	}
	if err := CanJoin(ready, FactionState{Locked: true}, s); !isErr(err, apperr.FactionLocked) {
		t.Fatalf("expected locked, got %v", err)
	}
	if err := CanJoin(ready, FactionState{Locked: false, HasLeader: true}, s); err != nil {
		t.Fatalf("should be able to join: %v", err)
	}
}
