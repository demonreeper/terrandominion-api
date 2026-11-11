-- 0050 Unbound titles (docs/freelancer-path-plan.md F2, phase 5): past the faction level a freelancer
-- keeps earning titles, Drifter, Wayfarer and Unbound, each a profile badge given in a message from
-- the seventh voice. A title belongs to no faction, so a badge's faction may be empty now.
ALTER TABLE td.player_badges DROP CONSTRAINT IF EXISTS player_badges_pkey;
ALTER TABLE td.player_badges ALTER COLUMN faction_id DROP NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS player_badges_once
    ON td.player_badges (player_id, badge, COALESCE(faction_id, 0), region);

-- The levels the titles come at. Starting values, tuned from admin SETTINGS.
INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('TitleDrifterLevel',  22, true, 'factions', 'Title: Drifter at level',  21, 200, 1,
     'A freelancer reaching this level is given the Drifter title'),
    ('TitleWayfarerLevel', 26, true, 'factions', 'Title: Wayfarer at level', 22, 200, 1,
     'A freelancer reaching this level is given the Wayfarer title'),
    ('TitleUnboundLevel',  30, true, 'factions', 'Title: Unbound at level',  23, 200, 1,
     'A freelancer reaching this level is given the Unbound title')
ON CONFLICT (key) DO NOTHING;
