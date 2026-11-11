-- 0044 faction names (owner, 2026-11-11): the six factions take the names chosen in the design,
-- replacing the placeholders "Faction One" to "Faction Six" seeded in 0003. The powers keep their own
-- names (persona): a faction and its power are named differently. Codes (faction_1 ...) stay: they
-- are identifiers, never shown.

CREATE TEMP TABLE faction_rename (id smallint, new_name text) ON COMMIT DROP;
INSERT INTO faction_rename VALUES
    (1, 'Lumen Collective'),      -- Signum
    (2, 'Forge Meridian'),        -- Shingō
    (3, 'Verdant Exchange'),      -- Ishāra
    (4, 'Veil Cipher'),           -- Sēma
    (5, 'Slipstream Vanguard'),   -- Tohu
    (6, 'Bastion Order');         -- Merki

-- A branch is named after its faction ("Faction One <region>"); keep the region part.
UPDATE td.faction_branches b
SET name = r.new_name || substr(b.name, length(f.name) + 1)
FROM td.factions f JOIN faction_rename r ON r.id = f.id
WHERE b.faction_id = f.id AND left(b.name, length(f.name)) = f.name;

UPDATE td.factions f SET name = r.new_name FROM faction_rename r WHERE r.id = f.id;

-- 0043 gave the powers' accounts in the wrong order for two of them. An account's id is its
-- faction's id + 1: faction 3 is Ishāra's (account 4) and faction 4 is Sēma's (account 5).
UPDATE td.players SET callsign = NULL WHERE id = 5 AND type = 'system';
UPDATE td.players SET display_name = 'Ishāra', callsign = 'Ishara' WHERE id = 4 AND type = 'system';
UPDATE td.players SET display_name = 'Sēma', callsign = 'Sema' WHERE id = 5 AND type = 'system';
