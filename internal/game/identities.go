package game

// Sign-ins (identities) attached to one account. A player can have one per provider ("google",
// "apple", "dev"), and needs at least one. Two accounts are never merged: a sign-in that already
// opens another account can't be linked to this one.

// Providers a player can link or unlink from the app. "dev" exists only on the dev server.
var LinkableProviders = []string{"google", "apple"}

func IsLinkable(provider string) bool {
	for _, p := range LinkableProviders {
		if p == provider {
			return true
		}
	}
	return false
}

type LinkOutcome int

const (
	// LinkNew: the sign-in is new; attach it to this player.
	LinkNew LinkOutcome = iota
	// LinkAlready: it already opens this account; nothing changes.
	LinkAlready
	// LinkTaken: it opens another account. Refused (no merging).
	LinkTaken
	// LinkProviderUsed: this account already has a different sign-in of the same provider.
	LinkProviderUsed
)

// DecideLink says what linking a sign-in does. owner is the account it opens now (ownerFound false
// when it opens none); linked are the providers this player already has.
func DecideLink(ownerFound bool, owner, player int64, provider string, linked []string) LinkOutcome {
	if ownerFound {
		if owner == player {
			return LinkAlready
		}
		return LinkTaken
	}
	for _, p := range linked {
		if p == provider {
			return LinkProviderUsed
		}
	}
	return LinkNew
}

type UnlinkOutcome int

const (
	UnlinkOK UnlinkOutcome = iota
	// UnlinkNotLinked: the player has no sign-in of that provider.
	UnlinkNotLinked
	// UnlinkLast: it is the player's only sign-in; removing it would lock them out.
	UnlinkLast
)

// DecideUnlink says whether a player with these sign-ins can remove the provider's one.
func DecideUnlink(linked []string, provider string) UnlinkOutcome {
	found := false
	for _, p := range linked {
		if p == provider {
			found = true
		}
	}
	switch {
	case !found:
		return UnlinkNotLinked
	case len(linked) <= 1:
		return UnlinkLast
	default:
		return UnlinkOK
	}
}
