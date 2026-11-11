package game

import (
	"fmt"
	"math"
	"sort"
	"time"

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

// TetherUpgradeCost is what the upgrade to tier 2 costs.
func TetherUpgradeCost(s Settings) map[string]int64 {
	cost := map[string]int64{}
	for resource, amount := range map[string]int{
		"alloy": s.TnUpgradeAlloy, "data": s.TnUpgradeData, "credit": s.TnUpgradeCredit,
	} {
		if amount > 0 {
			cost[resource] = int64(amount)
		}
	}
	return cost
}

// TetherLinks is how many nodes a Tether Node of this tier tethers.
func TetherLinks(s Settings, tier int) int {
	if tier >= 2 {
		return s.TnLinksTier2
	}
	return s.TnLinksTier1
}

// AddCosts totals two sets of resources without changing either.
func AddCosts(a, b map[string]int64) map[string]int64 {
	total := make(map[string]int64, len(a)+len(b))
	for resource, amount := range a {
		total[resource] += amount
	}
	for resource, amount := range b {
		total[resource] += amount
	}
	return total
}

// FreeSlots lists the link slots of a Tether Node that hold no node, lowest first.
func FreeSlots(used []int, links int) []int {
	taken := make(map[int]bool, len(used))
	for _, slot := range used {
		taken[slot] = true
	}
	free := make([]int, 0, links)
	for slot := 0; slot < links; slot++ {
		if !taken[slot] {
			free = append(free, slot)
		}
	}
	return free
}

// LinkCandidate is a free node a Tether Node could link to.
type LinkCandidate struct {
	NodeID   int64
	Resource string
}

// PickLinks chooses up to count candidates, one of each resource first, starting with resources
// the Tether Node doesn't already link. Duplicates are only used to fill slots once every resource
// on offer is taken, so a slot isn't left idle because a resource has no node nearby.
func PickLinks(candidates []LinkCandidate, linked []string, count int) []int64 {
	seen := make(map[string]bool, len(linked))
	for _, resource := range linked {
		seen[resource] = true
	}

	picked := make([]int64, 0, count)
	used := make(map[int64]bool, count)
	take := func(c LinkCandidate) {
		picked = append(picked, c.NodeID)
		used[c.NodeID] = true
		seen[c.Resource] = true
	}

	for _, c := range candidates {
		if len(picked) < count && !seen[c.Resource] {
			take(c)
		}
	}
	for _, c := range candidates {
		if len(picked) < count && !used[c.NodeID] {
			take(c)
		}
	}
	return picked
}

// WithinTetherInfo says whether a player is close enough to see what a Tether Node collects.
func WithinTetherInfo(s Settings, player Fix, lat, lon float64) bool {
	allowance := math.Max(0, math.Min(player.AccuracyM, s.TnInfoRangeM))
	return DistanceM(player.Lat, player.Lon, lat, lon) <= s.TnInfoRangeM+allowance
}

// Payment says where a cost is taken from.
type Payment struct {
	FromCarried  map[string]int64
	FromUploaded map[string]int64
}

// PayCost takes each resource from carried first and stored (uploaded) for the rest. Nothing is
// paid unless the whole cost can be; the error says how much of each resource is still missing.
func PayCost(cost, carried, uploaded map[string]int64) (Payment, *apperr.Error) {
	pay := Payment{FromCarried: map[string]int64{}, FromUploaded: map[string]int64{}}
	short := map[string]int64{}
	for resource, amount := range cost {
		fromCarried := min(amount, max(carried[resource], 0))
		fromUploaded := amount - fromCarried
		if missing := fromUploaded - max(uploaded[resource], 0); missing > 0 {
			short[resource] = missing
			continue
		}
		if fromCarried > 0 {
			pay.FromCarried[resource] = fromCarried
		}
		if fromUploaded > 0 {
			pay.FromUploaded[resource] = fromUploaded
		}
	}
	if len(short) > 0 {
		return Payment{}, ShortError(short)
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
		return Payment{}, GPSWeakError(s, in.Player.AccuracyM)
	}
	if err := checkTravel(s, in.LastAnyFix, in.Player); err != nil {
		return Payment{}, err
	}
	if in.ActiveOwned >= s.TnMaxPerPlayer {
		return Payment{}, CapError(s.TnMaxPerPlayer)
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
	return checkAtSpot(s, player, last, lat, lon, apperr.TetherTooFar, "the Tether Node")
}

// CheckAtDrop is the same check for a map drop.
func CheckAtDrop(s Settings, player Fix, last *Fix, lat, lon float64) (float64, *apperr.Error) {
	return checkAtSpot(s, player, last, lat, lon, apperr.DropTooFar, "the drop")
}

func checkAtSpot(s Settings, player Fix, last *Fix, lat, lon float64, tooFar *apperr.Error, target string) (float64, *apperr.Error) {
	if player.AccuracyM <= 0 || player.AccuracyM > s.HarvestMaxAccuracyM {
		return 0, GPSWeakError(s, player.AccuracyM)
	}
	distance := DistanceM(player.Lat, player.Lon, lat, lon)
	if distance > s.HarvestRangeM+math.Min(player.AccuracyM, s.HarvestRangeM) {
		return distance, RangeError(tooFar, distance, s.HarvestRangeM, target)
	}
	return distance, checkTravel(s, last, player)
}

// TetherDecayPerHour is how fast health falls so a full Tether Node empties over its lifetime.
func TetherDecayPerHour(s Settings) float64 {
	if s.TnLifetimeHours <= 0 {
		return 0
	}
	return s.TnMaxHealth / s.TnLifetimeHours
}

// TakeDrop moves as much of a drop as fits in the player's free carry room, resources in name
// order. What does not fit stays on the map.
func TakeDrop(contents map[string]int64, room int64) (taken, left map[string]int64) {
	taken, left = map[string]int64{}, map[string]int64{}
	resources := make([]string, 0, len(contents))
	for resource := range contents {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		amount := contents[resource]
		take := min(amount, max(room, 0))
		if take > 0 {
			taken[resource] = take
			room -= take
		}
		if amount > take {
			left[resource] = amount - take
		}
	}
	return taken, left
}

// TetherAttack is who is attacking whose Tether Node.
type TetherAttack struct {
	AttackerID      int64
	AttackerFaction *int
	AttackerLevel   int
	OwnerID         int64
	OwnerLevel      int
	TetherFaction   *int
	// OwnerFreelancer: the owner has not pledged. OwnerProtected: and has not attacked a faction's
	// Tether Node in the last ProtectionLossDays, so faction players can't hit their Tether Node.
	OwnerFreelancer bool
	OwnerProtected  bool
}

// CanAttackTether applies the attack rules. It reports whether the attacker loses their freelancer
// protection for doing it.
func CanAttackTether(s Settings, a TetherAttack) (losesProtection bool, err *apperr.Error) {
	if a.AttackerID == a.OwnerID {
		return false, apperr.TetherCantAttack
	}
	if a.TetherFaction != nil {
		return a.AttackerFaction == nil, nil
	}
	if a.AttackerFaction != nil {
		// Neutral standing (freelancer-path-plan.md F5): a freelancer's Tether Node is off limits to
		// faction players, unless that freelancer attacked a faction lately.
		if a.OwnerFreelancer && a.OwnerProtected {
			return false, apperr.TetherCantAttack.Say("This freelancer is neutral. Their Tether Node can be hit only if they attacked a faction in the last %s.",
				daysText(s.ProtectionLossDays))
		}
		return false, nil
	}
	if diff := a.AttackerLevel - a.OwnerLevel; diff > s.PvpLevelBand || -diff > s.PvpLevelBand {
		return false, apperr.TetherCantAttack
	}
	return false, nil
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

// TetherUrgency is what decides where a player's own Tether Node sits on the My Tether Nodes list.
type TetherUrgency struct {
	Ended       bool
	UnderAttack bool
	Threatened  bool // a spike is armed or a drone is inbound
	Jammed      bool
	ExpiresAt   time.Time
	EndedAt     time.Time
}

func (u TetherUrgency) rank() int {
	switch {
	case u.Ended:
		return 4
	case u.UnderAttack:
		return 0
	case u.Threatened:
		return 1
	case u.Jammed:
		return 2
	}
	return 3
}

// MoreUrgent orders the list: under attack, then spike or drone, then jammed, then the soonest to
// expire. Ended ones go last, the most recently ended first.
func MoreUrgent(a, b TetherUrgency) bool {
	if ra, rb := a.rank(), b.rank(); ra != rb {
		return ra < rb
	}
	if a.Ended {
		return a.EndedAt.After(b.EndedAt)
	}
	return a.ExpiresAt.Before(b.ExpiresAt)
}

// daysText says a number of days in a sentence: "3 days", "1 day".
func daysText(days float64) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%g days", days)
}
