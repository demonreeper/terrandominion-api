# Prompt for local Claude Code: rename the game to Tether Rise

Paste this into Claude Code in `D:\Development\Github\ProjectAR` (the workspace folder, since this touches both repos). Pull both repos first.

---

Rename the player-facing game name from "Terran Dominion" to "Tether Rise" in both repos. The full plan is in `References/temp/rename-plan.md`; follow its §3 and §4.

Rules:
- Change **only visible text and display names**. Keep every identifier listed in §4 exactly as it is: bundle ID / package `com.hisstudios.terrandominion`, Java package paths, the Go module path, repo and folder names, systemd unit and nginx file names, DB names, keystore name, PlayerPrefs `td.*` keys, `tdapi`, map style id `terran`, the share domain.
- Spelling: "Tether Rise" in sentences; `TETHER\nRISE` where the title is stacked (main menu, sign-in, admin players, share card); `TETHER RISE` where it's one line (splash, intro title). Since the new name is shorter, check that the stacked titles still look balanced at 104 px, and adjust size or letter spacing if they look small.
- Share image files: `tether-rise-card.png`, `tether-rise-stats.png`.
- Update the tests that assert on the name (`AdminSettingsTests.cs`, `pages_test.go`, `web_test.go`).
- Legal pages in `TerranDominion-Mobile/docs/legal/` (index, privacy, terms, delete-account, including the mailto subject) get the new name.
- Docs and comments: update headings and prose that name the game (README, CLAUDE.md, GAME_DESIGN.md, LORE.md, code comments, codemagic.yaml workflow name). Leave old dated plan docs alone.
- Player text stays country-neutral.
- New intro videos with the Tether Rise title card are in `References/temp/tether-rise-videos/`. Copy `intro-withaudio.mp4` and `intro-noaudio.mp4` over the files of the same names in `TerranDominion-Mobile/Assets/StreamingAssets/Video/`, keeping the existing `.meta` files. Same length, resolution and audio as the old ones; only the title card changed.
- **Main menu title looks like the video title card** (user, 2026-11-11). The label at the top of the main menu (`UiBuilder.MainMenu.cs`, now `TERRAN\nDOMINION`, 104 px, upper left) becomes the intro's end title: `TETHER` over `RISE`, **centred**, heaviest weight, wide letter spacing (about 0.18 em), near-white `#e8f6ff` with a soft cyan `#59d9ff` glow (layered Shadow/Outline components or a duplicate blurred text behind it), and the tagline `CLAIM THE WORLD YOU WALK.` under it in cyan with wide spacing. When the menu opens, play the intro's short settle effect once: a red copy and a cyan copy offset a few pixels left/right that fade into the white title in under a second. Respect reduced motion if the game has a setting for it. Reference image: `References/temp/tether-rise-videos/title-card.png`. Use the same look on the sign-in screen title; the share card and admin title just get the new text.
- **Make the voiced intro the default** (user, 2026-11-11). In `Assets/Scripts/Runtime/Ui/UiBuilder.Video.cs`, the automatic plays (first launch, after each sign-in: the `Play(IntroFile, …)` calls around lines 126 and 137) should play `intro-withaudio.mp4` with its own sound (music held silent, as the Settings "with sound" replay already does). Keep `intro-noaudio.mp4` as the alternative in Settings: label the two replay rows "INTRO" (voiced, default) and "INTRO, MUSIC ONLY". Keep the `td.intro.seen` key and the skip-unplayable fallback working for the new default file (if the voiced file can't play, fall back to the music-only one). Update the class doc comment and any tests that name the file.

When done:
1. Run a case-insensitive grep for `terran dominion`, `terran\ndominion` and `terran-dominion` in both repos. The only hits left should be in old plan docs or the README note about the concept PDF.
2. Run the Go tests (`go test ./...`) and the Unity EditMode tests.
3. Build and install the Android pretest on my phone, and check the splash, main menu, sign-in, share card and app label.
4. After the API deploy, copy the four updated legal pages (`TerranDominion-Mobile/docs/legal/` index, privacy, terms, delete-account + style.css) to the server folder that serves `https://demonreeper.xyz/terrandominion/` (set up in the sign-in plan, e.g. `/var/www/terrandominion-legal/`), keeping the same file names, and open the privacy URL to check that it shows Tether Rise.
5. Don't commit or push until I say so. Then make one branch and PR per repo, titled "Rename game to Tether Rise".
