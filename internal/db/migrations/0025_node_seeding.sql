-- 0025 Node seeding: the server keeps the map stocked by itself (drone strike plan, milestone 0).

-- What the seeder may stand on, loaded from the map's OpenStreetMap extract by `tdapi import-osm`
-- (scripts/import-osm.ps1). Replaced as a whole on every import.
CREATE TABLE td.osm_land (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    geom geometry (Geometry, 4326) NOT NULL          -- islands, from the joined coastline
);
CREATE INDEX osm_land_gix ON td.osm_land USING gist (geom);

CREATE TABLE td.osm_walkways (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL,                               -- footway, residential, park, square...
    geom geometry (LineString, 4326) NOT NULL
);
CREATE INDEX osm_walkways_gix ON td.osm_walkways USING gist (geom);

-- Where a node must never stand even on land: buildings, inland water, runways and taxiways.
CREATE TABLE td.osm_blocked (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL,
    geom geometry (Geometry, 4326) NOT NULL
);
CREATE INDEX osm_blocked_gix ON td.osm_blocked USING gist (geom);

-- One row per seeding cell once it has been looked at. The row is locked FOR UPDATE while a cell
-- is seeded, so two scans of the same cell can't both fill it.
CREATE TABLE td.seed_cells (
    cell_id   text PRIMARY KEY,                       -- "<cell metres>:<row>:<col>"
    min_lat   double precision NOT NULL,
    min_lon   double precision NOT NULL,
    max_lat   double precision NOT NULL,
    max_lon   double precision NOT NULL,
    target    integer NOT NULL DEFAULT 0,             -- how many seed nodes the cell should hold
    walk_m    real NOT NULL DEFAULT 0,                -- walkable path inside the cell
    seeded_at timestamptz,                            -- null until the first seeding pass
    topped_at timestamptz                             -- last top-up pass by the world tick
);

-- Areas an admin pre-seeds as a whole (POST /v1/admin/seed). More islands are added as rows.
CREATE TABLE td.seed_areas (
    id      text PRIMARY KEY,
    name    text NOT NULL,
    min_lat double precision NOT NULL,
    min_lon double precision NOT NULL,
    max_lat double precision NOT NULL,
    max_lon double precision NOT NULL
);

INSERT INTO td.seed_areas (id, name, min_lat, min_lon, max_lat, max_lon) VALUES
    ('male',      'Malé',      4.1660, 73.4990, 4.1815, 73.5245),
    ('hulhumale', 'Hulhumalé', 4.1950, 73.5270, 4.2460, 73.5560),
    ('villimale', 'Villimalé', 4.1700, 73.4795, 4.1780, 73.4900);

-- Rare roaming nodes end on their own.
ALTER TABLE td.nodes ADD COLUMN expires_at timestamptz;
CREATE INDEX nodes_source_idx ON td.nodes (source) WHERE status = 'active';

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('SeedCellM',          250,  true,  'Side of a seeding cell in metres'),
    ('SeedPerCell',        4,    true,  'Seed nodes a cell aims to hold (0 turns seeding off)'),
    ('SeedPathPerNodeM',   40,   true,  'A cell gets at most one seed node per this many metres of walkable path'),
    ('SeedSnapM',          15,   true,  'Farthest a seed node may stand from a walkable way'),
    ('SeedMinGapM',        25,   true,  'Closest a seed node may stand to any other active node'),
    ('SeedTnGapM',         10,   true,  'Closest a seed node may stand to an active Tether Node'),
    ('SeedRegenPerHour',   0.05, true,  'Reserve a seed node regrows per hour'),
    ('SeedWeightCrystal',  4,    true,  'Share of seed nodes that are crystal (weight)'),
    ('SeedWeightAlloy',    3,    true,  'Share of seed nodes that are alloy (weight)'),
    ('SeedWeightData',     2,    true,  'Share of seed nodes that are data (weight)'),
    ('SeedWeightCredit',   1,    true,  'Share of seed nodes that are credit (weight)'),
    ('SeedYieldCrystal',   12,   true,  'Crystal per harvest at a seeded or scattered node'),
    ('SeedYieldAlloy',     10,   true,  'Alloy per harvest at a seeded or scattered node'),
    ('SeedYieldData',      6,    true,  'Data per harvest at a seeded or scattered node'),
    ('SeedYieldCredit',    8,    true,  'Credit per harvest at a seeded or scattered node'),
    ('SeedCellsPerScan',   4,    false, 'Most unseeded cells one nearby scan seeds (nearest first); the rest wait for the next scan'),
    ('SeedTopUpPerTick',   10,   false, 'Most cells the world tick tops up each minute'),
    ('SeedTopUpMinutes',   30,   false, 'How long before the world tick looks at the same cell again'),
    ('RarePerKm2Day',      0.25, false, 'Rare roaming nodes per square kilometre of seeded land per day'),
    ('RareMinHours',       2,    false, 'Shortest life of a rare node'),
    ('RareMaxHours',       4,    false, 'Longest life of a rare node'),
    ('RareYieldMult',      3,    false, 'Rare node yield as a multiple of a normal node of its resource');
