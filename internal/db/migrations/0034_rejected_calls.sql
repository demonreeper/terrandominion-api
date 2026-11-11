-- 0034 A ringing call can be answered or rejected (owner, 2026-11-11). A rejected call stops ringing
-- and goes to the player's missed calls, in the message area, where it can be played once.
-- rejected_at: the player turned it down (or let it ring out in the app); played_at still ends it.

ALTER TABLE td.player_calls ADD COLUMN rejected_at timestamptz;
