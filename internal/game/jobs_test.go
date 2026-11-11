package game

import (
	"testing"
	"time"
)

func TestCheckJobPost(t *testing.T) {
	s := DefaultSettings()
	known := map[string]bool{"credit": true, "crystal": true}
	good := JobPost{Kind: JobHit, Slots: 2, Hours: 12, Reward: map[string]int64{"credit": 100, "crystal": 20}}
	if got := CheckJobPost(s, good, RankWarden, known, 0, 0); got != JobOK {
		t.Fatalf("good post refused: %s", got)
	}
	for name, c := range map[string]struct {
		post JobPost
		rank int
		open int
		day  int
		want string
	}{
		"member":        {good, RankMember, 0, 0, JobNotWarden},
		"kind":          {JobPost{Kind: "steal", Slots: 1, Hours: 2, Reward: good.Reward}, RankWarden, 0, 0, JobBadKind},
		"slots":         {JobPost{Kind: JobHit, Slots: s.JobSlotsMax + 1, Hours: 2, Reward: good.Reward}, RankWarden, 0, 0, JobBadSlots},
		"hours":         {JobPost{Kind: JobHit, Slots: 1, Hours: s.JobHoursMax + 1, Reward: good.Reward}, RankWarden, 0, 0, JobBadHours},
		"tiny reward":   {JobPost{Kind: JobScout, Slots: 1, Hours: 2, Reward: map[string]int64{"credit": 1}}, RankWarden, 0, 0, JobBadReward},
		"unknown":       {JobPost{Kind: JobScout, Slots: 1, Hours: 2, Reward: map[string]int64{"gold": 100}}, RankWarden, 0, 0, JobBadReward},
		"negative":      {JobPost{Kind: JobScout, Slots: 1, Hours: 2, Reward: map[string]int64{"credit": 200, "crystal": -50}}, RankWarden, 0, 0, JobBadReward},
		"faction full":  {good, RankRegionalCommander, s.JobsOpenPerFaction, 0, JobFactionFull},
		"posted enough": {good, RankWarden, 0, s.JobsPostedPerDay, JobPostedFull},
	} {
		if got := CheckJobPost(s, c.post, c.rank, known, c.open, c.day); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}

func TestEscrowAndRefund(t *testing.T) {
	reward := map[string]int64{"credit": 100, "crystal": 0}
	if e := Escrow(reward, 3); e["credit"] != 300 || len(e) != 1 {
		t.Errorf("escrow %v", e)
	}
	if r := RefundOf(reward, 3, 1); r["credit"] != 200 {
		t.Errorf("refund %v", r)
	}
	if r := RefundOf(reward, 3, 3); len(r) != 0 {
		t.Errorf("all paid, nothing back: %v", r)
	}
}

func TestCheckJobTake(t *testing.T) {
	s := DefaultSettings()
	ok := JobTaker{Freelancer: true, Unlocked: true, FreeSlots: 1, JobOpen: true}
	if got := CheckJobTake(s, JobHit, ok); got != JobOK {
		t.Fatalf("good take refused: %s", got)
	}
	with := func(f func(*JobTaker)) JobTaker { c := ok; f(&c); return c }
	for name, c := range map[string]struct {
		taker JobTaker
		kind  string
		want  string
	}{
		"member":        {with(func(t *JobTaker) { t.Freelancer = false }), JobScout, JobNotFree},
		"no call yet":   {with(func(t *JobTaker) { t.Unlocked = false }), JobScout, JobNotFree},
		"no slot":       {with(func(t *JobTaker) { t.FreeSlots = 0 }), JobScout, JobNoSlot},
		"taken twice":   {with(func(t *JobTaker) { t.AlreadyTaken = true }), JobScout, JobNoSlot},
		"ended":         {with(func(t *JobTaker) { t.JobOpen = false }), JobScout, JobNoSlot},
		"too many":      {with(func(t *JobTaker) { t.TakenNow = s.JobsTakenMax }), JobScout, JobTakenFull},
		"same phone":    {with(func(t *JobTaker) { t.SharesPhone = true }), JobScout, JobSamePhone},
		"own node":      {with(func(t *JobTaker) { t.OwnsTarget = true }), JobHit, JobAgainstSelf},
		"farmed":        {with(func(t *JobTaker) { t.HitsOnOwner = s.XpAttackPerTargetDay }), JobHit, JobFarmed},
		"farm, not hit": {with(func(t *JobTaker) { t.HitsOnOwner = 99 }), JobGuard, JobOK},
	} {
		if got := CheckJobTake(s, c.kind, c.taker); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}

func TestGuardTime(t *testing.T) {
	s := DefaultSettings() // 40 m, 90 s gap
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	if secs, counts := GuardTime(s, nil, now, 10, 8); !counts || secs != 0 {
		t.Errorf("first fix starts the clock: %d %v", secs, counts)
	}
	last := now.Add(-30 * time.Second)
	if secs, _ := GuardTime(s, &last, now, 10, 8); secs != 30 {
		t.Errorf("30 s near the node: %d", secs)
	}
	long := now.Add(-10 * time.Minute)
	if secs, _ := GuardTime(s, &long, now, 10, 8); secs != s.JobGuardGapSeconds {
		t.Errorf("a long gap counts only up to the limit: %d", secs)
	}
	if _, counts := GuardTime(s, &last, now, 200, 8); counts {
		t.Error("too far away counts nothing")
	}
	if _, counts := GuardTime(s, &last, now, 10, s.HarvestMaxAccuracyM+1); counts {
		t.Error("a poor fix counts nothing")
	}
}

func TestScoutDone(t *testing.T) {
	s := DefaultSettings()
	if !ScoutDone(s, 100, 8, 150) || ScoutDone(s, 200, 8, 150) || ScoutDone(s, 10, 0, 150) {
		t.Error("scout: inside the area with a good fix")
	}
}
