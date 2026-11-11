-- 0040 reports and moderation (docs/profile-plan.md, M1): players report another player for
-- griefing; mods warn, suspend or ban; admins lift. A ban or suspension only stops sign-in:
-- nothing the player owns is removed.

-- A suspended account can't sign in until suspended_until; then it is active again.
-- (The new status is first used by the server, not in this migration.)
ALTER TYPE td.account_status ADD VALUE IF NOT EXISTS 'suspended';
ALTER TABLE td.players ADD COLUMN suspended_until timestamptz;

CREATE TABLE td.player_sanctions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('warn', 'suspend', 'ban')),
    reason      text NOT NULL CHECK (length(reason) BETWEEN 3 AND 500),
    until       timestamptz,                       -- a suspension's end; null for a warning or a ban
    by_id       bigint REFERENCES td.players (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    lifted_at   timestamptz,                       -- an admin ended it early; the row stays as history
    lifted_by   bigint REFERENCES td.players (id) ON DELETE SET NULL,
    lift_reason text
);
CREATE INDEX player_sanctions_player_idx ON td.player_sanctions (player_id, id DESC);

CREATE TABLE td.player_reports (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reporter_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    target_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    reason      text NOT NULL CHECK (reason IN ('griefing')),
    detail      text NOT NULL,                     -- what happened, from a short list (game.ReportDetails)
    note        text NOT NULL DEFAULT '' CHECK (length(note) <= 300),
    context     jsonb NOT NULL DEFAULT '{}',       -- what the server knows about the two players
    status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'dismissed', 'actioned')),
    handled_by  bigint REFERENCES td.players (id) ON DELETE SET NULL,
    handled_at  timestamptz,
    sanction_id bigint REFERENCES td.player_sanctions (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (reporter_id <> target_id)
);
-- One open report per reporter per player.
CREATE UNIQUE INDEX player_reports_one_open ON td.player_reports (reporter_id, target_id) WHERE status = 'open';
CREATE INDEX player_reports_open_idx ON td.player_reports (target_id) WHERE status = 'open';
CREATE INDEX player_reports_reporter_idx ON td.player_reports (reporter_id, created_at DESC);

-- A warning reaches the player as a written message from "Moderation". Messages were only ever
-- from a power; one with a sender and no faction is from the game itself.
ALTER TABLE td.player_messages
    ALTER COLUMN faction_id DROP NOT NULL,
    ADD COLUMN sender text,
    ADD CONSTRAINT player_messages_from CHECK (faction_id IS NOT NULL OR sender IS NOT NULL);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('SuspendDays1',      1, false, 'moderation', 'Suspension: short (days)',   1, 365, 1, 'Days of the shortest suspension a moderator can give'),
    ('SuspendDays2',      3, false, 'moderation', 'Suspension: medium (days)',  1, 365, 1, 'Days of the middle suspension a moderator can give'),
    ('SuspendDays3',      7, false, 'moderation', 'Suspension: long (days)',    1, 365, 1, 'Days of the longest suspension a moderator can give'),
    ('ReportsPerDay',     5, false, 'moderation', 'Reports per player per day', 1, 100, 1, 'Reports one player may make in 24 hours'),
    ('ReportContextDays', 7, false, 'moderation', 'Report context (days)',      1, 90,  1, 'How far back the attacks, drone strikes, intercepts and contests attached to a report go'),
    ('SuspendRepeatDays', 30, false, 'moderation', 'Repeat suspension window (days)', 1, 365, 1, 'A second suspension inside this many days suggests the next step up')
ON CONFLICT (key) DO NOTHING;
