package game

import (
	"math"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// Faction bases, B0 (docs/base-building-plan.md, sections 1, 1b, 2 and 3): who may propose, where a base may
// stand, how many a faction may hold, and the slot layout per tier. Pure functions; the service gathers the
// nearby bases and flags and the map checks (land, walkway, buildings, sensitive places) and passes them in.

// RankLead is the atoll/city lead (regional-command-plan.md: appointed by the regional Commander).
const RankLead = 70

// BaseTopTier is the Capital; BaseTopTierNonCapital is where every other base stops (section 2).
const (
	BaseTopTier           = 5
	BaseTopTierNonCapital = 4
)

// BaseTierName names a tier for players.
func BaseTierName(tier int) string {
	switch tier {
	case 1:
		return "Spire"
	case 2:
		return "Outpost"
	case 3:
		return "Stronghold"
	case 4:
		return "Citadel"
	case 5:
		return "Capital"
	}
	return ""
}

// ValidBaseTier says whether a tier exists.
func ValidBaseTier(tier int) bool { return tier >= 1 && tier <= BaseTopTier }

// BaseZoneM is the zone radius at a tier.
func BaseZoneM(tier int, s Settings) float64 {
	return pickTier(tier, s.BaseZoneT1, s.BaseZoneT2, s.BaseZoneT3, s.BaseZoneT4, s.BaseZoneT5)
}

// BaseCoreHealth is the core's full health at a tier.
func BaseCoreHealth(tier int, s Settings) int {
	return int(pickTier(tier, float64(s.BaseCoreT1), float64(s.BaseCoreT2), float64(s.BaseCoreT3),
		float64(s.BaseCoreT4), float64(s.BaseCoreT5)))
}

// BaseSlotCount is how many module slots a tier has.
func BaseSlotCount(tier int, s Settings) int {
	return int(pickTier(tier, float64(s.BaseSlotsT1), float64(s.BaseSlotsT2), float64(s.BaseSlotsT3),
		float64(s.BaseSlotsT4), float64(s.BaseSlotsT5)))
}

func pickTier(tier int, t1, t2, t3, t4, t5 float64) float64 {
	switch tier {
	case 1:
		return t1
	case 2:
		return t2
	case 3:
		return t3
	case 4:
		return t4
	case 5:
		return t5
	}
	return 0
}

// BaseSlot is one module slot, as the server stores it.
type BaseSlot struct {
	Ring  int `json:"ring"`
	Index int `json:"index"`
}

// baseSlotRing is the ring the slots a tier opens go on (section 2: ring 1 at T1 and T2, ring 2 from T3,
// ring 3 from T4; the Capital's extra slots stay on ring 3).
func baseSlotRing(tier int) int {
	switch {
	case tier <= 2:
		return 1
	case tier == 3:
		return 2
	default:
		return 3
	}
}

// BaseSlotLayout lists every slot a base at this tier has, ring by ring. Each tier keeps the slots of the
// tiers below it, so raising a tier only adds slots and dropping one removes the newest.
func BaseSlotLayout(tier int, s Settings) []BaseSlot {
	var out []BaseSlot
	next := map[int]int{}
	have := 0
	for t := 1; t <= tier && t <= BaseTopTier; t++ {
		want := BaseSlotCount(t, s)
		ring := baseSlotRing(t)
		for ; have < want; have++ {
			out = append(out, BaseSlot{Ring: ring, Index: next[ring]})
			next[ring]++
		}
	}
	return out
}

// BaseCap is how many bases (open flags included) each faction may hold in a region with this many active
// members of all factions together (section 3): the base number plus one per step reached. A step of 0 is off.
func BaseCap(activeMembers int, s Settings) int {
	n := s.BaseCapBase
	for _, step := range []int{s.BaseCapStep1Players, s.BaseCapStep2Players, s.BaseCapStep3Players, s.BaseCapStep4Players} {
		if step > 0 && activeMembers >= step {
			n++
		}
	}
	return n
}

// Proposer is the player who wants to plant a flag, as the service found them.
type Proposer struct {
	InFaction bool
	Rank      int
	// HomeRegion is their branch's region; HereRegion where their fix is.
	HomeRegion, HereRegion string
	AccuracyM              float64
	HasOpenProposal        bool
	// CooldownUntil is when their last declined or expired proposal's cooldown ends (zero: none).
	CooldownUntil time.Time
	ProposalsOff  bool
	BasesEnabled  bool
}

// CanProposeBase checks the proposer (rule 1 of the placement rules and the anti-spam limits of section 1b). A
// Warden, atoll/city lead or the regional Commander, in their home region, with a good enough fix.
func CanProposeBase(p Proposer, now time.Time, s Settings) *apperr.Error {
	switch {
	case !p.BasesEnabled:
		return apperr.BasesOff
	case !p.InFaction:
		return apperr.NotAMember
	case p.Rank < RankWarden:
		return apperr.BaseRankTooLow
	case p.HomeRegion == "" || p.HomeRegion != p.HereRegion:
		return apperr.BaseNotHomeRegion
	case p.ProposalsOff:
		return apperr.BaseProposalsOff
	case p.AccuracyM <= 0 || p.AccuracyM > s.BaseSpotAccuracyM:
		return apperr.BaseAccuracy
	case p.HasOpenProposal:
		return apperr.BaseProposalOpen
	case now.Before(p.CooldownUntil):
		return apperr.BaseProposalCooldown.WithData(map[string]any{"until": p.CooldownUntil.UTC().Format(time.RFC3339)})
	}
	return nil
}

// BaseCanApprove: the regional Commander (or whoever holds the seat as acting Commander) approves.
func BaseCanApprove(rank int) bool { return rank >= RankRegionalCommander }

// BaseCooldownUntil is when a proposal decided at decidedAt stops blocking its proposer.
func BaseCooldownUntil(decidedAt time.Time, s Settings) time.Time {
	return decidedAt.Add(time.Duration(s.BaseProposalCooldownHours * float64(time.Hour)))
}

// NearBase is a standing base near a spot.
type NearBase struct {
	ID        int64
	FactionID int
	Name      string
	Lat, Lon  float64
	ZoneM     float64
}

// NearFlag is an open proposal near a spot. Its blocked area is the Spire's starting zone plus the gap.
type NearFlag struct {
	ID        int64
	FactionID int
	Lat, Lon  float64
}

// SpotMap is what the map layers say about a spot.
type SpotMap struct {
	OnLand        bool `json:"onLand"`
	NearWalkway   bool `json:"nearWalkway"`
	InBlocked     bool `json:"inBlocked"`
	NearSensitive bool `json:"nearSensitive"`
}

// BaseSpot is a place a base would stand, and what stands around it.
type BaseSpot struct {
	Lat, Lon  float64
	FactionID int
	// SelfID is the base itself when it grows (never counted against itself); 0 for a new spot. SelfFlagID is
	// the proposal being approved.
	SelfID     int64
	SelfFlagID int64
	Bases      []NearBase
	Flags      []NearFlag
	Map        SpotMap
	// Held counts the faction's standing bases and open flags in the region (the proposal being approved not
	// included); Cap is BaseCap for the region.
	Held, Cap int
}

// BaseSpotError is a refused spot: the code, and the base it is too close to when there is one.
type BaseSpotError struct {
	Err  *apperr.Error
	Near string
}

// CheckBaseSpot runs placement rules 2 to 5 for a new Spire at the spot (section 1), plus the flags of other
// factions (section 1b). The first broken rule is returned, or nil.
func CheckBaseSpot(sp BaseSpot, s Settings) *BaseSpotError {
	switch {
	case !sp.Map.OnLand || sp.Map.InBlocked || !sp.Map.NearWalkway:
		return &BaseSpotError{Err: apperr.BaseSpotTerrain}
	case sp.Map.NearSensitive:
		return &BaseSpotError{Err: apperr.BaseSpotSensitive}
	}
	zone := BaseZoneM(1, s)
	for _, b := range sp.Bases {
		if b.ID == sp.SelfID {
			continue
		}
		d := DistanceM(sp.Lat, sp.Lon, b.Lat, b.Lon)
		if b.FactionID == sp.FactionID && d < s.BaseSpacingSameFactionM {
			return &BaseSpotError{Err: apperr.BaseSpacingSame, Near: b.Name}
		}
		if b.FactionID != sp.FactionID && d < s.BaseSpacingOtherFactionM {
			return &BaseSpotError{Err: apperr.BaseSpacingOther, Near: b.Name}
		}
		if d < zone+b.ZoneM+s.BaseZoneGapM {
			return &BaseSpotError{Err: apperr.BaseZoneTooClose, Near: b.Name}
		}
	}
	if e := flagsClear(sp, zone, s); e != nil {
		return e
	}
	if sp.Held >= sp.Cap {
		return &BaseSpotError{Err: apperr.BaseCapReached}
	}
	return nil
}

// flagsClear: another faction's open flag holds its Spire's starting zone plus the gap, so a zone of radius
// zoneM at the spot must stay the gap outside that.
func flagsClear(sp BaseSpot, zoneM float64, s Settings) *BaseSpotError {
	for _, f := range sp.Flags {
		if f.ID == sp.SelfFlagID || f.FactionID == sp.FactionID {
			continue
		}
		if DistanceM(sp.Lat, sp.Lon, f.Lat, f.Lon) < zoneM+BaseZoneM(1, s)+s.BaseZoneGapM {
			return &BaseSpotError{Err: apperr.BaseFlagHeld}
		}
	}
	return nil
}

// CheckBaseGrowth is the border rule (section 2): a base may reach toTier only if its new zone stays the gap
// outside every other base's zone edge (own faction included) and outside other factions' flags. The base itself
// is sp.SelfID.
func CheckBaseGrowth(sp BaseSpot, toTier int, s Settings) *BaseSpotError {
	zone := BaseZoneM(toTier, s)
	for _, b := range sp.Bases {
		if b.ID == sp.SelfID {
			continue
		}
		if DistanceM(sp.Lat, sp.Lon, b.Lat, b.Lon) < zone+b.ZoneM+s.BaseZoneGapM {
			return &BaseSpotError{Err: apperr.BaseZoneTooClose, Near: b.Name}
		}
	}
	return flagsClear(sp, zone, s)
}

// BaseZoneRoomM is how far a base's zone could grow before it meets the gap of its nearest neighbour (the
// ghost ring in B3); +Inf with no neighbour.
func BaseZoneRoomM(sp BaseSpot, s Settings) float64 {
	room := math.Inf(1)
	for _, b := range sp.Bases {
		if b.ID == sp.SelfID {
			continue
		}
		room = math.Min(room, DistanceM(sp.Lat, sp.Lon, b.Lat, b.Lon)-b.ZoneM-s.BaseZoneGapM)
	}
	for _, f := range sp.Flags {
		if f.FactionID == sp.FactionID {
			continue
		}
		room = math.Min(room, DistanceM(sp.Lat, sp.Lon, f.Lat, f.Lon)-BaseZoneM(1, s)-s.BaseZoneGapM)
	}
	return room
}

// BaseSearchM is how far round a spot the service must look for bases and flags so every rule above sees them.
func BaseSearchM(s Settings) float64 {
	return math.Max(s.BaseSpacingSameFactionM,
		math.Max(s.BaseSpacingOtherFactionM, 2*BaseZoneM(BaseTopTier, s)+s.BaseZoneGapM))
}
