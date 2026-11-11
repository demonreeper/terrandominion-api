-- 0045 faction colours (owner, 2026-11-11): final. The design document's colours replace the ones
-- seeded with the placeholders in 0003. Everything that shows a faction or its power reads the
-- colour from here.
UPDATE td.factions f SET color = c.color
FROM (VALUES
    (1, '#4FD8FF'),   -- Lumen Collective
    (2, '#FFB52E'),   -- Forge Meridian
    (3, '#4CF08A'),   -- Verdant Exchange
    (4, '#B57CFF'),   -- Veil Cipher
    (5, '#FF5D6C'),   -- Slipstream Vanguard
    (6, '#DBE8F5')    -- Bastion Order
) AS c(id, color)
WHERE f.id = c.id;
