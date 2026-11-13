-- 0074 War at street level (owner, 2026-11-13): attacks, drones, hacks, contests and defending (intercept, defuse,
-- repair) are refused when the app reports the phone well above the ground (a balcony, a tower flat). Harvesting,
-- collecting and building are not. The app sends its height above ground and that height's accuracy as headers
-- (X-Fix-Height, X-Fix-Height-Acc); a missing or poor height never blocks. 0 turns the rule off.

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('WarMaxHeightM', 10, true, 'combat', 'War only below this height (metres)', 0, 500, 1,
     'Above ground; 0 turns it off. An unknown or poor height never blocks')
ON CONFLICT (key) DO NOTHING;
