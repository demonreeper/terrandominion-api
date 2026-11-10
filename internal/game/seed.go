package game

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Node seeding: the world is cut into square cells of SeedCellM metres, each aiming to hold
// SeedPerCell nodes on walkable land. Cells are rows of equal height (latitude bands); each row is
// cut into columns using the cosine of its own centre latitude, so a cell is SeedCellM wide on the
// ground wherever it is. The grid is fixed, so a cell id always means the same patch of ground.

const metresPerDegLat = 111320.0

// SeedCell is one cell of the seeding grid.
type SeedCell struct{ Row, Col int64 }

// Bounds is a cell's box in degrees.
type Bounds struct{ MinLat, MinLon, MaxLat, MaxLon float64 }

func rowCentreLat(cellM float64, row int64) float64 {
	return (float64(row) + 0.5) * cellM / metresPerDegLat
}

func degLonPerCell(cellM float64, row int64) float64 {
	return cellM / (metresPerDegLat * math.Cos(rowCentreLat(cellM, row)*math.Pi/180))
}

// SeedCellAt is the cell holding a point.
func SeedCellAt(cellM, lat, lon float64) SeedCell {
	row := int64(math.Floor(lat * metresPerDegLat / cellM))
	col := int64(math.Floor(lon / degLonPerCell(cellM, row)))
	return SeedCell{row, col}
}

// ID names a cell; it includes the cell size, so changing SeedCellM starts a fresh grid.
func (c SeedCell) ID(cellM float64) string {
	return fmt.Sprintf("%d:%d:%d", int(math.Round(cellM)), c.Row, c.Col)
}

func (c SeedCell) Bounds(cellM float64) Bounds {
	dLon := degLonPerCell(cellM, c.Row)
	return Bounds{
		MinLat: float64(c.Row) * cellM / metresPerDegLat,
		MaxLat: float64(c.Row+1) * cellM / metresPerDegLat,
		MinLon: float64(c.Col) * dLon,
		MaxLon: float64(c.Col+1) * dLon,
	}
}

// Centre is the middle of the cell.
func (b Bounds) Centre() (float64, float64) {
	return (b.MinLat + b.MaxLat) / 2, (b.MinLon + b.MaxLon) / 2
}

// SeedCellsIn lists every cell that overlaps a box, row by row.
func SeedCellsIn(cellM float64, box Bounds) []SeedCell {
	if cellM <= 0 {
		return nil
	}
	first := SeedCellAt(cellM, box.MinLat, box.MinLon).Row
	last := SeedCellAt(cellM, box.MaxLat, box.MinLon).Row
	var out []SeedCell
	for row := first; row <= last; row++ {
		dLon := degLonPerCell(cellM, row)
		for col := int64(math.Floor(box.MinLon / dLon)); col <= int64(math.Floor(box.MaxLon/dLon)); col++ {
			out = append(out, SeedCell{row, col})
		}
	}
	return out
}

// SeedCellsAround lists the cells a scan of radiusM touches, nearest centre first.
func SeedCellsAround(cellM, lat, lon, radiusM float64) []SeedCell {
	dLat := radiusM / metresPerDegLat
	dLon := radiusM / (metresPerDegLat * math.Cos(lat*math.Pi/180))
	cells := SeedCellsIn(cellM, Bounds{lat - dLat, lon - dLon, lat + dLat, lon + dLon})
	type ranked struct {
		cell SeedCell
		d    float64
	}
	list := make([]ranked, 0, len(cells))
	for _, c := range cells {
		b := c.Bounds(cellM)
		// Distance from the point to the nearest edge of the box (0 inside it).
		nearLat := math.Max(b.MinLat, math.Min(lat, b.MaxLat))
		nearLon := math.Max(b.MinLon, math.Min(lon, b.MaxLon))
		d := DistanceM(lat, lon, nearLat, nearLon)
		if d > radiusM {
			continue
		}
		cLat, cLon := b.Centre()
		list = append(list, ranked{c, DistanceM(lat, lon, cLat, cLon)})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].d < list[j].d })
	out := make([]SeedCell, len(list))
	for i, r := range list {
		out[i] = r.cell
	}
	return out
}

// SeedTarget is how many seed nodes a cell with walkM metres of walkable path should hold.
func SeedTarget(s Settings, walkM float64) int {
	target := s.SeedPerCell
	if s.SeedPathPerNodeM > 0 {
		target = min(target, int(math.Floor(walkM/s.SeedPathPerNodeM)))
	}
	return max(0, target)
}

// SeedKind is one kind of node the seeder (and the admin scatter tool) can place.
type SeedKind struct {
	Resource string
	Label    string
	Yield    int
	Weight   int
}

// SeedKinds is the mix: crystal is the common find, credit the rare one.
func SeedKinds(s Settings) []SeedKind {
	return []SeedKind{
		{"crystal", "Crystal Seam", s.SeedYieldCrystal, s.SeedWeightCrystal},
		{"alloy", "Alloy Cache", s.SeedYieldAlloy, s.SeedWeightAlloy},
		{"data", "Data Relay", s.SeedYieldData, s.SeedWeightData},
		{"credit", "Credit Drop", s.SeedYieldCredit, s.SeedWeightCredit},
	}
}

// PickSeedKind picks a kind by weight; r is uniform in [0, 1).
func PickSeedKind(kinds []SeedKind, r float64) SeedKind {
	total := 0
	for _, k := range kinds {
		total += max(0, k.Weight)
	}
	if total == 0 {
		return kinds[0]
	}
	pick := r * float64(total)
	for _, k := range kinds {
		w := float64(max(0, k.Weight))
		if pick < w {
			return k
		}
		pick -= w
	}
	return kinds[len(kinds)-1]
}

// SeedName is a node's display name, such as "Crystal Seam K-07"; letter is 0-25, number 0-99.
func SeedName(label string, letter, number int) string {
	return fmt.Sprintf("%s %c-%02d", label, 'A'+rune(letter%26), number%100)
}

// Spot is a place a node could stand.
type Spot struct{ Lat, Lon float64 }

// PickSeedSpots takes candidates in the order given (the caller shuffles them) and keeps those at
// least gapM from every spot already kept, up to need.
func PickSeedSpots(candidates []Spot, need int, gapM float64) []Spot {
	out := make([]Spot, 0, max(0, need))
	for _, c := range candidates {
		if len(out) >= need {
			break
		}
		clear := true
		for _, p := range out {
			if DistanceM(c.Lat, c.Lon, p.Lat, p.Lon) < gapM {
				clear = false
				break
			}
		}
		if clear {
			out = append(out, c)
		}
	}
	return out
}

// RareChance is the chance that one world tick of length tick spawns a rare node, given how much
// seeded land there is: RarePerKm2Day spread evenly over the day.
func RareChance(s Settings, seededKm2 float64, tick time.Duration) float64 {
	if seededKm2 <= 0 || s.RarePerKm2Day <= 0 {
		return 0
	}
	return math.Min(1, seededKm2*s.RarePerKm2Day*tick.Hours()/24)
}

// RareLifetime is how long a rare node lasts; r is uniform in [0, 1).
func RareLifetime(s Settings, r float64) time.Duration {
	hours := s.RareMinHours + r*math.Max(0, s.RareMaxHours-s.RareMinHours)
	return time.Duration(hours * float64(time.Hour))
}

// RareYield is a rare node's yield per harvest.
func RareYield(s Settings, kind SeedKind) int {
	return max(1, int(math.Round(float64(kind.Yield)*s.RareYieldMult)))
}
