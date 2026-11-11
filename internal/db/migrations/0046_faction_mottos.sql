-- 0046 faction mottos (owner, 2026-11-11): final. The six from the design, which are also lines
-- their powers speak. Shown on each faction's card.
UPDATE td.factions f SET motto = m.motto
FROM (VALUES
    (1, 'Every shard remembers the light.'),        -- Lumen Collective
    (2, 'What we build, the world must carry.'),    -- Forge Meridian
    (3, 'Everything has a price. We set it.'),      -- Verdant Exchange
    (4, 'You cannot take what you cannot see.'),    -- Veil Cipher
    (5, 'First to the node. First to the dawn.'),   -- Slipstream Vanguard
    (6, 'Hold the line. Hold the claim.')           -- Bastion Order
) AS m(id, motto)
WHERE f.id = m.id;
