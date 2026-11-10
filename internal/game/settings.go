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
	CellsDrone        int

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
		CellsDrone:        1,

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
	i("CellsDrone", &s.CellsDrone)
	i("PushEnabled", &s.PushEnabled)
	i("PushAttackThrottleMinutes", &s.PushAttackThrottleMinutes)
	i("PushMaxAgeMinutes", &s.PushMaxAgeMinutes)

	i("AccessTokenMinutes", &s.AccessTokenMinutes)
	i("SessionDays", &s.SessionDays)
	i("DeleteGraceDays", &s.DeleteGraceDays)
	i("InactiveDeleteDays", &s.InactiveDeleteDays)
	return s
}
