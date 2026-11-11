-- 0056 view region, R2 (docs/social-signal-plan.md Part B). A member can view any unlocked region of their own
-- faction (one with a Regional Commander), from anywhere, as often as they like: talk only. branch_id stays the
-- home region (rank, seat, vote, jobs, orders); viewing_branch_id is the region the FACTIONS screen shows
-- (null = home). An admin's MOVE REGION changes the home for a player who really moved.
ALTER TABLE td.faction_members
    ADD COLUMN viewing_branch_id bigint REFERENCES td.faction_branches (id) ON DELETE SET NULL;
CREATE INDEX faction_members_viewing_idx ON td.faction_members (viewing_branch_id) WHERE viewing_branch_id IS NOT NULL;
