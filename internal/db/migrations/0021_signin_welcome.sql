-- 0021 Google sign-in launch: a welcome big enough for ten Tether Nodes and a few energy cells
-- (into the uploaded inventory, so carry room stays free), and inactive deletion after a year.
UPDATE td.settings SET value = 800, updated_at = now() WHERE key = 'WelcomeCrystal';
UPDATE td.settings SET value = 600, updated_at = now() WHERE key = 'WelcomeAlloy';
UPDATE td.settings SET value = 200, updated_at = now() WHERE key = 'WelcomeData';
UPDATE td.settings SET value = 365, updated_at = now() WHERE key = 'InactiveDeleteDays';

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('WelcomeCells', 5, false, 'Energy cells granted at first sign-in, once per phone');
