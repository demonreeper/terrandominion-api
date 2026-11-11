package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
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
