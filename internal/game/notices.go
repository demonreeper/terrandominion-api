package game

import (
	"fmt"
	"math"
	"time"
)

// Notice severities: the app picks a notice's sound and haptic from these, not from its kind.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityDanger  = "danger"
)

// noticeSeverity is every notice kind the server writes. Danger: your Tether Node is being hurt
// or is about to be, act now. Warning: something needs you soon or went against you, with no
// damage. Info: reports of your own actions and good news.
var noticeSeverity = map[string]string{
	"attacked":      SeverityDanger,
	"destroyed":     SeverityDanger,
	"spike":         SeverityDanger,
	"drone_inbound": SeverityDanger,
	"spike_fired":   SeverityDanger,

	"jammed":            SeverityWarning,
	"contest":           SeverityWarning,
	"contest_lost":      SeverityWarning,
	"drone_intercepted": SeverityWarning,

	"spike_result": SeverityInfo,
	"drone_result": SeverityInfo,
	"defused":      SeverityInfo,
	"contest_won":  SeverityInfo,

	"drone_launched": SeverityInfo,
	"shield_fitted":  SeverityInfo,

	"mission_done": SeverityInfo,
	"invitation":   SeverityInfo,
	"call":         SeverityInfo,
	"warden":       SeverityInfo,
}

// NoticeSeverity is how urgent a notice kind is; unknown kinds are info.
func NoticeSeverity(kind string) string {
	if severity, ok := noticeSeverity[kind]; ok {
		return severity
	}
	return SeverityInfo
}

// NoticeKinds lists every kind with a severity, for tests and docs.
func NoticeKinds() []string {
	kinds := make([]string, 0, len(noticeSeverity))
	for kind := range noticeSeverity {
		kinds = append(kinds, kind)
	}
	return kinds
}

// Notice bodies. One style: sentences with full stops, "Tether Node" spelled out, numbers with
// units, and who and how much when known. place is the name of the node a Tether Node stands by
// ("Crystal Seam K-27"), or empty.

func someone(name string) string {
	if name == "" {
		return "Someone"
	}
	return name
}

func possessive(name string) string {
	return someone(name) + "'s"
}

// TetherPhrase names a Tether Node: "your Tether Node near Crystal Seam K-27" when owner is empty
// (the reader's own), else "Aishath's Tether Node near Crystal Seam K-27".
func TetherPhrase(owner, place string) string {
	who := "your"
	if owner != "" {
		who = possessive(owner)
	}
	if place == "" {
		return who + " Tether Node"
	}
	return who + " Tether Node near " + place
}

// Damage rounds a hit for display and for a notice's amount.
func Damage(d float64) int64 {
	return int64(math.Round(d))
}

// NoticeHit: "Aishath's EM Pulse hit your Tether Node near X for 25 damage. 75 health left." what
// is the weapon: "EM Pulse", "spike" or "drone".
func NoticeHit(attacker, what, place string, damage, healthLeft int64) string {
	return fmt.Sprintf("%s %s hit %s for %d damage. %d health left.", possessive(attacker), what, TetherPhrase("", place),
		damage, healthLeft)
}

// NoticeDestroyed: "Aishath's drone destroyed your Tether Node near X."
func NoticeDestroyed(attacker, what, place string) string {
	return fmt.Sprintf("%s %s destroyed %s.", possessive(attacker), what, TetherPhrase("", place))
}

// NoticeJammed: "Aishath jammed your Tether Node near X. It collects nothing for 30 min."
func NoticeJammed(attacker, place string, jam time.Duration) string {
	return fmt.Sprintf("%s jammed %s. It collects nothing for %s.", someone(attacker), TetherPhrase("", place), FormatDuration(jam))
}

// NoticeSpikePlanted: "Aishath planted a spike on your Tether Node near X. Defuse it within
// 10 min or it deals 40 damage."
func NoticeSpikePlanted(attacker, place string, fuse time.Duration, damage int64) string {
	return fmt.Sprintf("%s planted a spike on %s. Defuse it within %s or it deals %d damage.",
		someone(attacker), TetherPhrase("", place), FormatDuration(fuse), damage)
}

// NoticeDroneInbound: "Aishath's drone is inbound to your Tether Node near X. It lands in 45 s
// for 15 damage."
func NoticeDroneInbound(attacker, place string, flight time.Duration, damage int64) string {
	return fmt.Sprintf("%s drone is inbound to %s. It lands in %s for %d damage.",
		possessive(attacker), TetherPhrase("", place), FormatDuration(flight), damage)
}

// NoticeResult tells an attacker how their spike or drone did: "Your drone hit Aishath's Tether
// Node near X for 15 damage. 40 health left." or "Your spike destroyed Aishath's Tether Node near X."
func NoticeResult(what, owner, place string, damage, healthLeft int64, destroyed bool) string {
	if destroyed {
		return fmt.Sprintf("Your %s destroyed %s.", what, TetherPhrase(someone(owner), place))
	}
	return fmt.Sprintf("Your %s hit %s for %d damage. %d health left.", what, TetherPhrase(someone(owner), place),
		damage, healthLeft)
}

// NoticeDroneLaunched: "Your drone is away to Aishath's Tether Node near X. It lands in 20 s for 15 damage."
func NoticeDroneLaunched(owner, place string, flight time.Duration, damage int64) string {
	return fmt.Sprintf("Your drone is away to %s. It lands in %s for %d damage.",
		TetherPhrase(someone(owner), place), FormatDuration(flight), damage)
}

// NoticeInterceptedDefender: "You intercepted Aishath's drone at your Tether Node near X. 75% of its
// damage was stopped." or "You shot down Aishath's drone ..." when it stops everything.
func NoticeInterceptedDefender(attacker, place string, share float64) string {
	if share >= 1 {
		return fmt.Sprintf("You shot down %s drone at %s.", possessive(attacker), TetherPhrase("", place))
	}
	return fmt.Sprintf("You intercepted %s drone at %s. %d%% of its damage was stopped.",
		possessive(attacker), TetherPhrase("", place), percent(share))
}

// NoticeInterceptedAttacker: "Your drone was intercepted at Aishath's Tether Node near X. 75% of its
// damage was stopped." or "Your drone was shot down at ...".
func NoticeInterceptedAttacker(owner, place string, share float64) string {
	if share >= 1 {
		return fmt.Sprintf("Your drone was shot down at %s.", TetherPhrase(someone(owner), place))
	}
	return fmt.Sprintf("Your drone was intercepted at %s. %d%% of its damage was stopped.",
		TetherPhrase(someone(owner), place), percent(share))
}

// NoticeShieldFitted: "Shield Mk II fitted to your Tether Node near X."
func NoticeShieldFitted(mk int, place string) string {
	return fmt.Sprintf("Shield Mk %s fitted to %s.", RomanMk(mk), TetherPhrase("", place))
}

func percent(share float64) int { return int(math.Round(share * 100)) }

// NoticeDefused: "Aishath defused your spike on their Tether Node near X."
func NoticeDefused(owner, place string) string {
	where := "their Tether Node"
	if place != "" {
		where += " near " + place
	}
	return fmt.Sprintf("%s defused your spike on %s.", someone(owner), where)
}

// NoticeContest: "Aishath is contesting Crystal Seam K-27. Be within 40 m in the next 1 min to
// defend it."
func NoticeContest(challenger, node string, rangeM float64, window time.Duration) string {
	return fmt.Sprintf("%s is contesting %s. Be within %d m in the next %s to defend it.",
		someone(challenger), node, int(math.Round(rangeM)), FormatDuration(window))
}

// NoticeContestWon: "You won the contest for X against Aishath, 12 to 8. Its next 3 harvests are yours."
func NoticeContestWon(node, loser string, mine, theirs, prize int) string {
	harvests := "harvests are"
	if prize == 1 {
		harvests = "harvest is"
	}
	return fmt.Sprintf("You won the contest for %s against %s, %d to %d. Its next %d %s yours.",
		node, someone(loser), mine, theirs, prize, harvests)
}

// NoticeContestLost: "You lost the contest for X to Aishath, 8 to 12."
func NoticeContestLost(node, winner string, mine, theirs int) string {
	return fmt.Sprintf("You lost the contest for %s to %s, %d to %d.", node, someone(winner), mine, theirs)
}
