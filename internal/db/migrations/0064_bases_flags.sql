-- 0064 faction bases, B1 (docs/base-building-plan.md 1b, 2, 4): flags carry the base name the proposer picked, the
-- tier bills and assembly times (the Spire's starts on approval), and the base cap now counts every active player
-- in the region, freelancers too (owner, 2026-11-12).

-- The name picked from six generated ones when the flag was planted (never typed); the Spire takes it.
ALTER TABLE td.base_proposals ADD COLUMN name text NOT NULL DEFAULT '';
-- Names are unique among standing bases and open flags.
CREATE INDEX bases_name ON td.bases (name) WHERE status <> 'destroyed';
CREATE INDEX base_proposals_name ON td.base_proposals (name) WHERE status = 'open';

UPDATE td.settings SET
    label = 'Base cap step 1 (active players)',
    description = 'One more base for every faction once this many players were seen in the region within the days below, freelancers included (0: off)'
 WHERE key = 'BaseCapStep1Players';
UPDATE td.settings SET label = 'Base cap step 2 (active players)' WHERE key = 'BaseCapStep2Players';
UPDATE td.settings SET label = 'Base cap step 3 (active players)' WHERE key = 'BaseCapStep3Players';
UPDATE td.settings SET label = 'Base cap step 4 (active players)' WHERE key = 'BaseCapStep4Players';
UPDATE td.settings SET
    label = 'Active player means seen within (days)',
    description = 'Who counts toward the region''s signal population for the base cap: any player whose last fix in the region is this recent'
 WHERE key = 'BaseCapActiveDays';

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BaseCostT1Alloy',   400,   true, 'bases', 'Spire bill: alloy',        0, 1000000, 50, 'Raising a Spire (tier 1)'),
    ('BaseCostT1Crystal', 400,   true, 'bases', 'Spire bill: crystal',      0, 1000000, 50, 'Raising a Spire'),
    ('BaseCostT1Data',    150,   true, 'bases', 'Spire bill: data',         0, 1000000, 50, 'Raising a Spire'),
    ('BaseCostT1Credit',  50,    true, 'bases', 'Spire bill: credit',       0, 1000000, 50, 'Raising a Spire'),
    ('BaseCostT2Alloy',   1200,  true, 'bases', 'Outpost bill: alloy',      0, 1000000, 50, 'Upgrading to tier 2'),
    ('BaseCostT2Crystal', 600,   true, 'bases', 'Outpost bill: crystal',    0, 1000000, 50, 'Upgrading to tier 2'),
    ('BaseCostT2Data',    400,   true, 'bases', 'Outpost bill: data',       0, 1000000, 50, 'Upgrading to tier 2'),
    ('BaseCostT2Credit',  200,   true, 'bases', 'Outpost bill: credit',     0, 1000000, 50, 'Upgrading to tier 2'),
    ('BaseCostT3Alloy',   3000,  true, 'bases', 'Stronghold bill: alloy',   0, 1000000, 50, 'Upgrading to tier 3'),
    ('BaseCostT3Crystal', 1500,  true, 'bases', 'Stronghold bill: crystal', 0, 1000000, 50, 'Upgrading to tier 3'),
    ('BaseCostT3Data',    1000,  true, 'bases', 'Stronghold bill: data',    0, 1000000, 50, 'Upgrading to tier 3'),
    ('BaseCostT3Credit',  500,   true, 'bases', 'Stronghold bill: credit',  0, 1000000, 50, 'Upgrading to tier 3'),
    ('BaseCostT4Alloy',   7000,  true, 'bases', 'Citadel bill: alloy',      0, 1000000, 50, 'Upgrading to tier 4'),
    ('BaseCostT4Crystal', 3500,  true, 'bases', 'Citadel bill: crystal',    0, 1000000, 50, 'Upgrading to tier 4'),
    ('BaseCostT4Data',    2500,  true, 'bases', 'Citadel bill: data',       0, 1000000, 50, 'Upgrading to tier 4'),
    ('BaseCostT4Credit',  1200,  true, 'bases', 'Citadel bill: credit',     0, 1000000, 50, 'Upgrading to tier 4'),
    ('BaseCostT5Alloy',   15000, true, 'bases', 'Capital bill: alloy',      0, 1000000, 50, 'Upgrading to tier 5'),
    ('BaseCostT5Crystal', 8000,  true, 'bases', 'Capital bill: crystal',    0, 1000000, 50, 'Upgrading to tier 5'),
    ('BaseCostT5Data',    6000,  true, 'bases', 'Capital bill: data',       0, 1000000, 50, 'Upgrading to tier 5'),
    ('BaseCostT5Credit',  3000,  true, 'bases', 'Capital bill: credit',     0, 1000000, 50, 'Upgrading to tier 5'),
    ('BaseAssemblyMinT1', 15,    true, 'bases', 'Spire assembly (minutes)',      1, 20160, 1, 'Once funded'),
    ('BaseAssemblyMinT2', 120,   true, 'bases', 'Outpost assembly (minutes)',    1, 20160, 1, 'Once funded'),
    ('BaseAssemblyMinT3', 480,   true, 'bases', 'Stronghold assembly (minutes)', 1, 20160, 1, 'Once funded'),
    ('BaseAssemblyMinT4', 1440,  true, 'bases', 'Citadel assembly (minutes)',    1, 20160, 1, 'Once funded'),
    ('BaseAssemblyMinT5', 4320,  true, 'bases', 'Capital assembly (minutes)',    1, 20160, 1, 'Once funded')
ON CONFLICT (key) DO NOTHING;
