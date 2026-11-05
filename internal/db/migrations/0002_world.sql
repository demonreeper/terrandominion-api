-- 0002 world: resources, the fixed nodes freelancers harvest, and gear.

CREATE TABLE td.resource_types (
    code text PRIMARY KEY,
    name text NOT NULL,
    sort smallint NOT NULL
);

INSERT INTO td.resource_types (code, name, sort) VALUES
    ('crystal', 'Energy Crystal', 1),
    ('alloy',   'Alloy',          2),
    ('data',    'Data Cache',     3),
    ('credit',  'Credit',         4);

-- What a player is carrying. Losing a fight costs a cut of this.
CREATE TABLE td.player_resources (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    resource  text NOT NULL REFERENCES td.resource_types (code),
    carried   bigint NOT NULL DEFAULT 0 CHECK (carried >= 0),
    PRIMARY KEY (player_id, resource)
);

CREATE TYPE td.node_status AS ENUM ('active', 'hidden', 'retired');

-- Fixed resource sites on the real map. Placed by hand by admins (like seeding places) or
-- generated from OpenStreetMap; source/source_ref records which.
CREATE TABLE td.nodes (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name              text NOT NULL,
    resource          text NOT NULL REFERENCES td.resource_types (code),
    location          geography (Point, 4326) NOT NULL,
    country_code      char(2) NOT NULL DEFAULT 'MV',
    region_code       text,                    -- atoll or city, for branches and admin filters
    yield_per_harvest integer NOT NULL CHECK (yield_per_harvest > 0),
    reserve           real NOT NULL DEFAULT 1 CHECK (reserve BETWEEN 0 AND 1),
    regen_per_hour    real NOT NULL DEFAULT 0.05 CHECK (regen_per_hour >= 0),
    regen_at          timestamptz NOT NULL DEFAULT now(),
    status            td.node_status NOT NULL DEFAULT 'active',
    source            text NOT NULL DEFAULT 'manual',
    source_ref        text,
    created_by        bigint REFERENCES td.players (id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX nodes_location_gix ON td.nodes USING gist (location);
CREATE INDEX nodes_region_idx ON td.nodes (country_code, region_code) WHERE status = 'active';

-- Every harvest with the position the phone reported: the audit trail for GPS spoofing and the
-- source of the per-player speed check.
CREATE TABLE td.harvests (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    node_id     bigint NOT NULL REFERENCES td.nodes (id) ON DELETE CASCADE,
    amount      integer NOT NULL,
    reported    geography (Point, 4326) NOT NULL,
    accuracy_m  real NOT NULL,
    distance_m  real NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX harvests_player_idx ON td.harvests (player_id, created_at DESC);
CREATE INDEX harvests_player_node_idx ON td.harvests (player_id, node_id, created_at DESC);

CREATE TYPE td.gear_slot AS ENUM ('scanner', 'extractor', 'rig', 'drone_bay', 'shield');

-- The gear catalogue. Stats are slot-specific numbers kept as JSON so new stats need no
-- migration: scanner {range_m}, extractor {yield_mult, carry}, rig {hack}, drone_bay {drones,
-- range_m, damage}, shield {absorb}.
CREATE TABLE td.gear_types (
    id             text PRIMARY KEY,
    slot           td.gear_slot NOT NULL,
    tier           smallint NOT NULL CHECK (tier >= 1),
    name           text NOT NULL,
    min_level      integer NOT NULL DEFAULT 1,
    stats          jsonb NOT NULL,
    craft_cost     jsonb NOT NULL DEFAULT '{}',   -- {"crystal": 20, "alloy": 10}
    max_durability integer NOT NULL DEFAULT 100,
    upgrades_to    text REFERENCES td.gear_types (id),
    UNIQUE (slot, tier)
);

-- Owned gear. Crafted, upgraded, and later dropped; acquired_from records which.
CREATE TABLE td.player_gear (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id     bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    gear_type_id  text NOT NULL REFERENCES td.gear_types (id),
    slot          td.gear_slot NOT NULL,          -- copied from the type for the equip index
    durability    integer NOT NULL CHECK (durability >= 0),
    equipped      boolean NOT NULL DEFAULT false,
    mods          jsonb NOT NULL DEFAULT '{}',
    acquired_from text NOT NULL DEFAULT 'craft',  -- 'starter' | 'craft' | 'upgrade' | 'drop'
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX player_gear_player_idx ON td.player_gear (player_id);
CREATE UNIQUE INDEX player_gear_one_equipped_per_slot
    ON td.player_gear (player_id, slot) WHERE equipped;

INSERT INTO td.gear_types (id, slot, tier, name, min_level, stats, craft_cost, max_durability) VALUES
    ('scanner_mk1',   'scanner',   1, 'Survey Scanner Mk I',  1, '{"range_m": 300}',                    '{}', 100),
    ('extractor_mk1', 'extractor', 1, 'Field Extractor Mk I', 1, '{"yield_mult": 1.0, "carry": 200}',   '{}', 100),
    ('rig_mk1',       'rig',       1, 'Intrusion Rig Mk I',   3, '{"hack": 10}',                        '{"data": 20, "alloy": 10}', 100),
    ('drone_bay_mk1', 'drone_bay', 1, 'Drone Bay Mk I',       5, '{"drones": 1, "range_m": 150, "damage": 5}', '{"alloy": 40, "crystal": 20}', 100),
    ('shield_mk1',    'shield',    1, 'Personal Shield Mk I', 2, '{"absorb": 0.10}',                    '{"crystal": 30}', 100),
    ('scanner_mk2',   'scanner',   2, 'Survey Scanner Mk II', 5, '{"range_m": 500}',                    '{"data": 40, "alloy": 20}', 120),
    ('extractor_mk2', 'extractor', 2, 'Field Extractor Mk II',5, '{"yield_mult": 1.25, "carry": 350}',  '{"alloy": 60, "crystal": 30}', 120);

UPDATE td.gear_types SET upgrades_to = 'scanner_mk2' WHERE id = 'scanner_mk1';
UPDATE td.gear_types SET upgrades_to = 'extractor_mk2' WHERE id = 'extractor_mk1';
