-- 0052 friends (owner, 2026-11-11): players find each other by callsign, by full email, or from a
-- faction's branch, and send a friend request; once accepted, both see each other in a friend list
-- with a name each typed for friends. The Google/Apple name never leaves the server; email is only
-- matched whole and never shown.

-- The name a player shows their friends (typed, optional), and whether others may find them by email
-- and send them requests.
ALTER TABLE td.players
    ADD COLUMN friend_name        text CHECK (friend_name IS NULL OR length(friend_name) BETWEEN 1 AND 32),
    ADD COLUMN findable_by_email  boolean NOT NULL DEFAULT true,
    ADD COLUMN accept_friends     boolean NOT NULL DEFAULT true;

-- One row per pair (low id first). pending until the other side accepts.
CREATE TABLE td.friendships (
    low_id       bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    high_id      bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    requested_by bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    accepted_at  timestamptz,
    PRIMARY KEY (low_id, high_id),
    CHECK (low_id < high_id)
);
CREATE INDEX friendships_high_idx ON td.friendships (high_id);

-- Searches by email, counted per player so emails can't be tried by the thousand.
CREATE TABLE td.email_searches (
    player_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    searched_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_searches_idx ON td.email_searches (player_id, searched_at DESC);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('FriendsMax',           200, true, 'profile', 'Friends: most per player',            10, 2000, 10, 'Most friends (and pending requests) one player may have'),
    ('FriendRequestsPerDay', 20,  true, 'profile', 'Friends: requests per day',           1, 200,  1,  'Friend requests one player may send in 24 hours'),
    ('EmailSearchesPerDay',  10,  true, 'profile', 'Friends: email searches per day',     1, 100,  1,  'Searches by email one player may make in 24 hours')
ON CONFLICT (key) DO NOTHING;
