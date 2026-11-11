# Prompt for local Claude Code: walk together (signal link)

Start this **after the current build is finished**. `git pull` both repos first (API:
`docs/walk-together-plan.md`; mobile: `docs/GAME_DESIGN.md`). Paste this into Claude Code in
`D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API\docs\walk-together-plan.md` and the Walk together, Friends and anti-cheat parts of
`TerranDominion-Mobile\docs\GAME_DESIGN.md`. The plan is approved; follow it and don't guess anything it doesn't
say. If the current build isn't finished, finish it first.

Build it phase by phase: W1, W2, W2b, then the L4 lockstep check from W3. Don't start the integrity part of W3
(L7) until G3 exists. One branch per phase (e.g. `walk-w1`) in each repo it touches, one commit per phase per
repo; API and app ship together; breaking API changes are fine (only I play on dev). Use the next free migration
number. Every number is a setting in `td.settings` with a range, in a new settings group "Walk together". Run
`go test ./...` and the edit-mode tests, deploy to dev, install on my phone, then go straight on to the next phase
without waiting for me.

1. **W1 server:** `td.walk_links`, `td.walk_link_members`, `td.walk_link_asks`, `walked_together_m`; the
   `/v1/walk/open`, `/ask`, `/ask/{code}/accept`, `/heartbeat`, `DELETE /v1/walk` endpoints; the proximity check
   (fresh accurate fixes, range after accuracy, fixes within 20 s, different device ids, normal spoof checks);
   `game.WalkLinkBonus` read by harvest, drop pickup, scan radius and XP, using the confirmed table (scan
   25/40/50%, harvest and drops 15/25/30%, XP 10/20/30% for groups of 2/3/4). The link bonus **adds** to faction
   perk percentages, it doesn't multiply. **No bonus on Tether Node collects.** Bonus pauses unless the group moved
   50 m in 180 s; 120-minute daily bonus cap. Break rules from the plan, including: any attack on a linked
   member's TN, on the member, or (later) on a base of their faction ends the link. **No truce** between friends.
   L1, L2, L6. Spoof-exempt players 2 and 4 skip L1 to L5 but **not** Bluetooth.
2. **W2 app + Bluetooth:** WALK TOGETHER panel (friends screen and map HUD), the consent box, `LINKED x2` chip,
   `KEEP WALKING`, linked friends' markers (only while linked), wider scan ring, `+X% LINKED` tags, link sounds
   and haptics, `WALKED TOGETHER` profile stat. **Bluetooth proof** as in the plan: `TDProximity` plugin for
   Android (BLE advertise + scan, Android 12+ permissions with `neverForLocation`) and iOS (CoreBluetooth,
   `NSBluetoothAlwaysUsageDescription`), rotating server tokens, `bleOn` / `bleHeard` in heartbeats, the bonus
   needs Bluetooth (`WalkLinkRequireBle`), permission asked on first WALK TOGETHER, `BLUETOOTH OFF: NO BONUS` and
   `CONFIRMING...` chips. Server L3 twin-fix and L8 no-Bluetooth-contact reports from "Signal watch".
3. **W2b extras:** X1 shared scan, plus the group's best faction **scan** perk applied to every member while
   linked (only the scan perk); X2 duo/trio/quad group nodes (seeded by the node seeding job, locked badge when the
   group is too small, each member gets the full reward, yields x3/x5/x8); X3 link missions; X5 milestones (5, 25,
   100, 500 km, first link, first group node).
4. **W3 (part):** L4 lockstep check, report only, alongside G2's pattern checks.

Player-facing text is country-neutral and names no power except in perk tags. Nothing hints at a seventh persona.
Add one line to `docs/legal/privacy.html` in W2: while linked, friends see each other on the map, and the app uses
Bluetooth to confirm linked phones are near each other.

At the end, list what you built per phase, anything you couldn't do, and the settings you added. Then test on the
phones with Shinan: walk a few hundred metres in Malé linked, compare harvests and scan rings with walking alone,
turn Bluetooth off on one phone and check the bonus stops.
