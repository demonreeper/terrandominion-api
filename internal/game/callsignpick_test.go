package game

import (
	"math/rand"
	"strings"
	"testing"
)

// Every callsign the game can make up is one its own rules accept: the right shape, not reserved,
// not a staff word, and short enough.
func TestEveryMadeCallsignIsAllowed(t *testing.T) {
	seen := map[string]bool{}
	for _, list := range [][]string{callsignFirst, callsignSecond} {
		for _, word := range list {
			if seen[strings.ToLower(word)] {
				t.Errorf("%q is in the lists twice", word)
			}
			seen[strings.ToLower(word)] = true
		}
	}
	for _, first := range callsignFirst {
		for _, second := range callsignSecond {
			callsign := first + "-" + second + "-99"
			clean, ok := CleanCallsign(callsign)
			if !ok || clean != callsign {
				t.Errorf("%q is not a well-formed callsign", callsign)
			}
			if CallsignBlocked(callsign, nil) {
				t.Errorf("%q is reserved or a staff word", callsign)
			}
			if !IsMadeCallsign(callsign) {
				t.Errorf("%q is not recognised as made up", callsign)
			}
		}
	}
}

func TestMakeCallsign(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	made := map[string]bool{}
	for i := 0; i < 2000; i++ {
		callsign := MakeCallsign(r.Intn)
		if !IsMadeCallsign(callsign) || len(callsign) > CallsignMaxLength {
			t.Fatalf("made %q", callsign)
		}
		made[callsign] = true
	}
	if len(made) < 1900 {
		t.Errorf("only %d different callsigns in 2000", len(made))
	}
}

func TestIsMadeCallsign_RefusesTypedNames(t *testing.T) {
	for _, typed := range []string{
		"", "Echo-7", "Nightwalker", "Night-Heron", "Night-Heron-4", "Night-Heron-042", "Night-Heron-09",
		"night-heron-42", "NIGHT-HERON-42", "Heron-Night-42", "Night-Admin-42", "Night_Heron_42",
		"Night-Heron-4x", "Night-Heron-42-1", " Night-Heron-42",
	} {
		if IsMadeCallsign(typed) {
			t.Errorf("%q passed as made up", typed)
		}
	}
	if !IsMadeCallsign("Night-Heron-42") || !IsMadeCallsign("Zinc-Kite-10") {
		t.Error("a made-up callsign was refused")
	}
}
