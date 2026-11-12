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

// Faction bases, B2 (base-building-plan.md 4, 5, 6; ranks from faction-ranks-plan.md): a Warden or above starts a
// module or a module upgrade (from the holo table, anywhere); members give from their stored resources on site (inside
// the zone); Wardens and up pay from the base's vault anywhere in the region, Captains on site up to a daily cap;
// funded projects assemble on the tick, faster for each member on site; cancelling sends what was given to the vault.

var resourceOrder = [4]string{"alloy", "crystal", "data", "credit"}

func to64(a [4]int) [4]int64 { return [4]int64{int64(a[0]), int64(a[1]), int64(a[2]), int64(a[3])} }

// baseMember loads the caller's membership and the base, locked, and checks they belong to its faction and region.
func (s *Service) baseMember(ctx context.Context, tx *sql.Tx, playerID, baseID int64) (*store.Membership, store.BaseRef, error) {
	m, err := memberFor(ctx, tx, playerID)
	if err != nil {
		return nil, store.BaseRef{}, err
	}
	b, err := store.BaseRefForUpdate(ctx, tx, baseID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, b, apperr.BaseGone
	}
	if err != nil {
		return nil, b, err
	}
	if m.FactionID != b.FactionID || m.Region != b.Region {
		return nil, b, apperr.ProjectRefused.Say("Only its own faction's members of this region work on this base.")
	}
	return m, b, nil
}

// StartProjectRequest is POST /v1/bases/{id}/projects.
type StartProjectRequest struct {
	Kind   string `json:"kind"` // module, module_upgrade or tier (B3: the next tier, up to the Citadel)
	Ring   int    `json:"ring"`
	Index  int    `json:"index"`
	Module string `json:"module"`
}

func (s *Service) StartProject(ctx context.Context, playerID, baseID int64, req StartProjectRequest) (store.Project, error) {
	var out store.Project
	st := s.Settings(ctx)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, b, err := s.baseMember(ctx, tx, playerID, baseID)
		if err != nil {
			return err
		}
		toTier := 0
		if req.Kind == "tier" {
			toTier = b.Tier + 1
		}
		if !game.Can(m.Rank, m.Nomad, game.StartPower(req.Kind, toTier)) {
			if toTier == game.BaseTopTierNonCapital {
				return apperr.ProjectRefused.Say("Only an atoll/city lead or above grows a base to a Citadel.")
			}
			return apperr.ProjectRefused.Say("Only a Warden or above starts projects.")
		}
		if b.Status != "active" {
			return apperr.ProjectRefused.Say("The Spire has to stand first.")
		}
		// A fading base starts nothing new until a member walks to it (B6).
		if faded, err := store.BaseFaded(ctx, tx, baseID); err != nil {
			return err
		} else if faded {
			return apperr.ProjectRefused.Say("This base is fading. A member must walk to it first.")
		}
		running, err := store.RunningProjectRows(ctx, tx, baseID)
		if err != nil {
			return err
		}
		if len(running) >= st.BaseProjectsAtOnce {
			return apperr.ProjectRefused.Say("This base has as many projects running as it can.")
		}
		if req.Kind == "tier" {
			for _, p := range running {
				if p.Kind == "tier" {
					return apperr.ProjectRefused.Say("This base is already growing.")
				}
			}
			// The Capital is the regional Commander's seat (B8); every other base stops at the Citadel.
			if toTier > game.BaseTopTierNonCapital {
				return apperr.ProjectRefused.Say("A base grows no further than a Citadel for now.")
			}
			// The border rule (section 2): refused if the new zone would come within the gap of another zone.
			sp, err := s.baseSpot(ctx, tx, b.Lat, b.Lon, b.FactionID, b.Region, b.ID, 0, st)
			if err != nil {
				return err
			}
			if e := game.CheckBaseGrowth(sp, toTier, st); e != nil {
				return spotErr(e)
			}
			p := store.NewProject{BaseID: baseID, Kind: "tier", ToTier: toTier, StartedBy: playerID,
				Bill:        game.ProjectBill(game.BaseBill(toTier, st), b.FactionID, st),
				AssemblySec: int(game.BaseAssembly(toTier, st) / time.Second)}
			id, err := store.InsertProject(ctx, tx, p)
			if err != nil {
				return err
			}
			return s.findProject(ctx, tx, baseID, id, &out)
		}
		for _, p := range running {
			if p.Ring == req.Ring && p.Index == req.Index && (p.Kind == "module" || p.Kind == "module_upgrade") {
				return apperr.ProjectRefused.Say("That pad already has a project.")
			}
		}
		module, mk, exists, err := store.SlotModule(ctx, tx, baseID, req.Ring, req.Index)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.ProjectRefused.Say("This base has no such pad.")
		}
		var p store.NewProject
		switch req.Kind {
		case "module":
			if module != "" {
				return apperr.ProjectRefused.Say("That pad holds a module already.")
			}
			if !game.ValidModule(req.Module) {
				return apperr.BadRequest.With("module")
			}
			if b.Tier < game.ModuleMinTier(req.Module) {
				return apperr.ProjectRefused.Say("That module needs an Outpost or bigger.")
			}
			p = store.NewProject{Kind: "module", Module: req.Module, Mk: 1}
		case "module_upgrade":
			if module == "" {
				return apperr.ProjectRefused.Say("That pad is empty.")
			}
			if mk >= game.MaxModuleMk {
				return apperr.ProjectRefused.Say("That module is at its top Mk.")
			}
			p = store.NewProject{Kind: "module_upgrade", Module: module, Mk: mk + 1}
		default:
			return apperr.BadRequest.With("kind: module or module_upgrade")
		}
		p.BaseID, p.Ring, p.Index, p.StartedBy = baseID, req.Ring, req.Index, playerID
		p.Bill = game.ProjectBill(game.ModuleBill(p.Module, p.Mk, st), b.FactionID, st)
		p.AssemblySec = int(game.ModuleAssembly(p.Module, p.Mk, st) / time.Second)
		id, err := store.InsertProject(ctx, tx, p)
		if err != nil {
			return err
		}
		return s.findProject(ctx, tx, baseID, id, &out)
	})
	return out, err
}

func (s *Service) findProject(ctx context.Context, tx *sql.Tx, baseID, id int64, out *store.Project) error {
	projects, err := store.RunningProjects(ctx, tx, baseID)
	if err != nil {
		return err
	}
	for _, pr := range projects {
		if pr.ID == id {
			*out = pr
		}
	}
	return nil
}

// growBase raises a base a tier: zone, core health and the new slots. The border rule runs again first (a neighbour
// may have grown since); false when it holds the growth back.
func (s *Service) growBase(ctx context.Context, tx *sql.Tx, b store.BaseRef, toTier int, st game.Settings) (bool, error) {
	if err := store.LockBasePlacement(ctx, tx); err != nil {
		return false, err
	}
	sp, err := s.baseSpot(ctx, tx, b.Lat, b.Lon, b.FactionID, b.Region, b.ID, 0, st)
	if err != nil {
		return false, err
	}
	if game.CheckBaseGrowth(sp, toTier, st) != nil {
		return false, nil
	}
	if err := store.SetBaseTier(ctx, tx, b.ID, toTier, game.BaseCoreHealth(toTier, st)); err != nil {
		return false, err
	}
	if err := store.SetBaseZone(ctx, tx, b.ID, game.BaseZoneM(toTier, st)); err != nil {
		return false, err
	}
	_, err = store.SetBaseSlots(ctx, tx, b.ID, game.BaseSlotLayout(toTier, st))
	return err == nil, err
}

// GiveRequest is POST /v1/bases/{id}/projects/{pid}/give: the fix, what to give, and whether from the vault.
type GiveRequest struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AccuracyM float64 `json:"accuracyM"`
	Alloy     int64   `json:"alloy"`
	Crystal   int64   `json:"crystal"`
	Data      int64   `json:"data"`
	Credit    int64   `json:"credit"`
	FromVault bool    `json:"fromVault"`
}

// GiveResult is what was given (capped to what the bill still needed and what was held) and the project now.
type GiveResult struct {
	Given   store.Resources4 `json:"given"`
	Points  int64            `json:"points"`
	Funded  bool             `json:"funded"`
	Project store.Project    `json:"project"`
}

func (s *Service) GiveToProject(ctx context.Context, playerID, baseID, projectID int64, req GiveRequest) (GiveResult, error) {
	var out GiveResult
	st := s.Settings(ctx)
	now := s.now()
	offer := [4]int64{req.Alloy, req.Crystal, req.Data, req.Credit}
	for _, v := range offer {
		if v < 0 {
			return out, apperr.BadRequest.With("amounts")
		}
	}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, b, err := s.baseMember(ctx, tx, playerID, baseID)
		if err != nil {
			return err
		}
		p, err := store.ProjectForUpdate(ctx, tx, projectID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && (p.BaseID != baseID || p.Status != "funding")) {
			return apperr.ProjectGone
		}
		if err != nil {
			return err
		}
		onSite := req.AccuracyM > 0 && req.AccuracyM <= st.BaseSpotAccuracyM &&
			game.DistanceM(req.Lat, req.Lon, b.Lat, b.Lon) <= b.ZoneM
		var have [4]int64
		if req.FromVault {
			anywhere := game.Can(m.Rank, m.Nomad, game.PowerVaultAnywhere)
			onSiteOnly := game.Can(m.Rank, m.Nomad, game.PowerVaultOnSite)
			switch {
			case !anywhere && !onSiteOnly:
				return apperr.ProjectRefused.Say("Only a Captain or above pays from the vault.")
			case !anywhere && !onSite:
				return apperr.NotOnSite
			case anywhere && !onSite:
				// Anywhere in the region: the fix must be in the base's country.
				here, err := s.regionAt(ctx, RegionFix{Lat: req.Lat, Lon: req.Lon, AccuracyM: req.AccuracyM})
				if err != nil {
					return err
				}
				if here.Code != b.Region {
					return apperr.AwayFromHome
				}
			}
			if have, err = store.VaultForUpdate(ctx, tx, baseID); err != nil {
				return err
			}
			if !anywhere {
				today, err := store.VaultGivenSince(ctx, tx, playerID, now.Add(-24*time.Hour))
				if err != nil {
					return err
				}
				left := int64(st.CaptainVaultDailyCap) - today
				if left <= 0 {
					return apperr.VaultDailyCap
				}
				have = capTotal(have, left)
			}
		} else {
			if !game.Can(m.Rank, m.Nomad, game.PowerGive) {
				return apperr.ProjectRefused.Say("Your rank can't give.")
			}
			if !onSite {
				return apperr.NotOnSite
			}
			stored, err := store.Uploaded(ctx, tx, playerID)
			if err != nil {
				return err
			}
			for i, r := range resourceOrder {
				have[i] = stored[r]
			}
		}
		given := game.Give(offer, have, p.Bill, p.Given)
		if game.Total(given) == 0 {
			return apperr.NothingToGive
		}
		if req.FromVault {
			if err := store.AddVault(ctx, tx, baseID, playerID, projectID, "spend",
				[4]int64{-given[0], -given[1], -given[2], -given[3]}, now); err != nil {
				return err
			}
		} else {
			for i, r := range resourceOrder {
				if given[i] > 0 {
					if err := store.AddUploaded(ctx, tx, playerID, r, -given[i]); err != nil {
						return err
					}
				}
			}
			for i, r := range resourceOrder {
				if given[i] > 0 {
					if err := advanceMissions(ctx, tx, playerID, game.GoalBaseGive, r, int(given[i]), st, now); err != nil {
						return err
					}
				}
			}
			out.Points = game.GivePoints(given, st)
			if err := store.AddContributionPoints(ctx, tx, playerID, out.Points); err != nil {
				return err
			}
			if xp := game.GiveXP(given, st); xp > 0 {
				if _, _, err := awardXP(ctx, tx, playerID, "base_give", xp, projectID, st); err != nil {
					return err
				}
			}
		}
		if err := store.InsertContribution(ctx, tx, baseID, projectID, playerID, req.FromVault, given, out.Points); err != nil {
			return err
		}
		var total [4]int64
		for i := range total {
			total[i] = p.Given[i] + given[i]
		}
		out.Funded = game.Funded(p.Bill, total)
		if err := store.SetProjectGiven(ctx, tx, projectID, total, out.Funded, now); err != nil {
			return err
		}
		out.Given = store.Resources4{Alloy: given[0], Crystal: given[1], Data: given[2], Credit: given[3]}
		projects, err := store.RunningProjects(ctx, tx, baseID)
		if err != nil {
			return err
		}
		for _, pr := range projects {
			if pr.ID == projectID {
				out.Project = pr
			}
		}
		return nil
	})
	return out, err
}

// capTotal trims amounts so they add up to at most limit, in resource order.
func capTotal(a [4]int64, limit int64) [4]int64 {
	for i := range a {
		a[i] = max(0, min(a[i], limit))
		limit -= a[i]
	}
	return a
}

// CancelProject: whoever started it, or a higher rank. What was given goes to the base's vault, not back to members
// (so nobody farms points). The Spire's own project can't be cancelled.
func (s *Service) CancelProject(ctx context.Context, playerID, baseID, projectID int64) error {
	now := s.now()
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, _, err := s.baseMember(ctx, tx, playerID, baseID)
		if err != nil {
			return err
		}
		p, err := store.ProjectForUpdate(ctx, tx, projectID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && (p.BaseID != baseID || (p.Status != "funding" && p.Status != "assembling"))) {
			return apperr.ProjectGone
		}
		if err != nil {
			return err
		}
		if p.Kind == "spire" {
			return apperr.ProjectRefused.Say("The Spire's project can't be cancelled.")
		}
		starterRank := game.RankHead
		if starter, err := store.MembershipOf(ctx, tx, p.StartedBy); err != nil {
			return err
		} else if starter != nil {
			starterRank = starter.Rank
		} else {
			starterRank = 0
		}
		if !game.CanCancelProject(p.StartedBy == playerID, m.Rank, starterRank) {
			return apperr.ProjectRefused.Say("Only whoever started it, or a higher rank, can cancel it.")
		}
		if err := store.AddVault(ctx, tx, baseID, playerID, projectID, "cancel", p.Given, now); err != nil {
			return err
		}
		return store.CancelProject(ctx, tx, projectID, playerID, now)
	})
}

// projectTick advances every assembling project by the time since it was last looked at, at the presence rate, and
// finishes those that are done: the Spire stands (protected for a while), a module locks on, a Mk goes up.
func (s *Service) projectTick(ctx context.Context) {
	st := s.Settings(ctx)
	now := s.now()
	ids, err := store.AssemblingIDs(ctx, s.db, 200)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 project tick failed", "err", err)
		}
		return
	}
	for _, id := range ids {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			p, err := store.ProjectForUpdate(ctx, tx, id)
			if err != nil || p.Status != "assembling" {
				return err
			}
			b, err := store.BaseRefForUpdate(ctx, tx, p.BaseID)
			if err != nil {
				return err
			}
			since := now
			if p.TickedAt != nil {
				since = *p.TickedAt
			}
			dt := now.Sub(since).Seconds()
			present, err := store.MembersOnSite(ctx, tx, b.ID, b.FactionID, now.Add(-time.Duration(st.BasePresenceSec*float64(time.Second))))
			if err != nil {
				return err
			}
			assembled := p.Assembled + max(0, dt)*game.AssemblyRate(b.FactionID, present, st)
			done := assembled >= float64(p.AssemblySec)
			if done && p.Kind == "tier" {
				// A tier waits, assembled, while the border holds it back; it grows as soon as there is room.
				grown, err := s.growBase(ctx, tx, b, p.ToTier, st)
				if err != nil {
					return err
				}
				return store.AdvanceProject(ctx, tx, id, float64(p.AssemblySec), grown, now)
			}
			if err := store.AdvanceProject(ctx, tx, id, min(assembled, float64(p.AssemblySec)), done, now); err != nil {
				return err
			}
			if !done {
				return nil
			}
			switch p.Kind {
			case "spire":
				return store.SetBaseActive(ctx, tx, b.ID, now.Add(time.Duration(st.BaseProtectNewHours*float64(time.Hour))))
			case "module", "module_upgrade":
				if err := store.SetSlotModule(ctx, tx, b.ID, p.Ring, p.Index, p.Module, p.Mk, game.ModuleHealth(p.Mk, st)); err != nil {
					return err
				}
				// Modules need power: the Energy Lattices decide which are online (B5).
				return recomputePower(ctx, tx, b.ID, st)
			}
			return nil
		})
		if err != nil {
			slog.Error("E1900 project advance failed", "project", id, "err", err)
		}
	}
}
