# Prompt for local Claude Code: freelancer path, final pledge and Langis

`git pull` both repos first (API: `docs/freelancer-path-plan.md`, `docs/LORE.md`, `docs/profile-plan.md`; mobile:
`docs/GAME_DESIGN.md`). Langis's two calls are already copied to
`TerranDominion-Mobile\Assets\Resources\Audio\Voices\langis\` (`level18-call.ogg`, `level19-call.ogg`); add them in
git with the first phase that uses them. Paste this into Claude Code in `D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API\docs\freelancer-path-plan.md`, the new "Langis, the seventh voice" section and the
player's story in `TerranDominion-API\docs\LORE.md`, the Device record part of `docs\profile-plan.md`, and the
freelancer and faction parts of `TerranDominion-Mobile\docs\GAME_DESIGN.md`. The plan is approved; follow it and
don't guess anything it marks Open.

Build it phase by phase, in the plan's build order. One branch per phase (e.g. `freelancer-f1`) in each repo it
touches, one commit per phase per repo; API and app ship together; breaking API changes are fine (only I play on
dev). Every number is a setting in `td.settings` with a range. Run `go test ./...` and the edit-mode tests, deploy
to dev, install on my phone, then go straight on to the next phase without waiting for me.

1. **Pledge is final (copy and confirm):** call joining "pledge" everywhere a player sees it. FACTIONS header
   "You were invited. The choice is yours. It is also final."; each card a PLEDGE button with "Permanent. You
   can't leave or switch."; freelancers see "You can stay free, and pledge any time." Two-step confirm: what you
   gain and what you give up, then HOLD TO PLEDGE for 3 s, then the power's short line. Level 19 day-3 message
   gets "Once you choose, you cannot unchoose." Remove any "leave" wording. Narrator line once at 20: "Choose
   once. The powers do not share." Admin A2: RELEASE PLEDGE (reason required, logged).
2. **Device record:** `td.device_marks` (salted hashes of `ANDROID_ID` / an iOS Keychain id, and of the Google/Apple
   sign-in subject; faction_code; ban_sanction_id), kept after account deletion. A marked device or sign-in rejoins
   its faction at 20 ("Your pledge stands. A tether to a power never breaks.", one button). A ban marks both and
   refuses sign-up and sign-in with "This device is banned."; lifting the ban clears it. Warnings at sign-up, at
   pledge and on the delete-account screen. APPEAL link on both screens (email to support with a short appeal
   code); admin clears a mark from A2 player info. Update mobile `docs/legal/privacy.html` and
   `delete-account.html` with the plan's "Privacy text to ship with phase 2" in this same phase, not before.
3. **Freelancer benefits:** 6 daily missions keep coming after 20 for freelancers; neutral standing (faction
   players can't attack a freelancer's TN unless that freelancer attacked a faction in the last 3 days); daily
   favour (`FavourPerkPct` of each power's perk until midnight, for each power whose mission was finished today,
   shown with the same in-play tags as faction perks). Freelancer TN uploads stay open to everyone.
4. **Langis:** system account id 8 (`signal_unbound`, never returned by any API before the player's call is due).
   Halfway through level 18 a call from "Unknown caller" plays `langis/level18-call`; in its last seconds the caller
   id flickers to LANGIS, then the line drops. Missed call: push "Missed call from Unknown caller", plays once on
   next open. The JOBS menu row unlocks when the call ends. After the level 19 day-3 message, the written message
   from sender LANGIS (glitches in the first time); the next day the second call, `langis/level19-call`. "Stay free."
   signed LANGIS on the FACTIONS screen at 20. Rough static call screen, no silhouette or eyes, no symbols. Never
   contacts a member; silent for good after a pledge. Lines are in the plan, word for word.
5. **Unbound titles:** Drifter, Wayfarer, Unbound past 20 for freelancers, each a profile badge granted by a
   Langis message (lines in the plan). Thresholds are settings.
6. **JOBS board:** faction players (Warden and above) post jobs (hit a TN, scout an area, guard a node); reward
   held in escrow; the server checks completion; refunds on expiry or failure; only freelancers who had the Langis
   level 18 call can take them; anti-farming limits apply. Own phase, own branch.

F7 (broker) waits for trading. Don't start it.

At the end, list what you built per phase, anything you couldn't do, and the settings you added.
