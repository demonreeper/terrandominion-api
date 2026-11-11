package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// CheckAppBuild refuses an app build below MinAppBuild (MinAppBuildIOS on iPhones) with E1006.
// build <= 0 (no build sent) passes, so scripts and test players still work.
func (s *Service) CheckAppBuild(ctx context.Context, platform string, build int) error {
	minBuild, allowed := game.AppBuildAllowed(s.Settings(ctx), platform, build)
	if allowed {
		return nil
	}
	return apperr.AppOutdated.WithData(map[string]any{"build": build, "minBuild": minBuild}).
		With("%s build %d below %d", platform, build, minBuild)
}

// CheckMinAppBuildChange vets a new MinAppBuild / MinAppBuildIOS: a whole number of 0 or more, and
// never above the build of the app making the change (for the same platform), so an admin can't
// lock their own phone out. Other keys pass untouched.
func (s *Service) CheckMinAppBuildChange(key string, value float64, platform string, build int) error {
	ios := key == "MinAppBuildIOS"
	if key != "MinAppBuild" && !ios {
		return nil
	}
	if value < 0 || value != math.Trunc(value) {
		return apperr.BadRequest.Say("The minimum build must be a whole number, 0 or more.")
	}
	samePlatform := strings.EqualFold(platform, "ios") == ios
	if build > 0 && samePlatform && int(value) > build {
		return apperr.BadRequest.Say("That would lock out this phone's own build (%d).", build).
			WithData(map[string]any{"build": build})
	}
	return nil
}

func (s *Service) PublicSettings(ctx context.Context) ([]store.Setting, error) {
	return store.ListSettings(ctx, s.db, true)
}

// AdminSettingsView is the admin settings screen: the groups in order, every setting, the last
// changes, and the announcement now running (null when there is none).
type AdminSettingsView struct {
	Groups       []game.SettingGroup   `json:"groups"`
	Settings     []store.Setting       `json:"settings"`
	Changes      []store.SettingChange `json:"changes"`
	Announcement *store.Announcement   `json:"announcement"`
}

const settingChangesShown = 30

func (s *Service) AdminSettings(ctx context.Context) (AdminSettingsView, error) {
	view := AdminSettingsView{Groups: game.SettingGroups}
	var err error
	if view.Settings, err = store.ListSettings(ctx, s.db, false); err != nil {
		return view, err
	}
	for i := range view.Settings {
		view.Settings[i].Group = game.SettingGroupKey(view.Settings[i].Group)
	}
	if view.Changes, err = store.RecentSettingChanges(ctx, s.db, settingChangesShown); err != nil {
		return view, err
	}
	view.Announcement, err = store.CurrentAnnouncement(ctx, s.db)
	return view, err
}

// AdminUpdateSetting changes one tunable value, inside its range and never a locked one, and
// records who changed it, from what and to what. Returns the setting as it now stands.
func (s *Service) AdminUpdateSetting(ctx context.Context, adminID int64, key string, value float64) (store.Setting, error) {
	var updated store.Setting
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		setting, err := store.SettingForUpdate(ctx, tx, key)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("setting %q", key)
		}
		if err != nil {
			return err
		}
		rule := game.SettingRule{Min: setting.Min, Max: setting.Max, Step: setting.Step, Locked: setting.Locked}
		switch game.CheckSettingChange(rule, value) {
		case game.SettingLocked:
			return apperr.SettingLocked.With("setting %q", key)
		case game.SettingRange:
			return apperr.SettingOutOfRange.
				Say("%s takes %s to %s.", settingName(setting), trimNumber(setting.Min), trimNumber(setting.Max)).
				WithData(map[string]any{"min": setting.Min, "max": setting.Max, "step": setting.Step})
		case game.SettingNotWhole:
			return apperr.SettingOutOfRange.Say("%s takes whole numbers.", settingName(setting)).
				WithData(map[string]any{"min": setting.Min, "max": setting.Max, "step": setting.Step})
		}
		if _, err := store.UpdateSetting(ctx, tx, key, value, adminID); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"value": value, "old": setting.Value})
		if err := store.LogAdminAction(ctx, tx, adminID, "setting.update", key, string(detail)); err != nil {
			return err
		}
		updated = setting
		updated.Value = value
		return nil
	})
	if err == nil {
		s.invalidateSettings()
	}
	return updated, err
}

func settingName(setting store.Setting) string {
	if setting.Label != "" {
		return setting.Label
	}
	return setting.Key
}

// trimNumber writes 48 as "48" and 0.35 as "0.35".
func trimNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// AdminSetAnnouncement posts, replaces or (with empty text) removes the one-line announcement on
// everyone's main menu. Returns what is now running, or nil.
func (s *Service) AdminSetAnnouncement(ctx context.Context, adminID int64, text, kind string, hours int) (*store.Announcement, error) {
	if strings.TrimSpace(text) == "" {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if err := store.ClearAnnouncement(ctx, tx); err != nil {
				return err
			}
			return store.LogAdminAction(ctx, tx, adminID, "announcement.clear", "", "{}")
		})
		return nil, err
	}
	clean, ok := game.CleanAnnouncement(text, kind, hours)
	if !ok {
		return nil, apperr.AnnouncementBad
	}
	posted := &store.Announcement{Text: clean, Kind: kind, EndsAt: s.now().Add(time.Duration(hours) * time.Hour)}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.SetAnnouncement(ctx, tx, posted.Text, posted.Kind, posted.EndsAt, adminID); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"text": posted.Text, "kind": posted.Kind, "hours": hours})
		return store.LogAdminAction(ctx, tx, adminID, "announcement.set", "", string(detail))
	})
	if err != nil {
		return nil, err
	}
	return posted, nil
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

	// The same mix and yields as the seeder (game.SeedKinds), with fast regrowth for testing.
	kinds := game.SeedKinds(s.Settings(ctx))
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

			kind := game.PickSeedKind(kinds, rand.Float64())
			node := store.NewNode{
				Name:         game.SeedName(kind.Label, rand.IntN(26), rand.IntN(100)),
				Resource:     kind.Resource,
				Lat:          lat + p.y/metresPerDegLat,
				Lon:          lon + p.x/metresPerDegLon,
				Yield:        kind.Yield,
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
