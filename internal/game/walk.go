package game

import (
	"math"
	"time"
)

// Walk together (docs/walk-together-plan.md, approved 2026-11-11): two to four accepted friends who are
// physically together form a signal link. While they keep walking, every member scans further and gets more
// from node harvests, map drop pickups and harvest XP. Any faction mix. The rules here are pure; the service
// keeps the state.

// Why a linked member's bonus is paused (empty when it is on).
const (
	WalkPausedKeepWalking = "keep_walking"
	WalkPausedCapSpent    = "cap_spent"
	WalkPausedOutOfRange  = "out_of_range"
	WalkPausedWeakFix     = "weak_fix"
)

// Why a link (or a member's place in it) ended.
const (
	WalkEndLeft       = "left"
	WalkEndFellBehind = "fell_behind"
	WalkEndSilent     = "missed_heartbeats"
	WalkEndLocked     = "locked"
	WalkEndAttack     = "attack"
	WalkEndAlone      = "alone"
	WalkEndDisabled   = "disabled"
)

// WalkPcts is what a link of a given size adds, in percent: scan range, harvest and drop amount, harvest XP.
type WalkPcts struct {
	Scan  int `json:"scanPct"`
	Yield int `json:"yieldPct"`
	XP    int `json:"xpPct"`
}

// WalkLinkPcts is the confirmed table (owner 2026-11-11): 2 → 25/15/10, 3 → 40/25/20, 4 → 50/30/30 by
// default, all settings. Fewer than 2 gives nothing; more than 4 counts as 4.
func WalkLinkPcts(s Settings, size int) WalkPcts {
	switch {
	case size < 2:
		return WalkPcts{}
	case size == 2:
		return WalkPcts{s.WalkLinkScanPct2, s.WalkLinkYieldPct2, s.WalkLinkXpPct2}
	case size == 3:
		return WalkPcts{s.WalkLinkScanPct3, s.WalkLinkYieldPct3, s.WalkLinkXpPct3}
	}
	return WalkPcts{s.WalkLinkScanPct4, s.WalkLinkYieldPct4, s.WalkLinkXpPct4}
}

// WalkLinkBonus is what works on a player right now: the table for their group's size while their link is
// live, the bonus is on (moving, under the daily cap, and from W2 Bluetooth-confirmed), and the server heard
// from them within the grace time. Zero otherwise. Harvest, drop pickup, scan and XP read it at action time.
func WalkLinkBonus(s Settings, m WalkMemberState, now time.Time) WalkPcts {
	if s.WalkLinkEnabled != 1 || !m.Linked || !m.BonusActive || m.GroupSize < 2 {
		return WalkPcts{}
	}
	if now.Sub(m.LastSeenAt) > time.Duration(s.WalkLinkGraceSec)*time.Second {
		return WalkPcts{}
	}
	return WalkLinkPcts(s, m.GroupSize)
}

// WalkMemberState is a player's live link as the service reads it at action time.
type WalkMemberState struct {
	Linked      bool
	GroupSize   int
	BonusActive bool
	LastSeenAt  time.Time
}

// WalkFix is one phone's position for the link checks: where, how accurate, when the phone took it, and the
// fake-location flag.
type WalkFix struct {
	Lat, Lon  float64
	AccuracyM float64
	FixAt     time.Time
	Mock      bool
}

// WalkFixUsable is check 1: fresh (taken within WalkLinkFixAgeSec of now), accurate (WalkLinkMaxAccuracyM)
// and not marked fake. A spoof-exempt tester skips the fake flag (they test with a fake location).
func WalkFixUsable(s Settings, f WalkFix, now time.Time, exempt bool) bool {
	if f.AccuracyM <= 0 || f.AccuracyM > s.WalkLinkMaxAccuracyM {
		return false
	}
	age := now.Sub(f.FixAt)
	maxAge := time.Duration(s.WalkLinkFixAgeSec) * time.Second
	if age > maxAge || age < -maxAge {
		return false
	}
	return exempt || !f.Mock
}

// WalkGap is how far apart two fixes count: the distance less both accuracies, where the accuracies together
// may cover at most WalkLinkMaxAccuracyM, so two poor fixes can't "cover" a long gap.
func WalkGap(s Settings, a, b WalkFix) float64 {
	cover := math.Min(a.AccuracyM+b.AccuracyM, s.WalkLinkMaxAccuracyM)
	return math.Max(0, DistanceM(a.Lat, a.Lon, b.Lat, b.Lon)-cover)
}

// WalkTogether is checks 2 and 3 for a pair: within rangeM after accuracy, and the two fixes taken within
// WalkLinkFixAgeSec of each other.
func WalkTogether(s Settings, a, b WalkFix, rangeM float64) bool {
	apart := a.FixAt.Sub(b.FixAt)
	if apart < 0 {
		apart = -apart
	}
	if apart > time.Duration(s.WalkLinkFixAgeSec)*time.Second {
		return false
	}
	return WalkGap(s, a, b) <= rangeM
}

// WalkInRange says, for each member, whether they are within WalkLinkHoldM of at least one other member (and
// in time with them). A member without a fix (nil) is out.
func WalkInRange(s Settings, fixes []*WalkFix) []bool {
	out := make([]bool, len(fixes))
	for i, a := range fixes {
		if a == nil {
			continue
		}
		for j, b := range fixes {
			if i != j && b != nil && WalkTogether(s, *a, *b, s.WalkLinkHoldM) {
				out[i] = true
				break
			}
		}
	}
	return out
}

// WalkTrailPoint is one of a member's recent fixes.
type WalkTrailPoint struct {
	Lat, Lon float64
	At       time.Time
}

// WalkMoved is the "must be walking" rule: the player has moved at least WalkLinkMoveM within the last
// WalkLinkMoveWindowSec, measured from where they are now to the farthest point of their trail in the window.
// WalkLinkMoveM 0 never pauses.
func WalkMoved(s Settings, trail []WalkTrailPoint, lat, lon float64, now time.Time) bool {
	if s.WalkLinkMoveM <= 0 {
		return true
	}
	since := now.Add(-time.Duration(s.WalkLinkMoveWindowSec) * time.Second)
	for _, p := range trail {
		if p.At.Before(since) {
			continue
		}
		if DistanceM(p.Lat, p.Lon, lat, lon) >= s.WalkLinkMoveM {
			return true
		}
	}
	return false
}

// WalkCapLeft is how many bonus seconds a player has left today (WalkLinkDailyMinutes).
func WalkCapLeft(s Settings, usedSeconds int) int {
	return max(0, s.WalkLinkDailyMinutes*60-usedSeconds)
}

// WalkBonusSeconds is how much bonus time a heartbeat counts: the time since the last one, never more than
// two heartbeats (a gap is not paid for) and never more than what is left of the cap.
func WalkBonusSeconds(s Settings, since time.Duration, capLeft int) int {
	sec := int(since.Seconds())
	sec = min(sec, 2*s.WalkLinkHeartbeatSec)
	return max(0, min(sec, capLeft))
}

// WalkPaused decides a member's bonus at a heartbeat, in order: an unusable fix, out of range, the daily cap,
// then standing still. Empty means the bonus is on.
func WalkPaused(fixUsable, inRange, moved bool, capLeft int) string {
	switch {
	case !fixUsable:
		return WalkPausedWeakFix
	case !inRange:
		return WalkPausedOutOfRange
	case capLeft <= 0:
		return WalkPausedCapSpent
	case !moved:
		return WalkPausedKeepWalking
	}
	return ""
}

// WalkAmount is an amount with a faction perk multiplier and the link bonus added, never multiplied: a
// +25% perk and a +15% link make +40% (owner 2026-11-11). perkMult 1 is no perk.
func WalkAmount(amount int64, perkMult float64, linkPct int) int64 {
	if amount <= 0 {
		return amount
	}
	mult := perkMult + float64(max(0, linkPct))/100
	if mult == 1 {
		return amount
	}
	return max(1, int64(math.Round(float64(amount)*mult)))
}

// WalkScanRange is a scan range from the unperked one, with the perk multiplier and the link bonus added.
func WalkScanRange(baseM, perkMult float64, linkPct int) float64 {
	return baseM * (perkMult + float64(max(0, linkPct))/100)
}
