-- 0041 admin players (docs/profile-plan.md, A2): the failed location checks start being stored, so
-- an admin can see them on a player. Adjustments, role changes and callsign resets are rows of
-- td.admin_actions (target = the player's id), as every admin action already is.

CREATE TABLE td.infringements (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('gps_weak', 'travel_speed')),
    detail     text NOT NULL DEFAULT '',          -- the numbers behind it; never a position
    path       text NOT NULL DEFAULT '',          -- the action that was refused
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX infringements_player_idx ON td.infringements (player_id, created_at DESC);

-- A player's past admin actions (adjustments, role changes, callsign resets) are read by target.
CREATE INDEX IF NOT EXISTS admin_actions_target_idx ON td.admin_actions (target, id DESC);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('InfringementGapMinutes', 10, false, 'moderation', 'Infringement record gap (minutes)', 0, 1440, 1,
     'A failed location check of the same kind is stored at most once per player in this many minutes')
ON CONFLICT (key) DO NOTHING;
