package game

import "fmt"

// Captain squad missions (faction-ranks-plan.md, Captain and Squads; base-building-plan.md 10): a Captain posts one of
// the base mission kinds to their squad, naming one of the faction's standing bases in the region. Each squad member
// gets it as an extra mission for the day, in their own power's words, paying what a base mission pays.

// CaptainMissionResources are the resources a give mission may ask for.
var CaptainMissionResources = []string{"alloy", "crystal", "data", "credit"}

// CaptainMissionPost is what a Captain asks for: a goal (GoalBaseGive, GoalBaseStand or GoalBaseRepair), a resource
// for a give, and an amount (resource units, minutes on site, or modules repaired).
type CaptainMissionPost struct {
	Goal   string
	Key    string
	Amount int
}

// Reasons CheckCaptainMission refuses a post; the empty string means it may be posted.
const (
	CaptainMissionOK           = ""
	CaptainMissionBadKind      = "kind"
	CaptainMissionBadResource  = "resource"
	CaptainMissionBadAmount    = "amount"
	CaptainMissionNothingToFix = "repair"
)

// CheckCaptainMission checks a post against the settings. damaged says whether the named base has a module below full
// health (a repair mission is offered only then, and asks for one repair).
func CheckCaptainMission(p CaptainMissionPost, damaged bool, s Settings) string {
	switch p.Goal {
	case GoalBaseGive:
		known := false
		for _, r := range CaptainMissionResources {
			known = known || r == p.Key
		}
		if !known {
			return CaptainMissionBadResource
		}
		if p.Amount < 1 || p.Amount > s.CaptainMissionGiveMax {
			return CaptainMissionBadAmount
		}
	case GoalBaseStand:
		if p.Key != "" {
			return CaptainMissionBadResource
		}
		if p.Amount < 1 || p.Amount > s.CaptainMissionStandMaxMin {
			return CaptainMissionBadAmount
		}
	case GoalBaseRepair:
		if p.Key != "" {
			return CaptainMissionBadResource
		}
		if p.Amount != 1 {
			return CaptainMissionBadAmount
		}
		if !damaged {
			return CaptainMissionNothingToFix
		}
	default:
		return CaptainMissionBadKind
	}
	return CaptainMissionOK
}

// CaptainMissionsLeft is how many more missions a Captain may post today, having posted `posted`.
func CaptainMissionsLeft(posted int, s Settings) int {
	return max(0, s.CaptainMissionsPerDay-posted)
}

// CaptainMissionTitle says what a Captain's mission asks, naming the base: "Give 300 crystal at Coral Spire".
func CaptainMissionTitle(goal, key string, count int, base string) string {
	if base == "" {
		base = "your base"
	}
	switch goal {
	case GoalBaseGive:
		return fmt.Sprintf("Give %d %s at %s", count, key, base)
	case GoalBaseStand:
		if count == 1 {
			return "Stand at " + base + " for a minute"
		}
		return fmt.Sprintf("Stand at %s for %d minutes", base, count)
	case GoalBaseRepair:
		if count == 1 {
			return "Repair a module at " + base
		}
		return fmt.Sprintf("Repair modules at %s %d times", base, count)
	}
	return MissionTitle(goal, key, count)
}

// NoticeCaptainMission: "Reef-Runner posted a squad mission: Give 300 crystal at Coral Spire. It is in ORDERS."
func NoticeCaptainMission(captain, title string) string {
	return fmt.Sprintf("%s posted a squad mission: %s. It is in ORDERS.", someone(captain), title)
}
