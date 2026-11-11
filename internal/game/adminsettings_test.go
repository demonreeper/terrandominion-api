package game

import (
	"math"
	"strings"
	"testing"
)

func TestCheckSettingChange(t *testing.T) {
	whole := SettingRule{Min: 1, Max: 480, Step: 1}
	share := SettingRule{Min: 0, Max: 1, Step: 0.05}
	locked := SettingRule{Min: 1, Max: 1000, Step: 1, Locked: true}
	cases := []struct {
		name  string
		rule  SettingRule
		value float64
		want  string
	}{
		{"inside", whole, 48, SettingOK},
		{"at the minimum", whole, 1, SettingOK},
		{"at the maximum", whole, 480, SettingOK},
		{"below the minimum", whole, 0, SettingRange},
		{"above the maximum", whole, 481, SettingRange},
		{"a fraction where whole numbers are wanted", whole, 48.5, SettingNotWhole},
		{"a share takes fractions", share, 0.35, SettingOK},
		{"a share above 1", share, 1.01, SettingRange},
		{"a negative share", share, -0.05, SettingRange},
		{"not a number", share, math.NaN(), SettingRange},
		{"infinite", whole, math.Inf(1), SettingRange},
		// Locked wins over everything: even its own current value is refused.
		{"locked, in range", locked, 100, SettingLocked},
		{"locked, out of range", locked, 5000, SettingLocked},
	}
	for _, c := range cases {
		if got := CheckSettingChange(c.rule, c.value); got != c.want {
			t.Errorf("%s: CheckSettingChange(%+v, %v) = %q, want %q", c.name, c.rule, c.value, got, c.want)
		}
	}
}

func TestSettingGroupKey(t *testing.T) {
	if got := SettingGroupKey("tether"); got != "tether" {
		t.Errorf("known group = %q", got)
	}
	for _, unknown := range []string{"", "nonsense", "Tether"} {
		if got := SettingGroupKey(unknown); got != "other" {
			t.Errorf("SettingGroupKey(%q) = %q, want other", unknown, got)
		}
	}
	if last := SettingGroups[len(SettingGroups)-1].Key; last != "other" {
		t.Errorf("the last group should be other, got %q", last)
	}
}

func TestCleanAnnouncement(t *testing.T) {
	text, ok := CleanAnnouncement("  Double XP\n this  weekend.  ", "event", 48)
	if !ok || text != "Double XP this weekend." {
		t.Errorf("got %q, %v", text, ok)
	}
	bad := []struct {
		text, kind string
		hours      int
	}{
		{"", "info", 24},
		{"   ", "info", 24},
		{strings.Repeat("x", 141), "info", 24},
		{"Hello.", "shout", 24},
		{"Hello.", "", 24},
		{"Hello.", "info", 0},
		{"Hello.", "info", 721},
	}
	for _, c := range bad {
		if _, ok := CleanAnnouncement(c.text, c.kind, c.hours); ok {
			t.Errorf("CleanAnnouncement(%q, %q, %d) should be refused", c.text, c.kind, c.hours)
		}
	}
	if _, ok := CleanAnnouncement(strings.Repeat("x", 140), "warning", 720); !ok {
		t.Error("140 characters for 720 hours should be allowed")
	}
}
