-- 0009 tether tier 2: the upgrade to four links and the range of the info panel.
INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('TnLinksTier2',    4,   true, 'Links on a tier 2 Tether Node'),
    ('TnUpgradeAlloy',  120, true, 'Alloy to upgrade a Tether Node to tier 2'),
    ('TnUpgradeData',   60,  true, 'Data to upgrade a Tether Node to tier 2'),
    ('TnUpgradeCredit', 40,  true, 'Credit to upgrade a Tether Node to tier 2'),
    ('TnInfoRangeM',    40,  true, 'Distance at which a player sees what a Tether Node collects');
