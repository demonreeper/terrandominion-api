package game

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// The app greys DRONE out with the same rule (TetherPanelView.DroneOutOfRange), so a launch it
// allows is never refused for range: the bay's range plus the fix's accuracy, at most double.
func TestDroneRangeMkI(t *testing.T) {
	s := DefaultSettings()
	const rangeM = 150
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	from := func(metresNorth, accuracy float64) Fix {
		return Fix{Lat: 4.1755 + metresNorth/111320, Lon: 73.5093, AccuracyM: accuracy, At: at}
	}
	cases := []struct {
		name     string
		metres   float64
		accuracy float64
		ok       bool
	}{
		{"well inside", 100, 6, true},
		{"just past range, within accuracy", 154, 6, true},
		{"past range and accuracy", 160, 6, false},
		{"plan example, 220 m", 220, 6, false},
		{"accuracy can't more than double the range", 301, 50, false},
	}
	for _, c := range cases {
		_, err := CheckInRange(s, from(c.metres, c.accuracy), nil, 4.1755, 73.5093, rangeM)
		if (err == nil) != c.ok {
			t.Errorf("%s: got %v", c.name, err)
		}
		if err != nil && c.accuracy <= s.HarvestMaxAccuracyM && err.Code != apperr.OutOfRange.Code {
			t.Errorf("%s: expected out of range, got %v", c.name, err)
		}
	}
}
