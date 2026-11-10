-- 0030 Harvest missions count harvests, not amounts (owner, 2026-11-10): "Harvest crystal 20 times"
-- is 20 crystal harvests. The lines that read as amounts ("Twenty crystal today") now say harvests,
-- on the pool and on missions already given.

UPDATE td.mission_types SET line = v.line
FROM (VALUES
    ('sig_f1', 'Crystal holds the light of every hand that touched it. Harvest it three times.'),
    ('sig_f2', 'Patience is a harvest too. Ten crystal harvests, gathered without hurry.'),
    ('ish_f2', 'Information is the oldest currency. Harvest data three times.'),
    ('ish_f3', 'Credit moves those who hold it. Harvest it three times.'),
    ('ish_f4', 'The market listens to those who know. Eight data harvests.'),
    ('sem_f2', 'Hidden things leave traces. Harvest data five times, quietly.'),
    ('toh_f2', 'Speed needs metal. Harvest alloy six times.'),
    ('sig_m1', 'Lumen grows by what we gather. Twenty crystal harvests today.'),
    ('shi_m4', 'Fifteen alloy harvests. Forge builds with what you bring.'),
    ('ish_m2', 'Six credit harvests. Wealth is a signal others follow.'),
    ('ish_m3', 'Twelve data harvests. Know the market before it knows you.'),
    ('sem_m1', 'Ten data harvests, gathered where no one is looking.'),
    ('toh_m4', 'Twelve alloy harvests. Speed is built from metal.')
) AS v (id, line)
WHERE td.mission_types.id = v.id;

UPDATE td.player_missions m SET line = t.line
FROM td.mission_types t
WHERE m.mission_type_id = t.id AND m.goal = 'harvest' AND m.goal_key IS NOT NULL AND m.line <> t.line;
