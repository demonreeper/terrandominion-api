package game

import (
	"strings"
	"testing"
	"time"
)

func TestCheckReport(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		name             string
		reporter, target int64
		detail, note     string
		open             bool
		today            int
		want             string
	}{
		{"a good report", 2, 4, "repeated_attacks", "Three nights running.", false, 0, ReportOK},
		{"no note is fine", 2, 4, "following", "", false, 4, ReportOK},
		{"yourself", 2, 2, "other", "", false, 0, ReportSelf},
		{"an unknown thing that happened", 2, 4, "spam", "", false, 0, ReportBadDetail},
		{"nothing picked", 2, 4, "", "", false, 0, ReportBadDetail},
		{"a note that is too long", 2, 4, "other", strings.Repeat("x", 301), false, 0, ReportNoteLong},
		{"a note at the limit", 2, 4, "other", strings.Repeat("x", 300), false, 0, ReportOK},
		{"already an open report on them", 2, 4, "other", "", true, 0, ReportDuplicate},
		{"the fifth of the day is the last", 2, 4, "other", "", false, 5, ReportTooMany},
		// Reporting yourself is refused before anything else is looked at.
		{"yourself, with everything else wrong", 2, 2, "spam", strings.Repeat("x", 400), true, 9, ReportSelf},
	}
	for _, c := range cases {
		if got := CheckReport(s, c.reporter, c.target, c.detail, c.note, c.open, c.today); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := CleanReportNote("  twice\n in  one night  "); got != "twice in one night" {
		t.Errorf("CleanReportNote = %q", got)
	}
}

func TestCanSanction(t *testing.T) {
	player := Staff{ID: 10}
	mod := Staff{ID: 20, IsMod: true}
	otherMod := Staff{ID: 21, IsMod: true}
	admin := Staff{ID: 30, IsAdmin: true}
	otherAdmin := Staff{ID: 31, IsAdmin: true, IsMod: true}
	cases := []struct {
		name          string
		actor, target Staff
		want          bool
	}{
		{"a mod on a player", mod, player, true},
		{"a mod on another mod", mod, otherMod, false},
		{"a mod on an admin", mod, admin, false},
		{"a mod on themselves", mod, mod, false},
		{"an admin on a player", admin, player, true},
		{"an admin on a mod", admin, mod, true},
		{"an admin on another admin", admin, otherAdmin, false},
		{"an admin on themselves", admin, admin, false},
		{"a player on a player", player, Staff{ID: 11}, false},
		{"a player on a mod", player, mod, false},
	}
	for _, c := range cases {
		if got := CanSanction(c.actor, c.target); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCheckSanction(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		kind   string
		days   int
		reason string
		want   string
	}{
		{SanctionWarn, 0, "Leave the node alone.", SanctionOK},
		{SanctionBan, 0, "Repeated griefing after two suspensions.", SanctionOK},
		{SanctionSuspend, 1, "Griefing.", SanctionOK},
		{SanctionSuspend, 3, "Griefing.", SanctionOK},
		{SanctionSuspend, 7, "Griefing.", SanctionOK},
		{SanctionSuspend, 2, "Griefing.", SanctionBadDays},
		{SanctionSuspend, 0, "Griefing.", SanctionBadDays},
		{SanctionSuspend, 30, "Griefing.", SanctionBadDays},
		{"mute", 0, "Griefing.", SanctionBadKind},
		{"", 0, "Griefing.", SanctionBadKind},
		{SanctionWarn, 0, "", SanctionBadReason},
		{SanctionWarn, 0, " x ", SanctionBadReason},
		{SanctionBan, 0, strings.Repeat("x", 501), SanctionBadReason},
	}
	for _, c := range cases {
		if got := CheckSanction(s, c.kind, c.days, c.reason); got != c.want {
			t.Errorf("CheckSanction(%q, %d, %q) = %q, want %q", c.kind, c.days, c.reason, got, c.want)
		}
	}
	// The lengths are settings.
	tuned := SettingsFrom(map[string]float64{"SuspendDays1": 2, "SuspendDays2": 5, "SuspendDays3": 14})
	if CheckSanction(tuned, SanctionSuspend, 14, "Griefing.") != SanctionOK || CheckSanction(tuned, SanctionSuspend, 7, "Griefing.") != SanctionBadDays {
		t.Error("suspension lengths should follow the settings")
	}
}

func TestSuggestSanction(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	days := func(n int) time.Time { return now.Add(-time.Duration(n) * 24 * time.Hour) }
	cases := []struct {
		name string
		past []PastSanction
		want SanctionSuggestion
	}{
		{"a clean record", nil, SanctionSuggestion{Kind: SanctionWarn}},
		{"warned before", []PastSanction{{Kind: SanctionWarn, At: days(3)}}, SanctionSuggestion{Kind: SanctionSuspend, Days: 1}},
		{"suspended a day last week", []PastSanction{{Kind: SanctionSuspend, Days: 1, At: days(6)}}, SanctionSuggestion{Kind: SanctionSuspend, Days: 3}},
		{"suspended three days this month", []PastSanction{{Kind: SanctionSuspend, Days: 3, At: days(20)}}, SanctionSuggestion{Kind: SanctionSuspend, Days: 7}},
		{"suspended a week this month", []PastSanction{{Kind: SanctionSuspend, Days: 7, At: days(10)}}, SanctionSuggestion{Kind: SanctionBan}},
		// Outside the 30-day window an old suspension no longer steps the next one up.
		{"suspended a week long ago", []PastSanction{{Kind: SanctionSuspend, Days: 7, At: days(45)}}, SanctionSuggestion{Kind: SanctionSuspend, Days: 1}},
		{"banned before (and lifted by hand since)", []PastSanction{{Kind: SanctionBan, At: days(90)}}, SanctionSuggestion{Kind: SanctionBan}},
	}
	for _, c := range cases {
		if got := SuggestSanction(s, c.past, now); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestModerationWords(t *testing.T) {
	// A reporter is told it was reviewed, never what was done.
	if got := NoticeReportReviewed(); got != "Your report was reviewed." {
		t.Errorf("NoticeReportReviewed = %q", got)
	}
	if got := ModerationWarning("  Stop attacking the same node.  "); !strings.HasSuffix(got, "Stop attacking the same node.") {
		t.Errorf("ModerationWarning = %q", got)
	}
}
