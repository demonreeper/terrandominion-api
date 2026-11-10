-- 0023 My Tether Nodes: when one counts as under attack, and how long an ended one stays listed.
INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('TnUnderAttackMinutes', 10, true, 'A Tether Node counts as under attack for this many minutes after a hit'),
    ('TnEndedShowMinutes',   60, true, 'Minutes an ended Tether Node stays on My Tether Nodes');
