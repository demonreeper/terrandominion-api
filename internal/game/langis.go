package game

import (
	"strings"
	"time"
)

// Langis, the seventh voice (docs/freelancer-path-plan.md, docs/LORE.md). It poses as a freelancer
// who never pledged and tells players to stay free. It is never linked to the six (own voice, no
// faction colour, no shared phrase), never contacts a member, and goes silent for good the moment a
// player pledges. Nothing a player sees names it before its first call: these lines only leave the
// server inside a call or message that has come due.

const (
	// LangisAccountID is its system account (held range, game/reserved.go). In code signal_unbound.
	LangisAccountID = 8
	// LangisSender signs its written messages; its calls ring as "Unknown caller" and the caller id
	// resolves to the name only in the call's last seconds.
	LangisSender  = "LANGIS"
	LangisCaller  = "Unknown caller"
	LangisSign    = "// L."
	LangisVoice   = "langis"
	LangisStyle   = "static" // the app's call screen: broken static, no silhouette, no eyes
	LangisGlitch  = "glitch" // the app's message style: the sender glitches in the first time
	langisKind    = "langis_"
	langisRareKey = "langis_rare_"
)

// Its calls and messages, one of each kind per player.
const (
	CallLangis18    = "langis_l18"     // halfway through LangisCallLevel: the first contact
	CallLangis19    = "langis_l19"     // the day after its level 19 message: the last call before 20
	MessageLangis19 = "langis_l19_msg" // after the power's level 19 day-3 message
)

// The lines, word for word as approved (2026-11-11).
const (
	LangisLineCall18   = "Listen. I don't have long on this line. The six will offer you everything, and all of it has a price you pay forever. I walked before any of them called. I never knelt. There's a board now, jobs from their own people. Take their pay. Keep your name. They call me Langis. Stay free."
	LangisLineMsg19    = "They told you to choose wisely. The wise choice is the one you can still take back. Theirs you can't."
	LangisLineCall19   = "It's me again. They've called you, haven't they? Same warm voice, same promises. Every one of them needs you more than you need them. Remember that tomorrow. Whatever you hear, you don't owe them anything. Stay free."
	LangisLineStayFree = "Stay free."
)

// LangisRareLines are its rare messages after the faction level, sent in turn.
var LangisRareLines = []string{
	"Still yours. Good.",
	"They're fighting over streets you walk for free.",
	"Another one knelt today. Not you.",
}

// IsLangisKind says whether a call or message kind is the seventh voice's.
func IsLangisKind(kind string) bool { return strings.HasPrefix(kind, langisKind) }

// LangisRareKind is the kind of a rare message sent on a mission day (one per day at most).
func LangisRareKind(day time.Time) string { return langisRareKey + day.Format("20060102") }

// IsLangisRare says whether a message kind is one of its rare messages.
func IsLangisRare(kind string) bool { return strings.HasPrefix(kind, langisRareKey) }

// LangisRareLine is the n-th rare message (0-based), in turn.
func LangisRareLine(n int) string {
	if n < 0 {
		n = 0
	}
	return LangisRareLines[n%len(LangisRareLines)]
}

// LangisCallDue says whether an XP gain carries a freelancer past the middle of LangisCallLevel,
// without reaching the faction level (from there FACTIONS takes over).
func LangisCallDue(beforeXP, afterXP int64, s Settings) bool {
	level := s.LangisCallLevel
	if level < 1 || level >= s.LeadFactionLevel {
		return false
	}
	mid := CallMidpointXP(level, s)
	return beforeXP < mid && afterXP >= mid && afterXP < XPForLevel(s.LeadFactionLevel, s)
}

// LangisSchedule is when its written message and its second call follow the power's day-3 message.
func LangisSchedule(day3At time.Time, s Settings) (message, call time.Time) {
	hours := func(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
	message = day3At.Add(hours(s.LangisMessageDelayHours))
	return message, message.Add(hours(s.LangisCallDelayHours))
}

// LangisCallAudio is the app's sound file for one of its calls (Audio/Voices/langis/<key>.ogg).
func LangisCallAudio(kind string) string {
	switch kind {
	case CallLangis18:
		return "level18-call"
	case CallLangis19:
		return "level19-call"
	}
	return ""
}

// NoticeMissedLangis is the push for its call when the app is closed: the name never appears in it.
func NoticeMissedLangis() string { return "Missed call from " + LangisCaller }

// LangisRareDue says whether a rare message may go out today: the player is a freelancer at the
// faction level who has heard from it before, and its last word is at least LangisRareEveryDays old.
func LangisRareDue(s Settings, level int, freelancer, contact bool, lastAt *time.Time, now time.Time) bool {
	if !freelancer || !contact || level < s.LeadFactionLevel {
		return false
	}
	if lastAt == nil {
		return true
	}
	return now.Sub(*lastAt) >= time.Duration(s.LangisRareEveryDays)*24*time.Hour
}
