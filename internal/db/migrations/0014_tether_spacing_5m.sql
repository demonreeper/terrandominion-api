-- 0014 tether spacing: the owner set the minimum distance between Tether Nodes to 5 m.
UPDATE td.settings SET value = 5, updated_at = now() WHERE key = 'TnMinSpacingM';
