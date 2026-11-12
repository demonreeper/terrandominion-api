package game

import "time"

// Faction ranks v2 (docs/faction-ranks-plan.md, owner 2026-11-12): eight ranks as data, top first, each with its
// powers as a lookup, so a rank can be added or renamed without touching every check. Ranks are stored as numbers
// (faction_members.rank); a member holds the highest rank whose level is at or below theirs.

// Rank levels. Gaps leave room for ranks added later.
const (
	RankProspect          = 5
	RankMember            = 10
	RankCaptain           = 30
	RankWarden            = 50
	RankLead              = 70
	RankRegionalCommander = 90
	RankAssistantHead     = 95
	RankHead              = 100
)

// Power is one thing a rank lets a member do.
type Power string

const (
	PowerGive           Power = "give"            // give to base projects on site
	PowerUpgrade        Power = "upgrade"         // upgrade and repair at bases
	PowerVote           Power = "vote"            // vote in the region
	PowerHoldSeat       Power = "hold_seat"       // stand for and hold a seat
	PowerNomad          Power = "nomad"           // choose NOMAD
	PowerSquadLead      Power = "squad_lead"      // lead a squad
	PowerPostJobs       Power = "post_jobs"       // post on the JOBS board
	PowerBaseMissions   Power = "base_missions"   // post base missions to a squad
	PowerVaultOnSite    Power = "vault_on_site"   // give from the vault on site, up to the daily cap
	PowerVaultAnywhere  Power = "vault_anywhere"  // pay from the vault anywhere in the region
	PowerProposeBase    Power = "propose_base"    // plant a flag
	PowerStartProjects  Power = "start_projects"  // start base projects
	PowerStartT4        Power = "start_t4"        // start Citadel projects
	PowerApproveBase    Power = "approve_base"    // answer flags, raise bases
	PowerApproveNomads  Power = "approve_nomads"  // say yes or no to Nomads moving in
	PowerAppointCaptain Power = "appoint_captain" // make a member a Captain
	PowerAppointWarden  Power = "appoint_warden"  // make a member a Warden
	PowerAppointLead    Power = "appoint_lead"    // make a member an atoll/city lead
	PowerCommandOrders  Power = "command_orders"  // gets a power's command order each day
	PowerEveryRegion    Power = "every_region"    // acts in every region of the faction
)

// RankDef is one rung of the ladder.
type RankDef struct {
	Key    string  `json:"key"`
	Level  int     `json:"level"`
	Title  string  `json:"title"`
	Powers []Power `json:"powers"`
}

func powers(lists ...[]Power) []Power {
	var out []Power
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

var (
	prospectPowers = []Power{PowerGive}
	memberPowers   = powers(prospectPowers, []Power{PowerUpgrade, PowerVote, PowerHoldSeat, PowerNomad})
	captainPowers  = powers(memberPowers, []Power{PowerSquadLead, PowerPostJobs, PowerBaseMissions, PowerVaultOnSite})
	wardenPowers   = powers(captainPowers, []Power{PowerVaultAnywhere, PowerProposeBase, PowerStartProjects,
		PowerApproveNomads, PowerAppointCaptain, PowerCommandOrders})
	leadPowers      = powers(wardenPowers, []Power{PowerStartT4, PowerAppointWarden})
	commanderPowers = powers(leadPowers, []Power{PowerApproveBase, PowerAppointLead})
)

// Ranks is the ladder, top first.
var Ranks = []RankDef{
	{"global_head", RankHead, "Global Head", powers(commanderPowers, []Power{PowerEveryRegion})},
	// A Warden's powers in every region of the faction, nothing more; acts as Global Head when that seat is vacant.
	{"assistant_global_head", RankAssistantHead, "Assistant Global Head", powers(wardenPowers, []Power{PowerEveryRegion})},
	{"regional_commander", RankRegionalCommander, "Regional Commander", commanderPowers},
	{"lead", RankLead, "Atoll/City Lead", leadPowers},
	{"warden", RankWarden, "Warden", wardenPowers},
	{"captain", RankCaptain, "Captain", captainPowers},
	{"member", RankMember, "Member", memberPowers},
	{"prospect", RankProspect, "Prospect", prospectPowers},
}

// RankAt is the rank a stored level holds: the highest rung at or below it (a level under every rung is Prospect).
func RankAt(level int) RankDef {
	for _, r := range Ranks {
		if level >= r.Level {
			return r
		}
	}
	return Ranks[len(Ranks)-1]
}

// RankByKey finds a rung by its key.
func RankByKey(key string) (RankDef, bool) {
	for _, r := range Ranks {
		if r.Key == key {
			return r, true
		}
	}
	return RankDef{}, false
}

// ActingCommanderTitle is the rank title of a member holding the seat as acting Commander (leader succession).
const ActingCommanderTitle = "Acting Commander"

// RankTitle names a rank for players. The faction's power holds the Global Head seat for now; players see it as the
// faction's Commander (the hidden lore power, never a player).
func RankTitle(level int) string {
	if level >= RankHead {
		return "Commander"
	}
	return RankAt(level).Title
}

// Can says whether a rank holds a power. A Nomad (nomad true) has a Member's powers at most, and no vote or seat.
func Can(level int, nomad bool, p Power) bool {
	if nomad {
		if level > RankMember {
			level = RankMember
		}
		if p == PowerVote || p == PowerHoldSeat {
			return false
		}
	}
	for _, have := range RankAt(level).Powers {
		if have == p {
			return true
		}
	}
	return false
}

// PowersOf lists a member's powers, for the app.
func PowersOf(level int, nomad bool) []string {
	out := []string{}
	for _, p := range RankAt(level).Powers {
		if Can(level, nomad, p) {
			out = append(out, string(p))
		}
	}
	return out
}

// CanAppoint says whether a member at level may make another member (at targetLevel) the rank `to`: they need its
// appoint power, the target must be below it, and nobody appoints at or above their own rank.
func CanAppoint(level int, nomad bool, targetLevel int, to int) bool {
	var need Power
	switch to {
	case RankCaptain:
		need = PowerAppointCaptain
	case RankWarden:
		need = PowerAppointWarden
	case RankLead:
		need = PowerAppointLead
	default:
		return false
	}
	return Can(level, nomad, need) && targetLevel < to && to < level && targetLevel >= RankMember
}

// ActsAsGlobalHead: the Assistant Global Head acts as Global Head at once while that seat is vacant.
func ActsAsGlobalHead(level int, headVacant bool) bool {
	return level >= RankHead || (headVacant && level >= RankAssistantHead)
}

// RankSlots is how many of a rank a region may hold (0 for ranks without a limit).
func RankSlots(level int, s Settings) int {
	switch level {
	case RankCaptain:
		return s.RankSlotsCaptain
	case RankWarden:
		return s.RankSlotsWarden
	case RankLead:
		return s.RankSlotsLead
	}
	return 0
}

// PointsRank is the rank contribution points alone would give (never Regional Commander or above): Captain, Warden
// and Lead at their totals, else Member.
func PointsRank(points int64, s Settings) int {
	switch {
	case s.RankPointsLead > 0 && points >= int64(s.RankPointsLead):
		return RankLead
	case s.RankPointsWarden > 0 && points >= int64(s.RankPointsWarden):
		return RankWarden
	case s.RankPointsCaptain > 0 && points >= int64(s.RankPointsCaptain):
		return RankCaptain
	}
	return RankMember
}

// NextPromotion is the one rung a member climbs by points now (0 for none): from Member, Captain or Warden only,
// one step at a time, never past the points rank. Slots are checked by the caller.
func NextPromotion(level int, nomad bool, points int64, s Settings) int {
	if nomad || level < RankMember || level >= RankLead {
		return 0
	}
	target := PointsRank(points, s)
	for _, step := range []int{RankCaptain, RankWarden, RankLead} {
		if step > level && step <= target {
			return step
		}
	}
	return 0
}

// ProspectDone says whether a Prospect becomes Member: after ProspectDays or at ProspectPoints, whichever first.
func ProspectDone(joined time.Time, points int64, now time.Time, s Settings) bool {
	if s.ProspectPoints > 0 && points >= int64(s.ProspectPoints) {
		return true
	}
	return !now.Before(joined.Add(time.Duration(s.ProspectDays * float64(24*time.Hour))))
}

// Nomads (docs/faction-ranks-plan.md, Nomads).

// NomadMoveAt is when a Nomad may next move home regions: NomadMoveCooldownDays after their last move, or after they
// became a Nomad again (cooldownFrom; zero when there was neither).
func NomadMoveAt(cooldownFrom time.Time, s Settings) time.Time {
	if cooldownFrom.IsZero() {
		return time.Time{}
	}
	return cooldownFrom.Add(time.Duration(s.NomadMoveCooldownDays * float64(24*time.Hour)))
}

// NomadAskExpiry is when an ask made now comes down unanswered.
func NomadAskExpiry(now time.Time, s Settings) time.Time {
	return now.Add(time.Duration(s.NomadAskHours * float64(time.Hour)))
}
