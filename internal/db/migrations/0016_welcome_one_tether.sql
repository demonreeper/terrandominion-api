-- 0016 welcome: a new player starts with enough to build one Tether Node.
UPDATE td.settings SET value = 80, updated_at = now() WHERE key = 'WelcomeCrystal';
UPDATE td.settings SET value = 60, updated_at = now() WHERE key = 'WelcomeAlloy';

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('WelcomeData', 20, false, 'Data granted at first sign-in, once per phone');
