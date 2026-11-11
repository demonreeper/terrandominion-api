-- 0038 admin settings (docs/profile-plan.md, A1): every setting gets a group, a name, a range and
-- a lock, so the in-game admin screen can list, search and change them safely; plus the one-line
-- announcement shown on everyone's main menu.

ALTER TABLE td.settings
    ADD COLUMN grp       text NOT NULL DEFAULT 'other',
    ADD COLUMN label     text NOT NULL DEFAULT '',
    ADD COLUMN min_value double precision NOT NULL DEFAULT 0,
    ADD COLUMN max_value double precision NOT NULL DEFAULT 0,
    -- The plus and minus step. 1 means the value is a whole number.
    ADD COLUMN step      double precision NOT NULL DEFAULT 1,
    -- Locked: shown, never changed from the app (changing it would break what is already stored).
    ADD COLUMN locked    boolean NOT NULL DEFAULT false;

-- Groups, by what the key is about. Anything not matched stays in 'other'.
UPDATE td.settings SET grp = CASE
    WHEN key LIKE 'Tn%' OR key IN ('DropLifetimeHours', 'XpTnBuild') THEN 'tether'
    WHEN key LIKE 'Drone%' OR key LIKE 'InterceptShare%' OR key IN ('CellsIntercept', 'XpIntercept') THEN 'drones'
    WHEN key LIKE 'Contest%' OR key LIKE 'Cells%' OR key LIKE 'ShieldLevel%'
      OR key IN ('EmPulseDamage', 'JammerMinutes', 'SpikeDamage', 'SpikeFuseMinutes', 'WeaponRangeM', 'PvpLevelBand',
                 'ProtectionLossDays', 'XpAttack', 'XpAttackPerTargetDay', 'XpDestroy') THEN 'combat'
    WHEN key LIKE 'Craft%' OR key IN ('RepairAlloyPerPoint', 'XpCraft', 'XpUpgrade') THEN 'crafting'
    WHEN key LIKE 'Mission%' OR (key LIKE 'Call%' AND key <> 'CallsignChangeDays') THEN 'missions'
    WHEN key LIKE '%Faction%' OR key LIKE 'Perk%' OR key LIKE 'BaseSpacing%' THEN 'factions'
    WHEN key = 'CallsignChangeDays' THEN 'profile'
    WHEN key LIKE 'Level%' OR key LIKE 'Welcome%' THEN 'progress'
    WHEN key LIKE 'Harvest%' OR key LIKE 'Node%' OR key LIKE 'Seed%' OR key LIKE 'Rare%'
      OR key IN ('NearbyMaxRadiusM', 'MaxTravelSpeedKmh', 'XpHarvest') THEN 'harvest'
    WHEN key LIKE 'Push%' OR key LIKE 'MinAppBuild%'
      OR key IN ('AccessTokenMinutes', 'SessionDays', 'DeleteGraceDays', 'InactiveDeleteDays') THEN 'app'
    ELSE 'other' END;

-- Names, from the key: "TnBuildCrystal" reads "Tether Node build crystal".
UPDATE td.settings SET label = regexp_replace(key, '([a-z0-9])([A-Z])', '\1 \2', 'g');
UPDATE td.settings SET label = regexp_replace(label, '^Tn ', 'Tether Node ');
UPDATE td.settings SET label = regexp_replace(label, ' Tn ', ' Tether Node ', 'g');
UPDATE td.settings SET label = regexp_replace(label, '(^| )Xp( |$)', '\1XP\2', 'g');
UPDATE td.settings SET label = regexp_replace(label, ' M$', ' (metres)');
UPDATE td.settings SET label = regexp_replace(label, ' Mps$', ' (metres a second)');
UPDATE td.settings SET label = regexp_replace(label, ' Kmh$', ' (km/h)');
UPDATE td.settings SET label = regexp_replace(label, ' Sec$', ' (seconds)');
UPDATE td.settings SET label = replace(replace(replace(replace(label, 'Em Pulse', 'EM Pulse'), 'Pvp', 'PvP'), 'IOS', 'iOS'), 'Km2', 'km2');
UPDATE td.settings SET label = replace(replace(replace(replace(label, 'Mk1', 'Mk I'), 'Mk2', 'Mk II'), 'Mk3', 'Mk III'), 'Mk4', 'Mk IV');
UPDATE td.settings SET label = replace(replace(replace(replace(label, 'Shield1', 'Shield Mk I'), 'Shield2', 'Shield Mk II'), 'Shield3', 'Shield Mk III'), 'Shield4', 'Shield Mk IV');
UPDATE td.settings SET label = replace(replace(label, 'Tier1', 'tier 1'), 'Tier2', 'tier 2');

-- Ranges. Shares and chances run 0 to 1 in steps of 0.05.
UPDATE td.settings SET min_value = 0, max_value = 1, step = 0.05
WHERE key LIKE '%Share%' OR key LIKE '%Chance' OR key LIKE 'TnShield%'
   OR key IN ('TnDismantleRefund', 'TnUploadFee', 'HarvestReserveCost', 'NodeDepletedReserve', 'SeedRegenPerHour');
-- On/off switches.
UPDATE td.settings SET min_value = 0, max_value = 1, step = 1 WHERE key IN ('PushEnabled', 'DroneRangeRingDefault');
-- Everything else: a whole number from 0 (or 1 where 0 would switch the thing off or break it) up
-- to ten times today's value, at least 10.
UPDATE td.settings SET
    min_value = CASE
        WHEN key IN ('InactiveDeleteDays', 'SeedPerCell', 'MinAppBuild', 'MinAppBuildIOS', 'MissionsPerDay',
                     'MissionCreditPerLevel', 'MissionXpPerLevel') THEN 0
        WHEN key LIKE '%Hours' OR key LIKE '%Minutes' OR key LIKE '%Days' OR key LIKE '%Seconds' OR key LIKE '%Sec'
          OR key LIKE '%RangeM' OR key LIKE '%RadiusM' OR key LIKE '%Level%' OR key LIKE 'TnLinks%'
          OR key IN ('TnMaxHealth', 'TnMaxPerPlayer', 'DroneSpeedMps', 'SeedCellM', 'LevelBaseXp', 'MaxTravelSpeedKmh',
                     'MissionCommandMult', 'RareYieldMult') THEN 1
        ELSE 0 END,
    max_value = GREATEST(10, ceil(value::double precision * 10)),
    step = 1
WHERE max_value = 0;

UPDATE td.settings SET min_value = 1.05, max_value = 2, step = 0.05 WHERE key = 'LevelGrowth';
UPDATE td.settings SET min_value = 0, max_value = 5, step = 0.1 WHERE key = 'RepairAlloyPerPoint';
UPDATE td.settings SET min_value = 0, max_value = 5, step = 0.05 WHERE key = 'RarePerKm2Day';
UPDATE td.settings SET min_value = 1, max_value = 10, step = 0.5 WHERE key IN ('MissionCommandMult', 'RareYieldMult');
UPDATE td.settings SET min_value = 0, max_value = 1000000 WHERE key IN ('MinAppBuild', 'MinAppBuildIOS');
UPDATE td.settings SET min_value = -720, max_value = 840 WHERE key = 'MissionDayOffsetMinutes';
UPDATE td.settings SET max_value = 100 WHERE key LIKE '%Level%' AND key NOT IN ('LevelBaseXp', 'LevelGrowth');

-- Locked: changing these mid-season would break what is stored (levels already earned, the mix
-- of nodes already seeded, the seeding grid, the missions already dealt for the day).
UPDATE td.settings SET locked = true
WHERE key IN ('LevelBaseXp', 'LevelGrowth', 'SeedCellM', 'MissionDayOffsetMinutes')
   OR key LIKE 'SeedWeight%';

ALTER TABLE td.settings ADD CONSTRAINT settings_range CHECK (min_value <= max_value AND step > 0);

-- The announcement: one line on everyone's main menu until it ends. One row at most.
CREATE TABLE td.announcement (
    id         smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    body       text NOT NULL CHECK (length(body) BETWEEN 1 AND 140),
    kind       text NOT NULL CHECK (kind IN ('info', 'event', 'warning')),
    ends_at    timestamptz NOT NULL,
    posted_by  bigint REFERENCES td.players (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
