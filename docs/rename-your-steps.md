# Rename to Tether Rise: your steps

These are the things only you can do. Everything else is in `rename-plan.md` (what changes) and `rename-build-prompt.md` (what local Claude Code builds).

## Before the build
1. **Confirm the name.** Search `TETHER RISE` and `TETHERRISE` in the USPTO database (tmsearch.uspto.gov), classes 9 and 41, and in your own country's trademark registry. If either one shows a game or software mark, stop and tell me.
2. **Grab a domain and handles.** For example tetherrise.com or tetherrise.gg, plus @tetherrise on the socials you use. This isn't needed for the build, but do it early so nobody takes them.
3. **Start the build.** Paste `References\temp\rename-build-prompt.md` into your local Claude Code in `D:\Development\Github\ProjectAR`. The new intro videos are already in `References\temp\tether-rise-videos\`.

## After the build is on your phone
4. **Check on the phone.** Look at the app label under the icon, the splash, the voiced intro (it now plays by default), the main menu, sign-in, the share card and share text, and the profile web page from a share link.
5. **Say go for commits.** Local Claude Code makes one branch and PR per repo, titled "Rename game to Tether Rise". Merge them.
6. **Deploy the API** as usual (`scripts/deploy-dev.ps1`). Then live, once you're happy. Deploying updates the push title, the appeal email subject and the profile page.

## Consoles (about 30 minutes, after the PRs merge)
7. **App Store Connect** → your app → App Information → **Name**: `Tether Rise`. If the name is taken on the App Store, use `Tether Rise: AR`. TestFlight shows it right away. The public store name changes with the next version you submit.
8. **Google Play Console** → Grow → Store presence → Main store listing → **App name**: `Tether Rise`. Also update the short and full description wherever they name the game, keeping them country-neutral.
9. **Google Cloud Console** → APIs & Services → OAuth consent screen → Branding → **App name**: `Tether Rise`. Because the screen is published, Google may re-check the branding for a few days. Sign-in keeps working meanwhile.
10. **Firebase console** → Project settings → General → **Public-facing name**: `Tether Rise`.
11. **AdMob** → Apps → your app → App settings → **App name**: `Tether Rise`.
12. **Legal pages**: nothing for you to do. Local Claude Code copies the updated pages to the server, at the same web addresses the stores already link to (part of the build prompt). Just open https://demonreeper.xyz/terrandominion/privacy.html afterwards and check that it says Tether Rise.
13. **Ship builds.** Build the next Android pretest and iOS TestFlight builds the usual way (`build-ios.bat` → Codemagic).

## Don't change
- The bundle ID / package `com.hisstudios.terrandominion` in Unity, App Store Connect, Play, Firebase or Google Cloud. Changing it makes a new app and breaks sign-in and push.
- Firebase or Google Cloud **project IDs** (they can't be renamed anyway), the GitHub repo names, the server folders and services, the database names.
- The old teasers. Just stop posting them; any new teaser uses Tether Rise.

## Later, optional
- If you buy a domain, move the share links (`td-dev.demonreeper.xyz/p/…`) and the legal pages onto it. That's a separate small change, so ask me when you want it.
