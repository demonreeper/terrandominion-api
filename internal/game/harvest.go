package game

import (
	"math"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// Fix is a position the phone reported, with its accuracy radius in metres.
type Fix struct {
	Lat, Lon  float64
	AccuracyM float64
	At        time.Time
}

// HarvestInput is everything the harvest rule needs, loaded by the service in one transaction.
type HarvestInput struct {
	Settings Settings

	Player  Fix
	NodeLat float64
	NodeLon float64

	NodeYield   int
	NodeReserve float64 // 0..1

	ExtractorYieldMult float64
	CarryCapacity      int64
	CarriedTotal       int64

	// The player's previous harvest at this node, and their previous harvest anywhere.
	LastAtNode *time.Time
	LastAnyFix *Fix
}

type HarvestResult struct {
	Amount      int
	DistanceM   float64
	ReserveCost float64
}

// EvaluateHarvest decides whether a harvest succeeds and how much it yields. Order matters: the
// cheapest and most telling checks (GPS quality, distance, plausibility) come first, so a
// spoofed request learns as little as possible about the node.
func EvaluateHarvest(in HarvestInput) (HarvestResult, *apperr.Error) {
	s := in.Settings
	var out HarvestResult

	if in.Player.AccuracyM <= 0 || in.Player.AccuracyM > s.HarvestMaxAccuracyM {
		return out, apperr.HarvestGPSWeak
	}

	out.DistanceM = DistanceM(in.Player.Lat, in.Player.Lon, in.NodeLat, in.NodeLon)
	// Give the player the benefit of their GPS error, capped so a bad fix cannot stretch the range.
	allowance := math.Min(in.Player.AccuracyM, s.HarvestRangeM)
	if out.DistanceM > s.HarvestRangeM+allowance {
		return out, apperr.HarvestTooFar
	}

	if err := checkTravel(s, in.LastAnyFix, in.Player); err != nil {
		return out, err
	}

	if in.LastAtNode != nil && s.HarvestCooldownSec > 0 &&
		in.Player.At.Sub(*in.LastAtNode) < time.Duration(s.HarvestCooldownSec)*time.Second {
		return out, apperr.HarvestCooldown
	}

	if in.NodeReserve < 0.01 {
		return out, apperr.HarvestDepleted
	}

	room := in.CarryCapacity - in.CarriedTotal
	if room <= 0 {
		return out, apperr.HarvestCarryFull
	}

	amount := harvestAmount(in.NodeYield, in.ExtractorYieldMult, in.NodeReserve)
	if int64(amount) > room {
		amount = int(room)
	}

	out.Amount = amount
	out.ReserveCost = math.Min(in.NodeReserve, s.HarvestReserveCost)
	return out, nil
}

// harvestAmount is what one harvest yields. A drained node gives less: full yield above half
// reserve, tapering to a third near empty.
func harvestAmount(yield int, mult, reserve float64) int {
	if mult <= 0 {
		mult = 1
	}
	richness := math.Min(1, 0.33+reserve*1.34)
	amount := int(math.Round(float64(yield) * mult * richness))
	if amount < 1 {
		amount = 1
	}
	return amount
}

// checkTravel rejects a fix the player could not have reached since their last one.
func checkTravel(s Settings, last *Fix, now Fix) *apperr.Error {
	if last == nil || s.MaxTravelSpeedKmh <= 0 {
		return nil
	}
	hours := now.At.Sub(last.At).Hours()
	travelled := DistanceM(last.Lat, last.Lon, now.Lat, now.Lon)
	// Ignore jumps inside the combined GPS error; they are noise, not travel.
	travelled -= last.AccuracyM + now.AccuracyM
	if travelled > 0 && (hours <= 0 || travelled/1000/hours > s.MaxTravelSpeedKmh) {
		return apperr.HarvestImplausible.With("travelled %.0fm in %.0fs", travelled, hours*3600)
	}
	return nil
}

// DistanceM is the great-circle distance in metres. PostGIS does the heavy lifting for queries;
// this is for rule checks on a handful of points.
func DistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371008.8
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ValidCoordinate rejects impossible input before it reaches a query.
func ValidCoordinate(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
