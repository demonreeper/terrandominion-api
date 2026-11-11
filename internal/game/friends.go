package game

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Friends (owner, 2026-11-11). Players find each other by callsign, by a full email, or from a
// faction's branch, and send a request; once the other accepts, both see each other in a friend
// list with the name each typed for friends. The Google/Apple name never leaves the server, and an
// email is only ever matched whole, never shown.

const FriendNameMax = 32

// CleanFriendName tidies the name a player shows their friends: letters (any language), spaces,
// apostrophes, dots and hyphens, 1 to 32 characters. "" clears it. ok is false when it can't be used.
func CleanFriendName(typed string, blocked []string) (string, bool) {
	name := strings.Join(strings.Fields(typed), " ")
	if name == "" {
		return "", true
	}
	if utf8.RuneCountInString(name) > FriendNameMax {
		return name, false
	}
	for _, r := range name {
		// Marks too: Thaana's vowel signs (fili) and accents are combining marks, not letters.
		if !unicode.IsLetter(r) && !unicode.IsMark(r) && r != ' ' && r != '\'' && r != '.' && r != '-' {
			return name, false
		}
	}
	key := strings.ToLower(strings.NewReplacer(" ", "", "'", "", ".", "", "-", "").Replace(name))
	for _, word := range blocked {
		if word != "" && strings.Contains(key, strings.ToLower(word)) {
			return name, false
		}
	}
	return name, true
}

// IsEmailSearch says whether a search is for an email (it has an @): those match only the whole
// address, case aside, and only players who allow it.
func IsEmailSearch(q string) bool { return strings.Contains(q, "@") }

// NormaliseEmail is an email as it is compared: trimmed, lower case.
func NormaliseEmail(q string) string { return strings.ToLower(strings.TrimSpace(q)) }

// Why a friend request is refused.
const (
	FriendOK         = ""
	FriendSelf       = "self"
	FriendAlready    = "already"  // already friends, or a request is waiting
	FriendClosed     = "closed"   // the other player doesn't accept requests
	FriendFull       = "full"     // FriendsMax reached, on either side
	FriendTooMany    = "too_many" // FriendRequestsPerDay sent today
	FriendNotAPlayer = "not_player"
)

// FriendRequest is what the server knows when a player asks another to be friends.
type FriendRequest struct {
	Self          bool
	Exists        bool
	TargetAccepts bool
	TargetPlayer  bool // an ordinary account, not a system one
	MineCount     int
	TheirsCount   int
	SentToday     int
}

func CheckFriendRequest(s Settings, r FriendRequest) string {
	switch {
	case r.Self:
		return FriendSelf
	case !r.TargetPlayer:
		return FriendNotAPlayer
	case r.Exists:
		return FriendAlready
	case !r.TargetAccepts:
		return FriendClosed
	case r.MineCount >= s.FriendsMax || r.TheirsCount >= s.FriendsMax:
		return FriendFull
	case r.SentToday >= s.FriendRequestsPerDay:
		return FriendTooMany
	}
	return FriendOK
}

// FriendPair orders two ids the way td.friendships keeps them: low first.
func FriendPair(a, b int64) (low, high int64) {
	if a < b {
		return a, b
	}
	return b, a
}
