-- 0068 faction bases, B5 (docs/base-building-plan.md 8): attacks on modules and the core, drones and Overload Spikes
-- in flight or armed at a base, hacks on the vault, protection, tier drops and destruction, and the attack push.

-- A drone flying at a base, or an Overload Spike armed on it. ring/idx is the targeted module (null: the core).
CREATE TABLE td.base_strikes (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    base_id         bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    ring            smallint,
    idx             smallint,
    kind            text NOT NULL CHECK (kind IN ('drone', 'spike')),
    launched_by     bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    damage          double precision NOT NULL,
    mk              smallint NOT NULL DEFAULT 1,
    from_lat        double precision,
    from_lon        double precision,
    due_at          timestamptz NOT NULL,
    intercept_share double precision NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'hit', 'void', 'defused')),
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX base_strikes_due ON td.base_strikes (due_at) WHERE status = 'pending';
CREATE INDEX base_strikes_base ON td.base_strikes (base_id) WHERE status = 'pending';

-- One hack per player per base per BaseHackCooldownMin.
CREATE TABLE td.base_hacks (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    base_id   bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    at        timestamptz NOT NULL DEFAULT now(),
    stolen    jsonb NOT NULL DEFAULT '{}'
);
CREATE INDEX base_hacks_player ON td.base_hacks (player_id, base_id, at DESC);

-- The faction's members in the region are pushed once per base per BaseAttackPushMin.
ALTER TABLE td.bases ADD COLUMN attack_pushed_at timestamptz;

-- A destroyed Spire leaves half its vault on the map.
ALTER TABLE td.map_drops ADD COLUMN base_id bigint REFERENCES td.bases (id) ON DELETE SET NULL;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BaseProtectDropHours',     12,  true, 'bases', 'Protection after a tier drop (hours)', 0, 240, 1, 'After a base drops a tier in a fight'),
    ('BaseModulesPerLattice',    4,   true, 'bases', 'Modules one Energy Lattice powers',     1, 20,  1,
     'Modules beyond what the online Lattices power go offline'),
    ('BaseHackRangeM',           75,  true, 'bases', 'Hack range (metres)',                   10, 500, 5, 'From the base centre'),
    ('BaseHackData',             50,  true, 'bases', 'Hack cost (data)',                      0, 10000, 5, 'From the hacker''s storage'),
    ('BaseHackMinPct',           5,   true, 'bases', 'Hack steals at least (%)',              0, 100, 1, 'Of the vault above the safe floor'),
    ('BaseHackMaxPct',           15,  true, 'bases', 'Hack steals at most (%)',               0, 100, 1, 'Of the vault above the safe floor'),
    ('BaseHackCooldownMin',      30,  true, 'bases', 'Hack again after (minutes)',            0, 1440, 5, 'Per player per base'),
    ('BaseSafeFloorPct',         20,  true, 'bases', 'Vault safe floor (%)',                  0, 100, 1, 'The share of a vault a hack never takes'),
    ('BasePerkBastionFloorPct',  35,  true, 'bases', 'Bastion Order: vault safe floor (%)',   0, 100, 1, 'Faction perk on bases'),
    ('BasePerkBastionShieldPct', 10,  true, 'bases', 'Bastion Order: base shield stops more (points)', 0, 50, 1, 'Faction perk on bases'),
    ('BaseAttackPushMin',        30,  true, 'bases', 'Base under attack push every (minutes)', 1, 1440, 1,
     'The faction''s members in the region hear once per base in this time'),
    ('BaseDestroyDropPct',       50,  true, 'bases', 'A destroyed Spire drops (%) of its vault', 0, 100, 5, 'On the map; the rest is lost'),
    ('BasePackedRefundPct',      50,  true, 'bases', 'Packed-away module refund (%)',         0, 100, 5,
     'Of its cost, into the vault, when a tier drop removes its slot')
ON CONFLICT (key) DO NOTHING;
