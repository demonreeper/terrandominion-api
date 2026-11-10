package game

import (
	"math"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// The weapons a player can use on a Tether Node. Their ids are also their recipe and item ids.
const (
	WeaponEmPulse = "em_pulse"
	WeaponJammer  = "signal_jammer"
	WeaponSpike   = "overload_spike"
	ItemCell      = "energy_cell"
)

// Weapon is what one use of a weapon does and costs.
type Weapon struct {
	ID     string
	Name   string
	Cells  int
	Damage float64
	Jam    time.Duration
	Fuse   time.Duration
}

// WeaponByID prices a weapon from the settings; ok is false for an unknown id.
func WeaponByID(s Settings, id string) (Weapon, bool) {
	switch id {
	case WeaponEmPulse:
		return Weapon{ID: id, Name: "EM Pulse", Cells: s.CellsEmPulse, Damage: s.EmPulseDamage}, true
	case WeaponJammer:
		return Weapon{ID: id, Name: "Signal Jammer", Cells: s.CellsJammer,
			Jam: time.Duration(s.JammerMinutes * float64(time.Minute))}, true
	case WeaponSpike:
		return Weapon{ID: id, Name: "Overload Spike", Cells: s.CellsSpike, Damage: s.SpikeDamage,
			Fuse: time.Duration(s.SpikeFuseMinutes * float64(time.Minute))}, true
	}
	return Weapon{}, false
}

// CraftableGearIDs is the gear made from nothing on the Craft tab. The rig and shield join it when
// they do something.
var CraftableGearIDs = []string{"drone_bay_mk1"}

// DroneFlight is how long a strike over this distance takes.
func DroneFlight(s Settings, distanceM float64) time.Duration {
	seconds := s.DroneMinFlightSec
	if s.DroneSpeedMps > 0 {
		seconds = math.Max(seconds, distanceM/s.DroneSpeedMps)
	}
	return time.Duration(seconds * float64(time.Second))
}

// CheckInRange applies the harvest's checks (GPS quality, distance, travel speed) at a range of
// the caller's choosing, and reports the distance.
func CheckInRange(s Settings, player Fix, last *Fix, lat, lon, rangeM float64) (float64, *apperr.Error) {
	if player.AccuracyM <= 0 || player.AccuracyM > s.HarvestMaxAccuracyM {
		return 0, GPSWeakError(s, player.AccuracyM)
	}
	distance := DistanceM(player.Lat, player.Lon, lat, lon)
	if distance > rangeM+math.Min(player.AccuracyM, rangeM) {
		return distance, RangeError(apperr.OutOfRange, distance, rangeM, "")
	}
	return distance, checkTravel(s, last, player)
}

// ContestSide is what one side brings to a node contest.
type ContestSide struct {
	Level     int
	GearTiers int
	Allies    int
	// Showed is true for a defender who came to the node during the contest.
	Showed bool
}

// ContestScore is gear power + level + allies within range.
func ContestScore(s Settings, side ContestSide) int {
	score := side.Level + side.GearTiers*s.ContestGearPerTier + side.Allies*s.ContestAllyPoints
	if side.Showed {
		score += s.ContestJoinPoints
	}
	return score
}

// ChallengerWins decides a contest. A tie goes to the defender: the challenger has to take it.
func ChallengerWins(challenger, defender int) bool {
	return challenger > defender
}
