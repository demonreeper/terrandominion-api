# CLAUDE.md: Terran Dominion API

Go game server (PostgreSQL + PostGIS). Workspace layout, servers and deploy flow are in
`../CLAUDE.md` (the ProjectAR folder). Game rules: `../TerranDominion-Mobile/docs/GAME_DESIGN.md`.
Schema: `docs/DATABASE.md`. Endpoints and error codes: `docs/API.md`. Running and deploying:
`README.md`.

## Shape of the code

`httpapi -> service -> game` and `service -> store -> db`, never the other way.
- `internal/game`: rules as pure functions (no SQL, no HTTP), unit-tested.
- `internal/service`: one method per player action, **one transaction each**, rows locked
  `FOR UPDATE` where a decision depends on them.
- `internal/store`: SQL only, one function per query, works inside or outside a transaction.
- `internal/db/migrations`: numbered SQL, embedded, applied on start. **Never edit one that has run
  on a server; add a new number.**
- `internal/apperr`: numbered error codes (E1000–E1900). Never reuse a code; add new ones to
  `docs/API.md`.
- Every tunable number is a row in `td.settings` with a code default (`internal/game/settings.go`),
  changed with `PUT /v1/admin/settings/{key}`, picked up within a minute.
- The server decides: clients send intent and raw facts (GPS fix with accuracy); the server checks
  range, accuracy, travel speed, cooldowns and stores the evidence.
- Built to move onto Nakama later: `game` and `store` use only the standard library and
  `database/sql`; service methods map onto Nakama RPCs. Keep dependencies few.

## Working on it

- Windows: `.\scripts\dev.ps1 migrate|serve` runs locally with `.env` (git-ignored; holds the dev
  DB URL and secrets). Deploy to dev: `.\scripts\deploy-dev.ps1` (builds linux/**arm64**, copies
  as `tdapi.new`, renames over the running binary, restarts the service).
- `go test ./...` before committing. `go.sum` is committed.
- Test players: `POST /v1/auth/dev` (dev only). Make an admin by hand in the DB:
  `UPDATE td.players SET is_admin = true WHERE id = …`; the player signs in again to pick it up.
- DBeaver: keep **auto-commit on**. An open transaction there once blocked every sign-in
  (lock timeout, E1900) until it was ended.

## Map tiles (`deploy/maps`)

TileServer GL serves the dark Maldives style at `/tiles/{z}/{x}/{y}@2x.png` (nginx →
`127.0.0.1:8095/styles/terran/`). `maldives.mbtiles` comes from Planetiler; when
osmdata.openstreetmap.de or naciscdn.org can't be reached, `make_ocean.py` builds the sea and an
empty Natural Earth from the OSM extract. The server needs Ubuntu 24.04's ICU 74 and libpng 1.6.43 in
`/opt/td-maplibs` (see the service file); its own newer libraries make the renderer crash.
