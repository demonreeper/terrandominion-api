-- 0017 crafting: consumables a player owns (energy cells and weapons), and what crafting costs.
CREATE TABLE td.player_items (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    item      text NOT NULL,
    count     integer NOT NULL DEFAULT 0 CHECK (count >= 0),
    PRIMARY KEY (player_id, item)
);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('RepairAlloyPerPoint', 0.5, true,  'Alloy per point of missing durability to repair gear, rounded up'),
    ('CraftCellCrystal',    10,  false, 'Energy cell: crystal'),
    ('CraftCellData',       5,   false, 'Energy cell: data'),
    ('CraftPulseCrystal',   20,  false, 'EM Pulse: crystal'),
    ('CraftPulseData',      10,  false, 'EM Pulse: data'),
    ('CraftJammerData',     15,  false, 'Signal Jammer: data'),
    ('CraftJammerAlloy',    10,  false, 'Signal Jammer: alloy'),
    ('CraftSpikeAlloy',     30,  false, 'Overload Spike: alloy'),
    ('CraftSpikeCrystal',   20,  false, 'Overload Spike: crystal'),
    ('CraftSpikeCredit',    10,  false, 'Overload Spike: credit');
