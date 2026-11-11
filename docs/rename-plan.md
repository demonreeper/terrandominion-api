# Rename plan: Terran Dominion → Tether Rise

Date: 2026-11-11. Status: plan only; no code changed yet.
Why: "Terran Dominion" is the name of a StarCraft faction (Blizzard). A store complaint could pull the app, and searches for the game return StarCraft. Renaming before public launch is cheap.

## 1. Name check (done 2026-11-11, not legal advice)

| Check | Result |
|---|---|
| Games/apps called "Tether Rise" / "Tetherrise" (web, App Store, Google Play, Steam, itch.io, GitHub) | None found |
| US trademark "TETHERRISE" / "TETHER RISE" (web search of trademark listings) | None found |
| "TETHER" marks | Tether Operations (the USDT crypto company) has pending US filings for "TETHER": class 9 crypto/AI software, class 9 robots + AR/VR glasses, class 41 education. Tether, LLC has a class 9 contacts app. **None cover games.** Tether also runs a "RISE" investment fund. Low risk, but if anyone ever objects, it would be them. |
| Tetris look-alike | Different spelling and sound ("teth-er rise"). Low risk. |
| Domains (tetherrise.com / .gg / .app / .io, tetherrisegame.com, playtetherrise.com) | **Not checked**: domain lookups are blocked from this environment. Check at a registrar. |

**Your checks before you commit to it:**
1. Search the official USPTO database (tmsearch.uspto.gov) for `TETHER RISE` and `TETHERRISE`, classes 9 and 41.
2. Check your own country's trademark registry the same way.
3. Check and register a domain and social handles (for example tetherrise.com or tetherrise.gg, and @tetherrise).
4. Optional, before a wide launch: a one-off clearance search by a trademark lawyer.

## 2. Spelling

| Where | Form |
|---|---|
| Player-facing name (stores, app label, UI, legal pages, videos) | **Tether Rise** (two words; matches the title card) |
| Title art | `TETHER` / `RISE` on two lines, as in the render (/mnt/project-files/rename/tetherrise.png) |
| Handles, domains, file names | `tetherrise` |

The tagline stays: "Claim the world you walk." "Tether Node" stays as the in-game name.

## 3. What changes (player-visible, about 40 places)

### Mobile repo (TerranDominion-Mobile)
- `ProjectSettings/ProjectSettings.asset` `productName` → `Tether Rise` (sets the Android label and iOS display name). Also update `Assets/Scripts/Editor/ProjectDoctor.cs:144`, which enforces it.
- Titles: `UiBuilder.MainMenu.cs`, `UiBuilder.SignIn.cs`, `UiBuilder.AdminPlayers.cs`, `ShareCard.cs` (`TERRAN\nDOMINION` → `TETHER\nRISE`); `UiBuilder.Splash.cs`, `UiBuilder.cs` IntroTitle (`TERRAN DOMINION` → `TETHER RISE`). Because the new name is shorter, recheck the size and spacing.
- Main menu + sign-in title restyled like the intro's end title card (centred TETHER / RISE, white with cyan glow, tagline under it, brief red/cyan settle on open) (user, 2026-11-11).
- Copy: `Copy.cs` share text ("Find me in Tether Rise: …"), `ShareCard.cs` "FIND ME IN TETHER RISE" plus share PNG file names (`tether-rise-card.png`, `tether-rise-stats.png`), `AppLock.cs` (2 messages), `AdminPlayersLogic.cs` stats share line.
- Tests: `AdminSettingsTests.cs` expected strings.
- Comments, tooltips, docs (README, CLAUDE.md, GAME_DESIGN.md heading): update text only.
- `docs/legal/*.html` (index, privacy, terms, delete-account, including the mailto subject).
- `codemagic.yaml`: workflow name and comment only.

### API repo (TerranDominion-API)
- `internal/game/push.go:42` push title "Tether Rise".
- `internal/game/devicemarks.go:46` appeal email subject "Tether Rise appeal …".
- `internal/httpapi/pages.go` page titles; `web/profile.html` (og:site_name, kicker, "OPEN IN TETHER RISE", note); tests `pages_test.go`, `web_test.go`.
- `deploy/maps/styles/terran.json` `"name"` field and the systemd `Description=` lines: text only.
- Docs: README, CLAUDE.md, cmd/tdapi comment, docs/LORE.md heading, other plan headings (text only; old plans can stay as they are).

### Shared folder media
- **Startup intro** (`video/intro/`): the title card says TERRAN / DOMINION as on-screen text only. **The narration never says the name**, so no voice lines need re-recording. Re-render both cuts (voiced + music-only) with TETHER / RISE and replace the app copy in StreamingAssets.
- **Intro default** (user, 2026-11-11): the automatic intro (first launch, after sign-in) becomes the **voiced** cut `intro-withaudio.mp4`; the music-only cut stays as a Settings replay option. Part of the rename build.
- **TN tutorial** (`video/tn-tutorial/`): never shows or says the name (checked tutorial.js and the narration script). No change.
- **Old teasers 1–3**: per your rule they aren't edited. They become retired; don't post them anymore. New teasers use Tether Rise.
- Share card, store screenshots, feature graphic: regenerate after the build.
- App icon: has no text, so no change.

## 4. What does NOT change (invisible IDs; keep them)

| Item | Why it stays |
|---|---|
| Bundle ID / package `com.hisstudios.terrandominion` (Android + iOS) | Changing it creates a brand-new app in both stores and breaks Firebase, Google sign-in (OAuth clients + SHA-1), Sign in with Apple, APNs push and existing installs. Players never see it. |
| Java packages `com.hisstudios.terrandominion.*` | Invisible; tied to the bundle ID. |
| Go module `github.com/demonreeper/terrandominion/api` | Invisible; renaming means touching every import for no player benefit. |
| GitHub repos, folder names, server paths, systemd unit names, nginx config, DB `terrandominion_dev`, keystore file name | Invisible. GitHub repo rename is optional later (GitHub redirects), but it would break Codemagic `EXPORT_REPO` and scripts, so not now. |
| PlayerPrefs keys `td.*`, API name `tdapi`, map style id `terran` | Invisible; changing keys would reset players' local state. |
| Firebase project id, Google Cloud project id | Can't be renamed; only display names change. |
| Share domain `td-dev.demonreeper.xyz` | Works as is. Move share links to a new domain later if you register one. |

## 5. Your console steps (by hand, after the build is green)
The full checklist, including the name checks, deploy and legal page upload, is in `docs/rename-your-steps.md`.

1. **App Store Connect** → app → App Information → Name: `Tether Rise`. The name must be unique on the App Store; if it's taken there, use "Tether Rise: AR" or similar. The name goes live with the next version submission; TestFlight shows it right away.
2. **Google Play Console** → Store presence → Main store listing → App name: `Tether Rise`.
3. **Google Cloud** → OAuth consent screen → App name `Tether Rise` (shown on the Google sign-in sheet). The screen is published, so Google may ask to re-verify the branding; this takes a few days and sign-in keeps working meanwhile.
4. **Firebase** → Project settings → Public-facing name `Tether Rise` (shown in some auth emails/prompts).
5. **Apple Developer** → Sign in with Apple uses the App Store Connect name. Nothing extra to do.
6. Legal pages: re-upload the updated privacy/terms/delete-account pages to wherever they're hosted, keeping the same URLs (the stores link to them).
7. AdMob app name (display only).

## 6. Order
1. You confirm the name after your checks in §1.
2. Docs PR (API repo): this plan in `docs/rename-plan.md` + GAME_DESIGN/LORE headings. Only on your go.
3. Local Claude Code builds the code change in both repos (prompt: `changes/rename-build-prompt.md`), runs the tests, and installs on your phone.
4. Intro re-render (this thread can do it here) → copy into StreamingAssets.
5. You do the console steps in §5 and ship the next TestFlight/pretest build.

Effort: about half a day of code for local Claude Code, about 1 hour of video re-rendering, and about 30 minutes of console clicks. No server migration, no data change, no new store listings.
