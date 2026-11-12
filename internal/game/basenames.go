package game

import (
	"strings"
	"time"
)

// Base names (base-building-plan.md section 1): picked from six the game makes up, never typed, like callsigns.
// Each faction has its own words so its names feel like it: two words in capitals, "GLASS VIGIL". The words name
// no power, faction, place, country or belief, and none of the shapes the lore screens avoid.

// BaseNameOptionCount is how many names are offered at a time.
const BaseNameOptionCount = 6

var baseNameWords = map[int][2][]string{
	// Lumen Collective: light, ice, memory.
	1: {{"GLASS", "FROST", "PALE", "SHARD", "PRISM", "CLEAR", "COLD", "BRIGHT", "ICE", "SILVER", "WHITE", "STILL"},
		{"VIGIL", "LANTERN", "CAIRN", "REACH", "HOLLOW", "CREST", "ECHO", "BEACON", "MIRROR", "SUMMIT", "ARCHIVE", "FIELD"}},
	// Forge Meridian: metal and building.
	2: {{"IRON", "ANVIL", "RIVET", "STEEL", "COPPER", "BOLT", "GIRDER", "KILN", "CAST", "FORGED", "TEMPERED", "BRASS"},
		{"ROOT", "YARD", "WORKS", "FOUNDRY", "SPAN", "GATE", "FRAME", "HOLD", "KEEL", "BEAM", "MILL", "TRUSS"}},
	// Verdant Exchange: trade and price.
	3: {{"GILT", "LEDGER", "COIN", "VELVET", "AMBER", "JADE", "MINT", "TALLY", "GOLDEN", "SILK", "EMERALD", "TOKEN"},
		{"ARCADE", "VAULT", "COURT", "WHARF", "QUAY", "COUNTER", "HARBOUR", "TERRACE", "MART", "LEDGE", "ROW", "DOCK"}},
	// Veil Cipher: what isn't seen.
	4: {{"HUSH", "SHADE", "MUTE", "DUSK", "SMOKE", "WHISPER", "NIGHT", "OBSIDIAN", "ASH", "MIST", "HIDDEN", "QUIET"},
		{"LOCK", "WELL", "GROTTO", "ATTIC", "CELLAR", "MAZE", "KEY", "ALCOVE", "NOOK", "BURROW", "PASSAGE", "LOFT"}},
	// Slipstream Vanguard: speed, first to arrive.
	5: {{"SWIFT", "RAPID", "GALE", "DART", "SURGE", "BLAZE", "FLASH", "JET", "STREAK", "VOLT", "FIRST", "AIR"},
		{"WAKE", "RUN", "LANE", "RIDGE", "VECTOR", "RELAY", "SPRINT", "CURRENT", "TRAIL", "PASS", "LAUNCH", "DRIFT"}},
	// Bastion Order: holding ground.
	6: {{"STONE", "GRANITE", "IRONBARK", "OAKEN", "FLINT", "BASALT", "STEADFAST", "GREY", "HEAVY", "DEEP", "SLATE", "WARDED"},
		{"WALL", "KEEP", "MOAT", "REDOUBT", "TOWER", "BARRIER", "RAMPART", "GARRISON", "GATEHOUSE", "BUTTRESS", "FORT", "BULWARK"}},
}

// MakeBaseName makes one name up for a faction ("" for an unknown faction). intn(n) returns 0 to n-1.
func MakeBaseName(factionID int, intn func(n int) int) string {
	words, ok := baseNameWords[factionID]
	if !ok {
		return ""
	}
	return words[0][intn(len(words[0]))] + " " + words[1][intn(len(words[1]))]
}

// IsBaseName says whether the game could have offered this name to the faction: exactly "<FIRST> <SECOND>" from
// its own lists. The server takes nothing else.
func IsBaseName(factionID int, name string) bool {
	words, ok := baseNameWords[factionID]
	if !ok {
		return false
	}
	parts := strings.Split(name, " ")
	return len(parts) == 2 && inWords(words[0], parts[0]) && inWords(words[1], parts[1])
}

// BaseNameOptions makes up to count different names the faction can still take (taken says a name is in use).
func BaseNameOptions(factionID, count int, intn func(n int) int, taken func(string) bool) []string {
	out := make([]string, 0, count)
	seen := map[string]bool{}
	for tries := 0; len(out) < count && tries < count*40; tries++ {
		name := MakeBaseName(factionID, intn)
		if name == "" {
			break
		}
		if seen[name] || taken(name) {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// BaseBill is the four-resource bill (alloy, crystal, data, credit) to reach a tier.
func BaseBill(tier int, s Settings) [4]int {
	if !ValidBaseTier(tier) {
		return [4]int{}
	}
	return s.BaseCost[tier]
}

// BaseAssembly is how long a tier's project assembles once funded.
func BaseAssembly(tier int, s Settings) time.Duration {
	if !ValidBaseTier(tier) {
		return 0
	}
	return time.Duration(s.BaseAssemblyMin[tier] * float64(time.Minute))
}

// BaseProposalExpiry is when a flag planted now comes down unanswered.
func BaseProposalExpiry(now time.Time, s Settings) time.Time {
	return now.Add(time.Duration(s.BaseProposalHours * float64(time.Hour)))
}

// Flag notices. Country-neutral, no power speaks.

func NoticeBaseProposal(proposer, name string) string {
	return proposer + " planted a flag for " + name + ". Approve or decline it in PROPOSALS."
}

func NoticeBaseApproved(name string) string {
	return name + " is approved. The Spire project has started: fund it on site."
}

func NoticeBaseDeclined(name string) string {
	return "Your flag for " + name + " was declined. The flag is down."
}

func NoticeBaseExpired(name string) string {
	return "Your flag for " + name + " was not answered in time. The flag is down."
}
