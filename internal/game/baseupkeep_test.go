package game

import (
	"testing"
	"time"
)

func TestBaseUpkeep(t *testing.T) {
	s := DefaultSettings()
	now := time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	at := func(d float64) *time.Time { v := now.Add(-time.Duration(d * float64(day))); return &v }
	cases := []struct {
		name    string
		tier    int
		visit   float64
		faded   *time.Time
		dropped *time.Time
		want    UpkeepStep
	}{
		{"visited", 2, 6, nil, nil, UpkeepNone},
		{"a week away", 2, 7, nil, nil, UpkeepFade},
		{"fading 6 days", 3, 13, at(6), nil, UpkeepNone},
		{"fading a week", 3, 14, at(7), nil, UpkeepDropTier},
		{"dropped 3 days ago", 2, 20, at(10), at(3), UpkeepNone},
		{"dropped a week ago", 2, 21, at(14), at(7), UpkeepDropTier},
		{"spire fading 13 days", 1, 20, at(13), nil, UpkeepNone},
		{"spire fading 14 days", 1, 21, at(14), nil, UpkeepCollapse},
	}
	for _, c := range cases {
		if got := BaseUpkeep(c.tier, now.Add(-time.Duration(c.visit*float64(day))), c.faded, c.dropped, now, s); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	if FadedShieldShare(0.6, true, s) != 0.3 || FadedShieldShare(0.6, false, s) != 0.6 {
		t.Error("a faded shield is halved")
	}
}
