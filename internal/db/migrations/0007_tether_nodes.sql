-- 0007 tether nodes: build and dismantle at tier 1. A TN is an automatic collector any player can
-- build; collection, decay and attacks arrive in later migrations.

CREATE TYPE td.tether_status AS ENUM ('active', 'destroyed', 'decayed', 'dismantled');

CREATE TABLE td.tether_nodes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id smallint REFERENCES td.factions (id),      -- null for a freelancer's TN
    location   geography (Point, 4326) NOT NULL,          -- the GPS fix it was built at
    accuracy_m real NOT NULL,
    tier       smallint NOT NULL DEFAULT 1 CHECK (tier >= 1),
    health     real NOT NULL CHECK (health >= 0),
    spent      jsonb NOT NULL DEFAULT '{}',               -- everything paid so far, for the refund
    status     td.tether_status NOT NULL DEFAULT 'active',
    built_at   timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    ended_at   timestamptz
);

CREATE INDEX tether_nodes_location_gix ON td.tether_nodes USING gist (location) WHERE status = 'active';
CREATE INDEX tether_nodes_owner_idx ON td.tether_nodes (owner_id) WHERE status = 'active';

-- Which node each slot of a TN is tethered to. A node holds at most one link, and links are
-- deleted when a TN ends, so the node becomes free again.
CREATE TABLE td.tether_links (
    tn_id           bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    slot            smallint NOT NULL CHECK (slot >= 0),
    node_id         bigint NOT NULL UNIQUE REFERENCES td.nodes (id) ON DELETE CASCADE,
    linked_at       timestamptz NOT NULL DEFAULT now(),
    next_harvest_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tn_id, slot)
);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('TnMaxPerPlayer',      3,    true,  'Tether Nodes a player may own at once'),
    ('TnLinkRadiusM',       150,  true,  'Radius within which a Tether Node links to resource nodes'),
    ('TnLinksTier1',        1,    true,  'Links on a tier 1 Tether Node'),
    ('TnLifetimeHours',     48,   true,  'Hours until a Tether Node self-destructs'),
    ('TnMaxHealth',         100,  true,  'Starting health of a Tether Node'),
    ('TnDismantleRefund',   0.5,  true,  'Share of build and upgrade cost refunded on dismantle'),
    ('TnBuildCrystal',      80,   true,  'Crystal to build a Tether Node'),
    ('TnBuildAlloy',        60,   true,  'Alloy to build a Tether Node'),
    ('TnBuildData',         20,   true,  'Data to build a Tether Node'),
    ('NodeDepletedReserve', 0.10, false, 'Reserve below which a node counts as depleted'),
    ('XpTnBuild',           20,   false, 'XP for building a Tether Node');
