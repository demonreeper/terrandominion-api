package game

import (
	"fmt"
	"math"
	"time"
)

// Orders and daily missions (docs/regional-command-plan.md): each comes from one faction's lore
// power, in its own voice. Freelancers get missions from all six, members from their own faction,
// and Regional Commanders and Wardens get one command order among theirs.

// Mission goals: what an action counts toward. The service calls AdvanceMissions with these.
const (
	GoalHarvest       = "harvest"        // key: resource
	GoalBuildTether   = "build_tether"   //
	GoalUpgradeTether = "upgrade_tether" //
	GoalCollect       = "collect"        //
	GoalUpload        = "upload"         //
	GoalCraft         = "craft"          // key: recipe
	GoalUpgradeGear   = "upgrade_gear"   //
	GoalRepairGear    = "repair_gear"    //
	GoalAttack        = "attack"         // key: weapon
	GoalDrone         = "drone"          //
	GoalIntercept     = "intercept"      //
	GoalShield        = "fit_shield"     //
	GoalAppointWarden = "appoint_warden" //
)

// Mission audiences.
const (
	AudienceFreelancer = "freelancer"
	AudienceMember     = "member"
	AudienceCommand    = "command"
)

// MissionDay is the mission day a moment falls in (missions refresh at local midnight, at
// MissionDayOffsetMinutes from UTC), as a date, and when the next one starts.
func MissionDay(s Settings, now time.Time) (time.Time, time.Time) {
	offset := time.Duration(s.MissionDayOffsetMinutes) * time.Minute
	local := now.UTC().Add(offset)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	next := day.AddDate(0, 0, 1).Add(-offset)
	return day, next
}

// MissionReward is what finishing a mission pays: XP, credits, and now and then a low-level item.
type MissionReward struct {
	XP        int    `json:"xp"`
	Credits   int    `json:"credits"`
	Item      string `json:"item,omitempty"`
	ItemCount int    `json:"itemCount,omitempty"`
}

// MissionRewardFor grows with the player's level; command orders pay MissionCommandMult times as
// much. r is uniform in [0, 1) and decides the item.
func MissionRewardFor(s Settings, level int, command bool, r float64) MissionReward {
	mult := 1.0
	if command && s.MissionCommandMult > 0 {
		mult = s.MissionCommandMult
	}
	reward := MissionReward{
		XP:      int(math.Round(float64(s.MissionXpBase+s.MissionXpPerLevel*level) * mult)),
		Credits: int(math.Round(float64(s.MissionCreditBase+s.MissionCreditPerLevel*level) * mult)),
	}
	if r < s.MissionItemChance {
		reward.Item, reward.ItemCount = LowLevelItem(level), 1
	}
	return reward
}

// LowLevelItem is the item a mission may add: cells early, then the cheap weapons.
func LowLevelItem(level int) string {
	switch {
	case level < 5:
		return ItemCell
	case level < 10:
		return WeaponEmPulse
	case level < 15:
		return WeaponJammer
	default:
		return WeaponSpike
	}
}

// MissionCandidate is a mission type that may be given today.
type MissionCandidate struct {
	ID        string
	FactionID int
	Weight    int
}

// PickMissions draws up to n missions by weight. With oneEach, no two come from the same faction
// (a freelancer hears from n different lore powers). rnd(k) returns a uniform int in [0, k).
func PickMissions(candidates []MissionCandidate, n int, oneEach bool, rnd func(int) int) []MissionCandidate {
	pool := append([]MissionCandidate(nil), candidates...)
	out := make([]MissionCandidate, 0, n)
	for len(out) < n && len(pool) > 0 {
		total := 0
		for _, c := range pool {
			total += max(1, c.Weight)
		}
		pick := rnd(total)
		index := 0
		for i, c := range pool {
			w := max(1, c.Weight)
			if pick < w {
				index = i
				break
			}
			pick -= w
		}
		chosen := pool[index]
		out = append(out, chosen)
		kept := pool[:0]
		for i, c := range pool {
			if i == index || (oneEach && c.FactionID == chosen.FactionID) {
				continue
			}
			kept = append(kept, c)
		}
		pool = kept
	}
	return out
}

// MostFollowed is the faction whose missions the player finished most; ties go to the lowest id,
// so the answer is stable. ok is false when they finished none.
func MostFollowed(counts map[int]int) (int, bool) {
	best, bestCount := 0, 0
	for faction, count := range counts {
		if count > bestCount || (count == bestCount && count > 0 && faction < best) {
			best, bestCount = faction, count
		}
	}
	return best, bestCount > 0
}

// MissionTitle says what a mission asks, for players: "Harvest 3 crystal".
func MissionTitle(goal, key string, count int) string {
	times := func(one, many string) string {
		if count == 1 {
			return one
		}
		return fmt.Sprintf(many, count)
	}
	switch goal {
	case GoalHarvest:
		// Missions count harvests, not amounts: "Harvest crystal 20 times".
		if key != "" {
			return times("Harvest a "+key+" node", "Harvest "+key+" %d times")
		}
		return times("Harvest a node", "Harvest %d times")
	case GoalBuildTether:
		return times("Build a Tether Node", "Build %d Tether Nodes")
	case GoalUpgradeTether:
		return times("Upgrade a Tether Node", "Upgrade %d Tether Nodes")
	case GoalCollect:
		return times("Collect from a Tether Node", "Collect from Tether Nodes %d times")
	case GoalUpload:
		return times("Upload at a Tether Node", "Upload at Tether Nodes %d times")
	case GoalCraft:
		if name := itemName(key); name != "" {
			return times("Craft a"+anArticle(name)+name, fmt.Sprintf("Craft %%d %ss", name))
		}
		return times("Craft an item", "Craft %d items")
	case GoalUpgradeGear:
		return times("Upgrade a piece of gear", "Upgrade gear %d times")
	case GoalRepairGear:
		return times("Repair a piece of gear", "Repair gear %d times")
	case GoalAttack:
		return times("Attack a Tether Node", "Attack Tether Nodes %d times")
	case GoalDrone:
		return times("Launch a drone", "Launch %d drones")
	case GoalIntercept:
		return times("Intercept a drone", "Intercept %d drones")
	case GoalShield:
		return times("Fit a shield to a Tether Node", "Fit %d shields")
	case GoalAppointWarden:
		return times("Appoint a Warden", "Appoint %d Wardens")
	}
	return times("Complete the order", "Complete the order %d times")
}

func itemName(id string) string {
	switch id {
	case ItemCell:
		return "Energy Cell"
	case WeaponEmPulse:
		return "EM Pulse"
	case WeaponJammer:
		return "Signal Jammer"
	case WeaponSpike:
		return "Overload Spike"
	}
	return ""
}

func anArticle(name string) string {
	switch name[0] {
	case 'A', 'E', 'I', 'O', 'U':
		return "n "
	}
	return " "
}

// ItemName names an item for players ("Energy Cell"); unknown ids come back unchanged.
func ItemName(id string) string {
	if name := itemName(id); name != "" {
		return name
	}
	if mk, ok := ShieldMkOf(id); ok {
		return "TN Shield Mk " + RomanMk(mk)
	}
	return id
}

// NoticeMissionDone: "Signum: Harvest 3 crystal. Done. Claim your reward in ORDERS."
func NoticeMissionDone(persona, title string) string {
	return fmt.Sprintf("%s: %s. Done. Claim your reward in ORDERS.", persona, title)
}

// NoticeWarden: "Aishath made you a Warden of Faction One."
func NoticeWarden(commander, faction string) string {
	return fmt.Sprintf("%s made you a Warden of %s.", someone(commander), faction)
}
