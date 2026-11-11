// Package game holds the rules of Terran Dominion as plain functions over plain values: no SQL,
// no HTTP. The service layer loads state, asks these functions what happens, and stores the
// result. Keeping the rules here makes them unit-testable and lets them move onto another server
// framework (such as Nakama) unchanged.
package game

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

	WelcomeCrystal int
	WelcomeAlloy   int
	WelcomeData    int
	WelcomeCells   int

	BaseSpacingSameFactionM  float64
	BaseSpacingOtherFactionM float64

	TnMaxPerPlayer      int
	TnLinkRadiusM       float64
	TnMinSpacingM       float64
	TnLinksTier1        int
	TnLinksTier2        int
	TnUpgradeAlloy      int
	TnUpgradeData       int
	TnUpgradeCredit     int
	TnInfoRangeM        float64
	TnLifetimeHours     float64
	TnMaxHealth         float64
	TnDismantleRefund   float64
	TnOwnerShare        float64
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

	AccessTokenMinutes int
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

		WelcomeCrystal: 800,
		WelcomeAlloy:   600,
		WelcomeData:    200,
		WelcomeCells:   5,

		BaseSpacingSameFactionM:  500,
		BaseSpacingOtherFactionM: 150,

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
		TnUploadFee:          0.05,
		TnBuildCrystal:       80,
		TnBuildAlloy:         60,
		TnBuildData:          20,
		NodeDepletedReserve:  0.10,
		NodeRespawnMinM:      100,
		NodeRespawnMaxM:      300,
		DropLifetimeHours:    24,
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

	i("WelcomeCrystal", &s.WelcomeCrystal)
	i("WelcomeAlloy", &s.WelcomeAlloy)
	i("WelcomeData", &s.WelcomeData)
	i("WelcomeCells", &s.WelcomeCells)

	f("BaseSpacingSameFactionM", &s.BaseSpacingSameFactionM)
	f("BaseSpacingOtherFactionM", &s.BaseSpacingOtherFactionM)

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

	i("AccessTokenMinutes", &s.AccessTokenMinutes)
	i("SessionDays", &s.SessionDays)
	i("DeleteGraceDays", &s.DeleteGraceDays)
	i("InactiveDeleteDays", &s.InactiveDeleteDays)
	return s
}
