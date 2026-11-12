-- 0067 faction bases, B4 (docs/base-building-plan.md 5, 6, 10): the Collector Array, deposits and the vault log, the
-- Uplink, base missions, and the visit that keeps a base looked after.

-- Nodes inside a zone a base's Collector Arrays tether into its vault (TN rules: a cooldown that grows while no
-- member comes by). A node is held by one Tether Node or one base at most.
CREATE TABLE td.base_collector_links (
    node_id bigint PRIMARY KEY REFERENCES td.nodes (id) ON DELETE CASCADE,
    base_id bigint NOT NULL REFERENCES td.bases (id) ON DELETE CASCADE,
    next_at timestamptz NOT NULL DEFAULT now(),
    gathered bigint NOT NULL DEFAULT 0
);
CREATE INDEX base_collector_links_base ON td.base_collector_links (base_id);
CREATE INDEX base_collector_links_due ON td.base_collector_links (next_at);

-- Base missions (section 10): never drawn as ordinary missions; when the member's faction has a standing base in
-- their region, one of the day's member missions is a base mission. Each power says it in its own words.
ALTER TABLE td.mission_types ADD COLUMN base_mission boolean NOT NULL DEFAULT false;
INSERT INTO td.mission_types (id, faction_id, audience, goal, goal_key, count, min_level, max_level, line, base_mission) VALUES
    ('base_give_1',   1, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Bring alloy to the stake you hold. It will remember who did.', true),
    ('base_stand_1',  1, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Stand where your base stands. Be still a while.', true),
    ('base_repair_1', 1, 'member', 'base_repair', NULL,    1,   1, 1000, 'Mend what was broken at your base.', true),
    ('base_give_2',   2, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Alloy to the base. Build.', true),
    ('base_stand_2',  2, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Hold the site. Ten minutes.', true),
    ('base_repair_2', 2, 'member', 'base_repair', NULL,    1,   1, 1000, 'A module is down. Make it stand again.', true),
    ('base_give_3',   3, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Invest alloy in your base. It pays back.', true),
    ('base_stand_3',  3, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Be seen at your base. Presence is a price too.', true),
    ('base_repair_3', 3, 'member', 'base_repair', NULL,    1,   1, 1000, 'Damage costs more than mending. Repair a module.', true),
    ('base_give_4',   4, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Leave alloy where your base keeps watch.', true),
    ('base_stand_4',  4, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Wait at your base. Watch who passes.', true),
    ('base_repair_4', 4, 'member', 'base_repair', NULL,    1,   1, 1000, 'A wound in your base is a door. Close it.', true),
    ('base_give_5',   5, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Alloy to the base. Now.', true),
    ('base_stand_5',  5, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Ten minutes at the base. Then move.', true),
    ('base_repair_5', 5, 'member', 'base_repair', NULL,    1,   1, 1000, 'Fix a module. Fast.', true),
    ('base_give_6',   6, 'member', 'base_give',   'alloy', 200, 1, 1000, 'Give alloy to the base you hold.', true),
    ('base_stand_6',  6, 'member', 'base_stand',  NULL,    10,  1, 1000, 'Stand at your base. Hold it.', true),
    ('base_repair_6', 6, 'member', 'base_repair', NULL,    1,   1, 1000, 'Repair a module. Hold the line.', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BaseModCollectorAlloy',    350, true, 'bases', 'Collector Array Mk I bill: alloy',   0, 100000, 10, 'From the Outpost; Mk n costs n times this'),
    ('BaseModCollectorCrystal',  250, true, 'bases', 'Collector Array Mk I bill: crystal', 0, 100000, 10, ''),
    ('BaseModCollectorData',     150, true, 'bases', 'Collector Array Mk I bill: data',    0, 100000, 10, ''),
    ('BaseModCollectorCredit',   100, true, 'bases', 'Collector Array Mk I bill: credit',  0, 100000, 10, ''),
    ('BaseModCollectorMin',      45,  true, 'bases', 'Collector Array Mk I assembly (minutes)', 1, 10080, 1, 'Mk n takes n times this'),
    ('BaseCollectorLinksMk1',    2,   true, 'bases', 'Collector Array nodes: Mk I',  0, 20, 1, 'Free nodes inside the zone one array tethers'),
    ('BaseCollectorLinksMk2',    3,   true, 'bases', 'Collector Array nodes: Mk II', 0, 20, 1, ''),
    ('BaseCollectorLinksMk3',    3,   true, 'bases', 'Collector Array nodes: Mk III', 0, 20, 1, ''),
    ('BaseCollectorLinksMk4',    4,   true, 'bases', 'Collector Array nodes: Mk IV', 0, 20, 1, ''),
    ('BasePerkLumenCrystalPct',  25,  true, 'bases', 'Lumen Collective: crystal from Collector Arrays (%)', 0, 200, 1,
     'Faction perk on bases: more crystal into the vault')
ON CONFLICT (key) DO NOTHING;
