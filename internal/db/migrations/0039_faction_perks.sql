-- 0039 faction perks (docs/profile-plan.md, P1b): the perks' numbers as settings, tuned from the
-- admin settings screen. Each is a percentage; 0 switches the perk off. They apply to members
-- only, on the server (game.FactionPerks).

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('PerkSignumCrystalPct',       25,  true, 'factions', 'Perk: crystal bonus',            0, 100, 1,
     'Lumen Collective members get this much more crystal from each harvest and from what their Tether Nodes gather (percent)'),
    ('PerkShingoUpgradePct',       25,  true, 'factions', 'Perk: Tether Node upgrade discount', 0, 90, 1,
     'Forge Meridian members pay this much less to upgrade a Tether Node (percent)'),
    ('PerkSemaScanPct',            100, true, 'factions', 'Perk: scan range bonus',         0, 300, 1,
     'Veil Cipher members scan this much further (percent; 100 = twice as far)'),
    ('PerkSemaRarePct',            100, true, 'factions', 'Perk: rare node range bonus',    0, 300, 1,
     'Veil Cipher members see rare roaming nodes from this much further than a plain scan (percent; 100 = twice as far)'),
    ('PerkIsharaBuildPct',         20,  true, 'factions', 'Perk: Tether Node build discount', 0, 90, 1,
     'Verdant Exchange members pay this much less to build a Tether Node (percent)'),
    ('PerkIsharaMissionCreditPct', 25,  true, 'factions', 'Perk: mission credit bonus',     0, 200, 1,
     'Verdant Exchange members get this much more credit from missions (percent)'),
    ('PerkTohuDroneSpeedPct',      30,  true, 'factions', 'Perk: drone speed bonus',        0, 200, 1,
     'Slipstream Vanguard members'' drones fly this much faster (percent)'),
    ('PerkTohuCarryPct',           25,  true, 'factions', 'Perk: carry bonus',              0, 200, 1,
     'Slipstream Vanguard members carry this much more (percent)'),
    ('PerkMerkiDamagePct',         15,  true, 'factions', 'Perk: Tether Node damage reduction', 0, 90, 1,
     'Bastion Order members'' Tether Nodes take this much less damage (percent)'),
    ('PerkMerkiInterceptPct',      10,  true, 'factions', 'Perk: intercept bonus',          0, 100, 1,
     'Bastion Order members'' intercepts stop this much more of a drone''s damage (percentage points added to the share)')
ON CONFLICT (key) DO NOTHING;
