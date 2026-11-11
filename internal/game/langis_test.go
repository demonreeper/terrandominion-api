package game

import (
	"strings"
	"testing"
	"time"
)

func TestLangisCallDue(t *testing.T) {
	s := DefaultSettings() // LangisCallLevel 18
	mid := CallMidpointXP(18, s)
	if !LangisCallDue(mid-1, mid, s) {
		t.Error("crossing the middle of level 18 should ring")
	}
	if LangisCallDue(mid, mid+10, s) {
		t.Error("only the crossing rings")
	}
	if LangisCallDue(mid-1, XPForLevel(s.LeadFactionLevel, s), s) {
		t.Error("reaching the faction level in the same gain: FACTIONS takes over")
	}
	s.LangisCallLevel = s.LeadFactionLevel
	if LangisCallDue(0, 1<<40, s) {
		t.Error("a call level at the faction level never rings")
	}
}

func TestLangisSchedule(t *testing.T) {
	s := DefaultSettings() // 2 h after day 3, then 24 h
	day3 := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	message, call := LangisSchedule(day3, s)
	if !message.Equal(day3.Add(2*time.Hour)) || !call.Equal(day3.Add(26*time.Hour)) {
		t.Errorf("message %v call %v", message, call)
	}
}

// The lore rules: it never names a power or a faction, never says "AI", and shares no line with the
// six; it is the one voice that says its own name, once, in the level 18 call.
func TestLangisKeepsTheSecret(t *testing.T) {
	lines := append([]string{LangisLineCall18, LangisLineMsg19, LangisLineCall19, LangisLineStayFree}, LangisRareLines...)
	for _, line := range lines {
		for _, name := range personaNames {
			if strings.Contains(strings.ToLower(line), strings.ToLower(name)) {
				t.Errorf("names a power %s: %q", name, line)
			}
		}
		for _, faction := range []string{"lumen", "forge", "verdant", "veil", "slipstream", "bastion"} {
			if strings.Contains(strings.ToLower(line), faction) {
				t.Errorf("names a faction: %q", line)
			}
		}
		for _, power := range Powers {
			for kind, own := range powerLines[power] {
				if own == line {
					t.Errorf("shares power %d %s line %q", power, kind, line)
				}
			}
		}
	}
	if !strings.Contains(LangisLineCall18, "They call me Langis.") {
		t.Error("the level 18 call must say its name")
	}
	for _, line := range []string{LangisLineMsg19, LangisLineCall19} {
		if strings.Contains(line, "Langis") {
			t.Errorf("only the first call says the name: %q", line)
		}
	}
	if strings.Contains(NoticeMissedLangis(), "Langis") || NoticeMissedLangis() != "Missed call from Unknown caller" {
		t.Errorf("push %q", NoticeMissedLangis())
	}
}

func TestLangisKinds(t *testing.T) {
	for _, kind := range []string{CallLangis18, CallLangis19, MessageLangis19, LangisRareKind(time.Now())} {
		if !IsLangisKind(kind) {
			t.Errorf("%s is its kind", kind)
		}
	}
	if IsLangisKind(CallLevel19) || IsLangisRare(MessageLangis19) {
		t.Error("kinds mixed up")
	}
	if LangisCallAudio(CallLangis18) != "level18-call" || LangisCallAudio(CallLangis19) != "level19-call" {
		t.Error("audio keys")
	}
	if LangisRareLine(0) != LangisRareLines[0] || LangisRareLine(4) != LangisRareLines[1] {
		t.Error("rare lines come in turn")
	}
}

func TestLangisRareDue(t *testing.T) {
	s := DefaultSettings() // every 7 days, faction level 20
	now := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	week := now.Add(-7 * 24 * time.Hour)
	day := now.Add(-24 * time.Hour)
	if !LangisRareDue(s, 20, true, true, &week, now) || LangisRareDue(s, 20, true, true, &day, now) {
		t.Error("once a week at most")
	}
	if LangisRareDue(s, 19, true, true, nil, now) || LangisRareDue(s, 25, false, true, nil, now) || LangisRareDue(s, 25, true, false, nil, now) {
		t.Error("only a freelancer at the faction level who has heard from it")
	}
}
