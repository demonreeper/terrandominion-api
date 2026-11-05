-- 0003 factions: the fixed global factions, their regional branches, members, and bases.

CREATE TYPE td.faction_status AS ENUM ('locked', 'active');

-- Fixed and lore-based. Every faction starts locked; the first player at the lead level to claim
-- one becomes its head. The unique leader column stops one player heading two factions.
CREATE TABLE td.factions (
    id               smallint PRIMARY KEY,
    code             text NOT NULL UNIQUE,
    name             text NOT NULL,
    motto            text NOT NULL DEFAULT '',
    lore             text NOT NULL DEFAULT '',
    locked_reason    text NOT NULL DEFAULT '',   -- the in-world reason it waits for a commander
    color            char(7) NOT NULL,
    sort             smallint NOT NULL,
    status           td.faction_status NOT NULL DEFAULT 'locked',
    leader_player_id bigint UNIQUE REFERENCES td.players (id) ON DELETE SET NULL,
    claimed_at       timestamptz
);

-- Placeholder names and colours until the lore is written.
INSERT INTO td.factions (id, code, name, color, sort, locked_reason) VALUES
    (1, 'faction_1', 'Faction One',   '#3FC8FF', 1, 'Dormant: awaiting a commander.'),
    (2, 'faction_2', 'Faction Two',   '#FFB648', 2, 'Dormant: awaiting a commander.'),
    (3, 'faction_3', 'Faction Three', '#5CF28C', 3, 'Dormant: awaiting a commander.'),
    (4, 'faction_4', 'Faction Four',  '#B87CFF', 4, 'Dormant: awaiting a commander.'),
    (5, 'faction_5', 'Faction Five',  '#FF5F6D', 5, 'Dormant: awaiting a commander.'),
    (6, 'faction_6', 'Faction Six',   '#E8F2FF', 6, 'Dormant: awaiting a commander.');

-- A faction's presence in one region (an atoll or city), run by a sub-leader the head appoints.
CREATE TABLE td.faction_branches (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    faction_id       smallint NOT NULL REFERENCES td.factions (id),
    country_code     char(2) NOT NULL DEFAULT 'MV',
    region_code      text NOT NULL,
    name             text NOT NULL,
    leader_player_id bigint REFERENCES td.players (id) ON DELETE SET NULL,
    appointed_by     bigint REFERENCES td.players (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (faction_id, country_code, region_code)
);

-- A player belongs to at most one faction. No row means freelancer. Ranks are numbers for now
-- (1 member up to 100 head) until the rank ladder is designed.
CREATE TABLE td.faction_members (
    player_id  bigint PRIMARY KEY REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    branch_id  bigint REFERENCES td.faction_branches (id) ON DELETE SET NULL,
    rank       smallint NOT NULL DEFAULT 1 CHECK (rank BETWEEN 1 AND 100),
    joined_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX faction_members_faction_idx ON td.faction_members (faction_id, rank DESC);

CREATE TYPE td.base_status AS ENUM ('building', 'active', 'destroyed');

-- Faction bases. They grow in place (tier), so there is no relocation column.
CREATE TABLE td.bases (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    faction_id  smallint NOT NULL REFERENCES td.factions (id),
    branch_id   bigint REFERENCES td.faction_branches (id) ON DELETE SET NULL,
    tier        smallint NOT NULL DEFAULT 0 CHECK (tier >= 0),
    location    geography (Point, 4326) NOT NULL,
    status      td.base_status NOT NULL DEFAULT 'building',
    built_by    bigint REFERENCES td.players (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX bases_location_gix ON td.bases USING gist (location) WHERE status <> 'destroyed';

-- Spacing rules live in the database so no code path can skip them: one base per faction within
-- BaseSpacingSameFactionM, and no base within BaseSpacingOtherFactionM of another faction's.
-- The advisory lock serialises base inserts so two simultaneous builds cannot both pass.
CREATE FUNCTION td.check_base_spacing() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    same_m  double precision;
    other_m double precision;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext('td.bases.spacing'));

    SELECT value INTO same_m FROM td.settings WHERE key = 'BaseSpacingSameFactionM';
    SELECT value INTO other_m FROM td.settings WHERE key = 'BaseSpacingOtherFactionM';
    same_m := COALESCE(same_m, 500);
    other_m := COALESCE(other_m, 150);

    IF EXISTS (
        SELECT 1 FROM td.bases b
        WHERE b.id <> NEW.id AND b.status <> 'destroyed'
          AND b.faction_id = NEW.faction_id
          AND ST_DWithin(b.location, NEW.location, same_m)
    ) THEN
        RAISE EXCEPTION 'E1402 base too close to another base of the same faction'
            USING ERRCODE = 'check_violation';
    END IF;

    IF EXISTS (
        SELECT 1 FROM td.bases b
        WHERE b.id <> NEW.id AND b.status <> 'destroyed'
          AND b.faction_id <> NEW.faction_id
          AND ST_DWithin(b.location, NEW.location, other_m)
    ) THEN
        RAISE EXCEPTION 'E1403 base too close to another faction''s base'
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER bases_spacing
    BEFORE INSERT OR UPDATE OF location, status ON td.bases
    FOR EACH ROW WHEN (NEW.status <> 'destroyed')
    EXECUTE FUNCTION td.check_base_spacing();
