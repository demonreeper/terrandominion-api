// Package game holds the rules of Tether Rise as plain functions over plain values: no SQL,
// no HTTP. The service layer loads state, asks these functions what happens, and stores the
// result. Keeping the rules here makes them unit-testable and lets them move onto another server
// framework (such as Nakama) unchanged.
package game

import (
	"fmt"
	"strings"
)

// Settings mirrors the td.settings table. Every field has a default, so a missing row never
// breaks the game; the table only overrides.
type Settings struct {
	LevelBaseXp      float64
	LevelGrowth      float64
	LeadFactionLevel int
	JoinFactionLevel int
	XpHarvest        int
	XpCraft          int
	XpUpgrade        int
	// Attacks give XP (0031): a hit, extra for destroying, an intercept; capped per pair per day.
	XpAttack             int
	XpDestroy            int
	XpIntercept          int
	XpAttackPerTargetDay int

	HarvestRangeM       float64
	HarvestMaxAccuracyM float64
	HarvestCooldownSec  int
	HarvestReserveCost  float64
	MaxTravelSpeedKmh   float64
	NearbyMaxRadiusM    float64

	// Signal echoes (no-ar-support-plan.md milestone 7): how many per cell, the cell size, how long one
	// stays before the Undersignal moves it, how far the AR view sees them, and how far from the node or
	// Tether Node it is placed beside.
	EchoesPerCell   int
	EchoCellM       float64
	EchoMoveHours   float64
	EchoViewRadiusM float64
	EchoOffsetMinM  float64
	EchoOffsetMaxM  float64

	// Walk together (docs/walk-together-plan.md): the signal link between 2 to 4 friends walking together.
	WalkLinkEnabled       int
	WalkLinkMaxSize       int
	WalkLinkGraceSec      int
	WalkLinkHeartbeatSec  int
	WalkLinkFixAgeSec     int
	WalkLinkMoveWindowSec int
	WalkLinkDailyMinutes  int
	WalkLinkAskSec        int
	WalkLinkOpenSec       int
	WalkLinkScanPct2      int
	WalkLinkScanPct3      int
	WalkLinkScanPct4      int
	WalkLinkYieldPct2     int
	WalkLinkYieldPct3     int
	WalkLinkYieldPct4     int
	WalkLinkXpPct2        int
	WalkLinkXpPct3        int
	WalkLinkXpPct4        int
	WalkLinkKeepDays      int
	// W2: Bluetooth proof and the L3 / L8 checks.
	WalkLinkRequireBle     int
	WalkLinkBleGraceSec    int
	WalkLinkBleTokenSec    int
	WalkLinkBleHeardSec    int
	WalkLinkTwinFixes      int
	WalkLinkTwinM          float64
	WalkLinkNoBleReportSec int
	// W2b extras: group nodes (X2), link missions (X3), distance milestones (X5).
	GroupNodeRatePerCell float64
	GroupNodeMixDuo      int
	GroupNodeMixTrio     int
	GroupNodeMixQuad     int
	GroupNodeYieldX2     float64
	GroupNodeYieldX3     float64
	GroupNodeYieldX4     float64
	GroupNodeHours       float64
	LinkMissionChancePct int
	WalkMilestoneKm1     int
	WalkMilestoneKm2     int
	WalkMilestoneKm3     int
	WalkMilestoneKm4     int
	// W3 (part): L4 lockstep, report only.
	WalkLockstepSteps    int
	WalkLockstepM        float64
	WalkLockstepPairSec  float64
	WalkLinkFormM        float64
	WalkLinkHoldM        float64
	WalkLinkMaxAccuracyM float64
	WalkLinkMoveM        float64

	WelcomeCrystal int
	WelcomeAlloy   int
	WelcomeData    int
	WelcomeCredit  int
	WelcomeCells   int

	BaseSpacingSameFactionM  float64
	BaseSpacingOtherFactionM float64

	// Faction bases (base-building-plan.md, B0).
	BasesEnabled                                                    int
	BaseZoneGapM                                                    float64
	BaseSensitiveBufferM                                            float64
	BaseWalkwayM                                                    float64
	BaseSpotAccuracyM                                               float64
	BaseProposalHours                                               float64
	BaseProposalCooldownHours                                       float64
	BaseZoneT1, BaseZoneT2, BaseZoneT3, BaseZoneT4, BaseZoneT5      float64
	BaseSlotsT1, BaseSlotsT2, BaseSlotsT3, BaseSlotsT4, BaseSlotsT5 int
	BaseCoreT1, BaseCoreT2, BaseCoreT3, BaseCoreT4, BaseCoreT5      int
	BaseCapBase                                                     int
	BaseCapStep1Players, BaseCapStep2Players                        int
	BaseCapStep3Players, BaseCapStep4Players                        int
	BaseCapActiveDays                                               float64
	// Faction ranks v2 and Nomads (faction-ranks-plan.md, R1).
	ProspectDays          float64
	ProspectPoints        int
	CaptainSquadSize      int
	CaptainVaultDailyCap  int
	RankPointsCaptain     int
	RankPointsWarden      int
	RankPointsLead        int
	RankSlotsCaptain      int
	RankSlotsWarden       int
	RankSlotsLead         int
	NomadMoveCooldownDays float64
	NomadAskHours         float64
	// Leader succession: a Commander inactive this long loses the seat to an acting Commander; a vote may start this
	// many days after the seat changes and runs this many hours.
	CommanderInactiveDays float64
	CommanderVoteDays     float64
	CommanderVoteHours    float64
	// Captain squad missions: posts a day, and the most a give or stand mission may ask.
	CaptainMissionsPerDay     int
	CaptainMissionGiveMax     int
	CaptainMissionStandMaxMin int
	// Base projects (B2).
	BaseProjectsAtOnce       int
	BasePresenceStepPct      float64
	BasePresenceMaxMult      float64
	BasePresenceSec          float64
	BasePointsPerUnit        float64
	BaseGiveXpPer100         int
	BaseModuleHealth         int
	BaseProtectNewHours      float64
	BasePerkForgeAssemblyPct float64
	BasePerkVerdantCostPct   float64
	BaseCollectorLinks       [MaxModuleMk + 1]int
	// Base combat (B5).
	BaseProtectDropHours     float64
	BaseModulesPerLattice    int
	BaseHackRangeM           float64
	BaseHackData             int
	BaseHackMinPct           float64
	BaseHackMaxPct           float64
	BaseHackCooldownMin      float64
	BaseSafeFloorPct         float64
	BasePerkBastionFloorPct  float64
	BasePerkBastionShieldPct float64
	BaseAttackPushMin        float64
	BaseDestroyDropPct       float64
	BasePackedRefundPct      float64
	// Upkeep (B6).
	BaseUpkeepDays          float64
	BaseFadeDropDays        float64
	BaseSpireCollapseDays   float64
	BaseFadedShieldPct      float64
	BasePerkLumenCrystalPct float64
	BaseModuleCost          map[string][4]int
	BaseModuleMin           map[string]float64
	// BaseCost[tier] is the bill to reach that tier (alloy, crystal, data, credit); BaseAssemblyMin its assembly.
	BaseCost        [BaseTopTier + 1][4]int
	BaseAssemblyMin [BaseTopTier + 1]float64

	TnMaxPerPlayer    int
	TnLinkRadiusM     float64
	TnMinSpacingM     float64
	TnLinksTier1      int
	TnLinksTier2      int
	TnUpgradeAlloy    int
	TnUpgradeData     int
	TnUpgradeCredit   int
	TnInfoRangeM      float64
	TnLifetimeHours   float64
	TnMaxHealth       float64
	TnDismantleRefund float64
	TnOwnerShare      float64
	// The slowdown of a Tether Node whose owner stays away (game.TetherSpeed).
	TnFullSpeedHours    float64 // hours at full speed after it is built or last collected at
	TnSlowPctPerHour    float64 // then this much slower for each further hour (percent of full speed)
	TnSlowFloorPct      float64 // never slower than this (percent of full speed)
	TnUploadFee         float64
	TnBuildCrystal      int
	TnBuildAlloy        int
	TnBuildData         int
	NodeDepletedReserve float64
	NodeRespawnMinM     float64
	NodeRespawnMaxM     float64
	XpTnBuild           int
	DropLifetimeHours   float64
	PvpLevelBand        int
	ProtectionLossDays  float64
	// TnUnderAttackMinutes is how long after a hit a Tether Node counts as under attack.
	TnUnderAttackMinutes float64
	// TnEndedShowMinutes is how long an ended Tether Node stays on My Tether Nodes.
	TnEndedShowMinutes float64

	// MinAppBuild is the lowest app build number (Android version code) the server still serves;
	// raising it locks older builds out (a test group's APK, a build with a bad bug). 0 = no lock.
	// iOS build numbers count separately, so iPhones have their own.
	MinAppBuild    int
	MinAppBuildIOS int

	RepairAlloyPerPoint float64
	CraftCellCrystal    int
	CraftCellData       int
	CraftPulseCrystal   int
	CraftPulseData      int
	CraftJammerData     int
	CraftJammerAlloy    int
	CraftSpikeAlloy     int
	CraftSpikeCrystal   int
	CraftSpikeCredit    int

	WeaponRangeM         float64
	WarMaxHeightM        float64
	EmPulseDamage        float64
	JammerMinutes        float64
	SpikeDamage          float64
	SpikeFuseMinutes     float64
	CellsEmPulse         int
	CellsJammer          int
	CellsSpike           int
	ContestSeconds       int
	ContestPrizeHarvests int
	CellsContest         int
	ContestRangeM        float64
	ContestRecentMinutes float64
	ContestHoldMinutes   float64
	ContestGearPerTier   int
	ContestAllyPoints    int
	ContestJoinPoints    int

	DroneSpeedMps     float64
	DroneMinFlightSec float64

	// Drone strike plan, milestones 2-7. Cells per strike live in each Drone Bay's stats.cells.
	DroneLaunchWear       int
	DroneResultsMinutes   float64
	DroneRangeRingDefault int
	CellsIntercept        int
	InterceptShareMk1     float64
	InterceptShareMk2     float64
	InterceptShareMk3     float64
	InterceptShareMk4     float64
	TnShieldBase          float64
	TnShieldStep          float64
	TnShieldMin           float64
	TnShieldMax           float64
	ShieldLevelMk1        int
	ShieldLevelMk2        int
	ShieldLevelMk3        int
	ShieldLevelMk4        int
	CraftShield1Crystal   int
	CraftShield1Alloy     int
	CraftShield2Alloy     int
	CraftShield2Data      int
	CraftShield3Alloy     int
	CraftShield3Data      int
	CraftShield3Credit    int
	CraftShield4Alloy     int
	CraftShield4Data      int
	CraftShield4Credit    int

	// Orders and daily missions (docs/regional-command-plan.md).
	MissionsPerDay          int
	MissionDayOffsetMinutes int
	MissionXpBase           int
	MissionXpPerLevel       int
	MissionCreditBase       int
	MissionCreditPerLevel   int
	MissionItemChance       float64
	MissionCommandMult      float64

	// The powers' calls (docs/lore-correction-plan.md).
	CallPathLevel        int     // the level whose midpoint sets off the one call about the player's path
	CallRepeatHours      float64 // the repeat call follows the offer by this long
	CallMessageHours     float64 // the written message follows the offer by this long
	CallRingSeconds      int     // a call older than this has rung out: the app offers to play it once
	CallPushDelaySeconds int     // a call the app has not picked up for this long sends a push

	// Node seeding (drone strike plan, milestone 0).
	SeedCellM         float64
	SeedPerCell       int
	SeedPathPerNodeM  float64
	SeedSnapM         float64
	SeedMinGapM       float64
	SeedTnGapM        float64
	SeedRegenPerHour  float64
	SeedWeightCrystal int
	SeedWeightAlloy   int
	SeedWeightData    int
	SeedWeightCredit  int
	SeedYieldCrystal  int
	SeedYieldAlloy    int
	SeedYieldData     int
	SeedYieldCredit   int
	SeedCellsPerScan  int
	SeedTopUpPerTick  int
	SeedTopUpMinutes  float64
	RarePerKm2Day     float64
	RareMinHours      float64
	RareMaxHours      float64
	RareYieldMult     float64

	PushEnabled               int
	PushAttackThrottleMinutes int
	PushMaxAgeMinutes         int

	// Faction perks (docs/profile-plan.md, P1b), each a percentage; 0 switches the perk off.
	PerkSignumCrystalPct       int // more crystal from harvests and Tether Node collection
	PerkShingoUpgradePct       int // cheaper Tether Node upgrades
	PerkSemaScanPct            int // longer scan range (100 = twice as far)
	PerkSemaRarePct            int // rare roaming nodes seen from further (100 = twice as far)
	PerkIsharaBuildPct         int // cheaper Tether Node builds
	PerkIsharaMissionCreditPct int // more credit from missions
	PerkTohuDroneSpeedPct      int // faster drones
	PerkTohuCarryPct           int // more carry capacity
	PerkMerkiDamagePct         int // less damage to own Tether Nodes
	PerkMerkiInterceptPct      int // more of a drone's damage stopped by an intercept (added to the share)

	// The freelancer path (docs/freelancer-path-plan.md).
	FavourPerkPct        int // a power's daily favour is this percent of its faction perk (F4)
	MemberMissionsPerDay int // daily missions a member gets from their own power (F3)
	// The seventh voice (phase 4).
	LangisCallLevel         int     // halfway through this level its first call
	LangisMessageDelayHours float64 // its message, after the power's level 19 day-3 message
	LangisCallDelayHours    float64 // its second call, after its message
	LangisRareEveryDays     int     // after the faction level, at most one rare message in this many days
	// Unbound titles (F2): the levels a freelancer is given each at.
	TitleDrifterLevel  int
	TitleWayfarerLevel int
	TitleUnboundLevel  int
	// The JOBS board (F6).
	JobSlotsMax        int
	JobHoursMin        int
	JobHoursMax        int
	JobRewardMin       int
	JobRewardMax       int
	JobsOpenPerFaction int
	JobsPostedPerDay   int
	JobsTakenMax       int
	JobScoutRadiusM    int
	JobGuardMinutes    int
	JobGuardRadiusM    int
	JobGuardGapSeconds int
	// Friends.
	FriendsMax               int
	FriendRequestsPerDay     int
	EmailSearchesPerDay      int
	FriendRequestsPendingMax int
	FriendSearchPerMinute    int
	FriendRequestExpireDays  int
	FriendRedeclineDays      int
	// Regions (social-signal-plan.md Part B).
	CountryAtMaxKm float64
	// Anti-cheat (social-signal-plan.md Part C). The two switches are 1 (on) or 0.
	SpoofLockEnabled   int
	SpoofCheckMockFlag int
	SpoofLockMinutes1  int
	SpoofLockMinutes2  int
	SpoofLockMinutes3  int
	SpoofRepeatDays    int
	SpoofTeleportKmh   float64
	SpoofTeleportMinKm float64
	FixMaxAgeSeconds   int
	// Movement patterns (G2): report only.
	SpoofPatternEnabled  int
	SpoofPatternFixes    int
	SpoofPatternGapHours int

	// Moderation (docs/profile-plan.md, M1).
	SuspendDays1      int // the three lengths a suspension may have
	SuspendDays2      int
	SuspendDays3      int
	ReportsPerDay     int // reports one player may make in 24 hours
	ReportContextDays int // how far back a report's attached interactions go
	SuspendRepeatDays int // a second suspension inside this many days suggests the next step up
	// A failed location check of one kind is stored at most once per player in this many minutes.
	InfringementGapMinutes int

	// CallsignChangeDays is how long a player waits between callsign changes (the first pick is free).
	CallsignChangeDays int

	AccessTokenMinutes int
	// TokensValidAfter is a unix time: access tokens issued before it are refused (set when player
	// ids were moved, migration 0043). 0 refuses none.
	TokensValidAfter   float64
	SessionDays        int
	DeleteGraceDays    int
	InactiveDeleteDays int
}

func DefaultSettings() Settings {
	return Settings{
		LevelBaseXp:      100,
		LevelGrowth:      1.25,
		LeadFactionLevel: 20,
		JoinFactionLevel: 20,
		XpHarvest:        10,
		XpCraft:          15,
		XpUpgrade:        20,

		XpAttack:             25,
		XpDestroy:            50,
		XpIntercept:          20,
		XpAttackPerTargetDay: 3,

		HarvestRangeM:       40,
		HarvestMaxAccuracyM: 50,
		HarvestCooldownSec:  300,
		HarvestReserveCost:  0.10,
		MaxTravelSpeedKmh:   80,
		NearbyMaxRadiusM:    1500,

		EchoesPerCell:   3,
		EchoCellM:       400,
		EchoMoveHours:   24,
		EchoViewRadiusM: 150,
		EchoOffsetMinM:  6,
		EchoOffsetMaxM:  18,

		WalkLinkEnabled:       1,
		WalkLinkMaxSize:       4,
		WalkLinkFormM:         30,
		WalkLinkHoldM:         60,
		WalkLinkGraceSec:      45,
		WalkLinkHeartbeatSec:  15,
		WalkLinkMaxAccuracyM:  25,
		WalkLinkFixAgeSec:     20,
		WalkLinkMoveM:         50,
		WalkLinkMoveWindowSec: 180,
		WalkLinkDailyMinutes:  120,
		WalkLinkAskSec:        120,
		WalkLinkOpenSec:       60,
		WalkLinkScanPct2:      25,
		WalkLinkScanPct3:      40,
		WalkLinkScanPct4:      50,
		WalkLinkYieldPct2:     15,
		WalkLinkYieldPct3:     25,
		WalkLinkYieldPct4:     30,
		WalkLinkXpPct2:        10,
		WalkLinkXpPct3:        20,
		WalkLinkXpPct4:        30,
		WalkLinkKeepDays:      90,

		WalkLinkRequireBle:     1,
		WalkLinkBleGraceSec:    60,
		WalkLinkBleTokenSec:    60,
		WalkLinkBleHeardSec:    30,
		WalkLinkTwinFixes:      3,
		WalkLinkTwinM:          1,
		WalkLinkNoBleReportSec: 300,

		GroupNodeRatePerCell: 0.05,
		GroupNodeMixDuo:      60,
		GroupNodeMixTrio:     30,
		GroupNodeMixQuad:     10,
		GroupNodeYieldX2:     3,
		GroupNodeYieldX3:     5,
		GroupNodeYieldX4:     8,
		GroupNodeHours:       48,
		LinkMissionChancePct: 25,
		WalkMilestoneKm1:     5,
		WalkMilestoneKm2:     25,
		WalkMilestoneKm3:     100,
		WalkMilestoneKm4:     500,

		WalkLockstepSteps:   8,
		WalkLockstepM:       0.5,
		WalkLockstepPairSec: 5,

		WelcomeCrystal: 800,
		WelcomeAlloy:   600,
		WelcomeData:    200,
		WelcomeCredit:  100,
		WelcomeCells:   5,

		BaseSpacingSameFactionM:  3000,
		BaseSpacingOtherFactionM: 150,

		BasesEnabled:              1,
		BaseZoneGapM:              30,
		BaseSensitiveBufferM:      50,
		BaseWalkwayM:              15,
		BaseSpotAccuracyM:         20,
		BaseProposalHours:         48,
		BaseProposalCooldownHours: 24,
		BaseZoneT1:                25,
		BaseZoneT2:                40,
		BaseZoneT3:                60,
		BaseZoneT4:                80,
		BaseZoneT5:                100,
		BaseSlotsT1:               4,
		BaseSlotsT2:               8,
		BaseSlotsT3:               12,
		BaseSlotsT4:               18,
		BaseSlotsT5:               24,
		BaseCoreT1:                500,
		BaseCoreT2:                1500,
		BaseCoreT3:                3500,
		BaseCoreT4:                7000,
		BaseCoreT5:                12000,
		BaseCapBase:               2,
		BaseCapStep1Players:       50,
		BaseCapStep2Players:       150,
		BaseCapStep3Players:       300,
		BaseCapStep4Players:       600,
		BaseCapActiveDays:         14,
		ProspectDays:              7,
		ProspectPoints:            100,
		CaptainSquadSize:          10,
		CaptainVaultDailyCap:      500,
		CaptainMissionsPerDay:     2,
		CaptainMissionGiveMax:     500,
		CaptainMissionStandMaxMin: 30,
		RankPointsCaptain:         1000,
		RankPointsWarden:          2000,
		RankPointsLead:            5000,
		RankSlotsCaptain:          12,
		RankSlotsWarden:           6,
		RankSlotsLead:             3,
		NomadMoveCooldownDays:     30,
		NomadAskHours:             48,
		CommanderInactiveDays:     14,
		CommanderVoteDays:         7,
		CommanderVoteHours:        48,
		BaseProjectsAtOnce:        2,
		BasePresenceStepPct:       10,
		BasePresenceMaxMult:       2,
		BasePresenceSec:           90,
		BasePointsPerUnit:         1,
		BaseGiveXpPer100:          5,
		BaseModuleHealth:          300,
		BaseProtectNewHours:       24,
		BasePerkForgeAssemblyPct:  25,
		BasePerkVerdantCostPct:    15,
		BaseModuleCost: map[string][4]int{
			"lattice": {300, 200, 100, 50}, "shield": {250, 300, 150, 50},
			"vault": {400, 100, 100, 100}, "uplink": {200, 150, 200, 50}, "collector": {350, 250, 150, 100},
		},
		BaseModuleMin:            map[string]float64{"lattice": 30, "shield": 30, "vault": 30, "uplink": 30, "collector": 45},
		BaseCollectorLinks:       [MaxModuleMk + 1]int{0, 2, 3, 3, 4},
		BasePerkLumenCrystalPct:  25,
		BaseProtectDropHours:     12,
		BaseModulesPerLattice:    4,
		BaseHackRangeM:           75,
		BaseHackData:             50,
		BaseHackMinPct:           5,
		BaseHackMaxPct:           15,
		BaseHackCooldownMin:      30,
		BaseSafeFloorPct:         20,
		BasePerkBastionFloorPct:  35,
		BasePerkBastionShieldPct: 10,
		BaseAttackPushMin:        30,
		BaseDestroyDropPct:       50,
		BasePackedRefundPct:      50,
		BaseUpkeepDays:           7,
		BaseFadeDropDays:         7,
		BaseSpireCollapseDays:    14,
		BaseFadedShieldPct:       50,
		BaseCost: [BaseTopTier + 1][4]int{{}, {400, 400, 150, 50}, {1200, 600, 400, 200}, {3000, 1500, 1000, 500},
			{7000, 3500, 2500, 1200}, {15000, 8000, 6000, 3000}},
		BaseAssemblyMin: [BaseTopTier + 1]float64{0, 15, 120, 480, 1440, 4320},

		TnMaxPerPlayer:       3,
		TnLinkRadiusM:        150,
		TnMinSpacingM:        10,
		TnLinksTier1:         1,
		TnLinksTier2:         4,
		TnUpgradeAlloy:       120,
		TnUpgradeData:        60,
		TnUpgradeCredit:      40,
		TnInfoRangeM:         40,
		TnLifetimeHours:      48,
		TnMaxHealth:          100,
		TnDismantleRefund:    0.5,
		TnOwnerShare:         0.5,
		TnFullSpeedHours:     2,
		TnSlowPctPerHour:     10,
		TnSlowFloorPct:       20,
		TnUploadFee:          0.05,
		TnBuildCrystal:       80,
		TnBuildAlloy:         60,
		TnBuildData:          20,
		NodeDepletedReserve:  0.10,
		NodeRespawnMinM:      100,
		NodeRespawnMaxM:      300,
		DropLifetimeHours:    48,
		PvpLevelBand:         5,
		ProtectionLossDays:   3,
		XpTnBuild:            20,
		TnUnderAttackMinutes: 10,
		TnEndedShowMinutes:   60,

		RepairAlloyPerPoint: 0.5,
		CraftCellCrystal:    10,
		CraftCellData:       5,
		CraftPulseCrystal:   20,
		CraftPulseData:      10,
		CraftJammerData:     15,
		CraftJammerAlloy:    10,
		CraftSpikeAlloy:     30,
		CraftSpikeCrystal:   20,
		CraftSpikeCredit:    10,

		WeaponRangeM:         40,
		WarMaxHeightM:        10,
		EmPulseDamage:        25,
		JammerMinutes:        30,
		SpikeDamage:          40,
		SpikeFuseMinutes:     10,
		CellsEmPulse:         1,
		CellsJammer:          1,
		CellsSpike:           1,
		ContestSeconds:       60,
		ContestPrizeHarvests: 3,
		CellsContest:         1,
		ContestRangeM:        40,
		ContestRecentMinutes: 5,
		ContestHoldMinutes:   30,
		ContestGearPerTier:   5,
		ContestAllyPoints:    5,
		ContestJoinPoints:    5,

		DroneSpeedMps:     15,
		DroneMinFlightSec: 20,

		DroneLaunchWear:       5,
		DroneResultsMinutes:   10,
		DroneRangeRingDefault: 1,
		CellsIntercept:        1,
		InterceptShareMk1:     1.0,
		InterceptShareMk2:     0.75,
		InterceptShareMk3:     0.5,
		InterceptShareMk4:     0.25,
		TnShieldBase:          0.30,
		TnShieldStep:          0.15,
		TnShieldMin:           0.05,
		TnShieldMax:           0.75,
		ShieldLevelMk1:        5,
		ShieldLevelMk2:        10,
		ShieldLevelMk3:        15,
		ShieldLevelMk4:        20,
		CraftShield1Crystal:   30,
		CraftShield1Alloy:     20,
		CraftShield2Alloy:     80,
		CraftShield2Data:      40,
		CraftShield3Alloy:     160,
		CraftShield3Data:      80,
		CraftShield3Credit:    40,
		CraftShield4Alloy:     800,
		CraftShield4Data:      400,
		CraftShield4Credit:    200,

		MissionsPerDay:          6,
		MissionDayOffsetMinutes: 300,
		MissionXpBase:           30,
		MissionXpPerLevel:       5,
		MissionCreditBase:       5,
		MissionCreditPerLevel:   1,
		MissionItemChance:       0.25,
		MissionCommandMult:      2,

		CallPathLevel:        10,
		CallRepeatHours:      24,
		CallMessageHours:     48,
		CallRingSeconds:      60,
		CallPushDelaySeconds: 45,

		SeedCellM:         250,
		SeedPerCell:       4,
		SeedPathPerNodeM:  40,
		SeedSnapM:         15,
		SeedMinGapM:       25,
		SeedTnGapM:        10,
		SeedRegenPerHour:  0.05,
		SeedWeightCrystal: 4,
		SeedWeightAlloy:   3,
		SeedWeightData:    2,
		SeedWeightCredit:  1,
		SeedYieldCrystal:  12,
		SeedYieldAlloy:    10,
		SeedYieldData:     6,
		SeedYieldCredit:   8,
		SeedCellsPerScan:  4,
		SeedTopUpPerTick:  10,
		SeedTopUpMinutes:  30,
		RarePerKm2Day:     0.25,
		RareMinHours:      2,
		RareMaxHours:      4,
		RareYieldMult:     3,

		PushEnabled:               1,
		PushAttackThrottleMinutes: 10,
		PushMaxAgeMinutes:         15,

		PerkSignumCrystalPct:       25,
		PerkShingoUpgradePct:       25,
		PerkSemaScanPct:            100,
		PerkSemaRarePct:            100,
		PerkIsharaBuildPct:         20,
		PerkIsharaMissionCreditPct: 25,
		PerkTohuDroneSpeedPct:      30,
		PerkTohuCarryPct:           25,
		PerkMerkiDamagePct:         15,
		PerkMerkiInterceptPct:      10,
		FavourPerkPct:              40,
		MemberMissionsPerDay:       1,
		LangisCallLevel:            18,
		LangisMessageDelayHours:    2,
		LangisCallDelayHours:       24,
		LangisRareEveryDays:        7,
		TitleDrifterLevel:          22,
		TitleWayfarerLevel:         26,
		TitleUnboundLevel:          30,
		JobSlotsMax:                3,
		JobHoursMin:                1,
		JobHoursMax:                24,
		JobRewardMin:               10,
		JobRewardMax:               2000,
		JobsOpenPerFaction:         20,
		JobsPostedPerDay:           5,
		JobsTakenMax:               3,
		JobScoutRadiusM:            150,
		JobGuardMinutes:            60,
		JobGuardRadiusM:            40,
		JobGuardGapSeconds:         90,
		FriendsMax:                 200,
		FriendRequestsPerDay:       20,
		EmailSearchesPerDay:        10,
		FriendRequestsPendingMax:   50,
		FriendSearchPerMinute:      20,
		FriendRequestExpireDays:    14,
		FriendRedeclineDays:        7,
		CountryAtMaxKm:             60,
		SpoofLockEnabled:           1,
		SpoofCheckMockFlag:         1,
		SpoofLockMinutes1:          30,
		SpoofLockMinutes2:          120,
		SpoofLockMinutes3:          720,
		SpoofRepeatDays:            7,
		SpoofTeleportKmh:           300,
		SpoofTeleportMinKm:         5,
		FixMaxAgeSeconds:           120,
		SpoofPatternEnabled:        1,
		SpoofPatternFixes:          30,
		SpoofPatternGapHours:       24,

		SuspendDays1:      1,
		SuspendDays2:      3,
		SuspendDays3:      7,
		ReportsPerDay:     5,
		ReportContextDays: 7,
		SuspendRepeatDays: 30,

		InfringementGapMinutes: 10,

		CallsignChangeDays: 30,

		AccessTokenMinutes: 15,
		SessionDays:        60,
		DeleteGraceDays:    7,
		InactiveDeleteDays: 365,
	}
}

// SettingsFrom overlays table values on the defaults. Unknown keys are ignored.
func SettingsFrom(values map[string]float64) Settings {
	s := DefaultSettings()
	f := func(key string, dst *float64) {
		if v, ok := values[key]; ok {
			*dst = v
		}
	}
	i := func(key string, dst *int) {
		if v, ok := values[key]; ok {
			*dst = int(v)
		}
	}

	f("LevelBaseXp", &s.LevelBaseXp)
	f("LevelGrowth", &s.LevelGrowth)
	i("LeadFactionLevel", &s.LeadFactionLevel)
	i("JoinFactionLevel", &s.JoinFactionLevel)
	i("XpHarvest", &s.XpHarvest)
	i("XpCraft", &s.XpCraft)
	i("XpUpgrade", &s.XpUpgrade)
	i("XpAttack", &s.XpAttack)
	i("XpDestroy", &s.XpDestroy)
	i("XpIntercept", &s.XpIntercept)
	i("XpAttackPerTargetDay", &s.XpAttackPerTargetDay)

	f("HarvestRangeM", &s.HarvestRangeM)
	f("HarvestMaxAccuracyM", &s.HarvestMaxAccuracyM)
	i("HarvestCooldownSec", &s.HarvestCooldownSec)
	f("HarvestReserveCost", &s.HarvestReserveCost)
	f("MaxTravelSpeedKmh", &s.MaxTravelSpeedKmh)
	f("NearbyMaxRadiusM", &s.NearbyMaxRadiusM)

	i("EchoesPerCell", &s.EchoesPerCell)
	f("EchoCellM", &s.EchoCellM)
	f("EchoMoveHours", &s.EchoMoveHours)
	f("EchoViewRadiusM", &s.EchoViewRadiusM)
	f("EchoOffsetMinM", &s.EchoOffsetMinM)
	f("EchoOffsetMaxM", &s.EchoOffsetMaxM)

	i("WalkLinkEnabled", &s.WalkLinkEnabled)
	i("WalkLinkMaxSize", &s.WalkLinkMaxSize)
	f("WalkLinkFormM", &s.WalkLinkFormM)
	f("WalkLinkHoldM", &s.WalkLinkHoldM)
	i("WalkLinkGraceSec", &s.WalkLinkGraceSec)
	i("WalkLinkHeartbeatSec", &s.WalkLinkHeartbeatSec)
	f("WalkLinkMaxAccuracyM", &s.WalkLinkMaxAccuracyM)
	i("WalkLinkFixAgeSec", &s.WalkLinkFixAgeSec)
	f("WalkLinkMoveM", &s.WalkLinkMoveM)
	i("WalkLinkMoveWindowSec", &s.WalkLinkMoveWindowSec)
	i("WalkLinkDailyMinutes", &s.WalkLinkDailyMinutes)
	i("WalkLinkAskSec", &s.WalkLinkAskSec)
	i("WalkLinkOpenSec", &s.WalkLinkOpenSec)
	i("WalkLinkScanPct2", &s.WalkLinkScanPct2)
	i("WalkLinkScanPct3", &s.WalkLinkScanPct3)
	i("WalkLinkScanPct4", &s.WalkLinkScanPct4)
	i("WalkLinkYieldPct2", &s.WalkLinkYieldPct2)
	i("WalkLinkYieldPct3", &s.WalkLinkYieldPct3)
	i("WalkLinkYieldPct4", &s.WalkLinkYieldPct4)
	i("WalkLinkXpPct2", &s.WalkLinkXpPct2)
	i("WalkLinkXpPct3", &s.WalkLinkXpPct3)
	i("WalkLinkXpPct4", &s.WalkLinkXpPct4)
	i("WalkLinkKeepDays", &s.WalkLinkKeepDays)
	i("WalkLinkRequireBle", &s.WalkLinkRequireBle)
	i("WalkLinkBleGraceSec", &s.WalkLinkBleGraceSec)
	i("WalkLinkBleTokenSec", &s.WalkLinkBleTokenSec)
	i("WalkLinkBleHeardSec", &s.WalkLinkBleHeardSec)
	i("WalkLinkTwinFixes", &s.WalkLinkTwinFixes)
	f("WalkLinkTwinM", &s.WalkLinkTwinM)
	i("WalkLinkNoBleReportSec", &s.WalkLinkNoBleReportSec)
	f("GroupNodeRatePerCell", &s.GroupNodeRatePerCell)
	i("GroupNodeMixDuo", &s.GroupNodeMixDuo)
	i("GroupNodeMixTrio", &s.GroupNodeMixTrio)
	i("GroupNodeMixQuad", &s.GroupNodeMixQuad)
	f("GroupNodeYieldX2", &s.GroupNodeYieldX2)
	f("GroupNodeYieldX3", &s.GroupNodeYieldX3)
	f("GroupNodeYieldX4", &s.GroupNodeYieldX4)
	f("GroupNodeHours", &s.GroupNodeHours)
	i("LinkMissionChancePct", &s.LinkMissionChancePct)
	i("WalkMilestoneKm1", &s.WalkMilestoneKm1)
	i("WalkMilestoneKm2", &s.WalkMilestoneKm2)
	i("WalkMilestoneKm3", &s.WalkMilestoneKm3)
	i("WalkMilestoneKm4", &s.WalkMilestoneKm4)
	i("WalkLockstepSteps", &s.WalkLockstepSteps)
	f("WalkLockstepM", &s.WalkLockstepM)
	f("WalkLockstepPairSec", &s.WalkLockstepPairSec)

	i("WelcomeCrystal", &s.WelcomeCrystal)
	i("WelcomeAlloy", &s.WelcomeAlloy)
	i("WelcomeData", &s.WelcomeData)
	i("WelcomeCredit", &s.WelcomeCredit)
	i("WelcomeCells", &s.WelcomeCells)

	f("BaseSpacingSameFactionM", &s.BaseSpacingSameFactionM)
	f("BaseSpacingOtherFactionM", &s.BaseSpacingOtherFactionM)
	i("BasesEnabled", &s.BasesEnabled)
	f("BaseZoneGapM", &s.BaseZoneGapM)
	f("BaseSensitiveBufferM", &s.BaseSensitiveBufferM)
	f("BaseWalkwayM", &s.BaseWalkwayM)
	f("BaseSpotAccuracyM", &s.BaseSpotAccuracyM)
	f("BaseProposalHours", &s.BaseProposalHours)
	f("BaseProposalCooldownHours", &s.BaseProposalCooldownHours)
	f("BaseZoneT1", &s.BaseZoneT1)
	f("BaseZoneT2", &s.BaseZoneT2)
	f("BaseZoneT3", &s.BaseZoneT3)
	f("BaseZoneT4", &s.BaseZoneT4)
	f("BaseZoneT5", &s.BaseZoneT5)
	i("BaseSlotsT1", &s.BaseSlotsT1)
	i("BaseSlotsT2", &s.BaseSlotsT2)
	i("BaseSlotsT3", &s.BaseSlotsT3)
	i("BaseSlotsT4", &s.BaseSlotsT4)
	i("BaseSlotsT5", &s.BaseSlotsT5)
	i("BaseCoreT1", &s.BaseCoreT1)
	i("BaseCoreT2", &s.BaseCoreT2)
	i("BaseCoreT3", &s.BaseCoreT3)
	i("BaseCoreT4", &s.BaseCoreT4)
	i("BaseCoreT5", &s.BaseCoreT5)
	i("BaseCapBase", &s.BaseCapBase)
	i("BaseCapStep1Players", &s.BaseCapStep1Players)
	i("BaseCapStep2Players", &s.BaseCapStep2Players)
	i("BaseCapStep3Players", &s.BaseCapStep3Players)
	i("BaseCapStep4Players", &s.BaseCapStep4Players)
	f("BaseCapActiveDays", &s.BaseCapActiveDays)
	f("ProspectDays", &s.ProspectDays)
	i("ProspectPoints", &s.ProspectPoints)
	i("CaptainSquadSize", &s.CaptainSquadSize)
	i("CaptainVaultDailyCap", &s.CaptainVaultDailyCap)
	i("CaptainMissionsPerDay", &s.CaptainMissionsPerDay)
	i("CaptainMissionGiveMax", &s.CaptainMissionGiveMax)
	i("CaptainMissionStandMaxMin", &s.CaptainMissionStandMaxMin)
	i("RankPointsCaptain", &s.RankPointsCaptain)
	i("RankPointsWarden", &s.RankPointsWarden)
	i("RankPointsLead", &s.RankPointsLead)
	i("RankSlotsCaptain", &s.RankSlotsCaptain)
	i("RankSlotsWarden", &s.RankSlotsWarden)
	i("RankSlotsLead", &s.RankSlotsLead)
	f("NomadMoveCooldownDays", &s.NomadMoveCooldownDays)
	f("NomadAskHours", &s.NomadAskHours)
	f("CommanderInactiveDays", &s.CommanderInactiveDays)
	f("CommanderVoteDays", &s.CommanderVoteDays)
	f("CommanderVoteHours", &s.CommanderVoteHours)
	i("BaseProjectsAtOnce", &s.BaseProjectsAtOnce)
	f("BasePresenceStepPct", &s.BasePresenceStepPct)
	f("BasePresenceMaxMult", &s.BasePresenceMaxMult)
	f("BasePresenceSec", &s.BasePresenceSec)
	f("BasePointsPerUnit", &s.BasePointsPerUnit)
	i("BaseGiveXpPer100", &s.BaseGiveXpPer100)
	i("BaseModuleHealth", &s.BaseModuleHealth)
	f("BaseProtectNewHours", &s.BaseProtectNewHours)
	f("BasePerkForgeAssemblyPct", &s.BasePerkForgeAssemblyPct)
	f("BasePerkVerdantCostPct", &s.BasePerkVerdantCostPct)
	f("BasePerkLumenCrystalPct", &s.BasePerkLumenCrystalPct)
	f("BaseProtectDropHours", &s.BaseProtectDropHours)
	i("BaseModulesPerLattice", &s.BaseModulesPerLattice)
	f("BaseHackRangeM", &s.BaseHackRangeM)
	i("BaseHackData", &s.BaseHackData)
	f("BaseHackMinPct", &s.BaseHackMinPct)
	f("BaseHackMaxPct", &s.BaseHackMaxPct)
	f("BaseHackCooldownMin", &s.BaseHackCooldownMin)
	f("BaseSafeFloorPct", &s.BaseSafeFloorPct)
	f("BasePerkBastionFloorPct", &s.BasePerkBastionFloorPct)
	f("BasePerkBastionShieldPct", &s.BasePerkBastionShieldPct)
	f("BaseAttackPushMin", &s.BaseAttackPushMin)
	f("BaseDestroyDropPct", &s.BaseDestroyDropPct)
	f("BasePackedRefundPct", &s.BasePackedRefundPct)
	f("BaseUpkeepDays", &s.BaseUpkeepDays)
	f("BaseFadeDropDays", &s.BaseFadeDropDays)
	f("BaseSpireCollapseDays", &s.BaseSpireCollapseDays)
	f("BaseFadedShieldPct", &s.BaseFadedShieldPct)
	for mk := 1; mk <= MaxModuleMk; mk++ {
		i(fmt.Sprintf("BaseCollectorLinksMk%d", mk), &s.BaseCollectorLinks[mk])
	}
	// Copy the module maps: the defaults are shared by every Settings value.
	cost := map[string][4]int{}
	minutes := map[string]float64{}
	for k, v := range s.BaseModuleCost {
		cost[k] = v
	}
	for k, v := range s.BaseModuleMin {
		minutes[k] = v
	}
	for _, m := range BaseModules {
		key := "BaseMod" + strings.ToUpper(m[:1]) + m[1:]
		c := cost[m]
		for k, res := range []string{"Alloy", "Crystal", "Data", "Credit"} {
			i(key+res, &c[k])
		}
		cost[m] = c
		mm := minutes[m]
		f(key+"Min", &mm)
		minutes[m] = mm
	}
	s.BaseModuleCost, s.BaseModuleMin = cost, minutes
	for t := 1; t <= BaseTopTier; t++ {
		for k, res := range []string{"Alloy", "Crystal", "Data", "Credit"} {
			i(fmt.Sprintf("BaseCostT%d%s", t, res), &s.BaseCost[t][k])
		}
		f(fmt.Sprintf("BaseAssemblyMinT%d", t), &s.BaseAssemblyMin[t])
	}

	i("TnMaxPerPlayer", &s.TnMaxPerPlayer)
	f("TnLinkRadiusM", &s.TnLinkRadiusM)
	f("TnMinSpacingM", &s.TnMinSpacingM)
	i("TnLinksTier1", &s.TnLinksTier1)
	i("TnLinksTier2", &s.TnLinksTier2)
	i("TnUpgradeAlloy", &s.TnUpgradeAlloy)
	i("TnUpgradeData", &s.TnUpgradeData)
	i("TnUpgradeCredit", &s.TnUpgradeCredit)
	f("TnInfoRangeM", &s.TnInfoRangeM)
	f("TnLifetimeHours", &s.TnLifetimeHours)
	f("TnMaxHealth", &s.TnMaxHealth)
	f("TnDismantleRefund", &s.TnDismantleRefund)
	f("TnOwnerShare", &s.TnOwnerShare)
	f("TnFullSpeedHours", &s.TnFullSpeedHours)
	f("TnSlowPctPerHour", &s.TnSlowPctPerHour)
	f("TnSlowFloorPct", &s.TnSlowFloorPct)
	f("TnUploadFee", &s.TnUploadFee)
	i("TnBuildCrystal", &s.TnBuildCrystal)
	i("TnBuildAlloy", &s.TnBuildAlloy)
	i("TnBuildData", &s.TnBuildData)
	f("NodeDepletedReserve", &s.NodeDepletedReserve)
	f("NodeRespawnMinM", &s.NodeRespawnMinM)
	f("NodeRespawnMaxM", &s.NodeRespawnMaxM)
	f("DropLifetimeHours", &s.DropLifetimeHours)
	i("PvpLevelBand", &s.PvpLevelBand)
	f("ProtectionLossDays", &s.ProtectionLossDays)
	i("XpTnBuild", &s.XpTnBuild)
	f("TnUnderAttackMinutes", &s.TnUnderAttackMinutes)
	f("TnEndedShowMinutes", &s.TnEndedShowMinutes)
	i("MinAppBuild", &s.MinAppBuild)
	i("MinAppBuildIOS", &s.MinAppBuildIOS)

	f("RepairAlloyPerPoint", &s.RepairAlloyPerPoint)
	i("CraftCellCrystal", &s.CraftCellCrystal)
	i("CraftCellData", &s.CraftCellData)
	i("CraftPulseCrystal", &s.CraftPulseCrystal)
	i("CraftPulseData", &s.CraftPulseData)
	i("CraftJammerData", &s.CraftJammerData)
	i("CraftJammerAlloy", &s.CraftJammerAlloy)
	i("CraftSpikeAlloy", &s.CraftSpikeAlloy)
	i("CraftSpikeCrystal", &s.CraftSpikeCrystal)
	i("CraftSpikeCredit", &s.CraftSpikeCredit)

	f("WeaponRangeM", &s.WeaponRangeM)
	f("WarMaxHeightM", &s.WarMaxHeightM)
	f("EmPulseDamage", &s.EmPulseDamage)
	f("JammerMinutes", &s.JammerMinutes)
	f("SpikeDamage", &s.SpikeDamage)
	f("SpikeFuseMinutes", &s.SpikeFuseMinutes)
	i("CellsEmPulse", &s.CellsEmPulse)
	i("CellsJammer", &s.CellsJammer)
	i("CellsSpike", &s.CellsSpike)
	i("ContestSeconds", &s.ContestSeconds)
	i("ContestPrizeHarvests", &s.ContestPrizeHarvests)
	i("CellsContest", &s.CellsContest)
	f("ContestRangeM", &s.ContestRangeM)
	f("ContestRecentMinutes", &s.ContestRecentMinutes)
	f("ContestHoldMinutes", &s.ContestHoldMinutes)
	i("ContestGearPerTier", &s.ContestGearPerTier)
	i("ContestAllyPoints", &s.ContestAllyPoints)
	i("ContestJoinPoints", &s.ContestJoinPoints)

	f("DroneSpeedMps", &s.DroneSpeedMps)
	f("DroneMinFlightSec", &s.DroneMinFlightSec)
	i("DroneLaunchWear", &s.DroneLaunchWear)
	f("DroneResultsMinutes", &s.DroneResultsMinutes)
	i("DroneRangeRingDefault", &s.DroneRangeRingDefault)
	i("CellsIntercept", &s.CellsIntercept)
	f("InterceptShareMk1", &s.InterceptShareMk1)
	f("InterceptShareMk2", &s.InterceptShareMk2)
	f("InterceptShareMk3", &s.InterceptShareMk3)
	f("InterceptShareMk4", &s.InterceptShareMk4)
	f("TnShieldBase", &s.TnShieldBase)
	f("TnShieldStep", &s.TnShieldStep)
	f("TnShieldMin", &s.TnShieldMin)
	f("TnShieldMax", &s.TnShieldMax)
	i("ShieldLevelMk1", &s.ShieldLevelMk1)
	i("ShieldLevelMk2", &s.ShieldLevelMk2)
	i("ShieldLevelMk3", &s.ShieldLevelMk3)
	i("ShieldLevelMk4", &s.ShieldLevelMk4)
	i("CraftShield1Crystal", &s.CraftShield1Crystal)
	i("CraftShield1Alloy", &s.CraftShield1Alloy)
	i("CraftShield2Alloy", &s.CraftShield2Alloy)
	i("CraftShield2Data", &s.CraftShield2Data)
	i("CraftShield3Alloy", &s.CraftShield3Alloy)
	i("CraftShield3Data", &s.CraftShield3Data)
	i("CraftShield3Credit", &s.CraftShield3Credit)
	i("CraftShield4Alloy", &s.CraftShield4Alloy)
	i("CraftShield4Data", &s.CraftShield4Data)
	i("CraftShield4Credit", &s.CraftShield4Credit)
	i("MissionsPerDay", &s.MissionsPerDay)
	i("FavourPerkPct", &s.FavourPerkPct)
	i("MemberMissionsPerDay", &s.MemberMissionsPerDay)
	i("LangisCallLevel", &s.LangisCallLevel)
	f("LangisMessageDelayHours", &s.LangisMessageDelayHours)
	f("LangisCallDelayHours", &s.LangisCallDelayHours)
	i("LangisRareEveryDays", &s.LangisRareEveryDays)
	i("TitleDrifterLevel", &s.TitleDrifterLevel)
	i("TitleWayfarerLevel", &s.TitleWayfarerLevel)
	i("TitleUnboundLevel", &s.TitleUnboundLevel)
	i("JobSlotsMax", &s.JobSlotsMax)
	i("JobHoursMin", &s.JobHoursMin)
	i("JobHoursMax", &s.JobHoursMax)
	i("JobRewardMin", &s.JobRewardMin)
	i("JobRewardMax", &s.JobRewardMax)
	i("JobsOpenPerFaction", &s.JobsOpenPerFaction)
	i("JobsPostedPerDay", &s.JobsPostedPerDay)
	i("JobsTakenMax", &s.JobsTakenMax)
	i("JobScoutRadiusM", &s.JobScoutRadiusM)
	i("JobGuardMinutes", &s.JobGuardMinutes)
	i("JobGuardRadiusM", &s.JobGuardRadiusM)
	i("JobGuardGapSeconds", &s.JobGuardGapSeconds)
	i("FriendsMax", &s.FriendsMax)
	i("FriendRequestsPerDay", &s.FriendRequestsPerDay)
	i("EmailSearchesPerDay", &s.EmailSearchesPerDay)
	i("FriendRequestsPendingMax", &s.FriendRequestsPendingMax)
	i("FriendSearchPerMinute", &s.FriendSearchPerMinute)
	i("FriendRequestExpireDays", &s.FriendRequestExpireDays)
	i("FriendRedeclineDays", &s.FriendRedeclineDays)
	f("CountryAtMaxKm", &s.CountryAtMaxKm)
	i("SpoofLockEnabled", &s.SpoofLockEnabled)
	i("SpoofCheckMockFlag", &s.SpoofCheckMockFlag)
	i("SpoofLockMinutes1", &s.SpoofLockMinutes1)
	i("SpoofLockMinutes2", &s.SpoofLockMinutes2)
	i("SpoofLockMinutes3", &s.SpoofLockMinutes3)
	i("SpoofRepeatDays", &s.SpoofRepeatDays)
	f("SpoofTeleportKmh", &s.SpoofTeleportKmh)
	f("SpoofTeleportMinKm", &s.SpoofTeleportMinKm)
	i("FixMaxAgeSeconds", &s.FixMaxAgeSeconds)
	i("SpoofPatternEnabled", &s.SpoofPatternEnabled)
	i("SpoofPatternFixes", &s.SpoofPatternFixes)
	i("SpoofPatternGapHours", &s.SpoofPatternGapHours)
	i("MissionDayOffsetMinutes", &s.MissionDayOffsetMinutes)
	i("MissionXpBase", &s.MissionXpBase)
	i("MissionXpPerLevel", &s.MissionXpPerLevel)
	i("MissionCreditBase", &s.MissionCreditBase)
	i("MissionCreditPerLevel", &s.MissionCreditPerLevel)
	f("MissionItemChance", &s.MissionItemChance)
	f("MissionCommandMult", &s.MissionCommandMult)
	i("CallPathLevel", &s.CallPathLevel)
	f("CallRepeatHours", &s.CallRepeatHours)
	f("CallMessageHours", &s.CallMessageHours)
	i("CallRingSeconds", &s.CallRingSeconds)
	i("CallPushDelaySeconds", &s.CallPushDelaySeconds)
	f("SeedCellM", &s.SeedCellM)
	i("SeedPerCell", &s.SeedPerCell)
	f("SeedPathPerNodeM", &s.SeedPathPerNodeM)
	f("SeedSnapM", &s.SeedSnapM)
	f("SeedMinGapM", &s.SeedMinGapM)
	f("SeedTnGapM", &s.SeedTnGapM)
	f("SeedRegenPerHour", &s.SeedRegenPerHour)
	i("SeedWeightCrystal", &s.SeedWeightCrystal)
	i("SeedWeightAlloy", &s.SeedWeightAlloy)
	i("SeedWeightData", &s.SeedWeightData)
	i("SeedWeightCredit", &s.SeedWeightCredit)
	i("SeedYieldCrystal", &s.SeedYieldCrystal)
	i("SeedYieldAlloy", &s.SeedYieldAlloy)
	i("SeedYieldData", &s.SeedYieldData)
	i("SeedYieldCredit", &s.SeedYieldCredit)
	i("SeedCellsPerScan", &s.SeedCellsPerScan)
	i("SeedTopUpPerTick", &s.SeedTopUpPerTick)
	f("SeedTopUpMinutes", &s.SeedTopUpMinutes)
	f("RarePerKm2Day", &s.RarePerKm2Day)
	f("RareMinHours", &s.RareMinHours)
	f("RareMaxHours", &s.RareMaxHours)
	f("RareYieldMult", &s.RareYieldMult)
	i("PushEnabled", &s.PushEnabled)
	i("PushAttackThrottleMinutes", &s.PushAttackThrottleMinutes)
	i("PushMaxAgeMinutes", &s.PushMaxAgeMinutes)

	i("PerkSignumCrystalPct", &s.PerkSignumCrystalPct)
	i("PerkShingoUpgradePct", &s.PerkShingoUpgradePct)
	i("PerkSemaScanPct", &s.PerkSemaScanPct)
	i("PerkSemaRarePct", &s.PerkSemaRarePct)
	i("PerkIsharaBuildPct", &s.PerkIsharaBuildPct)
	i("PerkIsharaMissionCreditPct", &s.PerkIsharaMissionCreditPct)
	i("PerkTohuDroneSpeedPct", &s.PerkTohuDroneSpeedPct)
	i("PerkTohuCarryPct", &s.PerkTohuCarryPct)
	i("PerkMerkiDamagePct", &s.PerkMerkiDamagePct)
	i("PerkMerkiInterceptPct", &s.PerkMerkiInterceptPct)

	i("SuspendDays1", &s.SuspendDays1)
	i("SuspendDays2", &s.SuspendDays2)
	i("SuspendDays3", &s.SuspendDays3)
	i("ReportsPerDay", &s.ReportsPerDay)
	i("ReportContextDays", &s.ReportContextDays)
	i("SuspendRepeatDays", &s.SuspendRepeatDays)
	i("InfringementGapMinutes", &s.InfringementGapMinutes)

	i("CallsignChangeDays", &s.CallsignChangeDays)

	i("AccessTokenMinutes", &s.AccessTokenMinutes)
	f("TokensValidAfter", &s.TokensValidAfter)
	i("SessionDays", &s.SessionDays)
	i("DeleteGraceDays", &s.DeleteGraceDays)
	i("InactiveDeleteDays", &s.InactiveDeleteDays)
	return s
}
