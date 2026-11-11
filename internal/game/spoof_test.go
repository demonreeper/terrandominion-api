package game

import (
	"testing"
	"time"
)

func TestTeleportJump(t *testing.T) {
	s := DefaultSettings()
	at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	male := &Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 10, At: at}
	// Malé to Addu (~480 km) in a minute: a teleport.
	if j, yes := TeleportJump(s, male, Fix{Lat: -0.6, Lon: 73.1, AccuracyM: 10, At: at.Add(time.Minute)}); !yes || j.Km < 400 {
		t.Errorf("a 480 km jump in a minute is a teleport, got %+v %v", j, yes)
	}
	// A 1 km glitch at good accuracy is not.
	if _, yes := TeleportJump(s, male, Fix{Lat: 4.1845, Lon: 73.5093, AccuracyM: 10, At: at.Add(time.Second)}); yes {
		t.Error("a 1 km glitch is not a teleport")
	}
	// A flight, gate to gate (last fix before boarding, first after landing), is not.
	if _, yes := TeleportJump(s, male, Fix{Lat: -0.6, Lon: 73.1, AccuracyM: 20, At: at.Add(150 * time.Minute)}); yes {
		t.Error("two and a half hours for 480 km is a flight")
	}
	// Poor accuracy is taken off: 6 km apart with 3 km accuracy on each fix is no jump at all.
	if _, yes := TeleportJump(s, &Fix{Lat: 4.17, Lon: 73.5, AccuracyM: 3000, At: at},
		Fix{Lat: 4.224, Lon: 73.5, AccuracyM: 3000, At: at.Add(time.Second)}); yes {
		t.Error("accuracy is taken off both fixes")
	}
	if _, yes := TeleportJump(s, nil, *male); yes {
		t.Error("no last fix, no teleport")
	}
}

func TestFixStale(t *testing.T) {
	s := DefaultSettings()
	now := time.Now()
	old, future, fresh := now.Add(-3*time.Minute), now.Add(3*time.Minute), now.Add(-20*time.Second)
	if !FixStale(s, &old, now) || !FixStale(s, &future, now) || FixStale(s, &fresh, now) || FixStale(s, nil, now) {
		t.Error("older than 2 minutes or from the future is stale; a missing time is not")
	}
}

func TestLockStepsUp(t *testing.T) {
	s := DefaultSettings()
	for earlier, want := range map[int]int{0: 30, 1: 120, 2: 720, 5: 720} {
		if got := LockMinutes(s, earlier); got != want {
			t.Errorf("after %d earlier incidents: %d minutes, want %d", earlier, got, want)
		}
	}
	if LockLine(0) != LockLineFirst || LockLine(1) != LockLineRepeat {
		t.Error("the first line once, then the repeat line")
	}
}
