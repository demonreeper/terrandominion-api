package game

import (
	"math"
	"testing"
	"time"
)

func TestSeedCellIsSquareOnTheGround(t *testing.T) {
	const cellM = 250
	c := SeedCellAt(cellM, 4.1755, 73.5093)
	b := c.Bounds(cellM)
	if 4.1755 < b.MinLat || 4.1755 >= b.MaxLat || 73.5093 < b.MinLon || 73.5093 >= b.MaxLon {
		t.Fatalf("the point should be inside its own cell, got %+v", b)
	}
	midLat, midLon := b.Centre()
	height := DistanceM(b.MinLat, midLon, b.MaxLat, midLon)
	width := DistanceM(midLat, b.MinLon, midLat, b.MaxLon)
	if math.Abs(height-cellM) > 2 || math.Abs(width-cellM) > 2 {
		t.Fatalf("expected a 250 m square, got %.1f x %.1f", width, height)
	}
	if SeedCellAt(cellM, midLat, midLon) != c {
		t.Fatal("the centre should map back to the same cell")
	}
	if c.ID(cellM) != SeedCellAt(cellM, b.MinLat+1e-7, b.MinLon+1e-7).ID(cellM) {
		t.Fatal("the same cell should always have the same id")
	}
}

func TestSeedCellsAroundNearestFirst(t *testing.T) {
	const cellM = 250
	cells := SeedCellsAround(cellM, 4.1755, 73.5093, 300)
	if len(cells) < 4 || len(cells) > 16 {
		t.Fatalf("a 300 m scan should touch a handful of cells, got %d", len(cells))
	}
	if cells[0] != SeedCellAt(cellM, 4.1755, 73.5093) {
		t.Fatal("the cell the player stands in should come first")
	}
	seen := map[SeedCell]bool{}
	for _, c := range cells {
		if seen[c] {
			t.Fatalf("cell %v listed twice", c)
		}
		seen[c] = true
	}
}

func TestSeedCellsInCoversTheBox(t *testing.T) {
	const cellM = 250
	box := Bounds{4.1660, 73.4990, 4.1815, 73.5245}
	cells := SeedCellsIn(cellM, box)
	// About 1.7 km by 2.8 km: 7 or 8 rows of 11 or 12 columns.
	if len(cells) < 70 || len(cells) > 110 {
		t.Fatalf("unexpected cell count %d", len(cells))
	}
	for _, p := range [][2]float64{{box.MinLat, box.MinLon}, {box.MaxLat, box.MaxLon}, {4.1755, 73.5093}} {
		want := SeedCellAt(cellM, p[0], p[1])
		found := false
		for _, c := range cells {
			found = found || c == want
		}
		if !found {
			t.Fatalf("cell for %v missing", p)
		}
	}
}

func TestSeedTargetFollowsThePath(t *testing.T) {
	s := DefaultSettings()
	cases := map[float64]int{0: 0, 39: 0, 40: 1, 119: 2, 160: 4, 5000: 4}
	for walk, want := range cases {
		if got := SeedTarget(s, walk); got != want {
			t.Errorf("%.0f m of path: want %d, got %d", walk, want, got)
		}
	}
}

func TestPickSeedKindFollowsTheWeights(t *testing.T) {
	kinds := SeedKinds(DefaultSettings())
	counts := map[string]int{}
	const steps = 1000
	for i := 0; i < steps; i++ {
		counts[PickSeedKind(kinds, float64(i)/steps).Resource]++
	}
	if counts["crystal"] != 400 || counts["alloy"] != 300 || counts["data"] != 200 || counts["credit"] != 100 {
		t.Fatalf("expected 4:3:2:1, got %v", counts)
	}
}

func TestPickSeedSpotsKeepsTheGap(t *testing.T) {
	// Points 10 m apart along a street: with a 25 m gap every third one is kept.
	var cands []Spot
	for i := 0; i < 12; i++ {
		cands = append(cands, Spot{4.1755 + float64(i)*10/metresPerDegLat, 73.5093})
	}
	got := PickSeedSpots(cands, 4, 25)
	if len(got) != 4 {
		t.Fatalf("expected 4 spots, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if d := DistanceM(got[i-1].Lat, got[i-1].Lon, got[i].Lat, got[i].Lon); d < 25 {
			t.Fatalf("spots %d and %d only %.1f m apart", i-1, i, d)
		}
	}
	if len(PickSeedSpots(cands[:2], 4, 25)) != 1 {
		t.Fatal("two points 10 m apart can hold only one node")
	}
}

func TestRareNodes(t *testing.T) {
	s := DefaultSettings()
	// 4 km² for a whole day of 1-minute ticks should give about one rare node.
	perDay := RareChance(s, 4, time.Minute) * 24 * 60
	if math.Abs(perDay-1) > 1e-9 {
		t.Fatalf("expected 1 rare node a day on 4 km², got %v", perDay)
	}
	if RareChance(s, 0, time.Minute) != 0 {
		t.Fatal("no seeded land, no rare nodes")
	}
	if RareLifetime(s, 0) != 2*time.Hour || RareLifetime(s, 0.5) != 3*time.Hour {
		t.Fatal("rare nodes should last 2 to 4 hours")
	}
	if RareYield(s, SeedKinds(s)[0]) != 36 {
		t.Fatal("a rare crystal node should yield 3x 12")
	}
}

func TestSeedName(t *testing.T) {
	if got := SeedName("Crystal Seam", 10, 7); got != "Crystal Seam K-07" {
		t.Fatalf("got %q", got)
	}
}
