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

	BaseSpacingSameFactionM  float64
	BaseSpacingOtherFactionM float64

	TnMaxPerPlayer      int
	TnLinkRadiusM       float64
	TnLinksTier1        int
	TnLifetimeHours     float64
	TnMaxHealth         float64
	TnDismantleRefund   float64
	TnOwnerShare        float64
	TnUploadFee         float64
	TnBuildCrystal      int
	TnBuildAlloy        int
	TnBuildData         int
	NodeDepletedReserve float64
	XpTnBuild           int

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

		WelcomeCrystal: 50,
		WelcomeAlloy:   30,

		BaseSpacingSameFactionM:  500,
		BaseSpacingOtherFactionM: 150,

		TnMaxPerPlayer:      3,
		TnLinkRadiusM:       150,
		TnLinksTier1:        1,
		TnLifetimeHours:     48,
		TnMaxHealth:         100,
		TnDismantleRefund:   0.5,
		TnOwnerShare:        0.5,
		TnUploadFee:         0.05,
		TnBuildCrystal:      80,
		TnBuildAlloy:        60,
		TnBuildData:         20,
		NodeDepletedReserve: 0.10,
		XpTnBuild:           20,

		AccessTokenMinutes: 15,
		SessionDays:        60,
		DeleteGraceDays:    7,
		InactiveDeleteDays: 90,
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

	f("BaseSpacingSameFactionM", &s.BaseSpacingSameFactionM)
	f("BaseSpacingOtherFactionM", &s.BaseSpacingOtherFactionM)

	i("TnMaxPerPlayer", &s.TnMaxPerPlayer)
	f("TnLinkRadiusM", &s.TnLinkRadiusM)
	i("TnLinksTier1", &s.TnLinksTier1)
	f("TnLifetimeHours", &s.TnLifetimeHours)
	f("TnMaxHealth", &s.TnMaxHealth)
	f("TnDismantleRefund", &s.TnDismantleRefund)
	f("TnOwnerShare", &s.TnOwnerShare)
	f("TnUploadFee", &s.TnUploadFee)
	i("TnBuildCrystal", &s.TnBuildCrystal)
	i("TnBuildAlloy", &s.TnBuildAlloy)
	i("TnBuildData", &s.TnBuildData)
	f("NodeDepletedReserve", &s.NodeDepletedReserve)
	i("XpTnBuild", &s.XpTnBuild)

	i("AccessTokenMinutes", &s.AccessTokenMinutes)
	i("SessionDays", &s.SessionDays)
	i("DeleteGraceDays", &s.DeleteGraceDays)
	i("InactiveDeleteDays", &s.InactiveDeleteDays)
	return s
}
