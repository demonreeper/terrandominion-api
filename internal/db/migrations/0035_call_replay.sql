-- 0035 ALERTS keeps the calls (owner, 2026-11-11). A call stays in the player's call log once
-- answered or rejected. A rejected call can be played once; an answered call can be heard once more.
-- replayed_at: the answered call was heard again from ALERTS; after that it only stays listed.

ALTER TABLE td.player_calls ADD COLUMN replayed_at timestamptz;
