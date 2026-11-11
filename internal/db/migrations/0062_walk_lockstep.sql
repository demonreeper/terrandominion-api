-- 0062 walk together W3 (part) (docs/walk-together-plan.md L4, built with G2's pattern checks): linked members
-- whose paths match too perfectly (the same steps, jitter and all) make a Signal watch report. Report only.
-- L7 (phone integrity) waits for G3.

ALTER TABLE td.walk_link_members ADD COLUMN lockstep_reported boolean NOT NULL DEFAULT false;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('WalkLockstepSteps',   8,   false, 'walk', 'Lockstep: steps (L4)',                 3, 40,  1,
     'Linked members whose last this many moving steps all match are reported (report only)'),
    ('WalkLockstepM',       0.5, false, 'walk', 'Lockstep: steps match within (metres)', 0.1, 5, 0.1,
     'How closely two members'' steps must match to count as lockstep'),
    ('WalkLockstepPairSec', 5,   false, 'walk', 'Lockstep: fixes paired within (seconds)', 1, 30, 1,
     'Two members'' fixes this close in time are compared as one step')
ON CONFLICT (key) DO NOTHING;
