-- 0057 movement patterns, G2 (docs/social-signal-plan.md Part C, D3): the last fixes the server accepted from each
-- player, to spot what joystick and replay apps leave behind: the same accuracy every time, no jitter at all while
-- standing, a dead-straight line at a constant speed. Report only: never a lock.
CREATE TABLE td.player_fix_trail (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    lat        double precision NOT NULL,
    lon        double precision NOT NULL,
    accuracy_m double precision NOT NULL,
    fixed_at   timestamptz NOT NULL
);
CREATE INDEX player_fix_trail_idx ON td.player_fix_trail (player_id, id DESC);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('SpoofPatternEnabled',  1,  false, 'anticheat', 'Movement patterns: check',              0, 1,   1,
     '1: look for too-perfect movement in the last fixes (report only, never a lock)'),
    ('SpoofPatternFixes',    30, false, 'anticheat', 'Movement patterns: fixes looked at',    10, 200, 5,
     'How many of a player''s last fixes the pattern checks read'),
    ('SpoofPatternGapHours', 24, false, 'anticheat', 'Movement patterns: hours between reports', 1, 720, 1,
     'At most one pattern report per player in this many hours')
ON CONFLICT (key) DO NOTHING;
