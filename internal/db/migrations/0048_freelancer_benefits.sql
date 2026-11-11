-- 0048 freelancer benefits (docs/freelancer-path-plan.md phase 3): the daily favour, members' one
-- mission from their own power, and neutral standing (which reuses ProtectionLossDays).
INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('FavourPerkPct',        40, true, 'factions', 'Daily favour (% of the faction perk)', 0, 100, 1,
     'Each power whose mission a freelancer finishes today gives this percent of its faction perk until midnight (0 switches the favour off)'),
    ('MemberMissionsPerDay', 1,  true, 'missions', 'Member missions per day', 0, 6, 1,
     'Daily missions a faction member gets from their own power (Wardens and above also get a command order)')
ON CONFLICT (key) DO NOTHING;

UPDATE td.settings SET description = 'Days a freelancer loses neutral standing after attacking a faction Tether Node: until then faction players can''t hit their Tether Nodes'
WHERE key = 'ProtectionLossDays';
