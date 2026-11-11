package game

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Reports and moderation (docs/profile-plan.md, M1): the rules with no SQL in them.

// ReportReasonGriefing is the one reason a player can be reported for, for now.
const ReportReasonGriefing = "griefing"

// ReportDetails are what a reporter picks to say what happened, in the list's order.
var ReportDetails = []string{"repeated_attacks", "blocking_node", "following", "other"}

// ReportNoteMaxLength is the most a reporter's optional note may hold.
const ReportNoteMaxLength = 300

// Why CheckReport refuses a report.
const (
	ReportOK        = ""
	ReportSelf      = "self"      // you can't report yourself
	ReportBadDetail = "detail"    // not one of ReportDetails
	ReportNoteLong  = "note"      // the note is over ReportNoteMaxLength
	ReportDuplicate = "duplicate" // the reporter already has an open report on this player
	ReportTooMany   = "limit"     // the reporter reached ReportsPerDay
)

// CleanReportNote trims a reporter's note to one tidy paragraph.
func CleanReportNote(note string) string {
	return strings.Join(strings.Fields(note), " ")
}

// CheckReport says why a report is refused, or ReportOK. note is already cleaned; openOnTarget is
// whether the reporter has an open report on this player; today is how many reports they made in
// the last 24 hours.
func CheckReport(s Settings, reporterID, targetID int64, detail, note string, openOnTarget bool, today int) string {
	switch {
	case reporterID == targetID:
		return ReportSelf
	case !validReportDetail(detail):
		return ReportBadDetail
	case utf8.RuneCountInString(note) > ReportNoteMaxLength:
		return ReportNoteLong
	case openOnTarget:
		return ReportDuplicate
	case today >= s.ReportsPerDay:
		return ReportTooMany
	}
	return ReportOK
}

func validReportDetail(detail string) bool {
	for _, d := range ReportDetails {
		if d == detail {
			return true
		}
	}
	return false
}

// Sanction kinds.
const (
	SanctionWarn    = "warn"
	SanctionSuspend = "suspend"
	SanctionBan     = "ban"
)

// SanctionReasonMin and Max bound the reason every action needs.
const (
	SanctionReasonMin = 3
	SanctionReasonMax = 500
)

// Staff is what CanSanction needs to know about a player.
type Staff struct {
	ID      int64
	IsAdmin bool
	IsMod   bool
}

// CanSanction says whether actor may act on target: nobody on themselves; a mod not on a mod or
// an admin; an admin not on another admin.
func CanSanction(actor, target Staff) bool {
	switch {
	case actor.ID == target.ID:
		return false
	case actor.IsAdmin:
		return !target.IsAdmin
	case actor.IsMod:
		return !target.IsAdmin && !target.IsMod
	}
	return false
}

// SuspendDayChoices are the three lengths a suspension may have.
func SuspendDayChoices(s Settings) []int {
	return []int{s.SuspendDays1, s.SuspendDays2, s.SuspendDays3}
}

// Why CheckSanction refuses an action.
const (
	SanctionOK        = ""
	SanctionBadKind   = "kind"
	SanctionBadDays   = "days"
	SanctionBadReason = "reason"
)

// CheckSanction says why a sanction is refused, or SanctionOK: a known kind, a reason of 3 to 500
// characters, and for a suspension one of the three allowed lengths.
func CheckSanction(s Settings, kind string, days int, reason string) string {
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < SanctionReasonMin || n > SanctionReasonMax {
		return SanctionBadReason
	}
	switch kind {
	case SanctionWarn, SanctionBan:
		return SanctionOK
	case SanctionSuspend:
		for _, allowed := range SuspendDayChoices(s) {
			if days == allowed {
				return SanctionOK
			}
		}
		return SanctionBadDays
	}
	return SanctionBadKind
}

// PastSanction is one earlier action on a player, for SuggestSanction.
type PastSanction struct {
	Kind string
	Days int // a suspension's length
	At   time.Time
}

// SanctionSuggestion is the next step a mod is shown (never forced).
type SanctionSuggestion struct {
	Kind string `json:"kind"`
	Days int    `json:"days,omitempty"`
}

// SuggestSanction proposes the next step up from a player's history: a warning first; then the
// shortest suspension; a suspension inside SuspendRepeatDays of the last one steps up a length;
// after the longest, a ban. Lifted sanctions should be left out by the caller.
func SuggestSanction(s Settings, past []PastSanction, now time.Time) SanctionSuggestion {
	if len(past) == 0 {
		return SanctionSuggestion{Kind: SanctionWarn}
	}
	choices := SuspendDayChoices(s)
	window := time.Duration(s.SuspendRepeatDays) * 24 * time.Hour
	longest := 0
	for _, p := range past {
		if p.Kind == SanctionBan {
			return SanctionSuggestion{Kind: SanctionBan}
		}
		if p.Kind == SanctionSuspend && now.Sub(p.At) <= window && p.Days > longest {
			longest = p.Days
		}
	}
	if longest == 0 {
		return SanctionSuggestion{Kind: SanctionSuspend, Days: choices[0]}
	}
	for _, days := range choices {
		if days > longest {
			return SanctionSuggestion{Kind: SanctionSuspend, Days: days}
		}
	}
	return SanctionSuggestion{Kind: SanctionBan}
}

// Words a player reads.

// NoticeReportReviewed tells a reporter their report was looked at, and never what was done.
func NoticeReportReviewed() string { return "Your report was reviewed." }

// ModerationSender is who a warning's message is from.
const ModerationSender = "Moderation"

// ModerationWarning is the body of the message a warned player gets.
func ModerationWarning(reason string) string {
	return "This is a warning from the game's moderators. " + strings.TrimSpace(reason)
}
