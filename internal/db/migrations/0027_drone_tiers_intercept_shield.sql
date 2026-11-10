-- 0027 Drone strike plan, milestones 2-7: Drone Bay tiers, drones in flight, intercept, the Tether
-- Node shield. Every number is a starting value (owner, 2026-11-09/10), to tune in milestone 8.

-- Drone Bay tiers. cells is the energy cells one strike costs (it replaces the CellsDrone setting).
-- craft_cost of a tier is what upgrading to it costs (the existing Craft tab upgrade flow).
UPDATE td.gear_types SET stats = '{"drones": 1, "range_m": 150, "damage": 15, "cells": 1}' WHERE id = 'drone_bay_mk1';
INSERT INTO td.gear_types (id, slot, tier, name, min_level, stats, craft_cost, max_durability) VALUES
    ('drone_bay_mk2', 'drone_bay', 2, 'Drone Bay Mk II',  10, '{"drones": 1, "range_m": 500,  "damage": 20,  "cells": 1}', '{"alloy": 120, "data": 60, "credit": 20}',   100),
    ('drone_bay_mk3', 'drone_bay', 3, 'Drone Bay Mk III', 15, '{"drones": 2, "range_m": 1000, "damage": 25,  "cells": 1}', '{"alloy": 250, "data": 120, "credit": 60}',  100),
    ('drone_bay_mk4', 'drone_bay', 4, 'Drone Bay Mk IV',  20, '{"drones": 5, "range_m": 5000, "damage": 100, "cells": 3}', '{"alloy": 1250, "data": 600, "credit": 300}', 100);
UPDATE td.gear_types SET upgrades_to = 'drone_bay_mk2' WHERE id = 'drone_bay_mk1';
UPDATE td.gear_types SET upgrades_to = 'drone_bay_mk3' WHERE id = 'drone_bay_mk2';
UPDATE td.gear_types SET upgrades_to = 'drone_bay_mk4' WHERE id = 'drone_bay_mk3';

-- A strike remembers where it was launched from (the defender sees the direction), its Mk (the
-- bay's tier, for intercept and shield shares) and whether the owner intercepted it.
ALTER TABLE td.drone_strikes
    ADD COLUMN from_lat        double precision,
    ADD COLUMN from_lon        double precision,
    ADD COLUMN mk              smallint NOT NULL DEFAULT 1,
    ADD COLUMN intercepted     boolean NOT NULL DEFAULT false,
    ADD COLUMN intercept_share real NOT NULL DEFAULT 0,
    ADD COLUMN landed_damage   double precision,
    ADD COLUMN ended_at        timestamptz;
CREATE INDEX drone_strikes_player_recent ON td.drone_strikes (launched_by, created_at DESC);

-- The fitted shield's Mk (0 = none). It lasts as long as the Tether Node.
ALTER TABLE td.tether_nodes ADD COLUMN shield_tier smallint NOT NULL DEFAULT 0 CHECK (shield_tier BETWEEN 0 AND 4);

DELETE FROM td.settings WHERE key = 'CellsDrone';

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('DroneLaunchWear',       5,    true, 'Durability a Drone Bay loses per launch'),
    ('DroneResultsMinutes',   10,   true, 'How long a landed drone stays in GET /v1/drones'),
    ('DroneRangeRingDefault', 1,    true, 'Whether the map shows the Drone Bay range ring until the player turns it off (1 = on)'),
    ('CellsIntercept',        1,    true, 'Energy cells per intercept'),
    ('InterceptShareMk1',     1.0,  true, 'Share of a Mk I drone''s damage an intercept stops (1 = shot down)'),
    ('InterceptShareMk2',     0.75, true, 'Share of a Mk II drone''s damage an intercept stops'),
    ('InterceptShareMk3',     0.5,  true, 'Share of a Mk III drone''s damage an intercept stops'),
    ('InterceptShareMk4',     0.25, true, 'Share of a Mk IV drone''s damage an intercept stops'),
    ('TnShieldBase',          0.30, true, 'Share of an attack a Tether Node shield stops when its Mk matches the attack''s'),
    ('TnShieldStep',          0.15, true, 'Share added (or taken) for each Mk the shield is above (or below) the attack'),
    ('TnShieldMin',           0.05, true, 'Least share a fitted shield stops'),
    ('TnShieldMax',           0.75, true, 'Most share a shield stops'),
    ('ShieldLevelMk1',        5,    true, 'Level to craft a TN Shield Mk I'),
    ('ShieldLevelMk2',        10,   true, 'Level to craft a TN Shield Mk II'),
    ('ShieldLevelMk3',        15,   true, 'Level to craft a TN Shield Mk III'),
    ('ShieldLevelMk4',        20,   true, 'Level to craft a TN Shield Mk IV'),
    ('CraftShield1Crystal',   30,   true, 'Crystal to craft a TN Shield Mk I'),
    ('CraftShield1Alloy',     20,   true, 'Alloy to craft a TN Shield Mk I'),
    ('CraftShield2Alloy',     80,   true, 'Alloy to craft a TN Shield Mk II'),
    ('CraftShield2Data',      40,   true, 'Data to craft a TN Shield Mk II'),
    ('CraftShield3Alloy',     160,  true, 'Alloy to craft a TN Shield Mk III'),
    ('CraftShield3Data',      80,   true, 'Data to craft a TN Shield Mk III'),
    ('CraftShield3Credit',    40,   true, 'Credit to craft a TN Shield Mk III'),
    ('CraftShield4Alloy',     800,  true, 'Alloy to craft a TN Shield Mk IV'),
    ('CraftShield4Data',      400,  true, 'Data to craft a TN Shield Mk IV'),
    ('CraftShield4Credit',    200,  true, 'Credit to craft a TN Shield Mk IV');
