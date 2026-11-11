-- 0042 Tether Node slowdown (owner, 2026-11-11): Tether Nodes gathered too fast for an owner who
-- never came back. A Tether Node now gathers at full speed for TnFullSpeedHours after it is built
-- or after its owner last collected at it, then TnSlowPctPerHour slower each hour, down to
-- TnSlowFloorPct of full speed. Collecting at the Tether Node brings it back to full speed.
-- Slower means each link waits longer between harvests (the 5-minute cooldown divided by the speed),
-- so a neglected Tether Node also drains its resource nodes less.

-- When the owner last collected at it; null until the first collect (the clock then runs from built_at).
ALTER TABLE td.tether_nodes ADD COLUMN collected_at timestamptz;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('TnFullSpeedHours', 2,  true, 'tether', 'Tether Node: hours at full speed', 0, 48, 0.5,
     'Hours a Tether Node gathers at full speed after it is built or its owner last collected at it'),
    ('TnSlowPctPerHour', 10, true, 'tether', 'Tether Node: slowdown per hour (%)', 0, 100, 1,
     'After the full-speed hours, how much slower a Tether Node gathers for each further hour its owner stays away (percent of full speed; 0 switches the slowdown off)'),
    ('TnSlowFloorPct',   20, true, 'tether', 'Tether Node: slowest speed (%)', 1, 100, 1,
     'The slowest a neglected Tether Node gathers, as a percent of full speed')
ON CONFLICT (key) DO NOTHING;
