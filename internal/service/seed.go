package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Node seeding (drone strike plan, milestone 0): the server keeps the map stocked by itself. A cell
// is seeded the first time a scan touches it (or when an admin pre-seeds its area), and the world
// tick tops cells up when they fall under target.

const (
	// SeedSource marks nodes the seeder placed; their cell id is in source_ref.
	SeedSource = "seed"
	// RareSource marks rare roaming nodes, which end at expires_at.
	RareSource = "rare"
	// GroupSource marks walk-together group nodes (duo, trio, quad), which end at expires_at.
	GroupSource = "group"

	// seedStepM is how often a walkable way is sampled for spots; well under SeedMinGapM.
	seedStepM = 5
	// seedCandidates caps the spots one cell's query returns.
	seedCandidates = 400
)

type seedMode int

const (
	seedFirst  seedMode = iota // first pass: skip the cell if it was already seeded
	seedTopUp                  // the tick: fill back up to the stored target
	seedReseed                 // admin: clear the cell's seed nodes and start again
)

func toBox(b game.Bounds) store.Box {
	return store.Box{MinLat: b.MinLat, MinLon: b.MinLon, MaxLat: b.MaxLat, MaxLon: b.MaxLon}
}

// SeedCellResult is what one seeding pass did.
type SeedCellResult struct {
	CellID  string `json:"cellId"`
	Target  int    `json:"target"`
	Created int    `json:"created"`
	Retired int64  `json:"retired,omitempty"`
}

// seedCell runs one pass over one cell in its own transaction, the cell's row locked throughout.
func (s *Service) seedCell(ctx context.Context, settings game.Settings, id string, box store.Box,
	mode seedMode) (SeedCellResult, error) {
	res := SeedCellResult{CellID: id}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := store.LockSeedCell(ctx, tx, id, box)
		if err != nil {
			return err
		}
		if mode == seedFirst && row.SeededAt != nil {
			res.Target = row.Target
			return nil
		}
		topUp := mode == seedTopUp && row.SeededAt != nil
		target, walkM := row.Target, row.WalkM
		if !topUp {
			if walkM, err = store.WalkLengthInCell(ctx, tx, box); err != nil {
				return err
			}
			target = game.SeedTarget(settings, walkM)
		}
		res.Target = target
		if mode == seedReseed {
			if res.Retired, err = store.RetireNodesInCell(ctx, tx, box, SeedSource); err != nil {
				return err
			}
		}
		have, err := store.CountSeedNodesInCell(ctx, tx, box, SeedSource)
		if err != nil {
			return err
		}
		if need := target - have; need > 0 {
			created, err := s.placeNodes(ctx, tx, settings, box, need, func(kind game.SeedKind) store.NewNode {
				return store.NewNode{
					Name: game.SeedName(kind.Label, rand.IntN(26), rand.IntN(100)), Resource: kind.Resource,
					Yield: kind.Yield, RegenPerHour: settings.SeedRegenPerHour, Source: SeedSource, SourceRef: id,
				}
			})
			if err != nil {
				return err
			}
			res.Created = created
		}
		return store.MarkCellSeeded(ctx, tx, id, target, walkM, topUp)
	})
	return res, err
}

// placeNodes finds up to need spots in the box that obey every gap and creates a node at each.
func (s *Service) placeNodes(ctx context.Context, tx *sql.Tx, settings game.Settings, box store.Box, need int,
	build func(game.SeedKind) store.NewNode) (int, error) {
	cands, err := store.SeedCandidates(ctx, tx, box, seedStepM, settings.SeedSnapM, settings.SeedMinGapM,
		settings.SeedTnGapM, seedCandidates)
	if err != nil {
		return 0, err
	}
	spots := make([]game.Spot, len(cands))
	for i, c := range cands {
		spots[i] = game.Spot{Lat: c.Lat, Lon: c.Lon}
	}
	kinds := game.SeedKinds(settings)
	created := 0
	for _, spot := range game.PickSeedSpots(spots, need, settings.SeedMinGapM) {
		n := build(game.PickSeedKind(kinds, rand.Float64()))
		n.Lat, n.Lon = spot.Lat, spot.Lon
		if _, err := store.CreateNode(ctx, tx, n, 0); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// seedScan seeds the unseeded cells a scan touches, nearest first and at most SeedCellsPerScan of
// them, so one request stays quick; the rest are seeded by the next scans. Failures are logged,
// never shown: the player still gets the nodes that exist.
func (s *Service) seedScan(ctx context.Context, settings game.Settings, lat, lon, radiusM float64) {
	if settings.SeedPerCell <= 0 || settings.SeedCellM <= 0 || settings.SeedCellsPerScan <= 0 {
		return
	}
	cells := game.SeedCellsAround(settings.SeedCellM, lat, lon, radiusM)
	if len(cells) == 0 {
		return
	}
	ids := make([]string, len(cells))
	for i, c := range cells {
		ids[i] = c.ID(settings.SeedCellM)
	}
	seeded, err := store.SeededCells(ctx, s.db, ids)
	if err != nil {
		slog.Error("E1900 seeding check failed", "err", err)
		return
	}
	if len(seeded) == len(ids) {
		return
	}
	if loaded, err := store.OSMLoaded(ctx, s.db); err != nil || !loaded {
		return
	}
	done := 0
	for i, c := range cells {
		if seeded[ids[i]] {
			continue
		}
		if done >= settings.SeedCellsPerScan || ctx.Err() != nil {
			return
		}
		done++
		res, err := s.seedCell(ctx, settings, ids[i], toBox(c.Bounds(settings.SeedCellM)), seedFirst)
		if err != nil {
			slog.Error("E1900 cell seeding failed", "cell", ids[i], "err", err)
			continue
		}
		if res.Created > 0 {
			slog.Info("cell seeded", "cell", ids[i], "target", res.Target, "created", res.Created)
		}
	}
}

// AreaSeedResult is what pre-seeding an area did.
type AreaSeedResult struct {
	Area    string `json:"area"`
	Cells   int    `json:"cells"`
	Created int    `json:"created"`
}

// SeedArea pre-seeds every cell of an area, one transaction per cell. Cells already seeded are
// left alone. adminID 0 means the command line.
func (s *Service) SeedArea(ctx context.Context, adminID int64, areaID string) (AreaSeedResult, error) {
	area, err := s.SeedAreaCheck(ctx, areaID)
	if err != nil {
		return AreaSeedResult{}, err
	}
	settings := s.Settings(ctx)
	out := AreaSeedResult{Area: area.ID}
	box := game.Bounds{MinLat: area.Box.MinLat, MinLon: area.Box.MinLon, MaxLat: area.Box.MaxLat, MaxLon: area.Box.MaxLon}
	for _, c := range game.SeedCellsIn(settings.SeedCellM, box) {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		res, err := s.seedCell(ctx, settings, c.ID(settings.SeedCellM), toBox(c.Bounds(settings.SeedCellM)), seedFirst)
		if err != nil {
			return out, err
		}
		out.Cells++
		out.Created += res.Created
	}
	if adminID > 0 {
		detail, _ := json.Marshal(out)
		err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			return store.LogAdminAction(ctx, tx, adminID, "node.seed-area", area.ID, string(detail))
		})
	}
	slog.Info("area seeded", "area", area.ID, "cells", out.Cells, "created", out.Created)
	return out, err
}

// SeedAreaCheck finds an area and makes sure the map data it needs has been imported.
func (s *Service) SeedAreaCheck(ctx context.Context, areaID string) (store.SeedArea, error) {
	area, err := store.SeedAreaByID(ctx, s.db, areaID)
	if errors.Is(err, store.ErrNotFound) {
		return area, apperr.NotFound.With("seed area")
	}
	if err != nil {
		return area, err
	}
	loaded, err := store.OSMLoaded(ctx, s.db)
	if err != nil {
		return area, err
	}
	if !loaded {
		return area, apperr.SeedNoMap
	}
	return area, nil
}

// SeedAreaIDs lists the areas an admin can pre-seed.
func (s *Service) SeedAreaIDs(ctx context.Context) ([]store.SeedArea, error) {
	return store.SeedAreas(ctx, s.db)
}

// AdminReseedCell clears the seed nodes in the cell holding a point and seeds it again.
func (s *Service) AdminReseedCell(ctx context.Context, adminID int64, lat, lon float64) (SeedCellResult, error) {
	if !game.ValidCoordinate(lat, lon) {
		return SeedCellResult{}, apperr.BadRequest.With("coordinate")
	}
	loaded, err := store.OSMLoaded(ctx, s.db)
	if err != nil {
		return SeedCellResult{}, err
	}
	if !loaded {
		return SeedCellResult{}, apperr.SeedNoMap
	}
	settings := s.Settings(ctx)
	cell := game.SeedCellAt(settings.SeedCellM, lat, lon)
	res, err := s.seedCell(ctx, settings, cell.ID(settings.SeedCellM), toBox(cell.Bounds(settings.SeedCellM)), seedReseed)
	if err != nil {
		return res, err
	}
	detail, _ := json.Marshal(res)
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		return store.LogAdminAction(ctx, tx, adminID, "node.reseed-cell", res.CellID, string(detail))
	})
	return res, err
}

// SeedStatus is the admin readout for the cell a player stands in.
type SeedStatus struct {
	OSMLoaded  bool   `json:"osmLoaded"`
	CellID     string `json:"cellId"`
	Seeded     bool   `json:"seeded"`
	Target     int    `json:"target"`
	SeedNodes  int    `json:"seedNodes"`
	WalkM      int    `json:"walkM"`
	TotalCells int    `json:"totalCells"`
	TotalNodes int    `json:"totalNodes"`
}

func (s *Service) AdminSeedStatus(ctx context.Context, lat, lon float64) (SeedStatus, error) {
	if !game.ValidCoordinate(lat, lon) {
		return SeedStatus{}, apperr.BadRequest.With("coordinate")
	}
	settings := s.Settings(ctx)
	cell := game.SeedCellAt(settings.SeedCellM, lat, lon)
	out := SeedStatus{CellID: cell.ID(settings.SeedCellM)}
	var err error
	if out.OSMLoaded, err = store.OSMLoaded(ctx, s.db); err != nil {
		return out, err
	}
	row, ok, err := store.SeedCellInfo(ctx, s.db, out.CellID)
	if err != nil {
		return out, err
	}
	out.Seeded = ok && row.SeededAt != nil
	out.Target, out.WalkM = row.Target, int(row.WalkM)
	if out.SeedNodes, err = store.CountSeedNodesInCell(ctx, s.db, toBox(cell.Bounds(settings.SeedCellM)), SeedSource); err != nil {
		return out, err
	}
	out.TotalCells, out.TotalNodes, err = store.SeedTotals(ctx, s.db, SeedSource)
	return out, err
}

// seedTick is the world tick's seeding work: end rare nodes, top cells up, maybe place a rare one.
func (s *Service) seedTick(ctx context.Context, settings game.Settings, tick time.Duration) {
	if n, err := store.RetireEndedNodes(ctx, s.db, RareSource, settings.NodeDepletedReserve); err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 rare node cleanup failed", "err", err)
		}
		return
	} else if n > 0 {
		slog.Info("rare nodes ended", "count", n)
	}
	if settings.SeedPerCell <= 0 {
		return
	}
	cells, err := store.CellsToTopUp(ctx, s.db, SeedSource, settings.SeedTopUpMinutes, settings.SeedTopUpPerTick)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 seed top-up failed", "err", err)
		}
		return
	}
	for _, c := range cells {
		if ctx.Err() != nil {
			return
		}
		res, err := s.seedCell(ctx, settings, c.ID, c.Box, seedTopUp)
		if err != nil {
			slog.Error("E1900 seed top-up failed", "cell", c.ID, "err", err)
			continue
		}
		if res.Created > 0 {
			slog.Info("cell topped up", "cell", c.ID, "created", res.Created)
		}
	}
	s.maybeSpawnRare(ctx, settings, tick)
	s.groupNodeTick(ctx, settings, tick)
}

// groupNodeTick ends expired or spent group nodes and, at GroupNodeRatePerCell, places a new one on a random
// seeded spot (walk-together-plan.md X2): more duo than quad, lasting GroupNodeHours.
func (s *Service) groupNodeTick(ctx context.Context, settings game.Settings, tick time.Duration) {
	if n, err := store.RetireEndedNodes(ctx, s.db, GroupSource, settings.NodeDepletedReserve); err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 group node cleanup failed", "err", err)
		}
		return
	} else if n > 0 {
		slog.Info("group nodes ended", "count", n)
	}
	count, cell, ok, err := store.SeededLandCells(ctx, s.db)
	if err != nil || !ok {
		return
	}
	if rand.Float64() >= game.GroupNodeChance(settings, count, tick) {
		return
	}
	size := game.PickGroupNodeSize(settings, rand.Float64())
	if size == 0 {
		return
	}
	expires := s.now().Add(time.Duration(settings.GroupNodeHours * float64(time.Hour)))
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		_, err := s.placeNodes(ctx, tx, settings, cell.Box, 1, func(kind game.SeedKind) store.NewNode {
			return store.NewNode{
				Name:     game.GroupNodeName(size) + " " + game.SeedName(kind.Label, rand.IntN(26), rand.IntN(100)),
				Resource: kind.Resource, Yield: kind.Yield, RegenPerHour: 0,
				Source: GroupSource, SourceRef: cell.ID, ExpiresAt: &expires, GroupSize: size,
			}
		})
		return err
	})
	if err != nil {
		slog.Error("E1900 group node failed", "cell", cell.ID, "err", err)
		return
	}
	slog.Info("group node placed", "cell", cell.ID, "size", size, "until", expires)
}

// maybeSpawnRare places a rare roaming node at a random seeded spot, at RarePerKm2Day on average.
func (s *Service) maybeSpawnRare(ctx context.Context, settings game.Settings, tick time.Duration) {
	count, cell, ok, err := store.SeededLandCells(ctx, s.db)
	if err != nil || !ok {
		if err != nil && ctx.Err() == nil {
			slog.Error("E1900 rare node failed", "err", err)
		}
		return
	}
	km2 := float64(count) * settings.SeedCellM * settings.SeedCellM / 1e6
	if rand.Float64() >= game.RareChance(settings, km2, tick) {
		return
	}
	expires := s.now().Add(game.RareLifetime(settings, rand.Float64()))
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		_, err := s.placeNodes(ctx, tx, settings, cell.Box, 1, func(kind game.SeedKind) store.NewNode {
			return store.NewNode{
				Name:     "Rare " + game.SeedName(kind.Label, rand.IntN(26), rand.IntN(100)),
				Resource: kind.Resource, Yield: game.RareYield(settings, kind),
				RegenPerHour: settings.SeedRegenPerHour, Source: RareSource, SourceRef: cell.ID, ExpiresAt: &expires,
			}
		})
		return err
	})
	if err != nil {
		slog.Error("E1900 rare node failed", "cell", cell.ID, "err", err)
		return
	}
	slog.Info("rare node placed", "cell", cell.ID, "until", expires)
}

// ImportOSM replaces the seeder's map layers with an extract made by deploy/maps/osm_extract.py.
func (s *Service) ImportOSM(ctx context.Context, r io.Reader) (land, walk, blocked int, err error) {
	var file struct {
		Land    []store.OSMFeature `json:"land"`
		Walk    []store.OSMFeature `json:"walk"`
		Blocked []store.OSMFeature `json:"blocked"`
	}
	if err := json.NewDecoder(r).Decode(&file); err != nil {
		return 0, 0, 0, fmt.Errorf("read extract: %w", err)
	}
	if len(file.Land) == 0 || len(file.Walk) == 0 {
		return 0, 0, 0, fmt.Errorf("extract has %d land and %d walkway shapes; refusing to replace the map", len(file.Land), len(file.Walk))
	}
	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		return store.ReplaceOSM(ctx, tx, file.Land, file.Walk, file.Blocked)
	})
	return len(file.Land), len(file.Walk), len(file.Blocked), err
}
