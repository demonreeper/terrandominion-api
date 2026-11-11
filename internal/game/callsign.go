package game

import (
	"strings"
	"time"
)

// Callsigns and share codes (docs/profile-plan.md, P0). A callsign is the name other players see;
// the real name from Google or Apple never leaves the server.

const (
	CallsignMinLength = 3
	CallsignMaxLength = 16
	// PublicCodeLength is the share code's length; its alphabet is Crockford base32.
	PublicCodeLength = 8
	publicCodeChars  = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// StaffCallsignPrefixes can't start a callsign: nobody may pass for staff or for a placeholder
// name ("ADMIN_01", "OPERATIVE-K7QX3M9P").
var StaffCallsignPrefixes = []string{"admin", "moderator", "moderation", "support", "official", "staff", "operative"}

// ReservedCallsigns can't be taken, with or without digits after them: staff words, the game's own
// names, the six powers and the factions. Compared with letters and digits only, lower case.
var ReservedCallsigns = []string{
	"mod", "system", "deleted", "deletedplayer", "player", "freelancer",
	"terrandominion", "terran", "dominion", "hisstudios", "undersignal",
	"signum", "shingo", "sema", "ishara", "tohu", "merki",
	"lumen", "lumencollective", "forge", "forgemeridian", "veil", "veilcipher",
	"verdant", "verdantexchange", "slipstream", "slipstreamvanguard", "bastion", "bastionorder",
}

// CleanCallsign trims a typed callsign and says whether its shape is allowed: 3 to 16 characters,
// letters (A to Z), digits, '-' and '_', starting and ending with a letter or digit.
func CleanCallsign(typed string) (string, bool) {
	callsign := strings.TrimSpace(typed)
	if len(callsign) < CallsignMinLength || len(callsign) > CallsignMaxLength {
		return callsign, false
	}
	for i := 0; i < len(callsign); i++ {
		c := callsign[i]
		letterOrDigit := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !letterOrDigit && c != '-' && c != '_' {
			return callsign, false
		}
		if !letterOrDigit && (i == 0 || i == len(callsign)-1) {
			return callsign, false
		}
	}
	return callsign, true
}

// callsignKey is a callsign with only its letters and digits, lower case: what the reserved and
// blocked lists are compared with, so "Ad-min" and "ADMIN_" read as "admin".
func callsignKey(callsign string) string {
	var b strings.Builder
	for i := 0; i < len(callsign); i++ {
		c := callsign[i]
		switch {
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		}
	}
	return b.String()
}

// CallsignBlocked says whether a well-formed callsign is still refused: it starts with a staff
// word, it is a reserved name (digits after it don't help), or it contains a blocked word.
func CallsignBlocked(callsign string, blocked []string) bool {
	key := callsignKey(callsign)
	if len(key) < CallsignMinLength {
		return true // "a-_b": too little left to be a name
	}
	for _, prefix := range StaffCallsignPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	name := strings.TrimRight(key, "0123456789")
	for _, reserved := range ReservedCallsigns {
		if name == reserved {
			return true
		}
	}
	for _, word := range blocked {
		if word != "" && strings.Contains(key, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// CallsignChangeAt is when a player may next change their callsign: at once for the first pick
// (changedAt nil), otherwise changeDays after the last change.
func CallsignChangeAt(changedAt *time.Time, changeDays int) time.Time {
	if changedAt == nil {
		return time.Time{}
	}
	return changedAt.Add(time.Duration(changeDays) * 24 * time.Hour)
}

// ValidPublicCode says whether text is shaped like a share code. Lower case is accepted (links get
// retyped); callers upper-case it before looking it up.
func ValidPublicCode(code string) bool {
	if len(code) != PublicCodeLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !strings.ContainsRune(publicCodeChars, rune(strings.ToUpper(code[i : i+1])[0])) {
			return false
		}
	}
	return true
}

// PlaceholderCallsign is the name shown until a player picks a callsign.
func PlaceholderCallsign(publicCode string) string { return "OPERATIVE-" + publicCode }
