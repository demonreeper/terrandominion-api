package game

import (
	"sort"
	"time"
)

// The JOBS board (docs/freelancer-path-plan.md F6): faction players at Warden and above post paid
// jobs, freelancers who have had the seventh voice's level 18 call take them. The reward is held in
// escrow and paid when the server confirms the job; whatever is left goes back to the poster.

const (
	JobHit   = "hit"   // land a hit on a Tether Node
	JobScout = "scout" // scan inside an area
	JobGuard = "guard" // stay near a node for JobGuardMinutes
)

// JobKinds in the order the board offers them.
var JobKinds = []string{JobHit, JobScout, JobGuard}

// Why a job can't be posted or taken.
const (
	JobOK          = ""
	JobNotWarden   = "not_warden"   // only Captains and above post (faction-ranks-plan.md)
	JobBadKind     = "kind"         // not a job kind
	JobBadSlots    = "slots"        // 1 to JobSlotsMax
	JobBadHours    = "hours"        // JobHoursMin to JobHoursMax
	JobBadReward   = "reward"       // JobRewardMin to JobRewardMax per slot, known resources, nothing negative
	JobOwnTarget   = "own_target"   // a hit on the poster's own or their faction's Tether Node
	JobFactionFull = "faction_full" // JobsOpenPerFaction open already
	JobPostedFull  = "posted_full"  // JobsPostedPerDay posted in 24 hours
	JobNotFree     = "not_free"     // only freelancers who have had the level 18 call take jobs
	JobTakenFull   = "taken_full"   // JobsTakenMax taken and not finished
	JobNoSlot      = "no_slot"      // every slot is taken or the job ended
	JobAgainstSelf = "self"         // a hit on the taker's own Tether Node
	JobSamePhone   = "same_phone"   // the poster and the taker sign in on the same phone
	JobFarmed      = "farmed"       // the taker reached the day's attack limit on that owner
	JobBadTarget   = "target"       // a guard job names a node or a base, never both or neither
)

// CheckGuardTarget says whether a guard job names exactly one thing to guard: a node or a base
// (base-building-plan.md, Defence). Other kinds name neither.
func CheckGuardTarget(kind string, nodeID, baseID int64) string {
	if kind != JobGuard {
		if baseID != 0 {
			return JobBadTarget
		}
		return JobOK
	}
	if (nodeID != 0) == (baseID != 0) {
		return JobBadTarget
	}
	return JobOK
}

// GuardRadiusM is how near a guard has to be: a base's zone (0 for a node) or JobGuardRadiusM,
// whichever is larger. A base's guard spot is its core.
func GuardRadiusM(s Settings, zoneM float64) float64 {
	if r := float64(s.JobGuardRadiusM); r > zoneM {
		return r
	}
	return zoneM
}

// JobPost is what a poster asks for.
type JobPost struct {
	Kind   string
	Slots  int
	Hours  int
	Reward map[string]int64
}

// RewardTotal adds up a reward's amounts, all resources together.
func RewardTotal(reward map[string]int64) int64 {
	var total int64
	for _, amount := range reward {
		total += amount
	}
	return total
}

// CheckJobPost says why a job can't be posted, or JobOK. known lists the game's resources.
func CheckJobPost(s Settings, p JobPost, rank int, known map[string]bool, openForFaction, postedToday int) string {
	switch {
	case !Can(rank, false, PowerPostJobs):
		return JobNotWarden
	case p.Kind != JobHit && p.Kind != JobScout && p.Kind != JobGuard:
		return JobBadKind
	case p.Slots < 1 || p.Slots > s.JobSlotsMax:
		return JobBadSlots
	case p.Hours < s.JobHoursMin || p.Hours > s.JobHoursMax:
		return JobBadHours
	case openForFaction >= s.JobsOpenPerFaction:
		return JobFactionFull
	case postedToday >= s.JobsPostedPerDay:
		return JobPostedFull
	}
	for resource, amount := range p.Reward {
		if amount < 0 || !known[resource] {
			return JobBadReward
		}
	}
	if total := RewardTotal(p.Reward); total < int64(s.JobRewardMin) || total > int64(s.JobRewardMax) {
		return JobBadReward
	}
	return JobOK
}

// Escrow is what posting a job takes from the poster's stored resources: the reward for every slot.
func Escrow(reward map[string]int64, slots int) map[string]int64 {
	out := map[string]int64{}
	for resource, amount := range reward {
		if amount > 0 {
			out[resource] = amount * int64(slots)
		}
	}
	return out
}

// JobTaker is who wants to take a job, and what the server knows about them against it.
type JobTaker struct {
	Freelancer   bool
	Unlocked     bool // had the seventh voice's level 18 call
	TakenNow     int  // jobs taken and not finished
	OwnsTarget   bool // a hit on their own Tether Node
	SharesPhone  bool // signs in on a phone the poster signs in on
	HitsOnOwner  int  // attacks that paid them XP against the target's owner today
	FreeSlots    int  // slots left on the job
	AlreadyTaken bool
	JobOpen      bool
}

// CheckJobTake says why a freelancer can't take a job, or JobOK. The anti-farming limits: never
// against yourself, never for a poster on your own phone, and a hit job never past the day's attack
// limit on that owner (XpAttackPerTargetDay).
func CheckJobTake(s Settings, kind string, t JobTaker) string {
	switch {
	case !t.Freelancer || !t.Unlocked:
		return JobNotFree
	case !t.JobOpen || t.FreeSlots <= 0 || t.AlreadyTaken:
		return JobNoSlot
	case t.TakenNow >= s.JobsTakenMax:
		return JobTakenFull
	case t.SharesPhone:
		return JobSamePhone
	case kind == JobHit && t.OwnsTarget:
		return JobAgainstSelf
	case kind == JobHit && s.XpAttackPerTargetDay > 0 && t.HitsOnOwner >= s.XpAttackPerTargetDay:
		return JobFarmed
	}
	return JobOK
}

// GuardTime is how many seconds one fix adds to a guard job: the time since the last fix that counted,
// up to JobGuardGapSeconds, and only when this fix is near enough and good enough. A gap means the
// phone stopped reporting, so it counts only up to the limit. radiusM is the job's own radius (a base's
// zone); never less than JobGuardRadiusM.
func GuardTime(s Settings, last *time.Time, now time.Time, distanceM, accuracyM, radiusM float64) (seconds int, counts bool) {
	if accuracyM <= 0 || accuracyM > s.HarvestMaxAccuracyM || distanceM > GuardRadiusM(s, radiusM)+accuracyM {
		return 0, false
	}
	if last == nil {
		return 0, true
	}
	gap := now.Sub(*last).Seconds()
	if gap <= 0 {
		return 0, true
	}
	if gap > float64(s.JobGuardGapSeconds) {
		gap = float64(s.JobGuardGapSeconds)
	}
	return int(gap), true
}

// ScoutDone says whether a scan from this fix completes a scout job: inside the area, with a fix good
// enough to trust.
func ScoutDone(s Settings, distanceM, accuracyM, radiusM float64) bool {
	return accuracyM > 0 && accuracyM <= s.HarvestMaxAccuracyM && distanceM <= radiusM
}

// RefundOf is what goes back to the poster when a job ends: the reward for every slot not paid.
func RefundOf(reward map[string]int64, slots, done int) map[string]int64 {
	if left := slots - done; left > 0 {
		return Escrow(reward, left)
	}
	return map[string]int64{}
}

// SortedResources is a reward's resources in a fixed order, for stable payouts and text.
func SortedResources(reward map[string]int64) []string {
	keys := make([]string, 0, len(reward))
	for k := range reward {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NoticeJobDone tells the poster a freelancer finished one of their jobs; NoticeJobPaid tells the
// freelancer they were paid; NoticeJobEnded tells the poster what came back.
func NoticeJobDone(kind string) string {
	return "A freelancer finished your " + JobTitle(kind) + " job."
}
func NoticeJobPaid(kind string) string { return "Job done: " + JobTitle(kind) + ". You were paid." }
func NoticeJobEnded(kind string) string {
	return "Your " + JobTitle(kind) + " job ended. What wasn't paid out went back to your storage."
}

// NoticeBaseGuardEnded tells the poster their guard job at a base ended because the base fell;
// NoticeBaseGuardLost tells a freelancer who had taken it.
func NoticeBaseGuardEnded(base string) string {
	return "Your guard job at " + base + " ended: the base fell. What wasn't paid out went back to your storage."
}
func NoticeBaseGuardLost(base string) string {
	return "The guard job at " + base + " ended: the base fell."
}

// JobTitle is a job kind in a sentence.
func JobTitle(kind string) string {
	switch kind {
	case JobHit:
		return "hit"
	case JobScout:
		return "scout"
	case JobGuard:
		return "guard"
	}
	return kind
}
