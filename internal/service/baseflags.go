package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction bases, B1 (base-building-plan.md 1b): a Warden or lead plants a flag on the spot, the regional Commander
// approves or declines from anywhere in the region, unanswered flags come down after BaseProposalHours, and an
// approved flag becomes the seed beacon: a Spire in status building with its first project.

// memberFor is the caller's membership, or NotAMember.
func memberFor(ctx context.Context, q db.Querier, playerID int64) (*store.Membership, error) {
	m, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperr.NotAMember
	}
	return m, nil
}

// BaseNames is GET /v1/bases/names: six names the caller's faction can still take (call again for six more).
func (s *Service) BaseNames(ctx context.Context, playerID int64) ([]string, error) {
	m, err := memberFor(ctx, s.db, playerID)
	if err != nil {
		return nil, err
	}
	taken, err := store.TakenBaseNames(ctx, s.db, s.now())
	if err != nil {
		return nil, err
	}
	return game.BaseNameOptions(m.FactionID, game.BaseNameOptionCount, rand.Intn, func(n string) bool { return taken[n] }), nil
}

// ProposeRequest is POST /v1/bases/propose.
type ProposeRequest struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AccuracyM float64 `json:"accuracyM"`
	Name      string  `json:"name"`
}

// ProposeResult is the planted flag, or (for the Commander, whose own proposal counts as approved) the base.
type ProposeResult struct {
	Flag *store.Flag    `json:"flag,omitempty"`
	Base *store.MapBase `json:"base,omitempty"`
}

// ProposeBase plants the caller's faction flag on the spot they stand on.
func (s *Service) ProposeBase(ctx context.Context, playerID int64, req ProposeRequest) (ProposeResult, error) {
	var out ProposeResult
	req.Name = strings.ToUpper(strings.TrimSpace(req.Name))
	if e := validSpot(req.Lat, req.Lon); e != nil {
		return out, e
	}
	here, err := s.regionAt(ctx, RegionFix{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM})
	if err != nil {
		return out, err
	}
	st := s.Settings(ctx)
	now := s.now()
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := store.MembershipOf(ctx, tx, playerID)
		if err != nil {
			return err
		}
		p := game.Proposer{InFaction: m != nil, HereRegion: here.Code, AccuracyM: req.AccuracyM, BasesEnabled: st.BasesEnabled == 1}
		if m != nil {
			p.Rank, p.HomeRegion, p.Nomad = m.Rank, m.Region, m.Nomad
			open, last, err := store.ProposerState(ctx, tx, playerID, now)
			if err != nil {
				return err
			}
			p.HasOpenProposal = open
			if last != nil {
				p.CooldownUntil = game.BaseCooldownUntil(*last, st)
			}
			if p.ProposalsOff, err = store.BranchProposalsOff(ctx, tx, m.FactionID, m.Region); err != nil {
				return err
			}
		}
		if e := game.CanProposeBase(p, now, st); e != nil {
			return e
		}
		if !game.IsBaseName(m.FactionID, req.Name) {
			return apperr.BadRequest.With("pick one of the offered names")
		}
		if err := store.LockBasePlacement(ctx, tx); err != nil {
			return err
		}
		taken, err := store.TakenBaseNames(ctx, tx, now)
		if err != nil {
			return err
		}
		if taken[req.Name] {
			return apperr.BaseNameTaken
		}
		sp, err := s.baseSpot(ctx, tx, req.Lat, req.Lon, m.FactionID, here.Code, 0, 0, st)
		if err != nil {
			return err
		}
		if e := game.CheckBaseSpot(sp, st); e != nil {
			return spotErr(e)
		}

		flagID, err := store.InsertFlag(ctx, tx, store.NewFlag{
			FactionID: m.FactionID, Region: here.Code, ProposerID: playerID, Name: req.Name,
			Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM, ExpiresAt: game.BaseProposalExpiry(now, st),
		})
		if err != nil {
			return err
		}
		flag, err := store.FlagForUpdate(ctx, tx, flagID)
		if err != nil {
			return err
		}

		// The Commander raises a base directly: their own proposal counts as approved.
		if game.BaseCanApprove(m.Rank) {
			base, err := s.raiseSpire(ctx, tx, flag, playerID, st, now)
			if err != nil {
				return err
			}
			out.Base = &base
			return nil
		}

		out.Flag = &flag
		commanders, err := store.RegionCommanders(ctx, tx, m.FactionID, here.Code, game.RankRegionalCommander)
		if err != nil {
			return err
		}
		for _, c := range commanders {
			if err := notify(ctx, tx, store.NewNotice{PlayerID: c, ActorID: playerID, Kind: "base_proposal",
				Body: game.NoticeBaseProposal(flag.Proposer.Name, flag.Name)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		slog.Info("base flag planted", "player", playerID, "name", req.Name, "raised", out.Base != nil)
	}
	return out, err
}

// raiseSpire turns an open flag into the seed beacon: a Spire (status building) with its zone, slots, vault and the
// Spire project, and marks the flag approved.
func (s *Service) raiseSpire(ctx context.Context, tx *sql.Tx, flag store.Flag, by int64, st game.Settings, now time.Time) (store.MapBase, error) {
	id, err := store.InsertBase(ctx, tx, store.NewBase{
		FactionID: flag.FactionID, Region: flag.Region, Name: flag.Name, Tier: 1, Lat: flag.Lat, Lon: flag.Lon,
		Status: "building", CoreHealth: game.BaseCoreHealth(1, st), ZoneM: game.BaseZoneM(1, st),
		ProposalID: flag.ID, RaisedBy: flag.ProposerID,
	})
	if err != nil {
		return store.MapBase{}, err
	}
	if _, err := store.SetBaseSlots(ctx, tx, id, game.BaseSlotLayout(1, st)); err != nil {
		return store.MapBase{}, err
	}
	if _, err := store.InsertProject(ctx, tx, store.NewProject{
		BaseID: id, Kind: "spire", ToTier: 1, Bill: game.BaseBill(1, st),
		AssemblySec: int(game.BaseAssembly(1, st) / time.Second), StartedBy: by,
	}); err != nil {
		return store.MapBase{}, err
	}
	if err := store.DecideFlag(ctx, tx, flag.ID, "approved", by, id, now); err != nil {
		return store.MapBase{}, err
	}
	base, err := store.MapBaseByID(ctx, tx, id)
	base.CoreMax = game.BaseCoreHealth(base.Tier, st)
	base.TierName = game.BaseTierName(base.Tier)
	return base, err
}

// answerable loads a flag the caller may answer: an open one of their own faction in their region, and they hold
// the Commander's seat (or act in it).
func (s *Service) answerable(ctx context.Context, tx *sql.Tx, playerID, flagID int64, now time.Time) (store.Flag, *store.Membership, error) {
	m, err := memberFor(ctx, tx, playerID)
	if err != nil {
		return store.Flag{}, nil, err
	}
	if !game.BaseCanApprove(m.Rank) {
		return store.Flag{}, nil, apperr.NotRegionalCommander
	}
	flag, err := store.FlagForUpdate(ctx, tx, flagID)
	if errors.Is(err, store.ErrNotFound) {
		return flag, nil, apperr.FlagGone
	}
	if err != nil {
		return flag, nil, err
	}
	if flag.FactionID != m.FactionID || flag.Region != m.Region {
		return flag, nil, apperr.NotRegionalCommander
	}
	if flag.Status != "open" || !flag.ExpiresAt.After(now) {
		return flag, nil, apperr.FlagGone
	}
	return flag, m, nil
}

// ApproveFlag raises the Spire on an open flag. The placement rules run again (the flag itself and the cap place
// it holds don't count against it).
func (s *Service) ApproveFlag(ctx context.Context, playerID, flagID int64) (store.MapBase, error) {
	var out store.MapBase
	st := s.Settings(ctx)
	now := s.now()
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if st.BasesEnabled != 1 {
			return apperr.BasesOff
		}
		flag, _, err := s.answerable(ctx, tx, playerID, flagID, now)
		if err != nil {
			return err
		}
		if err := store.LockBasePlacement(ctx, tx); err != nil {
			return err
		}
		sp, err := s.baseSpot(ctx, tx, flag.Lat, flag.Lon, flag.FactionID, flag.Region, 0, flag.ID, st)
		if err != nil {
			return err
		}
		if e := game.CheckBaseSpot(sp, st); e != nil {
			return spotErr(e)
		}
		if out, err = s.raiseSpire(ctx, tx, flag, playerID, st, now); err != nil {
			return err
		}
		return notify(ctx, tx, store.NewNotice{PlayerID: flag.ProposerID, ActorID: playerID, Kind: "base_approved",
			Body: game.NoticeBaseApproved(flag.Name)})
	})
	return out, err
}

// DeclineFlag takes an open flag down; its proposer goes on the cooldown.
func (s *Service) DeclineFlag(ctx context.Context, playerID, flagID int64) error {
	now := s.now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		flag, _, err := s.answerable(ctx, tx, playerID, flagID, now)
		if err != nil {
			return err
		}
		if err := store.DecideFlag(ctx, tx, flag.ID, "declined", playerID, 0, now); err != nil {
			return err
		}
		return notify(ctx, tx, store.NewNotice{PlayerID: flag.ProposerID, ActorID: playerID, Kind: "base_declined",
			Body: game.NoticeBaseDeclined(flag.Name)})
	})
}

// ProposalsView is GET /v1/bases/proposals: for the Commander, every open flag of the faction in the region (with the
// nearest bases); for anyone else, their own open flag. Plus the switch and the cap.
type ProposalsView struct {
	Flags        []store.Flag `json:"flags"`
	CanAnswer    bool         `json:"canAnswer"`
	CanPropose   bool         `json:"canPropose"`
	ProposalsOff bool         `json:"proposalsOff"`
	Held         int          `json:"held"`
	Cap          int          `json:"cap"`
}

func (s *Service) Proposals(ctx context.Context, playerID int64) (ProposalsView, error) {
	out := ProposalsView{Flags: []store.Flag{}}
	st := s.Settings(ctx)
	now := s.now()
	m, err := memberFor(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	out.CanAnswer = game.BaseCanApprove(m.Rank)
	out.CanPropose = game.Can(m.Rank, m.Nomad, game.PowerProposeBase) && st.BasesEnabled == 1
	if out.ProposalsOff, err = store.BranchProposalsOff(ctx, s.db, m.FactionID, m.Region); err != nil {
		return out, err
	}
	if out.Held, err = store.BasesHeld(ctx, s.db, m.FactionID, m.Region, 0, now); err != nil {
		return out, err
	}
	active, err := store.ActivePlayersInRegion(ctx, s.db, m.Region, st.BaseCapActiveDays, st.CountryAtMaxKm, now)
	if err != nil {
		return out, err
	}
	out.Cap = game.BaseCap(active, st)

	var flags []store.Flag
	if out.CanAnswer {
		flags, err = store.RegionFlags(ctx, s.db, m.FactionID, m.Region, now)
	} else {
		flags, err = store.PlayerFlags(ctx, s.db, playerID, now)
	}
	if err != nil {
		return out, err
	}
	for i := range flags {
		if out.CanAnswer {
			if flags[i].Near, err = store.NearestBases(ctx, s.db, flags[i].Lat, flags[i].Lon, game.BaseSearchM(st), 3); err != nil {
				return out, err
			}
			for j := range flags[i].Near {
				flags[i].Near[j].DistanceM = math.Ceil(flags[i].Near[j].DistanceM)
			}
		}
	}
	if flags != nil {
		out.Flags = flags
	}
	return out, nil
}

// SetProposalsOff is the Commander's switch for the region.
func (s *Service) SetProposalsOff(ctx context.Context, playerID int64, off bool) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := memberFor(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if !game.BaseCanApprove(m.Rank) || m.BranchID == nil {
			return apperr.NotRegionalCommander
		}
		return store.SetBranchProposalsOff(ctx, tx, *m.BranchID, off)
	})
}

// BasesNearbyView is GET /v1/bases/nearby: the bases whose zone reaches the area and the open flags in it. Everyone
// sees every faction's bases and flags.
type BasesNearbyView struct {
	Bases []store.MapBase `json:"bases"`
	Flags []store.Flag    `json:"flags"`
}

const basesNearbyMaxM = 3000

func (s *Service) BasesNearby(ctx context.Context, lat, lon, radius float64) (BasesNearbyView, error) {
	out := BasesNearbyView{Bases: []store.MapBase{}, Flags: []store.Flag{}}
	if e := validSpot(lat, lon); e != nil {
		return out, e
	}
	if radius <= 0 || radius > basesNearbyMaxM {
		radius = basesNearbyMaxM
	}
	st := s.Settings(ctx)
	bases, err := store.MapBasesNear(ctx, s.db, lat, lon, radius)
	if err != nil {
		return out, err
	}
	for i := range bases {
		bases[i].CoreMax = game.BaseCoreHealth(bases[i].Tier, st)
		bases[i].TierName = game.BaseTierName(bases[i].Tier)
	}
	if bases != nil {
		out.Bases = bases
	}
	flags, err := store.FlagsNear(ctx, s.db, lat, lon, radius, s.now())
	if err != nil {
		return out, err
	}
	if flags != nil {
		out.Flags = flags
	}
	return out, nil
}

// BaseCardView is GET /v1/bases/{id}: the base for everyone; its slots and projects only for its own members.
type BaseCardView struct {
	Base     store.MapBase   `json:"base"`
	Member   bool            `json:"member"`
	Slots    []store.SlotRow `json:"slots,omitempty"`
	Projects []store.Project `json:"projects,omitempty"`
}

func (s *Service) BaseCard(ctx context.Context, playerID, baseID int64) (BaseCardView, error) {
	var out BaseCardView
	st := s.Settings(ctx)
	base, err := store.MapBaseByID(ctx, s.db, baseID)
	if errors.Is(err, store.ErrNotFound) {
		return out, apperr.BaseGone
	}
	if err != nil {
		return out, err
	}
	base.CoreMax = game.BaseCoreHealth(base.Tier, st)
	base.TierName = game.BaseTierName(base.Tier)
	out.Base = base
	m, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	// Slots are part of the base everyone sees (the holo table draws them); projects only members.
	if out.Slots, err = store.BaseSlots(ctx, s.db, baseID); err != nil {
		return out, err
	}
	if m != nil && m.FactionID == base.FactionID {
		out.Member = true
		if out.Projects, err = store.RunningProjects(ctx, s.db, baseID); err != nil {
			return out, err
		}
	}
	return out, nil
}

// baseTick brings unanswered flags down and tells their proposers.
func (s *Service) baseTick(ctx context.Context) {
	s.rankTick(ctx)
	now := s.now()
	due, err := store.DueFlags(ctx, s.db, now, 50)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 base tick failed", "err", err)
		}
		return
	}
	for _, id := range due {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			flag, err := store.FlagForUpdate(ctx, tx, id)
			if err != nil || flag.Status != "open" {
				return err
			}
			if err := store.DecideFlag(ctx, tx, id, "expired", 0, 0, now); err != nil {
				return err
			}
			return notify(ctx, tx, store.NewNotice{PlayerID: flag.ProposerID, Kind: "base_expired",
				Body: game.NoticeBaseExpired(flag.Name)})
		})
		if err != nil {
			slog.Error("E1900 flag expiry failed", "flag", id, "err", err)
		}
	}
}
