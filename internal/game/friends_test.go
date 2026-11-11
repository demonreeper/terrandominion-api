package game

import (
	"strings"
	"testing"
)

func TestCleanFriendName(t *testing.T) {
	for typed, want := range map[string]string{
		"  Aishath   Rasheed ": "Aishath Rasheed",
		"O'Brien":              "O'Brien",
		"Jean-Luc":             "Jean-Luc",
		"":                     "",
		"އަހުމަދު":             "އަހުމަދު", // Thaana letters count as letters
	} {
		got, ok := CleanFriendName(typed, nil)
		if !ok || got != want {
			t.Errorf("CleanFriendName(%q) = %q, %v", typed, got, ok)
		}
	}
	for _, bad := range []string{"Echo7", "a@b.com", "<script>", strings.Repeat("a", 33)} {
		if _, ok := CleanFriendName(bad, nil); ok {
			t.Errorf("%q should be refused", bad)
		}
	}
	if _, ok := CleanFriendName("Big Badword", []string{"badword"}); ok {
		t.Error("a blocked word is refused, spaces or not")
	}
}

func TestEmailSearch(t *testing.T) {
	if !IsEmailSearch("Someone@Example.com") || IsEmailSearch("Night-Heron") {
		t.Error("an email has an @")
	}
	if NormaliseEmail("  Someone@Example.COM ") != "someone@example.com" {
		t.Error("emails compare trimmed and in lower case")
	}
}

func TestCheckFriendRequest(t *testing.T) {
	s := DefaultSettings()
	ok := FriendRequest{TargetAccepts: true, TargetPlayer: true}
	if CheckFriendRequest(s, ok) != FriendOK {
		t.Fatal("a plain request is refused")
	}
	with := func(f func(*FriendRequest)) FriendRequest { r := ok; f(&r); return r }
	for want, r := range map[string]FriendRequest{
		FriendSelf:       with(func(r *FriendRequest) { r.Self = true }),
		FriendNotAPlayer: with(func(r *FriendRequest) { r.TargetPlayer = false }),
		FriendAlready:    with(func(r *FriendRequest) { r.Exists = true }),
		FriendClosed:     with(func(r *FriendRequest) { r.TargetAccepts = false }),
		FriendFull:       with(func(r *FriendRequest) { r.TheirsCount = s.FriendsMax }),
		FriendTooMany:    with(func(r *FriendRequest) { r.SentToday = s.FriendRequestsPerDay }),
		FriendPending:    with(func(r *FriendRequest) { r.PendingOut = s.FriendRequestsPendingMax }),
		FriendTooSoon:    with(func(r *FriendRequest) { r.DeclinedRecently = true }),
	} {
		if got := CheckFriendRequest(s, r); got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	}
	if low, high := FriendPair(9, 3); low != 3 || high != 9 {
		t.Error("pairs keep the low id first")
	}
}

func TestFriendSearchKind(t *testing.T) {
	for q, want := range map[string]string{"a@b.mv": "email", "K7QX3M9P": "code", "k7qx3m9p": "code", "Gal": "callsign"} {
		if kind, ok := FriendSearchKind(q); kind != want || !ok {
			t.Errorf("%q: %s %v, want %s", q, kind, ok, want)
		}
	}
	if _, ok := FriendSearchKind("Ga"); ok {
		t.Error("a callsign search needs 3 letters")
	}
}
