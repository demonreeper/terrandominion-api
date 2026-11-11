package game

import (
	"fmt"
	"time"
)

// Anti GPS spoofing (docs/social-signal-plan.md Part C). Hard signals (the phone's fake-location flag, a
// teleport) refuse the action and start a signal lock; soft ones (an old or future fix) are refused and
// noted. The client sends signals, the server decides.

// Infringement kinds added for spoofing.
const (
	InfringementMock     = "mock_location"
	InfringementTeleport = "teleport"
	InfringementPattern  = "pattern"
	InfringementStaleFix = "stale_fix"
)

// SanctionSpoofLock is the signal lock: location actions refused until it ends. Sign-in still works.
const SanctionSpoofLock = "spoof_lock"

// SignalWatchID is the system account that files the automatic reports and gives the locks.
const SignalWatchID = 9

// ReportReasonSpoof marks a report from Signal watch.
const ReportReasonSpoof = "spoof"

// Lock lines: which narrator line the app plays.
const (
	LockLineFirst  = "first"
	LockLineRepeat = "repeat"
)

// FixReport is what a request says about where the phone is.
type FixReport struct {
	Lat, Lon  float64
	AccuracyM float64
	// Mock: the phone marked the fix as fake. Unknown (old app builds) is false.
	Mock bool
	// FixAt is the fix's own time from the phone, nil when not sent.
	FixAt *time.Time
}

// HasPosition is false for a request that names no place (a job take): only the lock is checked.
func (f FixReport) HasPosition() bool { return f.Lat != 0 || f.Lon != 0 }

// FixStale says the phone's fix is older than FixMaxAgeSeconds, or more than that in the future.
func FixStale(s Settings, fixAt *time.Time, now time.Time) bool {
	if fixAt == nil || s.FixMaxAgeSeconds <= 0 {
		return false
	}
	limit := time.Duration(s.FixMaxAgeSeconds) * time.Second
	age := now.Sub(*fixAt)
	return age > limit || age < -limit
}

// Jump is how far and how fast a player moved between two fixes, after both fixes' accuracy.
type Jump struct {
	Km, Kmh, Seconds float64
}

// TeleportJump measures the move from the last accepted fix and says whether it is a teleport: faster
// than SpoofTeleportKmh and longer than SpoofTeleportMinKm after taking off both accuracies. A flight is
// fine (hours pass); a glitch of hundreds of metres never reaches the floor.
func TeleportJump(s Settings, last *Fix, now Fix) (Jump, bool) {
	if last == nil {
		return Jump{}, false
	}
	metres := DistanceM(last.Lat, last.Lon, now.Lat, now.Lon) - last.AccuracyM - now.AccuracyM
	if metres <= 0 {
		return Jump{}, false
	}
	seconds := now.At.Sub(last.At).Seconds()
	j := Jump{Km: metres / 1000, Seconds: seconds}
	if seconds <= 0 {
		seconds = 1
	}
	j.Kmh = j.Km / (seconds / 3600)
	return j, j.Km > s.SpoofTeleportMinKm && j.Kmh > s.SpoofTeleportKmh
}

// LockMinutes is the lock for an incident when the player already had earlier counted incidents inside
// the repeat window: first, second, then the longest for every later one.
func LockMinutes(s Settings, earlier int) int {
	switch {
	case earlier <= 0:
		return s.SpoofLockMinutes1
	case earlier == 1:
		return s.SpoofLockMinutes2
	}
	return s.SpoofLockMinutes3
}

// LockLine is the narrator line for an incident: the first in the window, or a repeat.
func LockLine(earlier int) string {
	if earlier <= 0 {
		return LockLineFirst
	}
	return LockLineRepeat
}

// SpoofIncident is a hard signal as the report and the admin record describe it.
type SpoofIncident struct {
	Kind    string  `json:"kind"`
	Km      float64 `json:"km,omitempty"`
	Kmh     float64 `json:"kmh,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
}

// SpoofDetail is the line moderators read: which signal and the numbers, never a position.
func SpoofDetail(i SpoofIncident) string {
	switch i.Kind {
	case InfringementTeleport:
		return formatJump(i)
	case InfringementMock:
		return "the phone marked the fix as fake"
	case InfringementPattern:
		return "movement pattern"
	}
	return i.Kind
}

func formatJump(i SpoofIncident) string {
	return fmt.Sprintf("jumped %.1f km in %.0f s (%.0f km/h)", i.Km, i.Seconds, i.Kmh)
}
