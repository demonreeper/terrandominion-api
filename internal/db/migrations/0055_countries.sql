-- 0055 region from GPS, R1 (docs/social-signal-plan.md Part B). A region is a whole country. td.countries holds
-- Natural Earth admin-0 shapes (public domain), loaded once per database by cmd/loadcountries; CountryAt finds the
-- country a fix is in, or the nearest one within CountryAtMaxKm (the 1:10m shapes miss reefs and lagoons).
-- Claiming and joining use the country the player stands in. A country can be claimed only when it is live
-- ("LiveRegions": the countries with live = true; the Maldives first). Admins add test regions on dev (codes XA..XZ)
-- so the whole flow can be tried in Malé; the server ignores them on production.

CREATE TABLE td.countries (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       text NOT NULL CHECK (code ~ '^[A-Z]{2}$'),
    name       text NOT NULL,
    -- main: this row's shape is the country itself, not an overseas territory filed under the same code.
    main       boolean NOT NULL DEFAULT true,
    test       boolean NOT NULL DEFAULT false,
    geom       geometry(MultiPolygon, 4326) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT test OR code LIKE 'X%')
);
CREATE INDEX countries_geom_idx ON td.countries USING gist (geom);
CREATE INDEX countries_code_idx ON td.countries (code);

-- Which countries are live (LiveRegions). Kept apart from the shapes so reloading them keeps the list.
CREATE TABLE td.live_regions (
    code       text PRIMARY KEY CHECK (code ~ '^[A-Z]{2}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO td.live_regions (code) VALUES ('MV') ON CONFLICT DO NOTHING;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('CountryAtMaxKm', 60, false, 'factions', 'Region: nearest country within (km)', 1, 500, 1,
     'A fix outside every country shape (a reef, a lagoon, the sea) counts as the nearest country within this distance')
ON CONFLICT (key) DO NOTHING;
