-- 0024 App build lock: the server refuses app builds below these numbers (0 = no lock).
INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('MinAppBuild',    0, true, 'Lowest Android app build (version code) the server accepts; raise it to lock older builds out'),
    ('MinAppBuildIOS', 0, true, 'Lowest iOS app build number the server accepts; raise it to lock older builds out');
