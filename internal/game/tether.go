package game

import (
	"math"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// TetherBuildCost is what one Tether Node costs. Resources costing nothing are left out.
func TetherBuildCost(s Settings) map[string]int64 {
	cost := map[string]int64{}
	for resource, amount := range map[string]int{
		"crystal": s.TnBuildCrystal, "alloy": s.TnBuildAlloy, "data": s.TnBuildData,
	} {
		if amount > 0 {
			cost[resource] = int64(amount)
		}
	}
	return cost
}

// Payment says where a cost is taken from.
type Payment struct {
	FromCarried  map[string]int64
	FromUploaded map[string]int64
}

// PayCost takes each resource from carried first and uploaded for the rest. Nothing is paid
// unless the whole cost can be.
func PayCost(cost, carried, uploaded map[string]int64) (Payment, *apperr.Error) {
	pay := Payment{FromCarried: map[string]int64{}, FromUploaded: map[string]int64{}}
	for resource, amount := range cost {
		fromCarried := min(amount, carried[resource])
		fromUploaded := amount - fromCarried
		if fromUploaded > uploaded[resource] {
			return Payment{}, apperr.TetherCost.With("%s: need %d, have %d carried and %d uploaded",
				resource, amount, carried[resource], uploaded[resource])
		}
		if fromCarried > 0 {
			pay.FromCarried[resource] = fromCarried
		}
		if fromUploaded > 0 {
			pay.FromUploaded[resource] = fromUploaded
		}
	}
	return pay, nil
}

// TetherBuildInput is everything the build rule needs, loaded by the service in one transaction.
type TetherBuildInput struct {
	Settings    Settings
	Player      Fix
	LastAnyFix  *Fix
	ActiveOwned int
	Carried     map[string]int64
	Uploaded    map[string]int64
}

// EvaluateTetherBuild decides whether a player may build here and now, and how they pay. GPS
// checks come first, as with a harvest.
func EvaluateTetherBuild(in TetherBuildInput) (Payment, *apperr.Error) {
	s := in.Settings
	if in.Player.AccuracyM <= 0 || in.Player.AccuracyM > s.HarvestMaxAccuracyM {
		return Payment{}, apperr.HarvestGPSWeak
	}
	if err := checkTravel(s, in.LastAnyFix, in.Player); err != nil {
		return Payment{}, err
	}
	if in.ActiveOwned >= s.TnMaxPerPlayer {
		return Payment{}, apperr.TetherCap
	}
	return PayCost(TetherBuildCost(s), in.Carried, in.Uploaded)
}

// TetherRefund is what dismantling returns, rounded down.
func TetherRefund(spent map[string]int64, share float64) map[string]int64 {
	refund := map[string]int64{}
	for resource, amount := range spent {
		if back := int64(math.Floor(float64(amount) * share)); back > 0 {
			refund[resource] = back
		}
	}
	return refund
}
