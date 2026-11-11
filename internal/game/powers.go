package game

import (
	"math"
	"time"
)

// The six lore powers, their calls and their written messages (docs/LORE.md,
// docs/lore-correction-plan.md). Each power is the voice of one faction; the ids are the faction
// ids seeded in migrations 0003 and 0029. Nothing here, in the API or in the app may link the six:
// no shared wording, and no power ever says its own name.
const (
	PowerSignum = 1 // Lumen Collective
	PowerShingo = 2 // Forge Meridian
	PowerIshara = 3 // Verdant Exchange
	PowerSema   = 4 // Veil Cipher
	PowerTohu   = 5 // Slipstream Vanguard
	PowerMerki  = 6 // Bastion Order
)

// Powers lists the six in id order.
var Powers = []int{PowerSignum, PowerShingo, PowerIshara, PowerSema, PowerTohu, PowerMerki}

// Actions the playstyle score counts that are not mission goals.
const (
	PlayDefuse   = "defuse"   // a spike defused at a Tether Node
	PlayDistance = "distance" // ground covered between two harvests; scored by DistancePoints
)

// PlaystylePoints is what one action adds to each power's score for a player ("Who calls" in the
// lore correction plan). Harvesting happens far more often than anything else, so the rarer actions
// weigh more; otherwise every player would look like a harvester. Starting values, to tune.
//
//	Signum  harvests, crystal most of all
//	Shingō  builds, upgrades and repairs Tether Nodes and gear
//	Sēma    data, and the quiet weapons: jammers and spikes
//	Ishāra  uploads, collects, credits
//	Tohu    drones, first strikes, ground covered (DistancePoints)
//	Merki   shields, intercepts, defusing
func PlaystylePoints(action, key string) map[int]int {
	switch action {
	case GoalHarvest:
		switch key {
		case "crystal":
			return map[int]int{PowerSignum: 3}
		case "data":
			return map[int]int{PowerSignum: 1, PowerSema: 2}
		case "credit":
			return map[int]int{PowerSignum: 1, PowerIshara: 2}
		}
		return map[int]int{PowerSignum: 2}
	case GoalBuildTether:
		return map[int]int{PowerShingo: 10}
	case GoalUpgradeTether:
		return map[int]int{PowerShingo: 12}
	case GoalUpgradeGear:
		return map[int]int{PowerShingo: 8}
	case GoalRepairGear:
		return map[int]int{PowerShingo: 5}
	case GoalCraft:
		switch key {
		case WeaponJammer, WeaponSpike:
			return map[int]int{PowerSema: 3}
		case WeaponEmPulse:
			return map[int]int{PowerTohu: 3}
		}
		if _, shield := ShieldMkOf(key); shield {
			return map[int]int{PowerMerki: 3}
		}
		return map[int]int{PowerShingo: 3}
	case GoalCollect:
		return map[int]int{PowerIshara: 4}
	case GoalUpload:
		return map[int]int{PowerIshara: 5}
	case GoalAttack:
		switch key {
		case WeaponJammer, WeaponSpike:
			return map[int]int{PowerSema: 8}
		}
		return map[int]int{PowerTohu: 6}
	case GoalDrone:
		return map[int]int{PowerTohu: 10}
	case GoalIntercept, GoalShield, PlayDefuse:
		return map[int]int{PowerMerki: 10}
	}
	return nil
}

// DistancePoints scores the ground a player covered between two harvests for Tohu: one point per
// 100 m, at most 5, so a GPS jump can't buy a caller.
func DistancePoints(metres float64) int {
	if metres < 100 || math.IsNaN(metres) {
		return 0
	}
	return min(5, int(metres/100))
}

// CallingPower is the one power that calls, messages and invites a player: the highest playstyle
// score. A tie goes to the power whose daily missions the player finished most, then to the lowest
// id so the answer is stable. With no score at all it is the power they followed most; ok is false
// when there is nothing to go on.
func CallingPower(scores, missions map[int]int) (power int, ok bool) {
	best, bestScore, bestMissions := 0, 0, -1
	for _, p := range Powers {
		score, done := scores[p], missions[p]
		if score <= 0 {
			continue
		}
		if score > bestScore || (score == bestScore && done > bestMissions) {
			best, bestScore, bestMissions = p, score, done
		}
	}
	if best != 0 {
		return best, true
	}
	return MostFollowed(missions)
}

// Call kinds, one row per player per kind.
const (
	CallLevel10       = "level10"        // one call about the player's path, no repeats
	CallLevel19       = "level19"        // the offer
	CallLevel19Repeat = "level19_repeat" // the repeat call, the next day
	MessageLevel19    = "level19_day3"   // the written message, the day after that
)

// CallAudioKey is the app's audio file for a call: Audio/Voices/<voice>/<key>.ogg.
func CallAudioKey(kind string) string {
	switch kind {
	case CallLevel10:
		return "level10"
	case CallLevel19:
		return "level19-call"
	case CallLevel19Repeat:
		return "level19-repeat"
	}
	return ""
}

// PowerVoice is the app's audio folder for a power. Plain letters: it is a file path, never shown.
func PowerVoice(power int) string {
	switch power {
	case PowerSignum:
		return "signum"
	case PowerShingo:
		return "shingo"
	case PowerIshara:
		return "ishara"
	case PowerSema:
		return "sema"
	case PowerTohu:
		return "tohu"
	case PowerMerki:
		return "merki"
	}
	return ""
}

// powerLines is what each power says. No power says its own name: the call screen shows who is
// calling. Each words the day 3 message its own way, because one sentence from all six would link
// them. The day 3 message also says the choice is final (freelancer-path-plan.md C4), each power in
// its own words.
var powerLines = map[int]map[string]string{
	PowerSignum: {
		CallLevel10:       "You gather like someone who knows the light runs out. Keep counting. I am.",
		CallLevel19:       "Freelancer. I have counted every shard you carried. Not one was wasted. The light remembers those who gather it. When you reach the threshold, stand with the Lumen Collective. I will wait.",
		CallLevel19Repeat: "Again. I am patient. The light is not.",
		MessageLevel19:    "You were given a chance. Few are. Choose wisely. Once you choose, you cannot unchoose. The light remembers.",
	},
	PowerShingo: {
		CallLevel10:       "Half the walkers out there build nothing. You build. Don't stop now.",
		CallLevel19:       "Listen. You build. You repair. You don't walk away from what you started. Most walkers leave nothing behind. You leave structures. Forge Meridian needs hands like yours. Reach level twenty. Then build something that lasts.",
		CallLevel19Repeat: "The offer stands. Like everything we build.",
		MessageLevel19:    "Chances are built once. This was yours. Choose wisely. A foundation is poured once. So is a choice.",
	},
	PowerSema: {
		CallLevel10:       "You found something today that no one else saw. So did I. I found you.",
		CallLevel19:       "You keep finding what others walk straight past. That is how I found you. The Veil never asks twice. For you, perhaps. Reach twenty, and I will show you what is hidden.",
		CallLevel19Repeat: "You let it ring. I noticed. I notice everything.",
		MessageLevel19:    "I do not call a third time. You were seen. Choose wisely. A choice, once made, cannot be unseen.",
	},
	PowerIshara: {
		CallLevel10:       "Your ledger is getting interesting. Keep it that way, and we may do business.",
		CallLevel19:       "I have reviewed your harvests, your uploads, your margins. Impressive. I would like to make you an offer. At level twenty it becomes binding. Consider the price of saying no.",
		CallLevel19Repeat: "My offer has not changed. The market has.",
		MessageLevel19:    "Final offer. You were given a chance most never get. Choose wisely. This contract has no exit clause.",
	},
	PowerTohu: {
		CallLevel10:       "You're fast. But not fast enough yet. Keep moving.",
		CallLevel19:       "No time. You reached nodes others never even saw. That is what we ride on. Slipstream Vanguard moves at dawn, every dawn. Hit twenty. Then keep up.",
		CallLevel19Repeat: "Still standing still? Move.",
		MessageLevel19:    "Last signal. The chance is moving. Choose fast, choose wisely. There is no turning back. None.",
	},
	PowerMerki: {
		CallLevel10:       "You held when you could have run. Remember that feeling. You will need it.",
		CallLevel19:       "I watched you hold your ground when running was easier. Few do. The Bastion Order does not forget a wall that stands. Reach twenty. Hold the line with us.",
		CallLevel19Repeat: "The line still holds. Your place in it is open.",
		MessageLevel19:    "The gate was opened for you. It will not stay open. Choose wisely. A wall once raised is never given back. Neither is a choice.",
	},
}

// PowerLine is what a power says in a call or writes in its message; "" when it has none.
func PowerLine(power int, kind string) string {
	return powerLines[power][kind]
}

// CallMidpointXP is the total XP at which a level is half done.
func CallMidpointXP(level int, s Settings) int64 {
	floor, next := XPForLevel(level, s), XPForLevel(level+1, s)
	return floor + (next-floor+1)/2
}

// CallsDue says which calls an XP gain sets off: the path call when it carries the player past the
// middle of CallPathLevel, and the offer when it carries them past the middle of the level before
// LeadFactionLevel without reaching LeadFactionLevel itself (there the FACTIONS screen takes over).
func CallsDue(beforeXP, afterXP int64, s Settings) (path, offer bool) {
	crossed := func(level int) bool {
		if level < 1 {
			return false
		}
		mid := CallMidpointXP(level, s)
		return beforeXP < mid && afterXP >= mid
	}
	path = s.CallPathLevel > 0 && crossed(s.CallPathLevel)
	offerLevel := s.LeadFactionLevel - 1
	offer = crossed(offerLevel) && afterXP < XPForLevel(s.LeadFactionLevel, s)
	return path, offer
}

// OfferSchedule is when the repeat call and the written message follow the offer.
func OfferSchedule(now time.Time, s Settings) (repeat, message time.Time) {
	hours := func(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
	return now.Add(hours(s.CallRepeatHours)), now.Add(hours(s.CallMessageHours))
}

// CallMissed reports whether a call that came due at dueAt has rung out: the app then offers to
// play it once instead of ringing.
func CallMissed(dueAt, now time.Time, s Settings) bool {
	return now.Sub(dueAt) > time.Duration(s.CallRingSeconds)*time.Second
}

// NoticeCall is the push for a call the app did not pick up: "Incoming call: Sēma".
func NoticeCall(persona string) string {
	return "Incoming call: " + persona
}

// NoticeInvited is the plain game text at LeadFactionLevel. No power speaks here.
func NoticeInvited() string {
	return "You were invited. The choice is yours. Open FACTIONS."
}
