package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Region from GPS (docs/social-signal-plan.md Part B, R1): claiming and joining use the country the player
// stands in. A country that isn't live (LiveRegions) can't be claimed. Admin test regions (XA..XZ) exist on dev
// only, so the flow can be tried in Malé.

// RegionFix is where a faction request was made from.
type RegionFix struct {
	Lat, Lon, AccuracyM float64
}

func (f RegionFix) known() bool { return f.Lat != 0 || f.Lon != 0 }

// regionAt resolves a fix to its region. A request with no fix (an old app) counts as the launch region.
func (s *Service) regionAt(ctx context.Context, fix RegionFix) (store.Country, error) {
	if !fix.known() {
		name, err := store.CountryName(ctx, s.db, game.LaunchRegion)
		if err != nil {
			return store.Country{}, err
		}
		if name == "" {
			name = game.RegionName(game.LaunchRegion)
		}
		return store.Country{Code: game.LaunchRegion, Name: name, Live: true}, nil
	}
	c, err := store.CountryAt(ctx, s.db, fix.Lat, fix.Lon, s.Settings(ctx).CountryAtMaxKm, !s.cfg.IsLive())
	if errors.Is(err, store.ErrNotFound) {
		return c, apperr.RegionUnknown
	}
	return c, err
}

// RegionHere is GET /v1/regions/here: the region a fix is in, and whether it can be claimed.
func (s *Service) RegionHere(ctx context.Context, fix RegionFix) (store.Country, error) {
	return s.regionAt(ctx, fix)
}

var testRegionCode = regexp.MustCompile(`^X[A-Z]$`)

// AdminTestRegions lists the test regions (dev only).
func (s *Service) AdminTestRegions(ctx context.Context) ([]store.TestRegion, []string, error) {
	if s.cfg.IsLive() {
		return nil, nil, apperr.Forbidden.With("test regions are for the dev server")
	}
	regions, err := store.TestRegions(ctx, s.db)
	if err != nil {
		return nil, nil, err
	}
	live, err := store.LiveRegions(ctx, s.db)
	return regions, live, err
}

// AdminSaveTestRegion draws (or redraws) a test region: a code XA..XZ, a name, and at least three points.
func (s *Service) AdminSaveTestRegion(ctx context.Context, adminID int64, r store.TestRegion) error {
	if s.cfg.IsLive() {
		return apperr.Forbidden.With("test regions are for the dev server")
	}
	r.Code = strings.ToUpper(strings.TrimSpace(r.Code))
	r.Name = strings.TrimSpace(r.Name)
	if !testRegionCode.MatchString(r.Code) || r.Name == "" || len(r.Points) < 3 {
		return apperr.BadRequest.Say("A test region needs a code XA to XZ, a name and at least 3 points.")
	}
	if err := store.SaveTestRegion(ctx, s.db, r); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"name": r.Name, "points": len(r.Points)})
	return store.LogAdminAction(ctx, s.db, adminID, "region.test", r.Code, string(detail))
}

func (s *Service) AdminDeleteTestRegion(ctx context.Context, adminID int64, code string) error {
	if s.cfg.IsLive() {
		return apperr.Forbidden.With("test regions are for the dev server")
	}
	code = strings.ToUpper(code)
	if err := store.DeleteTestRegion(ctx, s.db, code); err != nil {
		return err
	}
	return store.LogAdminAction(ctx, s.db, adminID, "region.test.delete", code, "{}")
}

// AdminSetLiveRegion adds a country to LiveRegions or takes it out (admins, logged).
func (s *Service) AdminSetLiveRegion(ctx context.Context, adminID int64, code string, live bool) ([]string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return nil, apperr.BadRequest.Say("A region is a two-letter country code.")
	}
	if err := store.SetLiveRegion(ctx, s.db, code, live); err != nil {
		return nil, err
	}
	if err := store.LogAdminAction(ctx, s.db, adminID, "region.live", code, fmt.Sprintf(`{"live":%v}`, live)); err != nil {
		return nil, err
	}
	return store.LiveRegions(ctx, s.db)
}

// RegionsView is GET /v1/factions/regions: a member's faction's unlocked regions (talk only away from home).
type RegionsView struct {
	Regions []store.UnlockedRegion `json:"regions"`
	Home    string                 `json:"home"`
	Viewing string                 `json:"viewing"`
}

func (s *Service) FactionRegions(ctx context.Context, playerID int64) (RegionsView, error) {
	view := RegionsView{Regions: []store.UnlockedRegion{}}
	m, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return view, err
	}
	if m == nil || m.BranchID == nil {
		return view, apperr.NotAMember
	}
	viewing, err := s.viewedBranch(ctx, playerID, *m.BranchID)
	if err != nil {
		return view, err
	}
	regions, err := store.UnlockedRegions(ctx, s.db, m.FactionID)
	if err != nil {
		return view, err
	}
	view.Home = m.Region
	for i := range regions {
		regions[i].Home = regions[i].BranchID == *m.BranchID
		regions[i].Viewing = regions[i].BranchID == viewing
		if regions[i].Viewing {
			view.Viewing = regions[i].Region
		}
	}
	view.Regions = regions
	return view, nil
}

// viewedBranch is the branch a member's FACTIONS screen shows: the one they chose while it is still unlocked,
// else home.
func (s *Service) viewedBranch(ctx context.Context, playerID, home int64) (int64, error) {
	viewing, err := store.ViewingBranch(ctx, s.db, playerID)
	if err != nil || viewing == 0 || viewing == home {
		return home, err
	}
	b, err := store.BranchByID(ctx, s.db, viewing)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !b.Unlocked) {
		// The region went dormant: back home.
		return home, store.SetViewingBranch(ctx, s.db, playerID, 0)
	}
	return viewing, err
}

// ViewRegion switches the region a member views: one of their faction's unlocked regions, or "home". No GPS
// check, no cost, no cooldown. It never changes the home region.
func (s *Service) ViewRegion(ctx context.Context, playerID int64, region string) (RegionsView, error) {
	m, err := store.MembershipOf(ctx, s.db, playerID)
	if err != nil {
		return RegionsView{}, err
	}
	if m == nil || m.BranchID == nil {
		return RegionsView{}, apperr.NotAMember
	}
	region = strings.ToUpper(strings.TrimSpace(region))
	target := int64(0)
	if region != "" && region != "HOME" && region != m.Region {
		regions, err := store.UnlockedRegions(ctx, s.db, m.FactionID)
		if err != nil {
			return RegionsView{}, err
		}
		for _, r := range regions {
			if r.Region == region {
				target = r.BranchID
			}
		}
		if target == 0 {
			return RegionsView{}, apperr.RegionLocked
		}
	}
	if err := store.SetViewingBranch(ctx, s.db, playerID, target); err != nil {
		return RegionsView{}, err
	}
	return s.FactionRegions(ctx, playerID)
}

// AdminMoveRegion changes a member's home region for a player who really moved (admins, logged). They become a
// plain member there; a commander's seat they held is left empty.
func (s *Service) AdminMoveRegion(ctx context.Context, adminID int64, code, region, reason string) (*PlayerInfo, error) {
	admin, err := store.GetPlayer(ctx, s.db, adminID)
	if err != nil {
		return nil, err
	}
	if !admin.IsAdmin {
		return nil, apperr.Forbidden
	}
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < game.SanctionReasonMin || n > game.SanctionReasonMax {
		return nil, apperr.SanctionBad
	}
	region = strings.ToUpper(strings.TrimSpace(region))
	targetID, err := staffTargetByCode(ctx, s.db, code)
	if err != nil {
		return nil, err
	}
	name, err := store.CountryName(ctx, s.db, region)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, apperr.BadRequest.Say("No region %s.", region)
	}
	var info *PlayerInfo
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, targetID); err != nil {
			return err
		}
		m, err := store.MembershipOf(ctx, tx, targetID)
		if err != nil {
			return err
		}
		if m == nil {
			return apperr.NotAMember
		}
		branch, err := store.BranchForUpdate(ctx, tx, m.FactionID, region, m.FactionName+" "+name)
		if err != nil {
			return err
		}
		if err := store.MoveMemberHome(ctx, tx, targetID, branch.ID, game.RankMember); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"from": m.Region, "to": region, "reason": reason})
		if err := store.LogAdminAction(ctx, tx, adminID, "player.move_region", fmt.Sprint(targetID), string(detail)); err != nil {
			return err
		}
		info, err = s.playerInfo(ctx, tx, admin, targetID)
		return err
	})
	return info, err
}
