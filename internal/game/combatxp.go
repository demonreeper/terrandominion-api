package game

import "time"

// XP ledger actions for fighting, counted against the other player for the daily cap.
const (
	XPActionAttack    = "attack"
	XPActionIntercept = "intercept"
)

// AttackXP is what a hit pays its attacker: XpAttack, plus XpDestroy when it destroyed the Tether
// Node, or nothing once XpAttackPerTargetDay hits on this player have paid today. A hit that did
// no damage (a drone shot down) pays nothing.
func AttackXP(s Settings, landed, destroyed bool, paidToday int) int {
	if !landed || paidToday >= s.XpAttackPerTargetDay {
		return 0
	}
	if destroyed {
		return s.XpAttack + s.XpDestroy
	}
	return s.XpAttack
}

// InterceptXP is what intercepting a drone pays the defender, under the same daily cap per attacker.
func InterceptXP(s Settings, paidToday int) int {
	if paidToday >= s.XpAttackPerTargetDay {
		return 0
	}
	return s.XpIntercept
}

// XPDayStart is when today's daily cap began: midnight at MissionDayOffsetMinutes (Maldives time),
// the same day the orders use.
func XPDayStart(s Settings, now time.Time) time.Time {
	day, _ := MissionDay(s, now)
	return day.Add(-time.Duration(s.MissionDayOffsetMinutes) * time.Minute)
}
