package game

import (
	"math"
	"strings"
	"unicode/utf8"
)

// Admin settings (docs/profile-plan.md, A1): the groups the in-game settings screen folds into, and
// the checks on a change and on the announcement.

// SettingGroup is one foldable section of the settings screen.
type SettingGroup struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// SettingGroups lists the groups in the order the screen shows them. A setting whose group is not
// here is shown under "other".
var SettingGroups = []SettingGroup{
	{"harvest", "Harvesting and nodes"},
	{"tether", "Tether Nodes"},
	{"combat", "Combat and contests"},
	{"drones", "Drones"},
	{"crafting", "Crafting and gear"},
	{"missions", "Missions and calls"},
	{"factions", "Factions and perks"},
	{"progress", "Levels and welcome"},
	{"profile", "Profile"},
	{"moderation", "Moderation"},
	{"anticheat", "Anti-cheat"},
	{"app", "App and accounts"},
	{"other", "Other"},
}

// SettingGroupKey is a setting's group as the screen files it: its own when known, else "other".
func SettingGroupKey(group string) string {
	for _, g := range SettingGroups {
		if g.Key == group {
			return group
		}
	}
	return "other"
}

// SettingRule is what a setting's row allows.
type SettingRule struct {
	Min, Max, Step float64
	Locked         bool
}

// Reasons CheckSettingChange refuses a value.
const (
	SettingOK       = ""
	SettingLocked   = "locked"
	SettingRange    = "range"
	SettingNotWhole = "whole"
)

// CheckSettingChange says why a new value is refused, or SettingOK: a locked setting never
// changes, the value must lie in min..max, and a setting stepped in whole numbers takes only
// whole numbers.
func CheckSettingChange(rule SettingRule, value float64) string {
	switch {
	case rule.Locked:
		return SettingLocked
	case math.IsNaN(value) || math.IsInf(value, 0) || value < rule.Min || value > rule.Max:
		return SettingRange
	case rule.Step >= 1 && value != math.Trunc(value):
		return SettingNotWhole
	}
	return SettingOK
}

// Announcement limits.
const (
	AnnouncementMaxLength = 140
	AnnouncementMaxHours  = 720
)

// AnnouncementKinds are the banner's looks, in the picker's order.
var AnnouncementKinds = []string{"info", "event", "warning"}

// CleanAnnouncement trims an announcement to one line and says whether it can be posted: 1 to 140
// characters, a known kind, and 1 to 720 hours.
func CleanAnnouncement(text, kind string, hours int) (string, bool) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || utf8.RuneCountInString(text) > AnnouncementMaxLength {
		return text, false
	}
	if hours < 1 || hours > AnnouncementMaxHours {
		return text, false
	}
	for _, k := range AnnouncementKinds {
		if k == kind {
			return text, true
		}
	}
	return text, false
}
