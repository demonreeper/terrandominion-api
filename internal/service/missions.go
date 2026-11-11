package service

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Orders and daily missions (docs/regional-command-plan.md). Each mission comes from one faction's
// lore power. Nothing here tells players the six powers are one.

// MissionsView is GET /v1/missions: today's missions and when the next ones come.
type MissionsView struct {
	Day         string                `json:"day"`
	RefreshesAt time.Time             `json:"refreshesAt"`
	Missions    []store.PlayerMission `json:"missions"`
	// Invitation is retired and always null: the level 20 invitation is plain game text on the
	// FACTIONS screen now (GET /v1/factions, "invited"), not a power speaking in ORDERS. The field
	// stays so app builds that still read it show nothing.
	Invitation *struct{} `json:"invitation"`
}

// Missions gives the player today's missions if they have none yet, and lists them.
func (s *Service) Missions(ctx context.Context, playerID int64) (MissionsView, error) {
	settings := s.Settings(ctx)
	day, next := game.MissionDay(settings, s.now())
	out := MissionsView{Day: day.Format("2006-01-02"), RefreshesAt: next}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		if err := ensureMissions(ctx, tx, playerID, day, settings); err != nil {
			return err
		}
		missions, err := store.MissionsForDay(ctx, tx, playerID, day)
		if err != nil {
			return err
		}
		for i := range missions {
			missions[i].Title = game.MissionTitle(missions[i].Goal, missions[i].GoalKey, missions[i].Count)
		}
		out.Missions = missions
		return nil
	})
	return out, err
}

// ensureMissions gives the day's missions once: freelancers one from each of MissionsPerDay
// different lore powers, members from their own faction, and Regional Commanders and Wardens one
// command order among theirs. The player must be locked.
func ensureMissions(ctx context.Context, q db.Querier, playerID int64, day time.Time, settings game.Settings) error {
	existing, err := store.MissionsForDay(ctx, q, playerID, day)
	if err != nil || len(existing) > 0 {
		return err
	}
	player, err := store.GetPlayer(ctx, q, playerID)
	if err != nil {
		return err
	}
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return err
	}
	n := max(0, settings.MissionsPerDay)
	give := func(audience string, factionID, count int, oneEach bool) error {
		rows, err := store.MissionCandidates(ctx, q, audience, factionID, player.Level)
		if err != nil {
			return err
		}
		byID := make(map[string]store.MissionTypeRow, len(rows))
		cands := make([]game.MissionCandidate, 0, len(rows))
		for _, r := range rows {
			byID[r.ID] = r
			cands = append(cands, game.MissionCandidate{ID: r.ID, FactionID: r.FactionID, Weight: r.Weight})
		}
		for _, picked := range game.PickMissions(cands, count, oneEach, rand.IntN) {
			reward := game.MissionRewardFor(settings, player.Level, audience == game.AudienceCommand, rand.Float64())
			if err := store.GiveMission(ctx, q, playerID, day, audience, byID[picked.ID], store.MissionReward{
				XP: reward.XP, Credits: reward.Credits, Item: reward.Item, ItemCount: reward.ItemCount,
			}); err != nil {
				return err
			}
		}
		return nil
	}

	if membership == nil {
		// Past the faction level a freelancer may hear from the seventh voice now and then.
		if err := maybeLangisRare(ctx, q, playerID, player.Level, true, day, settings, time.Now()); err != nil {
			return err
		}
		// A title whose level was reached before titles existed (or before its call) is given now.
		if err := grantTitles(ctx, q, playerID, player.Level, settings, time.Now()); err != nil {
			return err
		}
		return give(game.AudienceFreelancer, 0, n, true)
	}
	// A member hears only from their own power: MemberMissionsPerDay, and a command order for a
	// Warden or above (freelancer-path-plan.md F3).
	command, own := game.MissionCounts(settings, true, membership.Rank)
	if command > 0 {
		if err := give(game.AudienceCommand, membership.FactionID, command, false); err != nil {
			return err
		}
	}
	return give(game.AudienceMember, membership.FactionID, own, false)
}

// advanceMissions counts an action toward the player's missions for today, giving them first if
// the player hasn't opened ORDERS yet, and sends a notice for each one it finishes. Call it inside
// the action's transaction, with the player locked.
func advanceMissions(ctx context.Context, tx *sql.Tx, playerID int64, goal, key string, amount int,
	settings game.Settings, now time.Time) error {
	day, _ := game.MissionDay(settings, now)
	if err := ensureMissions(ctx, tx, playerID, day, settings); err != nil {
		return err
	}
	// Every action that can advance a mission also says something about how the player plays.
	if err := scorePlaystyle(ctx, tx, playerID, goal, key); err != nil {
		return err
	}
	finished, err := store.AdvanceMissions(ctx, tx, playerID, day, goal, key, amount)
	if err != nil {
		return err
	}
	for _, m := range finished {
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: playerID, Kind: "mission_done",
			Body: game.NoticeMissionDone(m.Persona, game.MissionTitle(m.Goal, m.GoalKey, m.Count)),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ClaimOutcome is what a claimed mission paid.
type ClaimOutcome struct {
	Reward   store.MissionReward `json:"reward"`
	XP       int                 `json:"xp"`
	Progress game.Progress       `json:"progress"`
	LevelUp  bool                `json:"levelUp"`
	// PerkTag is set when a faction perk raised the credit ("+25% ISHĀRA"); Reward.Credits is
	// what was paid, perk included.
	PerkTag string `json:"perkTag,omitempty"`
}

// ClaimMission pays a finished mission: XP, credits into storage, and its item if it has one.
func (s *Service) ClaimMission(ctx context.Context, playerID, missionID int64) (ClaimOutcome, error) {
	settings := s.Settings(ctx)
	var out ClaimOutcome
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		mission, err := store.MissionForUpdate(ctx, tx, playerID, missionID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.MissionGone
		}
		if err != nil {
			return err
		}
		switch mission.Status {
		case "claimed":
			return apperr.MissionGone
		case "active":
			return apperr.MissionNotDone.WithData(map[string]any{"progress": mission.Progress, "count": mission.Count})
		}
		// A faction perk may pay more credit, for as long as the player is a member.
		perkTag := ""
		if mission.Reward.Credits > 0 {
			perks, lines, err := s.perksOf(ctx, tx, playerID, settings)
			if err != nil {
				return err
			}
			if boosted := int(game.PerkAmount(int64(mission.Reward.Credits), perks.MissionCreditMult)); boosted > mission.Reward.Credits {
				mission.Reward.Credits = boosted
				perkTag = game.PerkTagIn(lines, game.PerkMissionCredit)
			}
			if err := store.AddUploaded(ctx, tx, playerID, "credit", int64(mission.Reward.Credits)); err != nil {
				return err
			}
		}
		if mission.Reward.Item != "" && mission.Reward.ItemCount > 0 {
			if err := store.AddItem(ctx, tx, playerID, mission.Reward.Item, mission.Reward.ItemCount); err != nil {
				return err
			}
		}
		if err := store.ClaimMission(ctx, tx, missionID); err != nil {
			return err
		}
		progress, levelUp, err := awardXP(ctx, tx, playerID, "mission", mission.Reward.XP, missionID, settings)
		if err != nil {
			return err
		}
		out = ClaimOutcome{Reward: mission.Reward, XP: mission.Reward.XP, Progress: progress, LevelUp: levelUp, PerkTag: perkTag}
		return nil
	})
	return out, err
}

// maybeInvite runs when a player's level rises: a freelancer crossing LeadFactionLevel is invited by
// the one power that speaks to them, the same one that made the level 19 offer. The invitation is
// plain game text (a notice, and the FACTIONS screen); no power speaks here. Whatever of the offer
// has not come due is dropped.
func maybeInvite(ctx context.Context, q db.Querier, playerID int64, before, after int, settings game.Settings,
	now time.Time) error {
	if before >= settings.LeadFactionLevel || after < settings.LeadFactionLevel {
		return nil
	}
	if err := store.CancelOffer(ctx, q, playerID, now); err != nil {
		return err
	}
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil || membership != nil {
		return err
	}
	factionID, err := offerPower(ctx, q, playerID)
	if err != nil {
		return err
	}
	invited, err := store.Invite(ctx, q, playerID, factionID)
	if err != nil || !invited {
		return err
	}
	return store.InsertNotice(ctx, q, store.NewNotice{
		PlayerID: playerID, Kind: "invitation", Body: game.NoticeInvited(),
		PushState: game.NoticePushState("invitation"),
	})
}

// BranchView is GET /v1/factions/branch: the members of the player's branch.
type BranchView struct {
	FactionCode string               `json:"factionCode"`
	FactionName string               `json:"factionName"`
	Region      string               `json:"region"`
	Members     []store.BranchMember `json:"members"`
	// CanAppoint is true for the branch's Regional Commander.
	CanAppoint bool `json:"canAppoint"`
}

func (s *Service) Branch(ctx context.Context, playerID int64) (BranchView, error) {
	membership, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return BranchView{}, err
	}
	if membership == nil || membership.BranchID == nil {
		return BranchView{}, apperr.NotInBranch
	}
	members, err := store.BranchMembers(ctx, s.db, *membership.BranchID)
	if err != nil {
		return BranchView{}, err
	}
	ids := make([]int64, 0, len(members))
	for i := range members {
		members[i].RankTitle = game.RankTitle(members[i].Rank)
		ids = append(ids, members[i].PlayerID)
	}
	states, err := store.FriendStates(ctx, s.db, playerID, ids)
	if err != nil {
		return BranchView{}, err
	}
	for i := range members {
		members[i].FriendState = states[members[i].PlayerID]
	}
	return BranchView{
		FactionCode: membership.FactionCode, FactionName: membership.FactionName, Region: membership.Region,
		Members: members, CanAppoint: membership.Rank >= game.RankRegionalCommander,
	}, nil
}

// AppointWarden lets a Regional Commander make a member of their branch a Warden. Both players are
// locked, lowest id first.
func (s *Service) AppointWarden(ctx context.Context, commanderID, targetID int64) error {
	if commanderID == targetID {
		return apperr.BadRequest.With("you can't appoint yourself")
	}
	settings := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		first, second := min(commanderID, targetID), max(commanderID, targetID)
		if _, err := store.GetPlayerForUpdate(ctx, tx, first); err != nil {
			return err
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, second); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return apperr.NotInBranch
			}
			return err
		}
		commander, err := store.MembershipOf(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		if commander == nil || commander.Rank < game.RankRegionalCommander || commander.BranchID == nil {
			return apperr.NotRegionalCommander
		}
		target, err := store.MembershipOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if target == nil || target.BranchID == nil || *target.BranchID != *commander.BranchID {
			return apperr.NotInBranch
		}
		if target.Rank >= game.RankWarden {
			return apperr.AlreadyWarden
		}
		if err := store.SetMemberRank(ctx, tx, targetID, game.RankWarden); err != nil {
			return err
		}
		names, err := store.GetPlayer(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		if err := notify(ctx, tx, store.NewNotice{
			PlayerID: targetID, ActorID: commanderID, Kind: "warden",
			Body: game.NoticeWarden(names.Name, commander.FactionName),
		}); err != nil {
			return err
		}
		return advanceMissions(ctx, tx, commanderID, game.GoalAppointWarden, "", 1, settings, s.now())
	})
}
