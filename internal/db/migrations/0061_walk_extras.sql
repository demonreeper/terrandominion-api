-- 0061 walk together W2b (docs/walk-together-plan.md, extras X1, X2, X3, X5): shared scan finds, duo / trio /
-- quad group nodes, link missions and distance milestones.

-- X1: what any linked member's scan found shows on every member's map until the link ends.
CREATE TABLE td.walk_link_finds (
    link_id  bigint NOT NULL REFERENCES td.walk_links (id) ON DELETE CASCADE,
    kind     text NOT NULL CHECK (kind IN ('node', 'drop')),
    ref_id   bigint NOT NULL,
    found_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (link_id, kind, ref_id)
);

-- X2: a node that opens only for a linked group of at least this many (2 duo, 3 trio, 4 quad). Each member
-- harvests it once.
ALTER TABLE td.nodes ADD COLUMN group_size smallint CHECK (group_size IS NULL OR group_size BETWEEN 2 AND 4);
CREATE TABLE td.group_node_harvests (
    node_id      bigint NOT NULL REFERENCES td.nodes (id) ON DELETE CASCADE,
    player_id    bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    harvested_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (node_id, player_id)
);

-- X3: link missions. Never drawn as ordinary missions; ensureMissions swaps one in at LinkMissionChancePct. The
-- line names no power (the plan's own words). One per power and audience, so it comes from the power that sends
-- the player's missions.
ALTER TABLE td.mission_types ADD COLUMN link_mission boolean NOT NULL DEFAULT false;
INSERT INTO td.mission_types (id, faction_id, audience, goal, goal_key, count, min_level, max_level, line, link_mission)
SELECT 'link_walk_' || a.audience || '_' || f.id, f.id, a.audience, 'walk_linked', NULL, 1000, 1, 1000,
       'Walk 1 km linked with a friend.', true
FROM td.factions f CROSS JOIN (VALUES ('freelancer'), ('member')) AS a(audience)
WHERE f.id BETWEEN 1 AND 6
ON CONFLICT (id) DO NOTHING;
INSERT INTO td.mission_types (id, faction_id, audience, goal, goal_key, count, min_level, max_level, line, link_mission)
SELECT 'link_duo_' || a.audience || '_' || f.id, f.id, a.audience, 'harvest_group_node', NULL, 1, 1, 1000,
       'Harvest a duo node.', true
FROM td.factions f CROSS JOIN (VALUES ('freelancer'), ('member')) AS a(audience)
WHERE f.id BETWEEN 1 AND 6
ON CONFLICT (id) DO NOTHING;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('GroupNodeRatePerCell', 0.05, false, 'walk', 'Group nodes per seeded cell a day',       0, 2,    0.01,
     'How many duo, trio and quad nodes the seeder places, on average, per seeded cell per day (0: none)'),
    ('GroupNodeMixDuo',      60,   false, 'walk', 'Group node mix: duo (weight)',            0, 100,  1,
     'Of the group nodes placed, how many are duo nodes (2 linked), by weight'),
    ('GroupNodeMixTrio',     30,   false, 'walk', 'Group node mix: trio (weight)',           0, 100,  1, 'Trio nodes (3 linked)'),
    ('GroupNodeMixQuad',     10,   false, 'walk', 'Group node mix: quad (weight)',           0, 100,  1, 'Quad nodes (4 linked)'),
    ('GroupNodeYieldX2',     3,    true,  'walk', 'Duo node yield (times a normal node)',    1, 20,   0.5,
     'Each member of the group gets the full reward'),
    ('GroupNodeYieldX3',     5,    true,  'walk', 'Trio node yield (times a normal node)',   1, 20,   0.5, 'Each member gets the full reward'),
    ('GroupNodeYieldX4',     8,    true,  'walk', 'Quad node yield (times a normal node)',   1, 20,   0.5, 'Each member gets the full reward'),
    ('GroupNodeHours',       48,   false, 'walk', 'Group node lasts (hours)',                1, 240,  1,
     'Then the Undersignal reclaims it'),
    ('LinkMissionChancePct', 25,   false, 'walk', 'Link mission chance (%)',                 0, 100,  1,
     'How often one of a day''s missions is a link mission (walk 1 km linked, harvest a duo node)'),
    ('WalkMilestoneKm1',     5,    false, 'walk', 'Walked together badge 1 (km)',            1, 10000, 1,
     'Bonus-active kilometres walked together for the first distance badge'),
    ('WalkMilestoneKm2',     25,   false, 'walk', 'Walked together badge 2 (km)',            1, 10000, 1, 'The second'),
    ('WalkMilestoneKm3',     100,  false, 'walk', 'Walked together badge 3 (km)',            1, 10000, 1, 'The third'),
    ('WalkMilestoneKm4',     500,  false, 'walk', 'Walked together badge 4 (km)',            1, 10000, 1, 'The fourth')
ON CONFLICT (key) DO NOTHING;
