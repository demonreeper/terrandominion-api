-- 0013 tether spacing: Tether Nodes may not stand on top of each other.
INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('TnMinSpacingM', 50, true, 'Minimum distance between any two active Tether Nodes');
