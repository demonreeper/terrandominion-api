package game

import "math"

// The daily favour (docs/freelancer-path-plan.md F4): each power whose mission a freelancer finishes
// today gives them a weaker version of its faction perk, FavourPerkPct of the member's, until the
// mission day ends at midnight. Members get their own faction's perk at full strength instead.

// scaledPerks is the settings with every faction perk at pct percent of its strength, rounded, and
// never below 1 for a perk that is on (a favour is always worth something).
func scaledPerks(s Settings, pct int) Settings {
	scale := func(v int) int {
		if v <= 0 || pct <= 0 {
			return 0
		}
		return max(1, int(math.Round(float64(v)*float64(pct)/100)))
	}
	s.PerkSignumCrystalPct = scale(s.PerkSignumCrystalPct)
	s.PerkShingoUpgradePct = scale(s.PerkShingoUpgradePct)
	s.PerkSemaScanPct = scale(s.PerkSemaScanPct)
	s.PerkSemaRarePct = scale(s.PerkSemaRarePct)
	s.PerkIsharaBuildPct = scale(s.PerkIsharaBuildPct)
	s.PerkIsharaMissionCreditPct = scale(s.PerkIsharaMissionCreditPct)
	s.PerkTohuDroneSpeedPct = scale(s.PerkTohuDroneSpeedPct)
	s.PerkTohuCarryPct = scale(s.PerkTohuCarryPct)
	s.PerkMerkiDamagePct = scale(s.PerkMerkiDamagePct)
	s.PerkMerkiInterceptPct = scale(s.PerkMerkiInterceptPct)
	return s
}

// FavourPerks is what today's favour gives a freelancer who finished missions from these powers.
// Each power's perk touches its own systems, so the favours simply add up.
func FavourPerks(s Settings, powers []int) Perks {
	scaled := scaledPerks(s, s.FavourPerkPct)
	out := NoPerks()
	base := NoPerks()
	for _, power := range powers {
		p := FactionPerks(scaled, power)
		if p.CrystalMult != base.CrystalMult {
			out.CrystalMult = p.CrystalMult
		}
		if p.UpgradeCostMult != base.UpgradeCostMult {
			out.UpgradeCostMult = p.UpgradeCostMult
		}
		if p.ScanRangeMult != base.ScanRangeMult {
			out.ScanRangeMult = p.ScanRangeMult
		}
		if p.RareRangeMult != base.RareRangeMult {
			out.RareRangeMult = p.RareRangeMult
		}
		if p.BuildCostMult != base.BuildCostMult {
			out.BuildCostMult = p.BuildCostMult
		}
		if p.MissionCreditMult != base.MissionCreditMult {
			out.MissionCreditMult = p.MissionCreditMult
		}
		if p.DroneSpeedMult != base.DroneSpeedMult {
			out.DroneSpeedMult = p.DroneSpeedMult
		}
		if p.CarryMult != base.CarryMult {
			out.CarryMult = p.CarryMult
		}
		if p.DamageTakenMult != base.DamageTakenMult {
			out.DamageTakenMult = p.DamageTakenMult
		}
		if p.InterceptBonus != base.InterceptBonus {
			out.InterceptBonus = p.InterceptBonus
		}
	}
	return out
}

// FavourPerkLines lists today's favours with their numbers and tags ("+10% SIGNUM"), the same
// shape as a faction's perks, so the app shows them the same way.
func FavourPerkLines(s Settings, powers []int) []PerkLine {
	scaled := scaledPerks(s, s.FavourPerkPct)
	lines := []PerkLine{}
	seen := map[int]bool{}
	for _, power := range powers {
		if seen[power] {
			continue
		}
		seen[power] = true
		lines = append(lines, FactionPerkLines(scaled, power)...)
	}
	return lines
}

// PerkTagIn is the tag of the perk with this key among a player's active perk lines, or "".
func PerkTagIn(lines []PerkLine, key string) string {
	for _, line := range lines {
		if line.Key == key {
			return line.Tag
		}
	}
	return ""
}

// MissionCounts is how many missions a player gets a day: a freelancer MissionsPerDay, one from
// each power (they keep all six after the faction level, F3); a member MemberMissionsPerDay from
// their own power, and a Warden or above a command order besides.
func MissionCounts(s Settings, member bool, rank int) (command, own int) {
	if !member {
		return 0, max(0, s.MissionsPerDay)
	}
	if rank >= RankWarden {
		command = 1
	}
	return command, max(0, s.MemberMissionsPerDay)
}
