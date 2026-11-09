-- 0020 push: phone tokens, the notices outbox, and each player's push switches.

-- One row per phone. A token belongs to the session that registered it, so a phone that signs in
-- as someone else moves to them.
CREATE TABLE td.push_tokens (
    token      text PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    session_id bigint NOT NULL REFERENCES td.sessions (id) ON DELETE CASCADE,
    platform   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_tokens_player ON td.push_tokens (player_id);
CREATE INDEX push_tokens_session ON td.push_tokens (session_id);

-- A notice written as 'pending' in the action's own transaction is the outbox: a failed attack
-- never sends a push. tn_id lets repeats on one Tether Node be throttled.
ALTER TABLE td.notices
    ADD COLUMN push_state text NOT NULL DEFAULT 'none'
        CHECK (push_state IN ('none', 'pending', 'sent', 'skipped', 'failed')),
    ADD COLUMN pushed_at timestamptz,
    ADD COLUMN tn_id bigint REFERENCES td.tether_nodes (id) ON DELETE SET NULL;
CREATE INDEX notices_push_pending ON td.notices (id) WHERE push_state = 'pending';
CREATE INDEX notices_push_throttle ON td.notices (player_id, tn_id, kind, pushed_at DESC) WHERE push_state = 'sent';

ALTER TABLE td.players
    ADD COLUMN push_attacks boolean NOT NULL DEFAULT true,
    ADD COLUMN push_results boolean NOT NULL DEFAULT true;

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('PushEnabled',               1,  false, 'Send phone notifications (0 = off)'),
    ('PushAttackThrottleMinutes', 10, false, 'One attack or jam push per Tether Node per this many minutes'),
    ('PushMaxAgeMinutes',         15, false, 'Notices older than this are not pushed, so an outage does not send old news');
