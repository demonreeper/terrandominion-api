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

// TetherHarvest is one link's collection. A Tether Node has no extractor, so it gets base yield.
type TetherHarvest struct {
	Amount      int64
	ToOwner     int64
	ToStock     int64
	ReserveCost float64
}

// EvaluateTetherHarvest applies the player harvest's yield and reserve rules to one link. It
// reports false when the node is too depleted to give anything this time.
func EvaluateTetherHarvest(s Settings, yield int, reserve float64) (TetherHarvest, bool) {
	if reserve < s.NodeDepletedReserve {
		return TetherHarvest{}, false
	}
	amount := int64(harvestAmount(yield, 1, reserve))
	toOwner := int64(math.Round(float64(amount) * s.TnOwnerShare))
	return TetherHarvest{
		Amount: amount, ToOwner: toOwner, ToStock: amount - toOwner,
		ReserveCost: math.Min(reserve, s.HarvestReserveCost),
	}, true
}

// CanUseTether says who may upload through a Tether Node: anyone at a freelancer's, only the
// faction's members at a faction's. The owner may always use their own.
func CanUseTether(ownerID int64, tetherFaction *int, playerID int64, playerFaction *int) bool {
	if playerID == ownerID || tetherFaction == nil {
		return true
	}
	return playerFaction != nil && *playerFaction == *tetherFaction
}

// CheckAtTether confirms a player's fix puts them at a Tether Node, on the same terms as a harvest,
// and returns how far away they are.
func CheckAtTether(s Settings, player Fix, last *Fix, lat, lon float64) (float64, *apperr.Error) {
	if player.AccuracyM <= 0 || player.AccuracyM > s.HarvestMaxAccuracyM {
		return 0, apperr.HarvestGPSWeak
	}
	distance := DistanceM(player.Lat, player.Lon, lat, lon)
	if distance > s.HarvestRangeM+math.Min(player.AccuracyM, s.HarvestRangeM) {
		return distance, apperr.TetherTooFar
	}
	return distance, checkTravel(s, last, player)
}

// SplitUpload divides carried resources into what the player keeps and the owner's charge. The
// owner uploading through their own Tether Node pays nothing.
func SplitUpload(carried map[string]int64, fee float64, isOwner bool) (kept, fees map[string]int64) {
	kept, fees = map[string]int64{}, map[string]int64{}
	for resource, amount := range carried {
		if amount <= 0 {
			continue
		}
		var charge int64
		if !isOwner {
			charge = int64(math.Round(float64(amount) * fee))
		}
		if amount > charge {
			kept[resource] = amount - charge
		}
		if charge > 0 {
			fees[resource] = charge
		}
	}
	return kept, fees
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
