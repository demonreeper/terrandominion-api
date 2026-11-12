package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// The JOBS board (docs/freelancer-path-plan.md F6). Faction players at Warden and above post paid
// jobs; freelancers who have had the seventh voice's level 18 call take them. The reward is held in
// escrow from the poster's stored resources, paid when the server sees the job done, and what is
// left goes back to the poster when the job runs out.

const jobsListed = 50

// JobsView is GET /v1/jobs: the board for a freelancer, or a faction's posted jobs for its Wardens.
type JobsView struct {
	Board    []store.Job `json:"board"`
	Posted   []store.Job `json:"posted"`
	CanTake  bool        `json:"canTake"`
	CanPost  bool        `json:"canPost"`
	Kinds    []string    `json:"kinds"`
	SlotsMax int         `json:"slotsMax"`
	HoursMin int         `json:"hoursMin"`
	HoursMax int         `json:"hoursMax"`
	// The rest of the post dialog's limits.
	RewardMin    int `json:"rewardMin"`
	RewardMax    int `json:"rewardMax"`
	GuardMinutes int `json:"guardMinutes"`
}

func (s *Service) Jobs(ctx context.Context, playerID int64) (JobsView, error) {
	settings := s.Settings(ctx)
	out := JobsView{Board: []store.Job{}, Posted: []store.Job{}, Kinds: game.JobKinds,
		SlotsMax: settings.JobSlotsMax, HoursMin: settings.JobHoursMin, HoursMax: settings.JobHoursMax,
		RewardMin: settings.JobRewardMin, RewardMax: settings.JobRewardMax, GuardMinutes: settings.JobGuardMinutes}
	membership, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	if membership != nil {
		out.CanPost = game.Can(membership.Rank, membership.Nomad, game.PowerPostJobs)
		if out.CanPost {
			out.Posted, err = store.PostedJobs(ctx, s.db, membership.FactionID, jobsListed)
		}
		return out, err
	}
	// The board opens to a freelancer with the seventh voice's level 18 call; until then it isn't there.
	if out.CanTake, err = store.LangisContact(ctx, s.db, playerID); err != nil || !out.CanTake {
		return out, err
	}
	out.Board, err = store.OpenJobs(ctx, s.db, playerID, jobsListed)
	return out, err
}

// PostJob is what a Warden asks for: a kind, a target (a Tether Node for a hit, a node for a guard,
// the poster's position for a scout), the reward per slot, slots and hours.
type PostJob struct {
	Kind      string
	TetherID  int64
	NodeID    int64
	Lat, Lon  float64
	AccuracyM float64
	Reward    map[string]int64
	Slots     int
	Hours     int
}

func jobRefusal(reason string) error {
	switch reason {
	case game.JobNotWarden:
		return apperr.JobNotYours.Say("Only Captains and above post jobs.")
	case game.JobFactionFull:
		return apperr.JobLimit.Say("Your faction has as many open jobs as it may.")
	case game.JobPostedFull:
		return apperr.JobLimit.Say("You have posted as many jobs as one day allows.")
	case game.JobTakenFull:
		return apperr.JobLimit.Say("You have as many jobs as you can take. Finish one first.")
	case game.JobOwnTarget:
		return apperr.JobBad.Say("You can't post a job against your own faction.")
	case game.JobNotFree:
		return apperr.JobNotYours.Say("Only freelancers take jobs.")
	case game.JobNoSlot:
		return apperr.JobGone
	case game.JobAgainstSelf:
		return apperr.JobNotYours.Say("You can't take a job against your own Tether Node.")
	case game.JobSamePhone:
		return apperr.JobNotYours.Say("You can't take a job posted from your own phone.")
	case game.JobFarmed:
		return apperr.JobNotYours.Say("You have hit that player's Tether Nodes enough for one day.")
	}
	return apperr.JobBad
}

func (s *Service) PostJob(ctx context.Context, posterID int64, p PostJob) (store.Job, error) {
	settings := s.Settings(ctx)
	now := s.now()
	var out store.Job
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, posterID); err != nil {
			return err
		}
		membership, err := store.MembershipOf(ctx, tx, posterID)
		if err != nil {
			return err
		}
		rank, faction := 0, 0
		if membership != nil {
			rank, faction = membership.Rank, membership.FactionID
		}
		stored, err := store.Uploaded(ctx, tx, posterID)
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for resource := range stored {
			known[resource] = true
		}
		open, err := store.OpenJobsOfFaction(ctx, tx, faction)
		if err != nil {
			return err
		}
		posted, err := store.JobsPostedSince(ctx, tx, posterID, now.Add(-24*time.Hour))
		if err != nil {
			return err
		}
		if why := game.CheckJobPost(settings, game.JobPost{Kind: p.Kind, Slots: p.Slots, Hours: p.Hours, Reward: p.Reward},
			rank, known, open, posted); why != game.JobOK {
			return jobRefusal(why)
		}
		// Jobs are posted only while standing in the home region (faction-ranks-plan.md: a move never lets a player
		// act in a country they aren't in).
		if away, err := store.AwayFromHome(ctx, tx, posterID, settings.CountryAtMaxKm); err != nil {
			return err
		} else if away {
			return apperr.AwayFromHome
		}

		job := store.NewJob{PosterID: posterID, FactionID: faction, Kind: p.Kind, Reward: p.Reward, Slots: p.Slots,
			Deadline: now.Add(time.Duration(p.Hours) * time.Hour)}
		switch p.Kind {
		case game.JobHit:
			target, err := store.TetherBriefOf(ctx, tx, p.TetherID)
			if errors.Is(err, store.ErrNotFound) || (err == nil && target.Status != "active") {
				return apperr.TetherGone
			}
			if err != nil {
				return err
			}
			// Never against your own faction or yourself: that would be paying friends to farm.
			if target.OwnerID == posterID || (target.FactionID != nil && *target.FactionID == faction) {
				return jobRefusal(game.JobOwnTarget)
			}
			ownerFaction, err := factionOf(ctx, tx, target.OwnerID)
			if err != nil {
				return err
			}
			if ownerFaction == faction {
				return jobRefusal(game.JobOwnTarget)
			}
			job.TetherID, job.Lat, job.Lon, job.RadiusM = p.TetherID, target.Lat, target.Lon, 0
		case game.JobGuard:
			lat, lon, active, err := store.NodeBrief(ctx, tx, p.NodeID)
			if errors.Is(err, store.ErrNotFound) || (err == nil && !active) {
				return apperr.NodeInactive
			}
			if err != nil {
				return err
			}
			job.NodeID, job.Lat, job.Lon, job.RadiusM = p.NodeID, lat, lon, float64(settings.JobGuardRadiusM)
		case game.JobScout:
			if !game.ValidCoordinate(p.Lat, p.Lon) {
				return apperr.BadRequest.With("bad coordinate")
			}
			job.Lat, job.Lon, job.RadiusM = p.Lat, p.Lon, float64(settings.JobScoutRadiusM)
		}

		// Escrow: the reward for every slot leaves the poster's storage now.
		escrow := game.Escrow(p.Reward, p.Slots)
		for resource, amount := range escrow {
			if stored[resource] < amount {
				return apperr.JobFunds.Say("You need %d %s in storage to pay every slot.", amount, resource).
					WithData(map[string]any{"short": map[string]int64{resource: amount - stored[resource]}})
			}
		}
		for _, resource := range game.SortedResources(escrow) {
			if err := store.AddUploaded(ctx, tx, posterID, resource, -escrow[resource]); err != nil {
				return err
			}
		}
		id, err := store.InsertJob(ctx, tx, job)
		if err != nil {
			return err
		}
		out, err = store.JobForUpdate(ctx, tx, id)
		return err
	})
	return out, err
}

// TakeJob gives a freelancer a slot on a job, with the anti-farming checks.
func (s *Service) TakeJob(ctx context.Context, playerID, jobID int64) (store.Job, error) {
	settings := s.Settings(ctx)
	now := s.now()
	var out store.Job
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		job, err := store.JobForUpdate(ctx, tx, jobID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.JobGone
		}
		if err != nil {
			return err
		}
		membership, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		unlocked, err := store.LangisContact(ctx, tx, playerID)
		if err != nil {
			return err
		}
		taken, err := store.TakenCount(ctx, tx, playerID)
		if err != nil {
			return err
		}
		already, err := store.HasTake(ctx, tx, jobID, playerID)
		if err != nil {
			return err
		}
		shares := false
		if job.PosterID != 0 {
			if shares, err = store.ShareAPhone(ctx, tx, playerID, job.PosterID); err != nil {
				return err
			}
		}
		hits := 0
		if job.Kind == game.JobHit && job.TetherOwn != 0 {
			day, _ := game.MissionDay(settings, now)
			start := day.Add(-time.Duration(settings.MissionDayOffsetMinutes) * time.Minute)
			if hits, err = store.PaidXPAgainst(ctx, tx, playerID, job.TetherOwn, "attack", start); err != nil {
				return err
			}
		}
		if why := game.CheckJobTake(settings, job.Kind, game.JobTaker{
			Freelancer: membership == nil, Unlocked: unlocked, TakenNow: taken, OwnsTarget: job.TetherOwn == playerID,
			SharesPhone: shares, HitsOnOwner: hits, FreeSlots: job.Slots - job.Taken, AlreadyTaken: already,
			JobOpen: job.Status == "open" && job.Deadline.After(now),
		}); why != game.JobOK {
			return jobRefusal(why)
		}
		if err := store.InsertTake(ctx, tx, jobID, playerID); err != nil {
			return err
		}
		out, err = store.JobForUpdate(ctx, tx, jobID)
		out.MyStatus = "taken"
		return err
	})
	return out, err
}

// finishJob pays a freelancer for a job the server has just seen done, and tells both sides.
func finishJob(ctx context.Context, tx *sql.Tx, takerID int64, job store.Job, now time.Time) error {
	if err := store.FinishTake(ctx, tx, job.TakeID, job.ID, now); err != nil {
		return err
	}
	for _, resource := range game.SortedResources(job.Reward) {
		if amount := job.Reward[resource]; amount > 0 {
			if err := store.AddUploaded(ctx, tx, takerID, resource, amount); err != nil {
				return err
			}
		}
	}
	if err := notify(ctx, tx, store.NewNotice{PlayerID: takerID, Kind: "job_paid", Body: game.NoticeJobPaid(job.Kind)}); err != nil {
		return err
	}
	if job.PosterID != 0 {
		return notify(ctx, tx, store.NewNotice{PlayerID: job.PosterID, Kind: "job_done", Body: game.NoticeJobDone(job.Kind)})
	}
	return nil
}

// jobHit counts a hit on a Tether Node toward the hitter's hit jobs on it. Call it inside the
// attack's (or the landing drone's) transaction.
func (s *Service) jobHit(ctx context.Context, tx *sql.Tx, playerID, tetherID int64) error {
	jobs, err := store.OpenTakesOf(ctx, tx, playerID, game.JobHit)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.TetherID == tetherID {
			if err := finishJob(ctx, tx, playerID, job, s.now()); err != nil {
				return err
			}
		}
	}
	return nil
}

// JobFix counts a scan's location fix toward a freelancer's scout and guard jobs: a scan inside a
// scout job's area finishes it; time near a guard job's node adds up until JobGuardMinutes.
// It never fails the scan: a problem is logged.
func (s *Service) JobFix(ctx context.Context, playerID int64, lat, lon, accuracyM float64) {
	if accuracyM <= 0 || !game.ValidCoordinate(lat, lon) {
		return
	}
	settings := s.Settings(ctx)
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		scouts, err := store.OpenTakesOf(ctx, tx, playerID, game.JobScout)
		if err != nil {
			return err
		}
		for _, job := range scouts {
			if game.ScoutDone(settings, game.DistanceM(lat, lon, job.Lat, job.Lon), accuracyM, job.RadiusM) {
				if err := finishJob(ctx, tx, playerID, job, now); err != nil {
					return err
				}
			}
		}
		guards, err := store.OpenTakesOf(ctx, tx, playerID, game.JobGuard)
		if err != nil {
			return err
		}
		for _, job := range guards {
			seconds, counts := game.GuardTime(settings, job.LastFixAt, now, game.DistanceM(lat, lon, job.Lat, job.Lon), accuracyM)
			if !counts {
				continue
			}
			if err := store.RecordGuard(ctx, tx, job.TakeID, seconds, now); err != nil {
				return err
			}
			if job.GuardSeconds+seconds >= settings.JobGuardMinutes*60 {
				if err := finishJob(ctx, tx, playerID, job, now); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("E1900 job progress failed", "player", playerID, "err", err)
	}
}

// expireJobs ends jobs past their deadline: unfinished takes fail, and the poster gets back the
// reward for every slot not paid. The tick calls it.
func (s *Service) expireJobs(ctx context.Context) {
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		due, err := store.DueJobs(ctx, tx, s.now(), 100)
		if err != nil {
			return err
		}
		for _, job := range due {
			refund := game.RefundOf(job.Reward, job.Slots, job.Done)
			if job.PosterID != 0 {
				for _, resource := range game.SortedResources(refund) {
					if err := store.AddUploaded(ctx, tx, job.PosterID, resource, refund[resource]); err != nil {
						return err
					}
				}
				if err := notify(ctx, tx, store.NewNotice{PlayerID: job.PosterID, Kind: "job_ended", Body: game.NoticeJobEnded(job.Kind)}); err != nil {
					return err
				}
			}
			if err := store.EndJob(ctx, tx, job.ID, refund); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("E1900 job expiry failed", "err", err)
	}
}
