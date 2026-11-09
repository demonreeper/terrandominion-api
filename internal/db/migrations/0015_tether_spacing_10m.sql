-- 0015 tether spacing: the owner set the minimum distance between Tether Nodes to 10 m.
UPDATE td.settings SET value = 10, updated_at = now() WHERE key = 'TnMinSpacingM';
