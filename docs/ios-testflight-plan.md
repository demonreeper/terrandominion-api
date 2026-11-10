# Plan: current build on iOS TestFlight (Codemagic)

Goal: the game as it is today (no new gameplay) installed on an iPhone through TestFlight, built on
Codemagic. Destined for `terrandominion-api/docs/ios-testflight-plan.md` once you say go.

## What I found in the repo (mobile `main`, 10 Nov 2026)

| Area | State | iOS impact |
|---|---|---|
| Engine | Unity `6000.6.7f1`, IL2CPP, AR Foundation 6.7.1 with **ARKit 6.7.1 already in the manifest**, ARKit loader asset present | Good. AR should build for iOS as is. |
| Build script | `Editor/BuildScript.cs` already has `BuildIos()` (writes an Xcode project to `Builds/iOS`) | Needs a `PrepareIosBuild` step, an iOS icon, and a build number from Codemagic. |
| Bundle id / team | iOS bundle id **empty**, team id empty, automatic signing off | Must set `com.hisstudios.terrandominion`. Codemagic does the signing. |
| Usage text | Camera and location descriptions set, target iOS 15.0, portrait only | Fine. |
| **Sign-in** | Only "Sign in with Google", and `GoogleSignInService.Supported` is **Android only**. `/v1/auth/dev` exists on the server only behind a flag, and the app no longer calls it. | **Blocker: on iPhone the game stops at the sign-in screen with "Google sign-in works on the phone only."** See the decision below. |
| Push | `PushService` is a no-op off Android; Firebase Messaging pods still get linked | Builds; no pushes on iOS yet (that is S3). |
| Haptics | Android only, silent on iOS | Fine for TestFlight. |
| Location | Android plugin on Android; `Input.location` on iOS | Should work. |
| Compass | Uses the Input System magnetometer, which Unity provides on Android only; falls back to `Input.compass` | Direction arrows may be weaker on iPhone. Test item, not a blocker. |
| Firebase | SDK tarballs and config files are git-ignored (`scripts/get-firebase.ps1`) | Codemagic must download the tarballs itself. |
| CI | **No `codemagic.yaml` in the repo**; Unity licence is **Personal** | Codemagic can't run Unity Personal, so the PC exports the Xcode project and Codemagic signs and uploads it. |

## Decision: Sign in with Apple (DECIDED 2026-11-10: option A)

The "as is" build can't be used on an iPhone at all, because its only sign-in is Android-only. So
this TestFlight build needs one small new piece: a way to sign in on iOS. Apple's rule (App Review
Guideline 4.8) says an app that offers Google sign-in must also offer an equivalent private option,
in practice Sign in with Apple. Internal TestFlight builds skip App Review, so the rule doesn't block
internal testing, but it does block external testers (Beta App Review) and the App Store.

| Option | Work | Verdict |
|---|---|---|
| **A. Sign in with Apple on iOS (Google stays Android-only)** | Small Objective-C bridge, an Apple button on iOS, `POST /v1/auth/apple` on the server (verify Apple's token, same shape as `google.go`). About a day for your local Claude Code. | **Recommended.** Meets 4.8 on day one, works for external testers, and is part of S3 anyway, so nothing is thrown away. |
| B. Google and Apple on iOS (full S3 sign-in) | A plus the GoogleSignIn pod, an iOS OAuth client, URL scheme | Later. Lets one Google account play on both phones. |
| C. Temporary tester login (turn on `/v1/auth/dev`, show it on iOS only) | Smallest | Not recommended: an unauthenticated login on the server, internal testers only, thrown away later. |

Consequences of A to know about:
- An Apple sign-in makes a **separate account** from your Android Google account (no account linking,
  as decided on 9 Nov). Fine while you are the only player.
- Apple also requires that deleting an account revokes its Apple sign-in (server calls Apple's
  `/auth/revoke`). Not needed for TestFlight; must be done before App Store submission.

## Part 1: your steps

Moved to its own file: [`ios-testflight-your-steps.md`](ios-testflight-your-steps.md).

## Part 2: code and config changes (for your local Claude Code)

### Mobile repo (`terrandominion-mobile`)
1. **Player settings (iOS):** bundle id `com.hisstudios.terrandominion`, Team ID from step 1,
   automatic signing off (Codemagic signs), keep target iOS 15.0, keep portrait, `iOSRequireARKit` off
   (AR is optional inside Field Ops, so non-ARKit iPhones still get the map).
2. **`BuildScript.PrepareIosBuild()`**, mirroring the Android one: switch to iOS, create the boot scene,
   apply safe fixes, set the iOS icon from `Assets/Icons/app-icon.png` (1024 px, flattened, no alpha:
   App Store Connect rejects transparent icons), assign the ARKit loader (`ArLoaderSetup` already knows it).
3. **Build number:** the local `StampBuild()` counter, as for Android (see item 7). Version string stays `1.0`.
4. **`Editor/IosPostBuild.cs`** (`[PostProcessBuild]`, `UnityEditor.iOS.Xcode`): add the Sign in with
   Apple entitlement and `AuthenticationServices.framework`; set `ITSAppUsesNonExemptEncryption = NO`.
5. **Sign in with Apple:**
   - `Assets/Plugins/iOS/TDAppleSignIn.mm`: `ASAuthorizationAppleIDProvider`, scopes name and email,
     SHA-256 of a nonce, returns `identityToken`, `authorizationCode` and the name (first sign-in only)
     to C# via a callback.
   - `Platform/AppleSignInService.cs`, same shape as `GoogleSignInService` (`Supported` = iPhone only).
   - `Ui/UiBuilder.SignIn.cs`: on iOS show a black **"Sign in with Apple"** button (Apple's button
     rules) in place of the Google one; Android unchanged.
   - `Ui/SignInController.cs` + `Net/ServerSession.cs`: `SignInApple()` posting to `/v1/auth/apple`,
     same session handling as Google.
6. (Dropped: no `get-firebase.sh`, since Unity no longer runs on Codemagic.)
7. **Unity runs on the PC, Codemagic does only the Xcode part** (Unity Personal can't be activated
   on Codemagic; decided 10 Nov). Windows Unity with iOS Build Support exports the Xcode project.
   - **Build number:** stays the local `StampBuild()` counter (shared with Android, always rising), so
     `TD_BUILD_NUMBER` is not needed. `TD_EXPIRES_UTC` override and the rolling 5 days still apply.
   - **EDM4U on Windows** can't run `pod install`; it writes the `Podfile` only. Check that
     `ProjectSettings/AndroidResolverDependencies.xml`'s iOS twin (`IOSResolver` settings) has
     "Podfile generation" on and "Cocoapods integration: Xcode workspace". Codemagic runs `pod install`.
   - **`..\References\temp\build-ios.bat`** (beside `build-apk.bat`): PrepareIosBuild →
     BuildIos → zip `Builds/iOS` to `Builds/terrandominion-ios-b<n>.zip` → 
     `gh release create ios-b<n> --repo demonreeper/terrandominion-mobile --prerelease --title "iOS B<n> (<commit>)" Builds/terrandominion-ios-b<n>.zip`.
     Writes each step's exit code to `build-status-ios.txt`. Keep only the last 5 `ios-b*` releases
     (`gh release delete --cleanup-tag`).
   - Check the zip size on the first run; GitHub allows 2 GB per release file. Exclude nothing Xcode needs.
   - **`codemagic.yaml`** at the repo root, modelled on MonoCity's (same `codemagic` integration,
     started by hand, keychain + `fetch-signing-files --create`, email on success and failure):
   ```yaml
   definitions:
     email_recipients: &email_recipients
       - thehussha@gmail.com

   workflows:
     ios-testflight:
       name: iOS TestFlight - Terran Dominion
       instance_type: mac_mini_m2
       max_build_duration: 60
       integrations:
         app_store_connect: codemagic    # same key as MonoCity
       environment:
         groups:
           - github                      # GITHUB_TOKEN: fine-grained, Contents read-only on this repo
         vars:
           APP_STORE_APPLE_ID: <from your step 3>
           BUNDLE_ID: "com.hisstudios.terrandominion"
         xcode: latest
         cocoapods: default
       # No triggering block on purpose: started by hand, after build-ios.bat has uploaded an export.
       scripts:
         - name: Download the newest Xcode export from the PC
           script: |
             set -e
             TAG=$(gh release list --repo demonreeper/terrandominion-mobile --limit 30 \
               --json tagName,createdAt --jq '[.[] | select(.tagName|startswith("ios-b"))] | sort_by(.createdAt) | last | .tagName')
             echo "Using export $TAG"
             gh release download "$TAG" --repo demonreeper/terrandominion-mobile --pattern '*.zip' --dir /tmp/export
             mkdir -p Builds && unzip -q /tmp/export/*.zip -d Builds
           # gh reads GITHUB_TOKEN from the environment.
         - name: CocoaPods (Firebase pods written by EDM)
           script: |
             cd Builds/iOS
             pod repo add-cdn trunk https://cdn.cocoapods.org/ || true
             pod install --repo-update
         - name: Set up keychain
           script: keychain initialize
         - name: Fetch signing files (creates cert/profile if missing)
           script: app-store-connect fetch-signing-files "$BUNDLE_ID" --type IOS_APP_STORE --create
         - name: Add certificates to keychain
           script: keychain add-certificates
         - name: Apply provisioning profiles
           script: xcode-project use-profiles --project Builds/iOS/Unity-iPhone.xcodeproj
         - name: Build IPA
           script: |
             xcode-project build-ipa --workspace Builds/iOS/Unity-iPhone.xcworkspace --scheme Unity-iPhone
       artifacts:
         - build/ios/ipa/*.ipa
         - /tmp/xcodebuild_logs/*.log
       publishing:
         email:
           recipients: *email_recipients
           notify:
             success: true
             failure: true
         app_store_connect:
           auth: integration
           submit_to_testflight: true
   ```
   Differences from MonoCity, on purpose: no Flutter or Unity steps; the build number comes from the
   export, not from App Store Connect (App Store Connect still refuses a repeat, so never re-upload
   the same `ios-b<n>`); no PlistBuddy purpose strings (Unity writes them, `IosPostBuild.cs` adds the
   rest); no Crashlytics. The local Claude Code confirms paths on the first run.
   - If `gh` is missing on the Codemagic image, install it with `brew install gh` in the first step.
8. **Test builds expire after 5 rolling days, on iOS and Android.** One small change in
   `Editor/BuildScript.cs`, used by both `BuildIos` and `BuildAndroidPretest`:
   - New `TestBuildEndUtc()`: `TD_EXPIRES_UTC` if set (manual override, as today), otherwise the end of
     the 5th day after the build in UTC+5, i.e. `(UtcNow + 5 h).Date + 5 days + 24 h - 5 h`, which is
     19:00 UTC. A build on 10 Nov locks at the end of 15 Nov.
   - `BuildAndroidPretest` no longer fails when `TD_EXPIRES_UTC` is missing; it uses the rolling date.
   - `BuildIos` always stamps an end time (every iOS build is a TestFlight test build for now).
   - The everyday Android APK (`BuildAndroid`, your own phone) stays without an end time.
   - `StampBuild()` already writes the end time into `BuildInfo.txt`, and `Ui/AppLock.cs` already locks
     on it (phone or server clock, whichever is later), so nothing else changes. The yaml needs no
     date: each Codemagic run gets a fresh 5 days.
   - Update the "Test builds for a group" paragraph in the mobile `CLAUDE.md` to match.
   - Check on the first iPhone run that the lock screen appears (set the phone clock past the date).
     TestFlight's own 90-day expiry still applies. To end a build early, raise `MinAppBuild` from
     ADMIN as today.
9. No change to push, haptics, gameplay, or Android.

### API repo (`terrandominion-api`)
1. `internal/auth/apple.go`: verify Apple identity tokens like `google.go` (keys from
   `https://appleid.apple.com/auth/keys`, issuer `https://appleid.apple.com`, audience = bundle id
   from new `TD_APPLE_BUNDLE_IDS`, nonce check).
2. `POST /v1/auth/apple {identityToken, authorizationCode, nonce, name, deviceId, platform, appVersion}`,
   provider `apple` in `td.player_identities` (the table already expects it). New error code next to 1100.
3. Add `TD_APPLE_BUNDLE_IDS=com.hisstudios.terrandominion` to the dev server `.env`, deploy to dev.
4. Deferred to before App Store submission: store the Apple refresh token from `authorizationCode`
   and revoke it on account deletion (needs the separate Sign in with Apple `.p8` key, S3 step 5).
5. This plan file goes to `docs/ios-testflight-plan.md`.

## Order

1. You: steps 2–3 (about 15 minutes; the account and Codemagic key already exist).
2. Local Claude Code: API `/v1/auth/apple` → deploy to dev.
3. Local Claude Code: mobile changes 1–7 on a branch; it can compile-check on Windows by switching to
   iOS (iOS Build Support module needed in Unity Hub), but the real build is on Codemagic.
4. You: step 5, then run the `ios-testflight` workflow in Codemagic.
5. You: step 6; install from TestFlight.

## Test on the iPhone
- Sign in with Apple → main menu footer shows the build number.
- Field Ops: location prompt, nodes, map, harvesting.
- Direction arrows and map heading (the compass fallback above).
- AR view and Tether Node placement (ARKit).
- Expected missing on iOS: push notifications, haptics, Google sign-in.

## Open
- Which Unity licence Codemagic uses.
