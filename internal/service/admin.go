package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

func (s *Service) PublicSettings(ctx context.Context) ([]store.Setting, error) {
	return store.ListSettings(ctx, s.db, true)
}

func (s *Service) AdminSettings(ctx context.Context) ([]store.Setting, error) {
	return store.ListSettings(ctx, s.db, false)
}

// AdminUpdateSetting changes one tunable value and records who changed it and from what.
func (s *Service) AdminUpdateSetting(ctx context.Context, adminID int64, key string, value float64) error {
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		ok, err := store.UpdateSetting(ctx, tx, key, value, adminID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.NotFound.With("setting %q", key)
		}
		detail, _ := json.Marshal(map[string]any{"value": value})
		return store.LogAdminAction(ctx, tx, adminID, "setting.update", key, string(detail))
	})
	if err == nil {
		s.invalidateSettings()
	}
	return err
}

var resources = map[string]bool{"crystal": true, "alloy": true, "data": true, "credit": true}

// AdminCreateNode places a node by hand, the way places are seeded in MonoCity.
func (s *Service) AdminCreateNode(ctx context.Context, adminID int64, n store.NewNode) (int64, error) {
	n.Name = strings.TrimSpace(n.Name)
	switch {
	case n.Name == "" || len(n.Name) > 80:
		return 0, apperr.BadRequest.With("node name")
	case !resources[n.Resource]:
		return 0, apperr.BadRequest.With("resource %q", n.Resource)
	case !game.ValidCoordinate(n.Lat, n.Lon):
		return 0, apperr.BadRequest.With("coordinate")
	case n.Yield <= 0 || n.Yield > 1000:
		return 0, apperr.BadRequest.With("yield")
	}
	if n.RegenPerHour <= 0 {
		n.RegenPerHour = 0.05
	}

	var id int64
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		id, err = store.CreateNode(ctx, tx, n, adminID)
		if err != nil {
			return err
		}
		detail, _ := json.Marshal(n)
		return store.LogAdminAction(ctx, tx, adminID, "node.create", n.Name, string(detail))
	})
	return id, err
}

// Test nodes ("scatter") ------------------------------------------------------------------

// ScatterSource marks nodes made by AdminScatterNodes, so they can all be cleared at once.
const ScatterSource = "scatter"

type scatterKind struct {
	resource string
	label    string
	yield    int
	weight   int
}

// Crystal is the common find, credit the rare one, matching what the client expects to see.
var scatterKinds = []scatterKind{
	{"crystal", "Crystal Seam", 12, 4},
	{"alloy", "Alloy Cache", 10, 3},
	{"data", "Data Relay", 6, 2},
	{"credit", "Credit Drop", 8, 1},
}

type ScatterResult struct {
	Created int     `json:"created"`
	IDs     []int64 `json:"ids"`
}

// AdminScatterNodes drops test nodes at random around a point, for trying harvesting before
// real nodes are placed. Spread evenly over the disc, at least minGap metres apart where
// possible, with fast regrowth so a tester is not stuck waiting.
func (s *Service) AdminScatterNodes(ctx context.Context, adminID int64, lat, lon float64, count int,
	radiusM float64) (ScatterResult, error) {
	switch {
	case !game.ValidCoordinate(lat, lon):
		return ScatterResult{}, apperr.BadRequest.With("coordinate")
	case count < 1 || count > 40:
		return ScatterResult{}, apperr.BadRequest.With("count must be 1 to 40")
	case radiusM < 30 || radiusM > 3000:
		return ScatterResult{}, apperr.BadRequest.With("radiusM must be 30 to 3000")
	}

	totalWeight := 0
	for _, k := range scatterKinds {
		totalWeight += k.weight
	}
	minGap := math.Min(25, radiusM/math.Sqrt(float64(count))/2)
	metresPerDegLat := 111320.0
	metresPerDegLon := 111320.0 * math.Cos(lat*math.Pi/180)

	type point struct{ x, y float64 }
	placed := make([]point, 0, count)
	result := ScatterResult{IDs: make([]int64, 0, count)}

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		for i := 0; i < count; i++ {
			// Uniform over the disc (sqrt of the radius), retrying a few times to keep a gap.
			var p point
			for attempt := 0; attempt < 30; attempt++ {
				r := radiusM * math.Sqrt(rand.Float64())
				a := rand.Float64() * 2 * math.Pi
				p = point{r * math.Cos(a), r * math.Sin(a)}
				clear := true
				for _, q := range placed {
					if math.Hypot(p.x-q.x, p.y-q.y) < minGap {
						clear = false
						break
					}
				}
				if clear {
					break
				}
			}
			placed = append(placed, p)

			pick := rand.IntN(totalWeight)
			kind := scatterKinds[0]
			for _, k := range scatterKinds {
				if pick < k.weight {
					kind = k
					break
				}
				pick -= k.weight
			}

			node := store.NewNode{
				Name:         fmt.Sprintf("%s %c-%02d", kind.label, 'A'+rand.IntN(26), rand.IntN(100)),
				Resource:     kind.resource,
				Lat:          lat + p.y/metresPerDegLat,
				Lon:          lon + p.x/metresPerDegLon,
				Yield:        kind.yield,
				RegenPerHour: 0.5,
				Source:       ScatterSource,
			}
			id, err := store.CreateNode(ctx, tx, node, adminID)
			if err != nil {
				return err
			}
			result.IDs = append(result.IDs, id)
		}
		detail, _ := json.Marshal(map[string]any{"lat": lat, "lon": lon, "count": count, "radiusM": radiusM})
		return store.LogAdminAction(ctx, tx, adminID, "node.scatter", fmt.Sprintf("%d nodes", count), string(detail))
	})
	result.Created = len(result.IDs)
	return result, err
}

// AdminClearScatteredNodes retires every test node made by AdminScatterNodes.
func (s *Service) AdminClearScatteredNodes(ctx context.Context, adminID int64) (int64, error) {
	var n int64
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		n, err = store.RetireNodesBySource(ctx, tx, ScatterSource)
		if err != nil {
			return err
		}
		return store.LogAdminAction(ctx, tx, adminID, "node.clear-scatter", fmt.Sprintf("%d nodes", n), "{}")
	})
	return n, err
}
