-- 0073 New players start with 100 credit (owner, 2026-11-12). Granted at first sign-in with the other welcome
-- resources, uploaded (so it doesn't fill the carry limit), once per phone.

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('WelcomeCredit', 100, false, 'progress', 'Welcome credit', 0, 100000, 10,
     'Credit granted at first sign-in, once per phone')
ON CONFLICT (key) DO NOTHING;
