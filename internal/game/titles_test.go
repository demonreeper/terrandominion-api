package game

import "testing"

func TestTitlesDue(t *testing.T) {
	s := DefaultSettings() // Drifter 22, Wayfarer 26, Unbound 30
	count := func(level int, freelancer, contact bool) int { return len(TitlesDue(s, level, freelancer, contact)) }
	for level, want := range map[int]int{19: 0, 20: 0, 21: 0, 22: 1, 25: 1, 26: 2, 30: 3, 80: 3} {
		if got := count(level, true, true); got != want {
			t.Errorf("level %d: %d titles, want %d", level, got, want)
		}
	}
	if count(40, false, true) != 0 {
		t.Error("a member earns no more titles")
	}
	if count(40, true, false) != 0 {
		t.Error("titles come from the seventh voice: none before it has called")
	}
	if TitlesDue(s, 30, true, true)[2].Badge != "unbound" || TitleMessageKind("drifter") != "langis_title_drifter" {
		t.Error("badges and kinds")
	}
	if !IsLangisKind(TitleMessageKind("drifter")) {
		t.Error("a title's message is the seventh voice's")
	}
}
