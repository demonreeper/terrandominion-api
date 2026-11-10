-- 0028 Regional command (owner, 2026-11-10): no player heads a whole faction. The shared lore AI
-- (internal name signal; players see a separate commander per faction) holds rank 100 above all
-- six; players claim regional command, one Regional Commander per faction
-- per region, and a region is a country (MV, the Maldives, at launch). Claims and joins now live
-- on faction_branches; factions.leader_player_id is left empty from here on.

-- The first player ever to command a faction's branch in a country founded it, and keeps a
-- permanent Founder badge (GAME_DESIGN.md, Factions).
ALTER TABLE td.faction_branches ADD COLUMN founded_by bigint REFERENCES td.players (id) ON DELETE SET NULL;

CREATE TABLE td.player_badges (
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    badge      text NOT NULL,                       -- founder
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    region     text NOT NULL,                       -- the country (MV)
    earned_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, badge, faction_id, region)
);

-- Anyone who claimed a whole faction becomes its Regional Commander for the Maldives.
INSERT INTO td.faction_branches (faction_id, country_code, region_code, name, leader_player_id)
SELECT f.id, 'MV', 'MV', f.name || ' Maldives', f.leader_player_id
FROM td.factions f WHERE f.leader_player_id IS NOT NULL
ON CONFLICT (faction_id, country_code, region_code) DO UPDATE SET leader_player_id = EXCLUDED.leader_player_id;

UPDATE td.faction_members m SET rank = 90, branch_id = b.id
FROM td.faction_branches b
WHERE b.faction_id = m.faction_id AND b.leader_player_id = m.player_id;

-- Members without a branch belong to their faction's Maldives branch.
INSERT INTO td.faction_branches (faction_id, country_code, region_code, name)
SELECT DISTINCT f.id, 'MV', 'MV', f.name || ' Maldives'
FROM td.faction_members m JOIN td.factions f ON f.id = m.faction_id
WHERE m.branch_id IS NULL
ON CONFLICT (faction_id, country_code, region_code) DO NOTHING;

UPDATE td.faction_members m SET branch_id = b.id
FROM td.faction_branches b
WHERE m.branch_id IS NULL AND b.faction_id = m.faction_id AND b.country_code = 'MV' AND b.region_code = 'MV';

-- They were first, so they founded it.
UPDATE td.faction_branches SET founded_by = leader_player_id WHERE leader_player_id IS NOT NULL AND founded_by IS NULL;
INSERT INTO td.player_badges (player_id, badge, faction_id, region)
SELECT founded_by, 'founder', faction_id, country_code FROM td.faction_branches WHERE founded_by IS NOT NULL
ON CONFLICT DO NOTHING;

UPDATE td.factions SET leader_player_id = NULL WHERE leader_player_id IS NOT NULL;
