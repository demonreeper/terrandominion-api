-- 0033 The powers' calls and messages (docs/lore-correction-plan.md, changes 6, 11, 13 and 16).
--
-- Players must not know the six powers are one, so nothing here joins them: a call or a message
-- belongs to one faction's power, like a mission does.

-- What each player mostly does, scored per power inside every action's transaction
-- (game.PlaystylePoints). The highest score is the one power that calls, messages and invites them.
CREATE TABLE td.player_playstyle (
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    points     bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, faction_id)
);

-- One row per player per kind: the level 10 path call, the level 19 offer and its repeat. A call
-- exists from the moment it is set off; the app sees it once due_at has passed. delivered_at: the
-- app fetched it (it rang, or was offered as a missed call). pushed_at: it was not fetched in time,
-- so a push went out. played_at: the player heard it; it is gone after that.
CREATE TABLE td.player_calls (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id    bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id   smallint NOT NULL REFERENCES td.factions (id),
    kind         text NOT NULL CHECK (kind IN ('level10', 'level19', 'level19_repeat')),
    line         text NOT NULL,
    due_at       timestamptz NOT NULL,
    delivered_at timestamptz,
    pushed_at    timestamptz,
    played_at    timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (player_id, kind)
);
CREATE INDEX player_calls_player_idx ON td.player_calls (player_id, due_at) WHERE played_at IS NULL;
CREATE INDEX player_calls_unanswered_idx ON td.player_calls (due_at)
    WHERE delivered_at IS NULL AND pushed_at IS NULL AND played_at IS NULL;

-- The powers' written messages, starting with the day 3 message after the level 19 offer. They stay
-- in the player's message area; read_at only clears the unread mark.
CREATE TABLE td.player_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    kind       text NOT NULL,
    body       text NOT NULL,
    due_at     timestamptz NOT NULL,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (player_id, kind)
);
CREATE INDEX player_messages_player_idx ON td.player_messages (player_id, due_at DESC);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('CallPathLevel',        10, false, 'Halfway through this level the matching power calls once, about the path the player is on'),
    ('CallRepeatHours',      24, false, 'Hours after the level 19 offer that the repeat call comes'),
    ('CallMessageHours',     48, false, 'Hours after the level 19 offer that the written message comes'),
    ('CallRingSeconds',      60, true,  'A call older than this has rung out: the app offers to play it once instead of ringing'),
    ('CallPushDelaySeconds', 45, false, 'A call the app has not fetched for this long sends a push');
