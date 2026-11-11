# Tether Rise API

Go game server for Tether Rise: PostgreSQL + PostGIS, JWT sessions, run behind nginx as a
systemd service, with separate dev and live instances on separate databases.

The mobile client is [terrandominion-mobile](https://github.com/demonreeper/terrandominion-mobile);
this repository owns the API contract it follows ([docs/API.md](docs/API.md)).

- Schema and rules: [docs/DATABASE.md](docs/DATABASE.md)
- Endpoints and error codes: [docs/API.md](docs/API.md)

## Layout

```
cmd/tdapi/            entry point: serve | migrate | job <name> | version
internal/
  game/               the rules, as pure functions. No SQL, no HTTP. Unit-tested.
  service/            one method per action; one transaction each; asks game, writes via store
  store/              SQL only, one function per query; works inside or outside a transaction
  httpapi/            thin handlers, middleware (auth, admin, rate limit, request ids), JSON
  auth/               Google ID-token verification, access tokens, refresh and device hashing
  db/                 connection, embedded migrations, transaction helper
  db/migrations/      numbered SQL files, applied in order, never edited after live
  jobs/               scheduled tasks run by cron (node-regrow, account-cleanup, session-prune)
  config/             environment configuration
  apperr/             numbered error codes shared by responses and logs
deploy/               systemd units, nginx site, crontab
docs/                 database and API reference
```

The dependency direction is one-way: `httpapi -> service -> game` and `service -> store -> db`.
Game rules never import SQL or HTTP, which keeps them testable and portable.

## Running locally

```bash
cp .env.example .env          # fill in TD_DATABASE_URL and the secrets
go mod tidy                   # first time: resolves dependencies and writes go.sum
set -a; . ./.env; set +a
go test ./...
go run ./cmd/tdapi serve      # applies migrations, then listens on TD_ADDR
```

On Windows (PowerShell), `scripts/dev.ps1` loads `.env` for you:

```powershell
go mod tidy
.\scripts\dev.ps1 migrate
.\scripts\dev.ps1 serve
```

With `TD_ENV=dev` and `TD_DEV_LOGIN=true`, `POST /v1/auth/dev {"name":"me","deviceId":"...",
"platform":"editor"}` signs in without Google, for the Unity editor and quick tests. It does not
exist on live.

The owner is player 1, the primary admin: set `TD_PRIMARY_ADMIN_EMAIL` in `.env` and the first sign-in with that
email on a database with no player 1 takes it (ids 2 to 10 are reserved, players start at 11). Make another admin
by hand: `UPDATE td.players SET is_admin = true WHERE id = …;`

## Deploying (same pattern as the MonoCity server)

1. Build: `make build` (or `.\scripts\build-linux.ps1` on Windows) produces `bin/tdapi` for
   Linux **arm64**, which is what the API server runs, with the git version stamped in.
2. Copy the binary to `/home/ubuntu/terrandominion-api/` (live) or
   `/home/ubuntu/terrandominion-api-dev/` (dev), with an env file beside it (`chmod 600`).
3. Install the units from `deploy/`: `terrandominion-api.service` (live, port 8091 — set TD_ADDR=127.0.0.1:8091 in its .env) and
   `terrandominion-api-dev.service` (dev, port 8090). `systemctl enable --now ...`
4. Add the nginx site from `deploy/nginx-terrandominion.conf` and get certificates.
5. Install the crontab lines from `deploy/crontab.example`.

Dev in one step from Windows: `.\scripts\deploy-dev.ps1` builds, copies the binary as `tdapi.new`,
renames it over the running one (it cannot be overwritten while the service has it open) and
restarts the service.

Migrations run on start-up, so deploying a binary with a new migration upgrades the database.
Deploy to dev first; live only after dev has run the migration.

## Configuration

| Variable | Meaning |
|---|---|
| `TD_ENV` | `dev` or `live` |
| `TD_ADDR` | listen address, e.g. `127.0.0.1:8090` (nginx proxies to it) |
| `TD_DATABASE_URL` | `postgres://user:pass@db-host:5432/terrandominion_dev?sslmode=disable` |
| `TD_JWT_SECRET` | 32+ random bytes; different for dev and live |
| `TD_DEVICE_PEPPER` | 16+ random bytes; never change once players exist |
| `TD_GOOGLE_CLIENT_IDS` | comma-separated OAuth client ids (Android, iOS, web) |
| `TD_APPLE_BUNDLE_IDS` | comma-separated app bundle ids for Sign in with Apple (`com.hisstudios.terrandominion`) |
| `TD_TRUST_PROXY` | `true` behind nginx (reads `X-Real-IP`) |
| `TD_DEV_LOGIN` | `true` to enable `/v1/auth/dev` (ignored on live) |

Tunable game numbers are not environment variables: they are rows in `td.settings`, changed with
`PUT /v1/admin/settings/{key}` and picked up within a minute.

## Design rules

These come from what went wrong and right on MonoCity's server:

1. **The server decides.** The client sends intent and raw facts (a GPS fix); the server checks
   accuracy, distance, travel speed and cooldowns, and stores the evidence (`td.harvests`).
2. **Invariants live in the database** as constraints, unique indexes and triggers, so a buggy or
   hostile request cannot break them.
3. **One action, one transaction.** Rows a decision depends on are locked (`FOR UPDATE`), so
   double taps and races resolve correctly.
4. **Every number is a setting** with a code default; no deploy needed to rebalance.
5. **Numbered errors everywhere**: the client switches on the code, the log carries it.
6. **Migrations only**: no hand-run SQL on live; the schema is the migrations.
7. **No secrets or personal ids in the repo, logs or file names**; refresh tokens and device ids
   are stored hashed.
8. **Accounts**: multi-device sessions, self-deletion with a grace period, inactive cleanup,
   anti-farming welcome grant once per phone.

## Moving onto Nakama later

Built to make that a contained job:
- `game` and `store` use only the standard library and `database/sql`, which is exactly what a
  Nakama Go module receives, so both can be imported by a Nakama module unchanged.
- `service` methods map one-to-one onto Nakama RPCs.
- Players have an internal id with `nakama_user_id` reserved; our tables sit in their own schema.
- What changes: sign-in and sessions (Nakama's), HTTP routes (become RPCs), and realtime features
  (duels, hacks) would use Nakama matches. Keep dependencies few: Nakama modules must be built
  with the same Go and library versions as Nakama.
