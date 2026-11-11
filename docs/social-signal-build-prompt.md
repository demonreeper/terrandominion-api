# Prompt for local Claude Code: anti GPS spoofing, friends, region visiting

Start this **after the freelancer build is finished**. `git pull` both repos first (API:
`docs/social-signal-plan.md`; mobile: `docs/GAME_DESIGN.md`). The three narrator lines are already copied to
`TerranDominion-Mobile\Assets\Resources\Audio\Voices\narrator\` (`undersignal-spoof-first.ogg`,
`undersignal-spoof-repeat.ogg`, `undersignal-spoof-return.ogg`); add them in git with phase G1. Paste this into
Claude Code in `D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API\docs\social-signal-plan.md` (Part A friends, Part B region visiting, Part C anti GPS
spoofing), the profile and moderation parts of `docs\profile-plan.md`, and the Friends, Factions and Anti-cheat
parts of `TerranDominion-Mobile\docs\GAME_DESIGN.md`. The plan is approved; follow it and don't guess anything it
marks Open. If the freelancer build isn't finished, finish it first.

Build it phase by phase in the plan's build order: G0, G1, FR1, R1, R2, R3, G2. Don't start G3. One branch per phase
(e.g. `antispoof-g0`) in each repo it touches, one commit per phase per repo; API and app ship together; breaking
API changes are fine (only I play on dev). Use the next free migration number. Every number is a setting in
`td.settings` with a range. Run `go test ./...` and the edit-mode tests, deploy to dev, install on my phone, then go
straight on to the next phase without waiting for me.

1. **G0 anti-spoofing, server:** teleport check (over `SpoofTeleportKmh` and `SpoofTeleportMinKm` after both fixes'
   accuracy) and stale or future fix check on every endpoint that takes a fix. A hard signal refuses the action
   and starts a **signal lock** (sanction kind `spoof_lock`, by the system account), 30 / 120 / 720 minutes stepping
   up within 7 days (all settings). During a lock every location action returns `SignalLocked` with `until` and
   `incident`; sign-in still works. `/v1/me` returns `signalLockedUntil` and `signalLockLine`. Every incident makes a
   report in the REPORTS queue from "Signal watch" tagged `AUTO: GPS spoofing`; mods see no coordinates, admins can.
   Dismiss lifts the lock and drops the incident from the count. Admin `spoof-exempt` flag on player info, logged;
   set it for player 2 and player 4 on dev. New settings group "Anti-cheat", with `SpoofLockEnabled` (off = report
   only).
2. **G1 anti-spoofing, app:** Android: read `isMock()` / `isFromMockProvider()` in `FusedLocation.java`; iOS 15+:
   `sourceInformation.isSimulatedBySoftware` via a small `TDLocation.mm`. Send `mock` and `fixAt` with every fix.
   Full-screen SIGNAL REJECTED screen in the narrator style (no power colour, no call screen) with a countdown, and
   the narrator line with its caption: `undersignal-spoof-first` for the first incident in the window,
   `undersignal-spoof-repeat` after that, and `undersignal-spoof-return` once on the next open after a lock ends.
   Captions are the plan's lines word for word. Open-report count on the ADMIN button. Add the plan's privacy line
   to `docs/legal/privacy.html` in this phase. Raise the minimum app build after it ships.
3. **FR1 friends:** search by exact email (switch on own profile, on by default), by callsign or share code, and
   from your faction roster; requests, accept, decline, cancel, remove; real names shown only between accepted
   friends; FRIENDS menu row, three tabs, profile button, push for request and accepted. Privacy text in this phase.
4. **R1 region from GPS:** `td.countries` (Natural Earth admin-0), `CountryAt`, `LiveRegions` (starts `MV`),
   claim and join use the country you stand in, admin test regions on dev.
5. **R2 view region:** `viewing_branch_id`; members of a faction can view any of their faction's **unlocked**
   regions from anywhere, talk only. Home region never changes. Jobs, orders, missions, votes and officer actions
   always use the home branch. Visitor tag in member lists.
6. **R3 app:** REGION row with SWITCH REGION (only when more than one region is unlocked), the switch list with
   "Talk only. Jobs and orders stay in your region.", BACK HOME, the visiting line. Copy stays country-neutral.
7. **G2:** movement pattern checks, report only, no lock.

Player-facing text is country-neutral. The anti-spoofing screens and lines name no power and no other persona.

At the end, list what you built per phase, anything you couldn't do, and the settings you added.
