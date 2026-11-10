package game

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// The builders here give a numbered error its player-facing sentence and the numbers behind it
// (error "data"), so the app can show the message as it is or say it its own way.

// resourceOrder is the order resources are named in, the same as the app's top bar.
var resourceOrder = map[string]int{"crystal": 0, "alloy": 1, "data": 2, "credit": 3}

// sortedResources lists a map's resources in display order, unknown ones last by name.
func sortedResources(amounts map[string]int64) []string {
	keys := make([]string, 0, len(amounts))
	for resource := range amounts {
		keys = append(keys, resource)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, aok := resourceOrder[keys[i]]
		b, bok := resourceOrder[keys[j]]
		switch {
		case aok && bok:
			return a < b
		case aok != bok:
			return aok
		}
		return keys[i] < keys[j]
	})
	return keys
}

// joinList joins phrases as "a", "a and b" or "a, b and c".
func joinList(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// ShortError says what a cost is still missing: {"short": {"alloy": 30}}.
func ShortError(short map[string]int64) *apperr.Error {
	parts := make([]string, 0, len(short))
	data := make(map[string]int64, len(short))
	for _, resource := range sortedResources(short) {
		parts = append(parts, fmt.Sprintf("%d more %s", short[resource], resource))
		data[resource] = short[resource]
	}
	return apperr.TetherCost.Say("You need %s.", joinList(parts)).WithData(map[string]any{"short": data})
}

// RangeError says how far the player is and how close they must be: {"distanceM", "rangeM"}.
// The distance is rounded up, so it never reads as inside the range. target names what to move
// closer to, or is empty.
func RangeError(base *apperr.Error, distanceM, rangeM float64, target string) *apperr.Error {
	distance, limit := int(math.Ceil(distanceM)), int(math.Round(rangeM))
	where := ""
	if target != "" {
		where = " of " + target
	}
	return base.Say("Move within %d m%s. You are %d m away.", limit, where, distance).
		WithData(map[string]any{"distanceM": distance, "rangeM": limit})
}

// GPSWeakError says how good a fix has to be: {"accuracyM", "maxAccuracyM"}.
func GPSWeakError(s Settings, accuracyM float64) *apperr.Error {
	limit := int(math.Round(s.HarvestMaxAccuracyM))
	return apperr.HarvestGPSWeak.Say("GPS signal too weak. Wait for a fix within %d m.", limit).
		WithData(map[string]any{"accuracyM": int(math.Ceil(math.Max(0, accuracyM))), "maxAccuracyM": limit})
}

// CellsError says how many energy cells an action needs: {"need", "have"}.
func CellsError(need, have int) *apperr.Error {
	return apperr.NoCells.Say("You need %s. You have %d.", plural(need, "energy cell", "energy cells"), have).
		WithData(map[string]any{"need": need, "have": have})
}

// LevelError says what level an action needs: {"needLevel", "level"}.
func LevelError(needLevel, level int) *apperr.Error {
	return apperr.FactionLevelLow.Say("You need level %d for that. You are level %d.", needLevel, level).
		WithData(map[string]any{"needLevel": needLevel, "level": level})
}

// CooldownError says when a node can be harvested again: {"readyInSec"}.
func CooldownError(readyInSec int) *apperr.Error {
	return apperr.HarvestCooldown.Say("This node is recharging for you. Ready in %s.",
		FormatDuration(time.Duration(readyInSec)*time.Second)).
		WithData(map[string]any{"readyInSec": readyInSec})
}

// SpacingError says how far apart Tether Nodes must stand: {"spacingM"}.
func SpacingError(spacingM float64) *apperr.Error {
	spacing := int(math.Round(spacingM))
	return apperr.TetherTooClose.Say("Too close to another Tether Node. Build at least %d m from it.", spacing).
		WithData(map[string]any{"spacingM": spacing})
}

// CapError says how many Tether Nodes a player may own: {"max"}.
func CapError(max int) *apperr.Error {
	return apperr.TetherCap.Say("You already own %s, the most you can.", plural(max, "Tether Node", "Tether Nodes")).
		WithData(map[string]any{"max": max})
}

// ItemError says which item an action needs: {"item"}.
func ItemError(itemID, name string) *apperr.Error {
	return apperr.ItemMissing.Say("You need %s %s. Craft one in Inventory.", article(name), name).
		WithData(map[string]any{"item": itemID})
}

// FormatDuration writes a duration the way messages show it: "45 s", "4 min 10 s", "2 h 5 min".
// It rounds up to the second, so a wait never reads shorter than it is.
func FormatDuration(d time.Duration) string {
	total := int(math.Ceil(d.Seconds()))
	if total < 0 {
		total = 0
	}
	h, m, sec := total/3600, total%3600/60, total%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d h %d min", h, m)
	case h > 0:
		return fmt.Sprintf("%d h", h)
	case m > 0 && sec > 0:
		return fmt.Sprintf("%d min %d s", m, sec)
	case m > 0:
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d s", sec)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func article(name string) string {
	if name != "" && strings.ContainsRune("AEIOUaeiou", rune(name[0])) {
		return "an"
	}
	return "a"
}
