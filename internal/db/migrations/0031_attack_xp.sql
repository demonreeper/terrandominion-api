-- 0031 Attacks give XP (owner, 2026-11-10). A hit that lands on another player's Tether Node pays
-- the attacker XpAttack, plus XpDestroy when it destroys the node; an intercept pays the defender
-- XpIntercept. Only XpAttackPerTargetDay of each pay per pair of players per day (Maldives time),
-- so two friends can't level by hitting each other: the attacks still work, they stop paying.
-- The ledger keeps who the XP was earned against, which is what the daily count reads.

ALTER TABLE td.xp_events ADD COLUMN target_player_id bigint REFERENCES td.players (id) ON DELETE SET NULL;

CREATE INDEX xp_events_target_idx ON td.xp_events (player_id, target_player_id, created_at DESC)
    WHERE target_player_id IS NOT NULL;

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('XpAttack',             25, false, 'XP for a hit on another player''s Tether Node (EM Pulse, jammer, spike when it fires, drone when it lands)'),
    ('XpDestroy',            50, false, 'Extra XP when that hit destroys the Tether Node'),
    ('XpIntercept',          20, false, 'XP for intercepting a drone at your own Tether Node'),
    ('XpAttackPerTargetDay',  3, false, 'Attacks (and, separately, intercepts) that pay XP per pair of players per day');
