package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

func tetherBase() TetherBuildInput {
	return TetherBuildInput{
		Settings: DefaultSettings(),
		Player:   Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 10, At: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)},
		Carried:  map[string]int64{"crystal": 80, "alloy": 60, "data": 20},
		Uploaded: map[string]int64{},
	}
}

func TestTetherBuildPaysCarriedFirst(t *testing.T) {
	in := tetherBase()
	in.Carried = map[string]int64{"crystal": 50, "alloy": 60, "data": 20}
	in.Uploaded = map[string]int64{"crystal": 40, "alloy": 500}
	pay, err := EvaluateTetherBuild(in)
	if err != nil {
		t.Fatalf("should be able to build: %v", err)
	}
	if pay.FromCarried["crystal"] != 50 || pay.FromUploaded["crystal"] != 30 {
		t.Fatalf("crystal should be 50 carried + 30 uploaded, got %+v", pay)
	}
	if pay.FromCarried["alloy"] != 60 || pay.FromUploaded["alloy"] != 0 {
		t.Fatalf("alloy should come only from carried, got %+v", pay)
	}
}

func TestTetherBuildRules(t *testing.T) {
	if _, err := EvaluateTetherBuild(tetherBase()); err != nil {
		t.Fatalf("exact cost should be enough: %v", err)
	}

	short := tetherBase()
	short.Carried["data"] = 19
	if _, err := EvaluateTetherBuild(short); err == nil || err.Code != apperr.TetherCost.Code {
		t.Fatalf("expected not enough resources, got %v", err)
	}

	full := tetherBase()
	full.ActiveOwned = full.Settings.TnMaxPerPlayer
	if _, err := EvaluateTetherBuild(full); err != apperr.TetherCap {
		t.Fatalf("expected cap reached, got %v", err)
	}

	weak := tetherBase()
	weak.Player.AccuracyM = 120
	if _, err := EvaluateTetherBuild(weak); err != apperr.HarvestGPSWeak {
		t.Fatalf("expected weak GPS, got %v", err)
	}

	teleport := tetherBase()
	teleport.LastAnyFix = &Fix{Lat: 4.0, Lon: 73.0, AccuracyM: 10, At: teleport.Player.At.Add(-time.Minute)}
	if _, err := EvaluateTetherBuild(teleport); err == nil || err.Code != apperr.HarvestImplausible.Code {
		t.Fatalf("expected implausible travel, got %v", err)
	}
}

func TestTetherRefund(t *testing.T) {
	got := TetherRefund(map[string]int64{"crystal": 80, "alloy": 60, "data": 21}, 0.5)
	if got["crystal"] != 40 || got["alloy"] != 30 || got["data"] != 10 {
		t.Fatalf("unexpected refund %+v", got)
	}
}
