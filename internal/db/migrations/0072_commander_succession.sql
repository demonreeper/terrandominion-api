-- 0072 Leader succession (GAME_DESIGN "Leader succession", docs/faction-ranks-plan.md): when a regional Commander
-- steps down, goes CommanderInactiveDays without signing in, is banned or deletes the account, the highest rank in
-- that country takes the seat at once as acting Commander (ties: most contribution points, then longest in the
-- faction; Prospects and Nomads skipped). For CommanderVoteDays after the seat changes any Member or higher there can
-- start a vote; it runs CommanderVoteHours, one ballot per voter, and a majority of the ballots cast wins. With no
-- majority, or with no vote started in time, the acting Commander keeps the seat and stops being acting. Nobody left:
-- the branch goes dormant and its seat can be claimed fresh.
--
-- The acting Commander holds the seat as any Commander does (leader_player_id and rank 90), so flag approval, the
-- base rules and RegionCommanders need no change. acting_prev_rank is the rank they go back to if a vote picks someone
-- else; seat_changed_at starts the vote window.
ALTER TABLE td.faction_branches
    ADD COLUMN leader_acting    boolean NOT NULL DEFAULT false,
    ADD COLUMN acting_prev_rank smallint,
    ADD COLUMN seat_changed_at  timestamptz;

-- A vote for a branch's Commander. One open vote per branch. status: open, then won (a majority), no_majority (the
-- acting Commander stays). An admin's END VOTE closes a stuck vote early, counting it as it stands (ended_by set).
CREATE TABLE td.commander_votes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    branch_id  bigint NOT NULL REFERENCES td.faction_branches (id) ON DELETE CASCADE,
    started_by bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    opened_at  timestamptz NOT NULL DEFAULT now(),
    closes_at  timestamptz NOT NULL,
    status     text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'won', 'no_majority')),
    winner_id  bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    ballots    integer NOT NULL DEFAULT 0,
    ended_at   timestamptz,
    ended_by   bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE UNIQUE INDEX commander_votes_one_open ON td.commander_votes (branch_id) WHERE status = 'open';
CREATE INDEX commander_votes_due ON td.commander_votes (closes_at) WHERE status = 'open';

-- One ballot per voter per vote. Secret while the vote runs: only the count is shown, and each voter sees their own.
CREATE TABLE td.commander_ballots (
    vote_id      bigint NOT NULL REFERENCES td.commander_votes (id) ON DELETE CASCADE,
    voter_id     bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    candidate_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    cast_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (vote_id, voter_id)
);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('CommanderInactiveDays', 14, true, 'ranks', 'Commander inactive after (days)', 0, 365, 1,
     'A regional Commander who has not signed in for this long loses the seat to an acting Commander (0: never)'),
    ('CommanderVoteDays',     7,  true, 'ranks', 'Vote window after the seat changes (days)', 1, 60, 1,
     'Members and up may start a vote for Commander this long after an acting Commander takes the seat; with none started, they become Commander'),
    ('CommanderVoteHours',    48, true, 'ranks', 'Vote for Commander lasts (hours)', 1, 336, 1,
     'A majority of the ballots cast wins; with none, the acting Commander keeps the seat')
ON CONFLICT (key) DO NOTHING;
