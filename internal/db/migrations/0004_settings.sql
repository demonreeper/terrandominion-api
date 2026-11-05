-- 0004 settings: starting values for every tunable rule. Change them with the admin API, not
-- with a new migration. Public ones are also sent to clients.

INSERT INTO td.settings (key, value, is_public, description) VALUES
    -- progression
    ('LevelBaseXp',              100,   true,  'XP needed to go from level 1 to 2'),
    ('LevelGrowth',              1.25,  true,  'Each level needs this many times the XP of the one before'),
    ('LeadFactionLevel',         20,    true,  'Level at which a player may claim an unclaimed faction as its head'),
    ('JoinFactionLevel',         20,    true,  'Level at which a player may join an active faction'),
    ('XpHarvest',                10,    false, 'XP for a successful harvest'),
    ('XpCraft',                  15,    false, 'XP for crafting gear'),
    ('XpUpgrade',                20,    false, 'XP for upgrading gear'),
    -- harvesting
    ('HarvestRangeM',            40,    true,  'Max distance from a node to harvest it, before GPS accuracy allowance'),
    ('HarvestMaxAccuracyM',      50,    true,  'Reject harvests when the phone reports worse GPS accuracy than this'),
    ('HarvestCooldownSec',       300,   true,  'Seconds before the same player can harvest the same node again'),
    ('HarvestReserveCost',       0.10,  false, 'Share of a node''s reserve one harvest uses'),
    ('MaxTravelSpeedKmh',        80,    false, 'Speed between two harvests above which the second is treated as spoofed'),
    ('NearbyMaxRadiusM',         1500,  true,  'Upper bound on the nearby-nodes query radius'),
    -- welcome
    ('WelcomeCrystal',           50,    false, 'Crystal granted at first sign-in, once per phone'),
    ('WelcomeAlloy',             30,    false, 'Alloy granted at first sign-in, once per phone'),
    -- bases
    ('BaseSpacingSameFactionM',  500,   true,  'A faction may have only one base within this distance'),
    ('BaseSpacingOtherFactionM', 150,   true,  'Minimum distance between bases of different factions'),
    -- accounts and sessions
    ('AccessTokenMinutes',       15,    false, 'Lifetime of an access token'),
    ('SessionDays',              60,    false, 'Lifetime of a refresh session without use'),
    ('DeleteGraceDays',          7,     true,  'Self-deletion grace: signing in within this many days cancels it'),
    ('InactiveDeleteDays',       90,    false, 'Accounts unseen this long are scheduled for deletion (0 = off)');
