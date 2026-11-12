-- 0063 faction bases, B0 (docs/base-building-plan.md, approved 2026-11-11): the data and placement rules. Nothing
-- in the app uses it yet; admins place, clear and re-tier bases and release flags from the API.

-- The early placeholder from 0003 was never used. It goes, with its trigger: the placement rules now live in
-- internal/game (pure functions, unit-tested) and the service runs them under an advisory lock.
DROP TRIGGER IF EXISTS bases_spacing ON td.bases;
DROP FUNCTION IF EXISTS td.check_base_spacing();
DROP TABLE IF EXISTS td.bases;
DROP TYPE IF EXISTS td.base_status;

-- building: the Spire's first project is assembling (the seed beacon). active: standing. destroyed: gone (kept
-- for history; a destroyed base blocks nothing).
CREATE TABLE td.bases (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    faction_id      smallint NOT NULL REFERENCES td.factions (id),
    region          char(2) NOT NULL,
    name            text NOT NULL,
    tier            smallint NOT NULL DEFAULT 1 CHECK (tier BETWEEN 1 AND 5),
    location        geography (Point, 4326) NOT NULL,
    status          text NOT NULL DEFAULT 'building' CHECK (status IN ('building', 'active', 'destroyed')),
    core_health     integer NOT NULL CHECK (core_health >= 0),
    -- Upkeep (section 7): the last time a member stood on site; fading since when (null while looked after).
    last_visit_at   timestamptz NOT NULL DEFAULT now(),
    faded_at        timestamptz,
    -- Untouchable until (section 8: 24 h new, 12 h after a tier drop in a fight).
    protected_until timestamptz,
    proposal_id     bigint,
    raised_by       bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    destroyed_at    timestamptz
);
CREATE INDEX bases_location_gix ON td.bases USING gist (location) WHERE status <> 'destroyed';
CREATE INDEX bases_faction_region ON td.bases (faction_id, region) WHERE status <> 'destroyed';

-- The zone: a circle round the base whose radius comes from the tier (BaseZoneT1..T5). Kept as its own row so
-- the map, the border rule and later the zone checks (on site, Collector Arrays) read one radius; it changes
-- only when the tier does.
CREATE TABLE td.base_zones (
    base_id    bigint PRIMARY KEY REFERENCES td.bases (id) ON DELETE CASCADE,
    radius_m   double precision NOT NULL CHECK (radius_m > 0),
    area       geography (Polygon, 4326) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX base_zones_area_gix ON td.base_zones USING gist (area);

-- Module slots on hex rings round the tower, stored as (ring, index) so the holo table, AR and the map draw the
-- same base. Ring 1 holds the slots T1 and T2 open, ring 2 those T3 opens, ring 3 those T4 and T5 open.
CREATE TABLE td.base_slots (
    base_id     bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    ring        smallint NOT NULL CHECK (ring BETWEEN 1 AND 3),
    idx         smallint NOT NULL CHECK (idx >= 0),
    -- Empty until a module locks on (B2): lattice, shield, vault, uplink, collector, sensor, hangar, fabricator,
    -- relay.
    module      text,
    mk          smallint CHECK (mk BETWEEN 1 AND 4),
    health      integer,
    max_health  integer,
    online      boolean NOT NULL DEFAULT true,
    PRIMARY KEY (base_id, ring, idx)
);

-- A flag: a Warden's, lead's or Commander's proposal for a base on the spot they stood on (section 1b). While
-- open it holds the ground against other factions. approved turns it into the seed beacon (base_id set).
CREATE TABLE td.base_proposals (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    faction_id  smallint NOT NULL REFERENCES td.factions (id),
    region      char(2) NOT NULL,
    proposer_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    location    geography (Point, 4326) NOT NULL,
    accuracy_m  double precision NOT NULL,
    status      text NOT NULL DEFAULT 'open'
                CHECK (status IN ('open', 'approved', 'declined', 'expired', 'released')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    decided_by  bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    decided_at  timestamptz,
    base_id     bigint REFERENCES td.bases (id) ON DELETE SET NULL
);
-- One open proposal per player.
CREATE UNIQUE INDEX base_proposals_one_open ON td.base_proposals (proposer_id) WHERE status = 'open';
CREATE INDEX base_proposals_open_gix ON td.base_proposals USING gist (location) WHERE status = 'open';
CREATE INDEX base_proposals_proposer ON td.base_proposals (proposer_id, decided_at DESC);

-- The regional Commander can turn proposals off for their region.
ALTER TABLE td.faction_branches ADD COLUMN base_proposals_off boolean NOT NULL DEFAULT false;

-- Projects (section 4): raising the Spire, a tier upgrade, a module, a module upgrade. A bill in four
-- resources, then an assembly time.
CREATE TABLE td.base_projects (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    base_id       bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('spire', 'tier', 'module', 'module_upgrade')),
    -- What it makes: the tier it reaches, or the slot and module (and Mk).
    to_tier       smallint,
    ring          smallint,
    idx           smallint,
    module        text,
    mk            smallint,
    bill_alloy    integer NOT NULL DEFAULT 0,
    bill_crystal  integer NOT NULL DEFAULT 0,
    bill_data     integer NOT NULL DEFAULT 0,
    bill_credit   integer NOT NULL DEFAULT 0,
    given_alloy   integer NOT NULL DEFAULT 0,
    given_crystal integer NOT NULL DEFAULT 0,
    given_data    integer NOT NULL DEFAULT 0,
    given_credit  integer NOT NULL DEFAULT 0,
    assembly_sec  integer NOT NULL CHECK (assembly_sec >= 0),
    -- Assembly progress in seconds, advanced by the tick with the presence speed-up.
    assembled_sec double precision NOT NULL DEFAULT 0,
    status        text NOT NULL DEFAULT 'funding'
                  CHECK (status IN ('funding', 'assembling', 'done', 'cancelled')),
    started_by    bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    started_at    timestamptz NOT NULL DEFAULT now(),
    funded_at     timestamptz,
    done_at       timestamptz,
    cancelled_by  bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    cancelled_at  timestamptz
);
CREATE INDEX base_projects_running ON td.base_projects (base_id) WHERE status IN ('funding', 'assembling');

ALTER TABLE td.bases ADD CONSTRAINT bases_proposal_fk
    FOREIGN KEY (proposal_id) REFERENCES td.base_proposals (id) ON DELETE SET NULL;

-- Contributions: who gave what toward which project (or straight into the vault), and the points it earned.
CREATE TABLE td.base_contributions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    base_id    bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    project_id bigint REFERENCES td.base_projects (id) ON DELETE SET NULL,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    -- A Warden+ paying from the vault earns no points.
    from_vault boolean NOT NULL DEFAULT false,
    alloy      integer NOT NULL DEFAULT 0 CHECK (alloy >= 0),
    crystal    integer NOT NULL DEFAULT 0 CHECK (crystal >= 0),
    data       integer NOT NULL DEFAULT 0 CHECK (data >= 0),
    credit     integer NOT NULL DEFAULT 0 CHECK (credit >= 0),
    points     integer NOT NULL DEFAULT 0 CHECK (points >= 0),
    at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX base_contributions_player ON td.base_contributions (player_id, at DESC);
CREATE INDEX base_contributions_project ON td.base_contributions (project_id);

-- Each base has its own vault (section 6).
CREATE TABLE td.base_vaults (
    base_id bigint PRIMARY KEY REFERENCES td.bases (id) ON DELETE CASCADE,
    alloy   bigint NOT NULL DEFAULT 0 CHECK (alloy >= 0),
    crystal bigint NOT NULL DEFAULT 0 CHECK (crystal >= 0),
    data    bigint NOT NULL DEFAULT 0 CHECK (data >= 0),
    credit  bigint NOT NULL DEFAULT 0 CHECK (credit >= 0)
);

-- Every movement in or out of a vault, visible to Wardens+ (VAULT LOG). Amounts are signed.
CREATE TABLE td.base_vault_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    base_id    bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    at         timestamptz NOT NULL DEFAULT now(),
    player_id  bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    -- deposit, collector, cancel, spend, hack, packed, admin
    kind       text NOT NULL,
    project_id bigint REFERENCES td.base_projects (id) ON DELETE SET NULL,
    alloy      bigint NOT NULL DEFAULT 0,
    crystal    bigint NOT NULL DEFAULT 0,
    data       bigint NOT NULL DEFAULT 0,
    credit     bigint NOT NULL DEFAULT 0
);
CREATE INDEX base_vault_log_base ON td.base_vault_log (base_id, at DESC);

-- Sensitive places from OpenStreetMap (section 1): worship, cemeteries, schools, hospitals, police, military and
-- government. No base or flag within BaseSensitiveBufferM. Loaded by import-osm with the seeder's layers.
CREATE TABLE td.osm_sensitive (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL,
    geom geometry (Geometry, 4326) NOT NULL
);
CREATE INDEX osm_sensitive_gix ON td.osm_sensitive USING gist (geom);

-- The two old spacing settings join the new group; the same-faction spacing was raised to 3 000 m by the owner
-- (2026-11-11).
UPDATE td.settings SET grp = 'bases', label = 'Own-faction bases apart (metres)', min_value = 100, max_value = 20000,
       step = 100, description = 'A base or flag must stand this far from every base of its own faction'
 WHERE key = 'BaseSpacingSameFactionM';
UPDATE td.settings SET value = 3000 WHERE key = 'BaseSpacingSameFactionM' AND value = 500;
UPDATE td.settings SET grp = 'bases', label = 'Other factions'' bases apart (metres)', min_value = 0, max_value = 2000,
       step = 10, description = 'A base or flag must stand this far from every other faction''s base centre'
 WHERE key = 'BaseSpacingOtherFactionM';

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BasesEnabled',              1,    true,  'bases', 'Bases on',                                0, 1,     1,
     '1: factions can propose and raise bases. 0: nobody can (standing bases stay)'),
    ('BaseZoneGapM',              30,   true,  'bases', 'Gap between zones (metres)',              0, 300,   5,
     'A new or growing zone must stay this far outside every other zone''s edge, own faction included'),
    ('BaseSensitiveBufferM',      50,   true,  'bases', 'Clear of sensitive places (metres)',      0, 500,   5,
     'No base or flag this close to worship, cemeteries, schools, hospitals, police, military or government'),
    ('BaseWalkwayM',              15,   true,  'bases', 'Within of a walkable way (metres)',       1, 100,   1,
     'The spot must be on land and this close to a walkable way, not in a building or water'),
    ('BaseSpotAccuracyM',         20,   true,  'bases', 'Fix accuracy needed (metres)',            5, 100,   1,
     'Proposing needs a fix at least this accurate'),
    ('BaseProposalHours',         48,   true,  'bases', 'Flag stands for (hours)',                 1, 336,   1,
     'An unanswered proposal expires after this and the flag comes down'),
    ('BaseProposalCooldownHours', 24,   true,  'bases', 'Cooldown after a declined flag (hours)', 0, 336,   1,
     'A declined or expired proposal stops that player proposing again for this long'),
    ('BaseZoneT1',                25,   true,  'bases', 'Zone radius: Spire (metres)',             5, 500,   5, 'Tier 1'),
    ('BaseZoneT2',                40,   true,  'bases', 'Zone radius: Outpost (metres)',           5, 500,   5, 'Tier 2'),
    ('BaseZoneT3',                60,   true,  'bases', 'Zone radius: Stronghold (metres)',        5, 500,   5, 'Tier 3'),
    ('BaseZoneT4',                80,   true,  'bases', 'Zone radius: Citadel (metres)',           5, 500,   5, 'Tier 4'),
    ('BaseZoneT5',                100,  true,  'bases', 'Zone radius: Capital (metres)',           5, 500,   5, 'Tier 5'),
    ('BaseSlotsT1',               4,    true,  'bases', 'Module slots: Spire',                     1, 40,    1, 'Ring 1'),
    ('BaseSlotsT2',               8,    true,  'bases', 'Module slots: Outpost',                   1, 40,    1, 'Ring 1'),
    ('BaseSlotsT3',               12,   true,  'bases', 'Module slots: Stronghold',                1, 40,    1,
     'The slots above the Outpost''s go on ring 2'),
    ('BaseSlotsT4',               18,   true,  'bases', 'Module slots: Citadel',                   1, 40,    1,
     'The slots above the Stronghold''s go on ring 3'),
    ('BaseSlotsT5',               24,   true,  'bases', 'Module slots: Capital',                   1, 40,    1,
     'The slots above the Citadel''s also go on ring 3'),
    ('BaseCoreT1',                500,  true,  'bases', 'Core health: Spire',                      1, 100000, 50, 'Tier 1'),
    ('BaseCoreT2',                1500, true,  'bases', 'Core health: Outpost',                    1, 100000, 50, 'Tier 2'),
    ('BaseCoreT3',                3500, true,  'bases', 'Core health: Stronghold',                 1, 100000, 50, 'Tier 3'),
    ('BaseCoreT4',                7000, true,  'bases', 'Core health: Citadel',                    1, 100000, 50, 'Tier 4'),
    ('BaseCoreT5',                12000,true,  'bases', 'Core health: Capital',                    1, 100000, 50, 'Tier 5'),
    ('BaseCapBase',               2,    true,  'bases', 'Bases per faction per region: base',      0, 50,    1,
     'Every faction may hold this many bases (and open flags) in a region, plus one per step below reached'),
    ('BaseCapStep1Players',       50,   false, 'bases', 'Base cap step 1 (active members)',        0, 100000, 10,
     'One more base for every faction once the region has this many active faction members, all factions together (0: off)'),
    ('BaseCapStep2Players',       150,  false, 'bases', 'Base cap step 2 (active members)',        0, 100000, 10, 'The next step (0: off)'),
    ('BaseCapStep3Players',       300,  false, 'bases', 'Base cap step 3 (active members)',        0, 100000, 10, 'The next step (0: off)'),
    ('BaseCapStep4Players',       600,  false, 'bases', 'Base cap step 4 (active members)',        0, 100000, 10, 'The next step (0: off)'),
    ('BaseCapActiveDays',         14,   false, 'bases', 'Active member means seen within (days)',  1, 90,    1,
     'Who counts toward the region''s signal population for the base cap')
ON CONFLICT (key) DO NOTHING;
