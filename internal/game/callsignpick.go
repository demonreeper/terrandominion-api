package game

import (
	"strconv"
	"strings"
)

// Callsigns are picked, not typed (owner, 2026-11-11): the game offers a handful it made up and the
// player takes one, so nothing offensive can be entered. A made-up callsign is two words and a
// two-digit number, "Night-Heron-42": at most 16 characters, about 170,000 of them.
//
// The words are plain and belong to nobody: no power, faction, place, country or belief, and none
// of the shapes the lore screens avoid.

var callsignFirst = []string{
	"Amber", "Ash", "Brisk", "Calm", "Cobalt", "Coral", "Dawn", "Deep", "Drift", "Dusk",
	"Echo", "Ember", "Flint", "Frost", "Gale", "Ghost", "Iron", "Jade", "Keen", "Mist",
	"Night", "Onyx", "Pale", "Quick", "Rapid", "Reef", "Rust", "Sable", "Salt", "Shade",
	"Sharp", "Slate", "Solar", "Steel", "Still", "Storm", "Tidal", "Vivid", "Wild", "Zinc",
}

var callsignSecond = []string{
	"Heron", "Falcon", "Osprey", "Raven", "Marlin", "Orca", "Manta", "Otter", "Lynx", "Viper",
	"Cobra", "Gecko", "Mantis", "Hornet", "Wasp", "Moth", "Tern", "Gull", "Crane", "Egret",
	"Ibis", "Shrike", "Finch", "Petrel", "Turtle", "Urchin", "Conch", "Squid", "Pike", "Tuna",
	"Wahoo", "Skate", "Eel", "Crab", "Fox", "Wolf", "Hare", "Stag", "Bison", "Badger",
	"Swift", "Kite",
}

const (
	callsignNumberMin = 10
	callsignNumberMax = 99
	// CallsignOptionCount is how many callsigns are offered at a time.
	CallsignOptionCount = 6
)

// MakeCallsign makes one callsign up. intn(n) returns a random number from 0 to n-1.
func MakeCallsign(intn func(n int) int) string {
	return callsignFirst[intn(len(callsignFirst))] + "-" + callsignSecond[intn(len(callsignSecond))] + "-" +
		strconv.Itoa(callsignNumberMin+intn(callsignNumberMax-callsignNumberMin+1))
}

// IsMadeCallsign says whether a callsign is one the game could have offered: exactly
// "<first>-<second>-<two digits>" with both words from the lists, in their own letter case. The
// server takes nothing else, so a callsign can't be typed even by calling the API by hand.
func IsMadeCallsign(callsign string) bool {
	parts := strings.Split(callsign, "-")
	if len(parts) != 3 || !inWords(callsignFirst, parts[0]) || !inWords(callsignSecond, parts[1]) {
		return false
	}
	if len(parts[2]) != 2 {
		return false
	}
	n, err := strconv.Atoi(parts[2])
	return err == nil && n >= callsignNumberMin && n <= callsignNumberMax
}

func inWords(words []string, word string) bool {
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}
