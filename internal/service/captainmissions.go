package service

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Captain squad missions (faction-ranks-plan.md, Captain and Squads; base-building-plan.md 10). A Captain (the
// base_missions power) posts one of the base mission kinds to their squad, naming one of the faction's standing bases
// in their region. Each squad member gets it at once as an extra mission for the day, in their own power's words, and
// it counts only at that base, on site, like every base mission. The post holds nothing open: the missions belong to
// the members (a member who leaves the squad keeps theirs, one who joins later doesn't get it), so a Captain who loses
// the rank leaves nothing behind.

// missionTitle says what a mission asks; a Captain's names its base.
func missionTitle(m store.PlayerMission) string {
	if m.FromCaptain {
		return game.CaptainMissionTitle(m.Goal, m.GoalKey, m.Count, m.BaseName)
	}
	return game.MissionTitle(m.Goal, m.GoalKey, m.Count)
}

// SquadMissions is the POST MISSION part of GET /v1/factions/squad, for a Captain: the day's limit and what is left,
// the bounds of an amount, the bases they may name, and today's posts.
type SquadMissions struct {
	PerDay    int                 `json:"perDay"`
	Left      int                 `json:"left"`
	GiveMax   int                 `json:"giveMax"`
	StandMax  int                 `json:"standMax"`
	Resources []string            `json:"resources"`
	Bases     []store.SquadBase   `json:"bases"`
	Today     []store.CaptainPost `json:"today"`
}

// squadMissions fills SquadMissions for a Captain.
func (s *Service) squadMissions(ctx context.Context, q db.Querier, captainID int64, m *store.Membership,
	st game.Settings) (*SquadMissions, error) {
	day, _ := game.MissionDay(st, s.now())
	bases, err := store.StandingBases(ctx, q, m.FactionID, m.Region)
	if err != nil {
		return nil, err
	}
	today, err := store.CaptainPostsOn(ctx, q, captainID, day)
	if err != nil {
		return nil, err
	}
	for i := range today {
		today[i].Title = game.CaptainMissionTitle(today[i].Goal, today[i].GoalKey, today[i].Count, today[i].BaseName)
	}
	return &SquadMissions{
		PerDay: st.CaptainMissionsPerDay, Left: game.CaptainMissionsLeft(len(today), st),
		GiveMax: st.CaptainMissionGiveMax, StandMax: st.CaptainMissionStandMaxMin,
		Resources: game.CaptainMissionResources, Bases: bases, Today: today,
	}, nil
}

// CaptainMissionRequest is POST /v1/factions/squad/missions.
type CaptainMissionRequest struct {
	BaseID int64 `json:"baseId"`
	// Goal: base_give, base_stand or base_repair.
	Goal     string `json:"goal"`
	Resource string `json:"resource"`
	Amount   int    `json:"amount"`
}

// CaptainMissionResult is what a post did.
type CaptainMissionResult struct {
	Post    store.CaptainPost `json:"post"`
	Members int               `json:"members"`
	Left    int               `json:"left"`
}

// PostCaptainMission: a Captain posts a base mission to their squad.
func (s *Service) PostCaptainMission(ctx context.Context, captainID int64, req CaptainMissionRequest) (CaptainMissionResult, error) {
	var out CaptainMissionResult
	st := s.Settings(ctx)
	now := s.now()
	day, _ := game.MissionDay(st, now)
	post := game.CaptainMissionPost{Goal: req.Goal, Key: strings.ToLower(strings.TrimSpace(req.Resource)), Amount: req.Amount}
	if post.Goal != game.GoalBaseGive {
		post.Key = ""
	}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		captain, err := store.GetPlayerForUpdate(ctx, tx, captainID)
		if err != nil {
			return err
		}
		m, err := memberFor(ctx, tx, captainID)
		if err != nil {
			return err
		}
		if m.BranchID == nil || !game.Can(m.Rank, m.Nomad, game.PowerBaseMissions) {
			return apperr.CaptainMissionRefused.Say("Only a Captain or above posts squad missions.")
		}
		posted, err := store.CaptainPostsOn(ctx, tx, captainID, day)
		if err != nil {
			return err
		}
		if game.CaptainMissionsLeft(len(posted), st) <= 0 {
			return apperr.CaptainMissionLimit.WithData(map[string]any{"perDay": st.CaptainMissionsPerDay})
		}
		b, err := store.BaseRefForUpdate(ctx, tx, req.BaseID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.BaseGone
		}
		if err != nil {
			return err
		}
		if b.FactionID != m.FactionID || b.Region != m.Region || b.Status != "active" {
			return apperr.CaptainMissionRefused.Say("Pick one of your faction's standing bases in your region.")
		}
		damaged := false
		if post.Goal == game.GoalBaseRepair {
			bases, err := store.StandingBases(ctx, tx, m.FactionID, m.Region)
			if err != nil {
				return err
			}
			for _, sb := range bases {
				damaged = damaged || (sb.ID == b.ID && sb.Damaged)
			}
		}
		switch game.CheckCaptainMission(post, damaged, st) {
		case game.CaptainMissionBadKind:
			return apperr.CaptainMissionRefused.Say("Pick give, stand or repair.")
		case game.CaptainMissionBadResource:
			return apperr.CaptainMissionRefused.Say("Pick alloy, crystal, data or credit.")
		case game.CaptainMissionBadAmount:
			return apperr.CaptainMissionRefused.Say("That amount is outside what a squad mission may ask.").
				WithData(map[string]any{"giveMax": st.CaptainMissionGiveMax, "standMax": st.CaptainMissionStandMaxMin})
		case game.CaptainMissionNothingToFix:
			return apperr.CaptainMissionRefused.Say("Nothing at that base needs repair.")
		}
		members, err := store.SquadMembersOf(ctx, tx, captainID)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			return apperr.CaptainMissionRefused.Say("Your squad is empty. Add Members and Prospects first.")
		}
		id, err := store.InsertCaptainPost(ctx, tx, captainID, m.FactionID, m.Region, b.ID, day, post.Goal, post.Key, post.Amount)
		if err != nil {
			return err
		}
		title := game.CaptainMissionTitle(post.Goal, post.Key, post.Amount, b.Name)
		given := 0
		for _, member := range members {
			t, err := store.CaptainMissionType(ctx, tx, member.FactionID, post.Goal)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			// What a base mission pays this member, no more.
			reward := game.MissionRewardFor(st, member.Level, false, rand.Float64())
			if err := store.GiveCaptainMission(ctx, tx, member.PlayerID, day, t, id, b.ID, post.Goal, post.Key, post.Amount,
				store.MissionReward{XP: reward.XP, Credits: reward.Credits, Item: reward.Item, ItemCount: reward.ItemCount}); err != nil {
				return err
			}
			if err := notify(ctx, tx, store.NewNotice{PlayerID: member.PlayerID, ActorID: captainID, Kind: "captain_mission",
				Body: game.NoticeCaptainMission(captain.Ref().Name, title)}); err != nil {
				return err
			}
			given++
		}
		if err := store.SetCaptainPostMembers(ctx, tx, id, given); err != nil {
			return err
		}
		out = CaptainMissionResult{
			Post: store.CaptainPost{ID: id, BaseID: b.ID, BaseName: b.Name, Goal: post.Goal, GoalKey: post.Key,
				Count: post.Amount, Members: given, At: now, Title: title},
			Members: given,
			Left:    game.CaptainMissionsLeft(len(posted)+1, st),
		}
		return nil
	})
	return out, err
}
