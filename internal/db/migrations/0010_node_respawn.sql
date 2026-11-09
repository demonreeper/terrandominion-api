-- 0010 node respawn: a depleted node is retired and a new one of the same resource appears nearby.
INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('NodeRespawnMinM', 100, false, 'Closest a respawned node may appear to the one it replaces'),
    ('NodeRespawnMaxM', 300, false, 'Farthest a respawned node may appear from the one it replaces');
