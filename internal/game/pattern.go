package game

import (
	"fmt"
	"math"
)

// Movement patterns (docs/social-signal-plan.md Part C, D3): real GPS wobbles, its accuracy changes, and people
// don't walk dead-straight at an exact speed. Fake-location apps often do all three. A pattern is only ever a
// report for a human: phones indoors can repeat a position, so it never locks.

// PatternFinding names what the fixes show, "" when nothing.
func PatternFinding(fixes []Fix, need int) string {
	if need < 10 {
		need = 10
	}
	if len(fixes) < need {
		return ""
	}
	fixes = fixes[len(fixes)-need:]
	if sameAccuracy(fixes) && samePlace(fixes) {
		return fmt.Sprintf("the same position and accuracy on %d fixes in a row (no jitter at all)", len(fixes))
	}
	if sameAccuracy(fixes) && moving(fixes) {
		return fmt.Sprintf("the same accuracy (%.1f m) on %d moving fixes", fixes[0].AccuracyM, len(fixes))
	}
	if run := straightRun(fixes); run >= 10 {
		return fmt.Sprintf("a dead-straight line at a constant speed over %d fixes", run)
	}
	return ""
}

func sameAccuracy(fixes []Fix) bool {
	for _, f := range fixes[1:] {
		if math.Abs(f.AccuracyM-fixes[0].AccuracyM) > 0.01 {
			return false
		}
	}
	return true
}

func samePlace(fixes []Fix) bool {
	for _, f := range fixes[1:] {
		if math.Abs(f.Lat-fixes[0].Lat) > 1e-7 || math.Abs(f.Lon-fixes[0].Lon) > 1e-7 {
			return false
		}
	}
	return fixes[len(fixes)-1].At.Sub(fixes[0].At).Minutes() >= 2
}

func moving(fixes []Fix) bool {
	return DistanceM(fixes[0].Lat, fixes[0].Lon, fixes[len(fixes)-1].Lat, fixes[len(fixes)-1].Lon) > 50
}

// straightRun is the longest run of steps (walking pace or faster) whose bearing stays within 1 degree and speed
// within 2% of the run's first step.
func straightRun(fixes []Fix) int {
	best, run := 0, 0
	var bearing0, speed0 float64
	for i := 1; i < len(fixes); i++ {
		a, b := fixes[i-1], fixes[i]
		seconds := b.At.Sub(a.At).Seconds()
		metres := DistanceM(a.Lat, a.Lon, b.Lat, b.Lon)
		if seconds <= 0 || metres/seconds < 1 {
			run = 0
			continue
		}
		bearing, speed := bearingDeg(a, b), metres/seconds
		if run > 0 && angleGap(bearing, bearing0) <= 1 && math.Abs(speed-speed0) <= speed0*0.02 {
			run++
		} else {
			run, bearing0, speed0 = 1, bearing, speed
		}
		if run+1 > best {
			best = run + 1
		}
	}
	return best
}

func bearingDeg(a, b Fix) float64 {
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

func angleGap(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
