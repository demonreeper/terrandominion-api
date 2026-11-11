# Build prompt: phones without AR + AR extras

Paste into the local Claude Code session in `D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API/docs/no-ar-support-plan.md` (the plan; approved 2026-11-11) and the mobile
`CLAUDE.md`. Build the milestones in order, one at a time, and after each one: run the HUD captures,
build with `..\References\temp\build-apk.bat`, install on the phone, and tell me what to check.
Don't wait for my go between milestones unless a build fails or something in the plan is unclear.

1. ARCore Optional (`Assets/XR/Settings/ARCoreSettings.asset`, Requirement Optional) and the startup
   availability check; hide the AR toggle on unsupported phones; replace the "Install or update Google
   Play Services for AR" message; Settings > USE AR VIEW.
2. Build TN from the map (no AR placement when AR is off or missing).
3. First-TN walkthrough map-only path.
4. Holo table for TNs (VIEW TN from the map, and an AR / HOLO TABLE switch in AR).
5. Living TNs in AR.
6. AR photo mode (share sheet + save).
7. Signal echoes + CODEX: needs the API side first (echo placement, `GET /v1/echoes/nearby`,
   `POST /v1/echoes/{id}/read`, codex in `/me` and profile badges). Use only the five echo lines in the
   plan (approved 2026-11-11); never add others yourself.
8. Sky battles: only if drone strikes are already built; otherwise skip and say so.

Rules: everything built in code; the lore screen rules in CLAUDE.md; nothing AR-only may give resources,
XP or combat information. Update `docs/GAME_DESIGN.md` (Field and AR) with the rules as each milestone lands.
Don't commit or push without my go.
