-- 0022 notices say who and how much: the other player (display name read at list time) and the
-- damage or resources involved. Severity is not stored; it follows from the kind.
ALTER TABLE td.notices
    ADD COLUMN actor_id bigint REFERENCES td.players (id) ON DELETE SET NULL,
    ADD COLUMN amount   bigint;
