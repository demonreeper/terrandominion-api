package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction bases, B0 (docs/base-building-plan.md): the placement rules run on real data, and the admin tools
// (place, clear, set tier, release a flag, check a spot). Players' routes come with B1.

// baseSpot gathers what the placement rules need at a spot: nearby bases and flags, the map layers and the
// faction's count against its cap in the region.
func (s *Service) baseSpot(ctx context.Context, q db.Querier, lat, lon float64, factionID int, region string,
	selfID, selfFlag int64, st game.Settings) (game.BaseSpot, error) {
	sp := game.BaseSpot{Lat: lat, Lon: lon, FactionID: factionID, SelfID: selfID, SelfFlagID: selfFlag}
	search := game.BaseSearchM(st)
	var err error
	if sp.Bases, err = store.NearBases(ctx, q, lat, lon, search); err != nil {
		return sp, err
	}
	if sp.Flags, err = store.NearFlags(ctx, q, lat, lon, search, s.now()); err != nil {
		return sp, err
	}
	if sp.Map, err = store.BaseSpotMap(ctx, q, lat, lon, st.BaseWalkwayM, st.BaseSensitiveBufferM); err != nil {
		return sp, err
	}
	if sp.Held, err = store.BasesHeld(ctx, q, factionID, region, selfFlag, s.now()); err != nil {
		return sp, err
	}
	active, err := store.ActivePlayersInRegion(ctx, q, region, st.BaseCapActiveDays, st.CountryAtMaxKm, s.now())
	if err != nil {
		return sp, err
	}
	sp.Cap = game.BaseCap(active, st)
	return sp, nil
}

// spotErr turns a refused spot into the error the app shows, naming the base it is too close to.
func spotErr(e *game.BaseSpotError) *apperr.Error {
	if e.Near == "" {
		return e.Err
	}
	return e.Err.WithData(map[string]any{"near": e.Near})
}

// BaseSpotCheck is an admin's look at a spot: which rule (if any) refuses it, and why.
type BaseSpotCheck struct {
	Region  string       `json:"region"`
	Faction string       `json:"faction"`
	OK      bool         `json:"ok"`
	Code    int          `json:"code,omitempty"`
	Message string       `json:"message,omitempty"`
	Near    string       `json:"near,omitempty"`
	Map     game.SpotMap `json:"map"`
	Held    int          `json:"held"`
	Cap     int          `json:"cap"`
	Bases   int          `json:"basesNearby"`
	Flags   int          `json:"flagsNearby"`
	// RoomM is how far a Spire here could grow before the border rule stops it (-1: no neighbour).
	RoomM float64 `json:"roomM"`
}

func validSpot(lat, lon float64) *apperr.Error {
	if !game.ValidCoordinate(lat, lon) || (lat == 0 && lon == 0) {
		return apperr.BadRequest.With("coordinate")
	}
	return nil
}

// AdminCheckBaseSpot runs the placement rules (2 to 5, and other factions' flags) for a faction at a spot,
// without placing anything.
func (s *Service) AdminCheckBaseSpot(ctx context.Context, factionCode string, lat, lon float64) (BaseSpotCheck, error) {
	var out BaseSpotCheck
	if e := validSpot(lat, lon); e != nil {
		return out, e
	}
	region, err := s.regionAt(ctx, RegionFix{Lat: lat, Lon: lon})
	if err != nil {
		return out, err
	}
	st := s.Settings(ctx)
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		faction, err := store.FactionByCodeForUpdate(ctx, tx, factionCode)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("faction %q", factionCode)
		}
		if err != nil {
			return err
		}
		sp, err := s.baseSpot(ctx, tx, lat, lon, faction.ID, region.Code, 0, 0, st)
		if err != nil {
			return err
		}
		out = BaseSpotCheck{Region: region.Code, Faction: faction.Code, OK: true, Map: sp.Map, Held: sp.Held, Cap: sp.Cap,
			Bases: len(sp.Bases), Flags: len(sp.Flags), RoomM: -1}
		if room := game.BaseZoneRoomM(sp, st); !math.IsInf(room, 1) {
			out.RoomM = math.Round(room*10) / 10
		}
		if e := game.CheckBaseSpot(sp, st); e != nil {
			out.OK, out.Code, out.Message, out.Near = false, e.Err.Code, e.Err.Message, e.Near
		}
		return nil
	})
	return out, err
}

// AdminPlaceBaseRequest is POST /v1/admin/bases. Force skips the placement rules (spacing, zones, flags, cap,
// terrain), for testing.
type AdminPlaceBaseRequest struct {
	Faction string  `json:"faction"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Tier    int     `json:"tier"`
	Name    string  `json:"name"`
	Force   bool    `json:"force"`
}

// AdminPlaceBase puts a standing base on the map at once: no flag, no project.
func (s *Service) AdminPlaceBase(ctx context.Context, adminID int64, req AdminPlaceBaseRequest) (store.BaseRow, error) {
	var out store.BaseRow
	req.Name = strings.ToUpper(strings.TrimSpace(req.Name))
	if req.Tier == 0 {
		req.Tier = 1
	}
	switch {
	case !game.ValidBaseTier(req.Tier):
		return out, apperr.BadRequest.With("tier 1 to 5")
	case req.Name == "" || utf8.RuneCountInString(req.Name) > 32:
		return out, apperr.BadRequest.With("name, up to 32")
	}
	if e := validSpot(req.Lat, req.Lon); e != nil {
		return out, e
	}
	region, err := s.regionAt(ctx, RegionFix{Lat: req.Lat, Lon: req.Lon})
	if err != nil {
		return out, err
	}
	st := s.Settings(ctx)
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		faction, err := store.FactionByCodeForUpdate(ctx, tx, req.Faction)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("faction %q", req.Faction)
		}
		if err != nil {
			return err
		}
		if err := store.LockBasePlacement(ctx, tx); err != nil {
			return err
		}
		if !req.Force {
			sp, err := s.baseSpot(ctx, tx, req.Lat, req.Lon, faction.ID, region.Code, 0, 0, st)
			if err != nil {
				return err
			}
			if e := game.CheckBaseSpot(sp, st); e != nil {
				return spotErr(e)
			}
			if req.Tier > 1 {
				if e := game.CheckBaseGrowth(sp, req.Tier, st); e != nil {
					return spotErr(e)
				}
			}
		}
		id, err := store.InsertBase(ctx, tx, store.NewBase{
			FactionID: faction.ID, Region: region.Code, Name: req.Name, Tier: req.Tier, Lat: req.Lat, Lon: req.Lon,
			Status: "active", CoreHealth: game.BaseCoreHealth(req.Tier, st), ZoneM: game.BaseZoneM(req.Tier, st),
			RaisedBy: adminID,
		})
		if err != nil {
			return err
		}
		if _, err := store.SetBaseSlots(ctx, tx, id, game.BaseSlotLayout(req.Tier, st)); err != nil {
			return err
		}
		if out, err = store.BaseForUpdate(ctx, tx, id); err != nil {
			return err
		}
		detail, _ := json.Marshal(req)
		return store.LogAdminAction(ctx, tx, adminID, "base.place", fmt.Sprint(id), string(detail))
	})
	return out, err
}

// AdminSetBaseTier moves a base to a tier: zone, core health and slots follow. Raising it runs the border rule
// unless forced. Slots a lower tier doesn't have are removed (modules come with B2; their packing away with B6).
func (s *Service) AdminSetBaseTier(ctx context.Context, adminID, baseID int64, tier int, force bool) (store.BaseRow, error) {
	var out store.BaseRow
	if !game.ValidBaseTier(tier) {
		return out, apperr.BadRequest.With("tier 1 to 5")
	}
	st := s.Settings(ctx)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.LockBasePlacement(ctx, tx); err != nil {
			return err
		}
		b, err := store.BaseForUpdate(ctx, tx, baseID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.BaseGone
		}
		if err != nil {
			return err
		}
		if tier > b.Tier && !force {
			sp, err := s.baseSpot(ctx, tx, b.Lat, b.Lon, b.FactionID, b.Region, b.ID, 0, st)
			if err != nil {
				return err
			}
			if e := game.CheckBaseGrowth(sp, tier, st); e != nil {
				return spotErr(e)
			}
		}
		if err := store.SetBaseTier(ctx, tx, b.ID, tier, game.BaseCoreHealth(tier, st)); err != nil {
			return err
		}
		if err := store.SetBaseZone(ctx, tx, b.ID, game.BaseZoneM(tier, st)); err != nil {
			return err
		}
		if _, err := store.SetBaseSlots(ctx, tx, b.ID, game.BaseSlotLayout(tier, st)); err != nil {
			return err
		}
		if out, err = store.BaseForUpdate(ctx, tx, b.ID); err != nil {
			return err
		}
		return store.LogAdminAction(ctx, tx, adminID, "base.tier", fmt.Sprint(b.ID),
			fmt.Sprintf(`{"from":%d,"to":%d,"force":%t}`, b.Tier, tier, force))
	})
	return out, err
}

// AdminClearBase removes a base and everything of it.
func (s *Service) AdminClearBase(ctx context.Context, adminID, baseID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		b, err := store.BaseForUpdate(ctx, tx, baseID)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.BaseGone
		}
		if err != nil {
			return err
		}
		if err := endBaseJobs(ctx, tx, b.ID, b.Name); err != nil {
			return err
		}
		if err := store.DeleteBase(ctx, tx, b.ID); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"name": b.Name, "faction": b.Faction, "tier": b.Tier})
		return store.LogAdminAction(ctx, tx, adminID, "base.clear", fmt.Sprint(b.ID), string(detail))
	})
}

// AdminReleaseFlag takes an open flag down, freeing its ground. No cooldown for the proposer.
func (s *Service) AdminReleaseFlag(ctx context.Context, adminID, flagID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ok, err := store.ReleaseFlag(ctx, tx, flagID, adminID, s.now())
		if err != nil {
			return err
		}
		if !ok {
			return apperr.FlagGone
		}
		return store.LogAdminAction(ctx, tx, adminID, "base.flag.release", fmt.Sprint(flagID), "{}")
	})
}

// AdminBasesView is GET /v1/admin/bases: the standing bases and open flags of a region (all with "").
type AdminBasesView struct {
	Bases []store.BaseRow `json:"bases"`
	Flags []store.FlagRow `json:"flags"`
}

func (s *Service) AdminBases(ctx context.Context, region string) (AdminBasesView, error) {
	var out AdminBasesView
	var err error
	region = strings.ToUpper(strings.TrimSpace(region))
	if out.Bases, err = store.ListBases(ctx, s.db, region); err != nil {
		return out, err
	}
	out.Flags, err = store.ListOpenFlags(ctx, s.db, region, s.now())
	return out, err
}
