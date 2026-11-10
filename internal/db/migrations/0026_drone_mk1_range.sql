-- 0026 Drone Bay Mk I reaches 150 m (owner, 2026-11-09); 1 km moves to Mk III (drone strike plan, milestone 1).
UPDATE td.gear_types SET stats = '{"drones": 1, "range_m": 150, "damage": 15}' WHERE id = 'drone_bay_mk1';
