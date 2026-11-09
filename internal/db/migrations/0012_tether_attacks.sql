-- 0012 tether attacks: the permission rules and their penalty. The weapons that deal the damage
-- are still to be designed.
ALTER TABLE td.players ADD COLUMN protection_lost_until timestamptz;

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('PvpLevelBand',       5, true, 'Freelancers may attack a freelancer''s Tether Node within this many levels'),
    ('ProtectionLossDays', 3, true, 'Days a freelancer loses protection after attacking a faction Tether Node');
