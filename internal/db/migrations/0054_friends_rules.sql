-- 0054 friends, FR1 (docs/social-signal-plan.md Part A): the plan's rules on top of 0052. A decline is
-- silent but stops the same sender asking again for FriendRedeclineDays; pending requests expire after
-- FriendRequestExpireDays; a player may have FriendRequestsPendingMax requests out; searches are rate limited.

CREATE TABLE td.friend_declines (
    from_player bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    to_player   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    declined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (from_player, to_player)
);

-- Email search compares the whole address in lower case.
CREATE INDEX IF NOT EXISTS player_identities_email_lower_idx ON td.player_identities (lower(email));

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('FriendRequestsPendingMax', 50, true, 'profile', 'Friends: requests waiting (most)',     5, 500, 5,
     'Most friend requests one player may have waiting for an answer'),
    ('FriendSearchPerMinute',    20, true, 'profile', 'Friends: searches per minute',         1, 120, 1,
     'Player searches one player may make in a minute'),
    ('FriendRequestExpireDays',  14, true, 'profile', 'Friends: request expires (days)',      1, 90,  1,
     'A friend request nobody answers is dropped after this many days'),
    ('FriendRedeclineDays',      7,  true, 'profile', 'Friends: ask again after a decline (days)', 0, 90, 1,
     'After a decline the same player can''t ask again for this many days')
ON CONFLICT (key) DO NOTHING;
