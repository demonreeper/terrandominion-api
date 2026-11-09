-- 0005 carry: carry capacity was too small for play testing: raise it 10x (all numbers are tuning values).
-- Field Extractor Mk I 200 -> 2000, Mk II 350 -> 3500. Players' gear reads its stats from
-- gear_types, so existing players get the new capacity at once.
UPDATE td.gear_types SET stats = jsonb_set(stats, '{carry}', '2000') WHERE id = 'extractor_mk1';
UPDATE td.gear_types SET stats = jsonb_set(stats, '{carry}', '3500') WHERE id = 'extractor_mk2';
