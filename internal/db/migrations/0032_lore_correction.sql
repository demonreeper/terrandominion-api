-- 0032 Lore correction (docs/lore-correction-plan.md, changes 2 and 4; story in docs/LORE.md).

-- The Verdant Exchange power is Ishāra (owner, 2026-11-11). Missions and orders read the name from
-- this row; notices keep the text they were written with, so those are corrected too.
UPDATE td.factions SET persona = 'Ishāra' WHERE persona = 'Ishaaraath';
UPDATE td.notices SET body = replace(body, 'Ishaaraath', 'Ishāra') WHERE body LIKE '%Ishaaraath%';

-- Freelancers hear from all six powers every day: one mission each.
UPDATE td.settings SET value = 6, description = 'Daily missions each player gets (freelancers: one from each power)',
                       updated_at = now()
WHERE key = 'MissionsPerDay';

-- A player may stay a freelancer for ever, so the freelancer missions that stopped at level 19 no
-- longer stop: without this a freelancer at level 20 had none.
UPDATE td.mission_types SET max_level = 1000 WHERE audience = 'freelancer' AND max_level = 19;
