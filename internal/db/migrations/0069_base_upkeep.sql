-- 0069 faction bases, B6 (docs/base-building-plan.md 7): upkeep. A base needs a member on site every BaseUpkeepDays or it
-- fades (no new projects, shield halved, dimmed on the map, its members told); each further BaseFadeDropDays it drops a
-- tier; a faded Spire collapses after BaseSpireCollapseDays. Any member on site ends the fade at once (bases.last_visit_at,
-- kept by the visit tick since B4).

-- When the fade last cost the base a tier (the next drop counts from it).
ALTER TABLE td.bases ADD COLUMN fade_dropped_at timestamptz;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BaseUpkeepDays',        7,  true, 'bases', 'A member on site at least every (days)', 1, 90, 1,
     'Missed, the base fades: no new projects, shield halved, dimmed on the map'),
    ('BaseFadeDropDays',      7,  true, 'bases', 'A faded base drops a tier every (days)', 1, 90, 1,
     'Outer modules packed away to the vault, as in a fight'),
    ('BaseSpireCollapseDays', 14, true, 'bases', 'A faded Spire collapses after (days)', 1, 180, 1,
     'Counted from when it began to fade; half its vault drops on the map'),
    ('BaseFadedShieldPct',    50, true, 'bases', 'A faded base''s shield works at (%)', 0, 100, 5, 'The plan: halved')
ON CONFLICT (key) DO NOTHING;
