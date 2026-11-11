package game

import (
	"testing"
	"time"
)

func TestCleanCallsign(t *testing.T) {
	cases := []struct {
		typed string
		want  string
		ok    bool
	}{
		{"ECHO-7", "ECHO-7", true},
		{"  Echo_7  ", "Echo_7", true},
		{"abc", "abc", true},
		{"ABCDEFGHIJKLMNOP", "ABCDEFGHIJKLMNOP", true},    // 16
		{"ABCDEFGHIJKLMNOPQ", "ABCDEFGHIJKLMNOPQ", false}, // 17
		{"ab", "ab", false},
		{"", "", false},
		{"has space", "has space", false},
		{"dot.name", "dot.name", false},
		{"émile", "émile", false},
		{"-echo", "-echo", false},
		{"echo_", "echo_", false},
		{"<b>x</b>", "<b>x</b>", false},
	}
	for _, c := range cases {
		got, ok := CleanCallsign(c.typed)
		if got != c.want || ok != c.ok {
			t.Errorf("CleanCallsign(%q) = %q, %v; want %q, %v", c.typed, got, ok, c.want, c.ok)
		}
	}
}

func TestCallsignBlocked(t *testing.T) {
	blocked := []string{"badword"}
	refused := []string{
		"admin", "ADMIN", "Ad-min", "admin_01", "Moderator", "support-team", "OPERATIVE-K7QX3M9P",
		"Signum", "SHINGO", "sema", "Ishara99", "tohu_7", "Merki", "LumenCollective", "Forge-Meridian", "mod", "MOD-1",
		"xBadWordx", "bad-word", "BAD_WORD_1", "a-_-b",
	}
	for _, callsign := range refused {
		if !CallsignBlocked(callsign, blocked) {
			t.Errorf("CallsignBlocked(%q) = false, want true", callsign)
		}
	}
	allowed := []string{"ECHO-7", "Nightwalker", "reef_runner", "K7QX", "Semaphore", "Modern", "Forgemaster", "Atoll-Fox"}
	for _, callsign := range allowed {
		if CallsignBlocked(callsign, blocked) {
			t.Errorf("CallsignBlocked(%q) = true, want false", callsign)
		}
	}
}

func TestCallsignChangeAt(t *testing.T) {
	if got := CallsignChangeAt(nil, 30); !got.IsZero() {
		t.Errorf("first pick should be free, got %v", got)
	}
	changed := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	want := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	if got := CallsignChangeAt(&changed, 30); !got.Equal(want) {
		t.Errorf("CallsignChangeAt = %v, want %v", got, want)
	}
	// 29 days in: still too soon. 30 days in: allowed.
	if now := changed.Add(29 * 24 * time.Hour); !now.Before(CallsignChangeAt(&changed, 30)) {
		t.Error("29 days after a change should still be too soon")
	}
	if now := changed.Add(30 * 24 * time.Hour); now.Before(CallsignChangeAt(&changed, 30)) {
		t.Error("30 days after a change should be allowed")
	}
}

func TestValidPublicCode(t *testing.T) {
	for _, code := range []string{"K7QX3M9P", "k7qx3m9p", "00000000", "ZZZZZZZZ"} {
		if !ValidPublicCode(code) {
			t.Errorf("ValidPublicCode(%q) = false, want true", code)
		}
	}
	// I, L, O and U are not in the alphabet; wrong lengths and other characters are refused.
	for _, code := range []string{"", "K7QX3M9", "K7QX3M9PP", "K7QX3M9I", "K7QX3M9L", "K7QX3M9O", "K7QX3M9U", "K7QX-M9P", "K7QX 3M9"} {
		if ValidPublicCode(code) {
			t.Errorf("ValidPublicCode(%q) = true, want false", code)
		}
	}
	if got := PlaceholderCallsign("K7QX3M9P"); got != "OPERATIVE-K7QX3M9P" {
		t.Errorf("PlaceholderCallsign = %q", got)
	}
}
