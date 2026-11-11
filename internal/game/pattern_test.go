package game

import (
	"math/rand"
	"testing"
	"time"
)

func trail(n int, step func(i int) Fix) []Fix {
	out := make([]Fix, n)
	for i := range out {
		out[i] = step(i)
	}
	return out
}

func TestPatternFinding(t *testing.T) {
	at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	frozen := trail(30, func(i int) Fix {
		return Fix{Lat: 4.1755, Lon: 73.5093, AccuracyM: 5, At: at.Add(time.Duration(i) * 10 * time.Second)}
	})
	if PatternFinding(frozen, 30) == "" {
		t.Error("the same position and accuracy for five minutes is a pattern")
	}
	joystick := trail(30, func(i int) Fix {
		return Fix{Lat: 4.1755 + float64(i)*0.00005, Lon: 73.5093, AccuracyM: 3 + float64(i%3), At: at.Add(time.Duration(i) * 5 * time.Second)}
	})
	if PatternFinding(joystick, 30) == "" {
		t.Error("a dead-straight line at an exact speed is a pattern")
	}
	r := rand.New(rand.NewSource(7))
	walk := trail(30, func(i int) Fix {
		return Fix{Lat: 4.1755 + float64(i)*0.00004 + r.Float64()*0.00002, Lon: 73.5093 + r.Float64()*0.00003,
			AccuracyM: 4 + r.Float64()*6, At: at.Add(time.Duration(i) * 5 * time.Second)}
	})
	if got := PatternFinding(walk, 30); got != "" {
		t.Errorf("a real walk is no pattern, got %q", got)
	}
	standing := trail(30, func(i int) Fix {
		return Fix{Lat: 4.1755 + r.Float64()*0.00003, Lon: 73.5093 + r.Float64()*0.00003, AccuracyM: 8 + r.Float64()*4,
			At: at.Add(time.Duration(i) * 10 * time.Second)}
	})
	if got := PatternFinding(standing, 30); got != "" {
		t.Errorf("standing with GPS jitter is no pattern, got %q", got)
	}
	if PatternFinding(frozen[:20], 30) != "" {
		t.Error("too few fixes: nothing to say")
	}
}
