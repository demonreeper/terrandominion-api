package game

import (
	"math"
	"strings"
	"unicode/utf8"
)

// Admin players and stats (docs/profile-plan.md, A2): the rules with no SQL in them.

// The failed location checks that are stored on a player.
const (
	InfringementGPSWeak     = "gps_weak"     // the fix was too poor to act on
	InfringementTravelSpeed = "travel_speed" // the fix could not have been reached since the last one
)

// AdjustMax is the most one adjustment may add to or take from XP or one resource.
const AdjustMax = 1_000_000

// Adjustment is what an admin adds to (or with a minus, takes from) a player.
type Adjustment struct {
	XP      int64
	Carried map[string]int64
	Stored  map[string]int64
	Reason  string
}

// Why CheckAdjust refuses an adjustment.
const (
	AdjustOK       = ""
	AdjustReason   = "reason"   // a reason of 3 to 500 characters is required
	AdjustNothing  = "nothing"  // every amount is zero
	AdjustTooMuch  = "too_much" // an amount is over AdjustMax
	AdjustResource = "resource" // not a resource the game has
	AdjustBelow    = "below"    // it would take more than the player has
)

// CheckAdjust says why an adjustment is refused, or AdjustOK. xp, carried and stored are what the
// player has now; a resource missing from them is one the game doesn't have.
func CheckAdjust(a Adjustment, xp int64, carried, stored map[string]int64) string {
	reason := strings.TrimSpace(a.Reason)
	if n := utf8.RuneCountInString(reason); n < SanctionReasonMin || n > SanctionReasonMax {
		return AdjustReason
	}
	changed := a.XP != 0
	if over(a.XP) {
		return AdjustTooMuch
	}
	if xp+a.XP < 0 {
		return AdjustBelow
	}
	for _, side := range []struct{ change, have map[string]int64 }{{a.Carried, carried}, {a.Stored, stored}} {
		for resource, amount := range side.change {
			if amount == 0 {
				continue
			}
			changed = true
			have, known := side.have[resource]
			switch {
			case !known:
				return AdjustResource
			case over(amount):
				return AdjustTooMuch
			case have+amount < 0:
				return AdjustBelow
			}
		}
	}
	if !changed {
		return AdjustNothing
	}
	return AdjustOK
}

func over(amount int64) bool { return amount > AdjustMax || amount < -AdjustMax }

// RetentionPct is the share, in whole percent, of the players who joined a week ago that came back.
func RetentionPct(retained, of int) int {
	if of <= 0 || retained <= 0 {
		return 0
	}
	return int(math.Round(100 * float64(retained) / float64(of)))
}

// CleanPlayerSearch tidies what an admin typed into the player search: trimmed, and without the
// "OPERATIVE-" in front of a share code. Under two characters is no search.
func CleanPlayerSearch(typed string) (string, bool) {
	q := strings.TrimSpace(typed)
	if upper := strings.ToUpper(q); strings.HasPrefix(upper, "OPERATIVE-") {
		q = q[len("OPERATIVE-"):]
	}
	if utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 80 {
		return "", false
	}
	return q, true
}

// NoticeCallsignReset is the written message a player gets when an admin resets their callsign.
const NoticeCallsignReset = "Your callsign was removed because it isn't allowed. Choose a new one from your profile."
