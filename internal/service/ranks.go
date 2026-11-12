package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction ranks v2 and Nomads, R1 (docs/faction-ranks-plan.md): appointing Captains, Wardens and leads (slots per
// region), Captain squads, Nomads (become, ask to move, answer, settle), the tick's promotions (Prospect to Member,
// and up the ladder by contribution points when a slot is free) and the admin tools.

// rankBelowOf is the level of the next rung up (the exclusive top of a rank's band).
func rankBandTop(level int) int {
	top := game.RankHead + 1
	for _, r := range game.Ranks {
		if r.Level > level && r.Level < top {
			top = r.Level
		}
	}
	return top
}

// setRank changes a member's rank and keeps squads straight: a member promoted above Member leaves any squad, and
// one who can no longer lead a squad loses theirs.
func setRank(ctx context.Context, tx *sql.Tx, playerID int64, level int, nomad bool) error {
	if err := store.SetMemberRank(ctx, tx, playerID, level); err != nil {
		return err
	}
	if level > game.RankMember {
		if err := store.LeaveSquad(ctx, tx, playerID); err != nil {
			return err
		}
	}
	if !game.Can(level, nomad, game.PowerSquadLead) {
		return store.DissolveSquad(ctx, tx, playerID)
	}
	return nil
}

// slotFree says whether the branch has room for one more of this rank.
func slotFree(ctx context.Context, q db.Querier, branchID int64, level int, st game.Settings) (bool, error) {
	switch level {
	case game.RankCaptain, game.RankWarden, game.RankLead:
		n, err := store.CountRankInBranch(ctx, q, branchID, level, rankBandTop(level))
		return n < game.RankSlots(level, st), err
	}
	return true, nil
}

// AppointRank: a member with the appoint power makes a member of their branch a Captain, Warden or lead (rankKey).
func (s *Service) AppointRank(ctx context.Context, appointerID, targetID int64, rankKey string) error {
	if appointerID == targetID {
		return apperr.BadRequest.With("you can't appoint yourself")
	}
	to, ok := game.RankByKey(rankKey)
	if !ok || (to.Level != game.RankCaptain && to.Level != game.RankWarden && to.Level != game.RankLead) {
		return apperr.BadRequest.With("rank")
	}
	st := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		first, second := min(appointerID, targetID), max(appointerID, targetID)
		if _, err := store.GetPlayerForUpdate(ctx, tx, first); err != nil {
			return err
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, second); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return apperr.NotInBranch
			}
			return err
		}
		appointer, err := store.MembershipOf(ctx, tx, appointerID)
		if err != nil {
			return err
		}
		target, err := store.MembershipOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if appointer == nil || appointer.BranchID == nil {
			return apperr.NotInBranch
		}
		if target == nil || target.BranchID == nil || *target.BranchID != *appointer.BranchID {
			return apperr.NotInBranch
		}
		if target.Nomad {
			return apperr.RankNomad
		}
		if target.Rank >= to.Level {
			return apperr.AlreadyWarden.Say("They already hold that rank or a higher one.")
		}
		if !game.CanAppoint(appointer.Rank, appointer.Nomad, target.Rank, to.Level) {
			return apperr.RankCantAppoint
		}
		free, err := slotFree(ctx, tx, *appointer.BranchID, to.Level, st)
		if err != nil {
			return err
		}
		if !free {
			return apperr.RankSlotsFull
		}
		if err := setRank(ctx, tx, targetID, to.Level, false); err != nil {
			return err
		}
		names, err := store.GetPlayer(ctx, tx, appointerID)
		if err != nil {
			return err
		}
		kind, body := "rank_up", game.NoticeRankUp(names.Name, to.Title, appointer.FactionName)
		if to.Level == game.RankWarden {
			kind, body = "warden", game.NoticeWarden(names.Name, appointer.FactionName)
		}
		if err := notify(ctx, tx, store.NewNotice{PlayerID: targetID, ActorID: appointerID, Kind: kind, Body: body}); err != nil {
			return err
		}
		if to.Level == game.RankWarden {
			return advanceMissions(ctx, tx, appointerID, game.GoalAppointWarden, "", 1, st, s.now())
		}
		return nil
	})
}

// AppointWarden is the older route's way in (POST /v1/factions/branch/wardens).
func (s *Service) AppointWarden(ctx context.Context, appointerID, targetID int64) error {
	return s.AppointRank(ctx, appointerID, targetID, "warden")
}

// ---- squads ------------------------------------------------------------------------------------

// SquadView is GET /v1/factions/squad: a Captain's squad, and who in the branch they can still add.
type SquadView struct {
	Captain  bool                 `json:"captain"`
	Size     int                  `json:"size"`
	Max      int                  `json:"max"`
	Members  []store.BranchMember `json:"members"`
	Eligible []store.BranchMember `json:"eligible"`
	// For a member in someone's squad: their Captain.
	MyCaptain *store.PlayerRef `json:"myCaptain,omitempty"`
}

func (s *Service) Squad(ctx context.Context, playerID int64) (SquadView, error) {
	out := SquadView{Members: []store.BranchMember{}, Eligible: []store.BranchMember{}}
	st := s.Settings(ctx)
	m, err := memberFor(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	if m.BranchID == nil {
		return out, apperr.NotInBranch
	}
	out.Captain = game.Can(m.Rank, m.Nomad, game.PowerSquadLead)
	out.Max = st.CaptainSquadSize
	members, err := store.BranchMembers(ctx, s.db, *m.BranchID)
	if err != nil {
		return out, err
	}
	for _, b := range members {
		b.RankTitle = game.RankTitle(b.Rank)
		if b.Nomad {
			b.RankTitle = "Nomad"
		}
		b.RankKey = game.RankAt(b.Rank).Key
		switch {
		case b.PlayerID == playerID:
			continue
		case b.CaptainID != nil && *b.CaptainID == playerID:
			b.MySquad = true
			out.Members = append(out.Members, b)
		case out.Captain && b.CaptainID == nil && b.Rank <= game.RankMember:
			out.Eligible = append(out.Eligible, b)
		}
	}
	out.Size = len(out.Members)
	captain, err := store.SquadCaptainOf(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	if captain != nil {
		p, err := store.GetPlayer(ctx, s.db, *captain)
		if err != nil {
			return out, err
		}
		ref := p.Ref()
		out.MyCaptain = &ref
	}
	return out, nil
}

// SquadAdd: a Captain adds a Member or Prospect of their region to their squad (in one squad at most).
func (s *Service) SquadAdd(ctx context.Context, captainID, targetID int64) error {
	st := s.Settings(ctx)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, captainID); err != nil {
			return err
		}
		c, err := memberFor(ctx, tx, captainID)
		if err != nil {
			return err
		}
		if !game.Can(c.Rank, c.Nomad, game.PowerSquadLead) || c.BranchID == nil {
			return apperr.RankCantAppoint.Say("Only a Captain or above leads a squad.")
		}
		t, err := store.MembershipOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if t == nil || t.BranchID == nil || *t.BranchID != *c.BranchID {
			return apperr.NotInBranch
		}
		if t.Rank > game.RankMember {
			return apperr.SquadRefused.Say("A squad is for Members and Prospects.")
		}
		if in, err := store.SquadCaptainOf(ctx, tx, targetID); err != nil {
			return err
		} else if in != nil {
			return apperr.SquadRefused.Say("They are already in a squad.")
		}
		size, err := store.SquadSize(ctx, tx, captainID)
		if err != nil {
			return err
		}
		if size >= st.CaptainSquadSize {
			return apperr.SquadRefused.Say("Your squad is full.")
		}
		return store.AddSquadMember(ctx, tx, captainID, targetID)
	})
}

// SquadRemove: a Captain takes someone out of their squad, or a member leaves theirs (targetID = themselves).
func (s *Service) SquadRemove(ctx context.Context, playerID, targetID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		captain, err := store.SquadCaptainOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if captain == nil {
			return nil
		}
		if targetID != playerID && *captain != playerID {
			return apperr.SquadRefused.Say("That player isn't in your squad.")
		}
		return store.LeaveSquad(ctx, tx, targetID)
	})
}

// ---- Nomads ------------------------------------------------------------------------------------

// BecomeNomad: a Member or higher gives up any rank above Member (and their squad, and a seat) and can then move
// home regions. Becoming one again after settling restarts the move cooldown.
func (s *Service) BecomeNomad(ctx context.Context, playerID int64) error {
	now := s.now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if m.Nomad {
			return apperr.NomadRefused.Say("You are a Nomad already.")
		}
		if !game.Can(m.Rank, false, game.PowerNomad) {
			return apperr.NomadRefused.Say("Prospects can't become Nomads yet.")
		}
		was, err := store.WasNomad(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var from *time.Time
		if was {
			from = &now
		}
		if err := store.ClearSeat(ctx, tx, playerID); err != nil {
			return err
		}
		if err := store.SetNomad(ctx, tx, playerID, true, game.RankMember, from); err != nil {
			return err
		}
		if err := store.DissolveSquad(ctx, tx, playerID); err != nil {
			return err
		}
		slog.Info("nomad", "player", playerID, "gave up", m.Rank)
		return nil
	})
}

// Settle: a Nomad becomes an ordinary Member of their current home region again.
func (s *Service) Settle(ctx context.Context, playerID int64) error {
	now := s.now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if !m.Nomad {
			return apperr.NomadRefused.Say("You aren't a Nomad.")
		}
		if err := store.CancelOpenAsks(ctx, tx, playerID, now); err != nil {
			return err
		}
		return store.SetNomad(ctx, tx, playerID, false, game.RankMember, nil)
	})
}

// AskMove: a Nomad asks to move home to another unlocked region of their faction. The receiving region's Commander
// and Wardens are told.
func (s *Service) AskMove(ctx context.Context, playerID int64, region string) (store.NomadAsk, error) {
	var out store.NomadAsk
	region = strings.ToUpper(strings.TrimSpace(region))
	st := s.Settings(ctx)
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if !m.Nomad {
			return apperr.NomadRefused.Say("Only Nomads can move. Choose NOMAD first.")
		}
		if region == "" || region == m.Region {
			return apperr.NomadRefused.Say("Pick another region.")
		}
		if m.CooldownFrom != nil {
			if at := game.NomadMoveAt(*m.CooldownFrom, st); now.Before(at) {
				return apperr.NomadCooldown.WithData(map[string]any{"until": at.UTC().Format(time.RFC3339)})
			}
		}
		branch, err := store.BranchInRegion(ctx, tx, m.FactionID, region)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !branch.Unlocked) {
			return apperr.RegionLocked
		}
		if err != nil {
			return err
		}
		if open, err := store.OpenAskOf(ctx, tx, playerID, now); err != nil {
			return err
		} else if open != nil {
			return apperr.NomadRefused.Say("You already asked. Wait for an answer or cancel it.")
		}
		id, err := store.InsertNomadAsk(ctx, tx, playerID, m.FactionID, m.Region, region, game.NomadAskExpiry(now, st))
		if err != nil {
			return err
		}
		if out, err = store.NomadAskForUpdate(ctx, tx, id); err != nil {
			return err
		}
		approvers, err := store.BranchApprovers(ctx, tx, branch.ID, game.RankWarden)
		if err != nil {
			return err
		}
		for _, a := range approvers {
			if err := notify(ctx, tx, store.NewNotice{PlayerID: a, ActorID: playerID, Kind: "nomad_ask",
				Body: game.NoticeNomadAsk(out.Player.Name)}); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// CancelMove withdraws the caller's open ask.
func (s *Service) CancelMove(ctx context.Context, playerID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		return store.CancelOpenAsks(ctx, tx, playerID, s.now())
	})
}

// NomadView is GET /v1/factions/nomads: the caller's own ask, and the asks to move into their region they may answer.
type NomadView struct {
	Mine      *store.NomadAsk  `json:"mine,omitempty"`
	CanAnswer bool             `json:"canAnswer"`
	Asks      []store.NomadAsk `json:"asks"`
}

func (s *Service) Nomads(ctx context.Context, playerID int64) (NomadView, error) {
	out := NomadView{Asks: []store.NomadAsk{}}
	now := s.now()
	m, err := memberFor(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	if out.Mine, err = store.OpenAskOf(ctx, s.db, playerID, now); err != nil {
		return out, err
	}
	out.CanAnswer = game.Can(m.Rank, m.Nomad, game.PowerApproveNomads)
	if out.CanAnswer {
		if out.Asks, err = store.AsksTo(ctx, s.db, m.FactionID, m.Region, now); err != nil {
			return out, err
		}
	}
	return out, nil
}

// AnswerMove: the receiving region's Commander or a Warden there (or the Global Head) says yes or no. On yes the
// Nomad's home region moves: Member there (still a Nomad), points, level and inventory kept, TNs stay put.
func (s *Service) AnswerMove(ctx context.Context, playerID, askID int64, yes bool) error {
	now := s.now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		ask, err := store.NomadAskForUpdate(ctx, tx, askID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NomadAskGone
		}
		if err != nil {
			return err
		}
		if ask.Status != "open" || !ask.ExpiresAt.After(now) {
			return apperr.NomadAskGone
		}
		head := game.ActsAsGlobalHead(m.Rank, false)
		if m.FactionID != ask.FactionID || (!head && (m.Region != ask.ToRegion || !game.Can(m.Rank, m.Nomad, game.PowerApproveNomads))) {
			return apperr.RankCantAppoint.Say("Only that region's Commander or a Warden there can answer.")
		}
		if !yes {
			if err := store.DecideAsk(ctx, tx, askID, "declined", playerID, now); err != nil {
				return err
			}
			return notify(ctx, tx, store.NewNotice{PlayerID: ask.PlayerID, ActorID: playerID, Kind: "nomad_declined",
				Body: game.NoticeNomadDeclined()})
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, ask.PlayerID); err != nil {
			return err
		}
		target, err := store.MembershipOf(ctx, tx, ask.PlayerID)
		if err != nil {
			return err
		}
		if target == nil || !target.Nomad || target.FactionID != ask.FactionID {
			_ = store.DecideAsk(ctx, tx, askID, "cancelled", playerID, now)
			return apperr.NomadAskGone
		}
		branch, err := store.BranchInRegion(ctx, tx, ask.FactionID, ask.ToRegion)
		if err != nil {
			return err
		}
		if err := store.LeaveSquad(ctx, tx, ask.PlayerID); err != nil {
			return err
		}
		if err := store.MoveNomadHome(ctx, tx, ask.PlayerID, branch.ID, game.RankMember, now); err != nil {
			return err
		}
		if err := store.DecideAsk(ctx, tx, askID, "approved", playerID, now); err != nil {
			return err
		}
		return notify(ctx, tx, store.NewNotice{PlayerID: ask.PlayerID, ActorID: playerID, Kind: "nomad_moved",
			Body: game.NoticeNomadMoved()})
	})
}

// ---- the tick: asks expire, Prospects become Members, points promote -------------------------------

var lastRankTick time.Time

func (s *Service) rankTick(ctx context.Context) {
	if s.now().Sub(lastRankTick) < time.Minute {
		return
	}
	lastRankTick = s.now()
	st := s.Settings(ctx)
	now := s.now()
	due, err := store.DueAsks(ctx, s.db, now, 50)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 rank tick failed", "err", err)
		}
		return
	}
	for _, a := range due {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if err := store.DecideAsk(ctx, tx, a.ID, "expired", 0, now); err != nil {
				return err
			}
			return notify(ctx, tx, store.NewNotice{PlayerID: a.PlayerID, Kind: "nomad_expired", Body: game.NoticeNomadExpired()})
		})
		if err != nil {
			slog.Error("E1900 nomad ask expiry failed", "ask", a.ID, "err", err)
		}
	}

	minPoints := st.RankPointsCaptain
	for _, p := range []int{st.RankPointsWarden, st.RankPointsLead} {
		if p > 0 && (minPoints <= 0 || p < minPoints) {
			minPoints = p
		}
	}
	candidates, err := store.RankCandidates(ctx, s.db, game.RankProspect, game.RankLead, minPoints)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 rank candidates failed", "err", err)
		}
		return
	}
	for _, c := range candidates {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if c.Rank <= game.RankProspect {
				if !game.ProspectDone(c.JoinedAt, c.Points, now, st) {
					return nil
				}
				return setRank(ctx, tx, c.PlayerID, game.RankMember, c.Nomad)
			}
			// Most points first (the query's order): the queue for a full rank goes by points.
			to := game.NextPromotion(c.Rank, c.Nomad, c.Points, st)
			if to == 0 {
				return nil
			}
			free, err := slotFree(ctx, tx, c.BranchID, to, st)
			if err != nil || !free {
				return err
			}
			if err := setRank(ctx, tx, c.PlayerID, to, false); err != nil {
				return err
			}
			def := game.RankAt(to)
			return notify(ctx, tx, store.NewNotice{PlayerID: c.PlayerID, Kind: "rank_up",
				Body: game.NoticeRankEarned(def.Title)})
		})
		if err != nil {
			slog.Error("E1900 promotion failed", "player", c.PlayerID, "err", err)
		}
	}
}

// ---- admin -------------------------------------------------------------------------------------

// AdminSetRank sets a member's rank directly (Prospect to Lead; the Commander's seat is claimed, and the two top
// seats are elected later).
func (s *Service) AdminSetRank(ctx context.Context, adminID int64, code, rankKey string) error {
	to, ok := game.RankByKey(rankKey)
	if !ok || to.Level > game.RankLead {
		return apperr.BadRequest.With("rank: prospect, member, captain, warden or lead")
	}
	targetID, err := s.PlayerIDByCode(ctx, code)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := store.MembershipOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if m == nil {
			return apperr.NotAMember
		}
		if m.Rank >= game.RankRegionalCommander {
			if err := store.ClearSeat(ctx, tx, targetID); err != nil {
				return err
			}
		}
		nomad := m.Nomad && to.Level <= game.RankMember
		if m.Nomad && !nomad {
			if err := store.SetNomad(ctx, tx, targetID, false, to.Level, nil); err != nil {
				return err
			}
		}
		if err := setRank(ctx, tx, targetID, to.Level, nomad); err != nil {
			return err
		}
		return store.LogAdminAction(ctx, tx, adminID, "rank.set", code, fmt.Sprintf(`{"from":%d,"to":%q}`, m.Rank, to.Key))
	})
}

// AdminClearNomadCooldown lets a Nomad move again at once.
func (s *Service) AdminClearNomadCooldown(ctx context.Context, adminID int64, code string) error {
	targetID, err := s.PlayerIDByCode(ctx, code)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.ClearNomadCooldown(ctx, tx, targetID); err != nil {
			return err
		}
		return store.LogAdminAction(ctx, tx, adminID, "nomad.cooldown.clear", code, "{}")
	})
}
