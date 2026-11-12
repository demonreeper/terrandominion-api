package game

import "time"

// Faction bases, B6 (base-building-plan.md 7): upkeep. Bases are permanent only while the faction looks after them,
// so ground never stays locked by a dead faction.

// UpkeepStep is what the upkeep tick does to a base now.
type UpkeepStep int

const (
	UpkeepNone     UpkeepStep = iota
	UpkeepFade                // no member on site for BaseUpkeepDays: it fades
	UpkeepDropTier            // faded, and another BaseFadeDropDays have passed: it loses a tier
	UpkeepCollapse            // a faded Spire after BaseSpireCollapseDays
)

// BaseUpkeep decides a base's upkeep: lastVisit is when a member last stood on site; fadedAt when it began to fade
// (nil: not fading); droppedAt when the fade last cost it a tier (nil: not yet).
func BaseUpkeep(tier int, lastVisit time.Time, fadedAt, droppedAt *time.Time, now time.Time, s Settings) UpkeepStep {
	day := 24 * time.Hour
	if fadedAt == nil {
		if now.Sub(lastVisit) >= time.Duration(s.BaseUpkeepDays*float64(day)) {
			return UpkeepFade
		}
		return UpkeepNone
	}
	if tier <= 1 {
		if now.Sub(*fadedAt) >= time.Duration(s.BaseSpireCollapseDays*float64(day)) {
			return UpkeepCollapse
		}
		return UpkeepNone
	}
	from := *fadedAt
	if droppedAt != nil && droppedAt.After(from) {
		from = *droppedAt
	}
	if now.Sub(from) >= time.Duration(s.BaseFadeDropDays*float64(day)) {
		return UpkeepDropTier
	}
	return UpkeepNone
}

// FadedShieldShare is a faded base's shield: BaseFadedShieldPct of what it would stop.
func FadedShieldShare(share float64, faded bool, s Settings) float64 {
	if !faded {
		return share
	}
	return share * clamp01(s.BaseFadedShieldPct/100)
}

// NoticeBaseFading is the members' message (the plan's own words).
func NoticeBaseFading(base string) string { return base + " is fading. Walk to it." }
