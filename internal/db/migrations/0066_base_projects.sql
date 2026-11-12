-- 0066 faction bases, B2 (docs/base-building-plan.md 4 and 5): projects, giving on site, the vault for Wardens and
-- Captains (faction-ranks-plan.md), the presence speed-up, contribution points and the first four modules. The tables
-- came with 0063; this adds the numbers.

-- Who gave from the vault today (the Captain's daily cap) is read from base_contributions.from_vault.
CREATE INDEX base_contributions_vault_day ON td.base_contributions (player_id, at) WHERE from_vault;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('BaseProjectsAtOnce',       2,   true,  'bases', 'Projects at once per base',             1, 10,    1, 'Raising the Spire counts'),
    ('BasePresenceStepPct',      10,  true,  'bases', 'Assembly speed-up per member on site (%)', 0, 100, 1,
     'Each member on site with the app open speeds assembly by this much'),
    ('BasePresenceMaxMult',      2,   true,  'bases', 'Assembly speed-up cap (times)',          1, 10,    0.1, 'The most members on site can speed it up'),
    ('BasePresenceSec',          90,  false, 'bases', 'On site means a fix within (seconds)',   15, 600,  5,
     'A member counts as on site while their last fix inside the zone is this recent'),
    ('BasePointsPerUnit',        1,   true,  'bases', 'Contribution points per resource given', 0, 100,   0.1,
     'Points for each unit given from your own storage (vault payments earn none)'),
    ('BaseGiveXpPer100',         5,   true,  'bases', 'XP per 100 resources given',             0, 1000,  1, 'A little XP for giving'),
    ('BaseModuleHealth',         300, true,  'bases', 'Module health per Mk',                   1, 100000, 10, 'A module''s full health is this times its Mk'),
    ('BaseProtectNewHours',      24,  true,  'bases', 'New base protection (hours)',            0, 240,   1, 'From the moment the Spire stands'),
    ('BasePerkForgeAssemblyPct', 25,  true,  'bases', 'Forge Meridian: assembly faster (%)',    0, 100,   1, 'Faction perk on bases'),
    ('BasePerkVerdantCostPct',   15,  true,  'bases', 'Verdant Exchange: projects cheaper (%)', 0, 90,    1, 'Faction perk on bases'),
    ('BaseModLatticeAlloy',      300, true,  'bases', 'Energy Lattice Mk I bill: alloy',   0, 100000, 10, 'Upgrading to Mk n costs n times this'),
    ('BaseModLatticeCrystal',    200, true,  'bases', 'Energy Lattice Mk I bill: crystal', 0, 100000, 10, ''),
    ('BaseModLatticeData',       100, true,  'bases', 'Energy Lattice Mk I bill: data',    0, 100000, 10, ''),
    ('BaseModLatticeCredit',     50,  true,  'bases', 'Energy Lattice Mk I bill: credit',  0, 100000, 10, ''),
    ('BaseModLatticeMin',        30,  true,  'bases', 'Energy Lattice Mk I assembly (minutes)', 1, 10080, 1, 'Mk n takes n times this'),
    ('BaseModShieldAlloy',       250, true,  'bases', 'Shield Matrix Mk I bill: alloy',    0, 100000, 10, 'Upgrading to Mk n costs n times this'),
    ('BaseModShieldCrystal',     300, true,  'bases', 'Shield Matrix Mk I bill: crystal',  0, 100000, 10, ''),
    ('BaseModShieldData',        150, true,  'bases', 'Shield Matrix Mk I bill: data',     0, 100000, 10, ''),
    ('BaseModShieldCredit',      50,  true,  'bases', 'Shield Matrix Mk I bill: credit',   0, 100000, 10, ''),
    ('BaseModShieldMin',         30,  true,  'bases', 'Shield Matrix Mk I assembly (minutes)', 1, 10080, 1, 'Mk n takes n times this'),
    ('BaseModVaultAlloy',        400, true,  'bases', 'Vault Mk I bill: alloy',            0, 100000, 10, 'Upgrading to Mk n costs n times this'),
    ('BaseModVaultCrystal',      100, true,  'bases', 'Vault Mk I bill: crystal',          0, 100000, 10, ''),
    ('BaseModVaultData',         100, true,  'bases', 'Vault Mk I bill: data',             0, 100000, 10, ''),
    ('BaseModVaultCredit',       100, true,  'bases', 'Vault Mk I bill: credit',           0, 100000, 10, ''),
    ('BaseModVaultMin',          30,  true,  'bases', 'Vault Mk I assembly (minutes)',     1, 10080, 1, 'Mk n takes n times this'),
    ('BaseModUplinkAlloy',       200, true,  'bases', 'Uplink Mk I bill: alloy',           0, 100000, 10, 'Upgrading to Mk n costs n times this'),
    ('BaseModUplinkCrystal',     150, true,  'bases', 'Uplink Mk I bill: crystal',         0, 100000, 10, ''),
    ('BaseModUplinkData',        200, true,  'bases', 'Uplink Mk I bill: data',            0, 100000, 10, ''),
    ('BaseModUplinkCredit',      50,  true,  'bases', 'Uplink Mk I bill: credit',          0, 100000, 10, ''),
    ('BaseModUplinkMin',         30,  true,  'bases', 'Uplink Mk I assembly (minutes)',    1, 10080, 1, 'Mk n takes n times this')
ON CONFLICT (key) DO NOTHING;

-- The tick advances assembling projects by the time since it last looked.
ALTER TABLE td.base_projects ADD COLUMN ticked_at timestamptz;
