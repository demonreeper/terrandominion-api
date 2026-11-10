package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

func TestWeaponsMatchThePlan(t *testing.T) {
	s := DefaultSettings()
	pulse, _ := WeaponByID(s, WeaponEmPulse)
	if pulse.Damage != 25 || pulse.Cells != 1 {
		t.Errorf("pulse = %+v", pulse)
	}
	jammer, _ := WeaponByID(s, WeaponJammer)
	if jammer.Jam != 30*time.Minute || jammer.Damage != 0 {
		t.Errorf("jammer = %+v", jammer)
	}
	spike, _ := WeaponByID(s, WeaponSpike)
	if spike.Damage != 40 || spike.Fuse != 10*time.Minute {
		t.Errorf("spike = %+v", spike)
	}
	if _, ok := WeaponByID(s, "drone_strike"); ok {
		t.Error("drone strike waits for a later phase")
	}
}

func TestCheckInRange(t *testing.T) {
	s := DefaultSettings()
	const lat, lon = 4.1755, 73.5093
	const degreeM = 111320.0
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	at := func(metres, accuracy float64) Fix {
		return Fix{Lat: lat + metres/degreeM, Lon: lon, AccuracyM: accuracy, At: now}
	}
	if _, err := CheckInRange(s, at(30, 5), nil, lat, lon, 40); err != nil {
		t.Errorf("30 m away: %v", err)
	}
	if _, err := CheckInRange(s, at(60, 5), nil, lat, lon, 40); !isErr(err, apperr.OutOfRange) {
		t.Errorf("60 m at 5 m accuracy: got %v", err)
	}
	if _, err := CheckInRange(s, at(10, 0), nil, lat, lon, 40); !isErr(err, apperr.HarvestGPSWeak) {
		t.Errorf("no accuracy: got %v", err)
	}
	// A poor fix earns allowance, capped at the range itself.
	if _, err := CheckInRange(s, at(79, 45), nil, lat, lon, 40); err != nil {
		t.Errorf("79 m at 45 m accuracy: %v", err)
	}
	if _, err := CheckInRange(s, at(85, 45), nil, lat, lon, 40); !isErr(err, apperr.OutOfRange) {
		t.Errorf("85 m at 45 m accuracy: got %v", err)
	}
}

func TestDroneFlight(t *testing.T) {
	s := DefaultSettings()
	if got := DroneFlight(s, 900); got != 60*time.Second {
		t.Errorf("900 m took %v, want 60s", got)
	}
	if got := DroneFlight(s, 30); got != 20*time.Second {
		t.Errorf("a short hop took %v, want the 20s minimum", got)
	}
}

func TestContestScore(t *testing.T) {
	s := DefaultSettings()
	base := ContestSide{Level: 3, GearTiers: 2}
	if got := ContestScore(s, base); got != 13 {
		t.Errorf("base = %d, want 13", got)
	}
	base.Allies, base.Showed = 2, true
	if got := ContestScore(s, base); got != 28 {
		t.Errorf("with allies and showing up = %d, want 28", got)
	}
	if ChallengerWins(10, 10) {
		t.Error("a tie should go to the defender")
	}
	if !ChallengerWins(11, 10) {
		t.Error("the higher challenger should win")
	}
}
