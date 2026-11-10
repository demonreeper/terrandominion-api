package game

import (
	"testing"
	"time"
)

func TestAttackXP(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		name              string
		landed, destroyed bool
		paid, want        int
	}{
		{"hit", true, false, 0, 25},
		{"hit destroys", true, true, 0, 75},
		{"third paying hit", true, false, 2, 25},
		{"cap reached", true, false, 3, 0},
		{"destroy past cap", true, true, 5, 0},
		{"shot down", false, false, 0, 0},
	}
	for _, c := range cases {
		if got := AttackXP(s, c.landed, c.destroyed, c.paid); got != c.want {
			t.Errorf("%s: AttackXP = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestInterceptXP(t *testing.T) {
	s := DefaultSettings()
	if got := InterceptXP(s, 0); got != 20 {
		t.Errorf("InterceptXP(0) = %d, want 20", got)
	}
	if got := InterceptXP(s, 3); got != 0 {
		t.Errorf("InterceptXP(3) = %d, want 0", got)
	}
}

func TestXPDayStart(t *testing.T) {
	s := DefaultSettings() // UTC+5
	// 03:00 UTC on 10 Nov is 08:00 in the Maldives: the day began at 19:00 UTC on 9 Nov.
	now := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	want := time.Date(2026, 11, 9, 19, 0, 0, 0, time.UTC)
	if got := XPDayStart(s, now); !got.Equal(want) {
		t.Errorf("XPDayStart = %v, want %v", got, want)
	}
	// 20:00 UTC is already the next Maldives day.
	now = time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	want = time.Date(2026, 11, 10, 19, 0, 0, 0, time.UTC)
	if got := XPDayStart(s, now); !got.Equal(want) {
		t.Errorf("XPDayStart late = %v, want %v", got, want)
	}
}
