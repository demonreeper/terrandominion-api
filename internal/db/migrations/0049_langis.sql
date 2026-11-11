-- 0049 the seventh voice (docs/freelancer-path-plan.md phase 4, docs/LORE.md "Langis"): a voice
-- that poses as a freelancer and tells players to stay free. It is never mentioned anywhere a player
-- sees before its first call: the account and its lines stay on the server until that call is due.

-- Its system account, in the held range (game/reserved.go). Nobody signs in as it; it is never found
-- by search, share code, report or sanction, like the six powers' accounts.
INSERT INTO td.players (id, display_name, type, country_code)
OVERRIDING SYSTEM VALUE VALUES (8, 'signal_unbound', 'system', 'MV')
ON CONFLICT (id) DO NOTHING;

-- Its calls belong to no faction: the sender names it instead, like Moderation's messages.
ALTER TABLE td.player_calls
    ALTER COLUMN faction_id DROP NOT NULL,
    ADD COLUMN sender text,
    DROP CONSTRAINT player_calls_kind_check,
    ADD CONSTRAINT player_calls_kind_check CHECK (kind IN ('level10', 'level19', 'level19_repeat', 'langis_l18', 'langis_l19')),
    ADD CONSTRAINT player_calls_from CHECK (faction_id IS NOT NULL OR sender IS NOT NULL);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('LangisCallLevel',          18, false, 'missions', 'Seventh voice: first call level', 2, 19, 1,
     'Halfway through this level the seventh voice calls a freelancer once ("Unknown caller")'),
    ('LangisMessageDelayHours',  2,  false, 'missions', 'Seventh voice: message after day 3 (hours)', 0, 72, 0.5,
     'Hours after the power''s level 19 day-3 message that the seventh voice''s written message arrives'),
    ('LangisCallDelayHours',     24, false, 'missions', 'Seventh voice: second call after its message (hours)', 1, 96, 1,
     'Hours after its written message that the seventh voice calls the second time'),
    ('LangisRareEveryDays',      7,  false, 'missions', 'Seventh voice: days between rare messages', 1, 90, 1,
     'After the faction level, a freelancer hears from the seventh voice at most once in this many days')
ON CONFLICT (key) DO NOTHING;
