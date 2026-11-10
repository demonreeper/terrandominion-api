# Plan: iOS parity with Android (push and the rest)

The first iOS build (mobile PR #8, Sign in with Apple, Codemagic export flow) is in TestFlight review.
This plan closes the remaining gaps so the iPhone build does everything the Android build does.
Destined for `terrandominion-api/docs/ios-parity-plan.md`. Your own steps are in
`ios-parity-your-steps.md`.

## What is still Android-only (mobile `main` 328e84e, API `main` 5ca3f93)

| Feature | Android | iPhone today | Phase |
|---|---|---|---|
| Push notifications | Firebase Messaging, channels, tap opens the Tether Node | `PushService` compiled out (`#if UNITY_ANDROID`), `Supported = false`; server sends Android-only fields | **P1** |
| Push permission prompt and "notifications are off" row in Settings | Asks after the first Tether Node; opens app notification settings | Hidden because `Supported = false` | **P1** |
| Server push payload | `android` block (channel, priority, collapse key) | No `apns` block: no sound, no grouping, no priority | **P1** |
| Token platform | Sent as `"android"` | Hard-coded `"android"` in `PushService` | **P1** |
| Haptics | Vibrator with 4 strengths | Silent (`#if UNITY_ANDROID`) | **P2** |
| Compass heading | Input System magnetometer + gravity | Falls back to `Input.compass` (not tested on iPhone yet) | **P2** (check first) |
| Account deletion revokes Apple sign-in | n/a | Server ignores `authorizationCode` (`accounts.go:74`) | **P3** (App Store rule 5.1.1(v); not needed for TestFlight) |
| Google sign-in | Credential Manager | Not offered (decided 10 Nov: Apple only for the first build) | **P4** |
| Account linking | None (one sign-in per account) | None | **P5** (both platforms) |

Not gaps: location (`Input.location` on iOS), AR (ARKit), the 5-day test lock, build numbers,
privacy link, the sign-in screen.

## P1. Push notifications on iOS

Firebase Messaging already covers iOS through APNs. Firebase 14 arrives as Swift packages, which
the current Codemagic flow already builds.

### App (mobile repo)
1. **`PushService`**: change the compile guard to `#if (UNITY_ANDROID || UNITY_IOS) && !UNITY_EDITOR` for the
   Firebase part. Split the platform bits:
   - Android: as today (channels, `POST_NOTIFICATIONS`, `PushBridge`).
   - iOS: `Supported = true`; `RequestPermission` calls the new native `TDPush_requestPermission`
     (alert, sound, badge) and returns the answer; `NotificationsEnabled` reads
     `TDPush_notificationsEnabled`; `OpenSystemSettings` calls `TDPush_openSettings`
     (`UIApplicationOpenNotificationSettingsURLString` on iOS 16+, `UIApplicationOpenSettingsURLString` before).
   - Token upload sends `platform = "ios"` on iPhone.
2. **Ask at the same moment as Android, not at launch.** On iOS, `StartFirebase()` runs only once
   notifications are allowed (already allowed, or the player says yes in `PushPromptController`).
   Firebase Messaging on iOS registers for remote notifications when it starts, and starting it
   early would show Apple's prompt on first launch, against the decision in the sign-in/push plan.
   Add `FirebaseMessagingAutoInitEnabled = NO` to Info.plist so the token isn't fetched before then.
3. **`Assets/Plugins/iOS/TDPush.mm`**: `UNUserNotificationCenter` permission request and settings
   read, the settings deep link, and `UNUserNotificationCenterDelegate willPresentNotification`
   returning **no banner** while the game is in front (same as Android: the in-game notice shows
   instead). Leave Firebase's own delegate swizzling on, so a tap still reaches
   `MessageReceived` with `NotificationOpened = true` and opens the Tether Node.
4. **`IosPostBuild.cs`**: add `capabilities.AddPushNotifications(false)` (production `aps-environment`)
   and `AddBackgroundModes(BackgroundModesOptions.RemoteNotifications)`; set
   `FirebaseMessagingAutoInitEnabled = NO`.
5. **`GoogleService-Info.plist`** in `Assets/Firebase/` (git-ignored) on **every PC that runs
   `build-ios.bat`**. `PrepareIosBuild` fails with a clear message when it is missing, the same way
   the Android side needs `google-services.json`.
6. Test on device: allow, background the app, attack your own Tether Node from the Android phone,
   check the banner, tap it, check it opens that Tether Node. Deny, check the Settings row appears
   and its button opens the iOS settings page.

### Server (API repo)
1. **`internal/push/push.go`**: add an `apns` block next to `android`, leaving Android unchanged:
   ```json
   "apns": {
     "headers": { "apns-priority": "10" or "5", "apns-collapse-id": "<group>" },
     "payload": { "aps": { "sound": "default", "thread-id": "<group>" } }
   }
   ```
   High-priority messages (`m.High`) get `10`, others `5`. `apns-collapse-id` is capped at 64 bytes, so
   truncate the group. One message works for both platforms, so no per-platform send path.
2. Unit test the wire JSON for both blocks (`push_test.go` already tests the Android shape).
3. Deploy to dev. No database change: `td.push_tokens.platform` already accepts `ios`.

## P2. Haptics and compass

1. **`Assets/Plugins/iOS/TDHaptics.mm`**: `UIImpactFeedbackGenerator` mapped from `Haptic`:
   Tick → light, Short → medium, Medium → heavy, Heavy → heavy plus a `UINotificationFeedbackGenerator`
   warning. `HapticsService.Pulse` calls it under `#if UNITY_IOS && !UNITY_EDITOR`. The Settings
   toggle already exists.
2. **Compass: check before building.** On the TestFlight build, open Field Ops and turn on the spot.
   If the arrow and map follow the phone smoothly, nothing to do. If it is stuck or lags, add
   `TDHeading.mm` (`CLLocationManager` heading, `trueHeading` when located) and use it in
   `GeoLocationService.ReadHeading` on iOS before the `Input.compass` fallback. (The Input System
   magnetometer the Android path uses doesn't exist on iOS.)

## P3. Apple sign-in revocation (before the App Store, not needed for TestFlight)

Apple requires that deleting an account also revokes its Sign in with Apple grant.
1. On sign-in, exchange `authorizationCode` at `https://appleid.apple.com/auth/token` (client secret =
   ES256 JWT signed with the Sign in with Apple key, `iss` Team ID, `sub` bundle id, `aud`
   `https://appleid.apple.com`, 5-minute lifetime). Store the returned refresh token encrypted
   (AES-GCM with a new `TD_APPLE_TOKEN_KEY`) on the `apple` row of `td.player_identities` (migration
   adds the column).
2. When an account is deleted, `POST https://appleid.apple.com/auth/revoke` with that token. A
   failure is logged and retried by the existing jobs loop, never blocks deletion.
3. Config: `TD_APPLE_KEY_FILE`, `TD_APPLE_KEY_ID`, `TD_APPLE_TEAM_ID`, `TD_APPLE_TOKEN_KEY` in the dev
   `.env`; the `.p8` lives on the server with `chmod 600`. Without them the server signs in as today
   and skips revocation, with a log line.

## P4. Google sign-in on iOS (DECIDED 2026-11-10: do it now)

Lets a player who started on Android sign in to the same account on an iPhone. Without it, an
iPhone is always a separate account (Apple) until account linking exists.
- App: `GoogleSignIn` pod through an EDM `Dependencies.xml`, `TDGoogleSignIn.mm` bridge returning
  the ID token for the nonce, the URL scheme in Info.plist (`IosPostBuild`), both buttons on iOS
  (Apple first, as Apple's rules prefer an equal option).
- iOS client ID `486136868815-f101kj1a4miutrvvt6gav6n1gm6raote.apps.googleusercontent.com`, URL scheme
  `com.googleusercontent.apps.486136868815-f101kj1a4miutrvvt6gav6n1gm6raote` (not secret).
- Server: add the iOS client ID to `TD_GOOGLE_CLIENT_IDS`. Nothing else, since `/v1/auth/google` already
  checks audiences from that list.
- Codemagic: the Podfile is no longer empty, so `pod install` must succeed (drop the fallback to the
  `.xcodeproj` for this workflow, or keep it and fail clearly when GoogleSignIn is missing).

## P5. Account linking (DECIDED 2026-11-10: like MonoCity)

**Goal (owner, 10 Nov):** a player who moves from Android to iPhone, or from iPhone to Android, picks up
exactly where they left off.

How each move is covered:
- **Android → iPhone:** they sign in with Google on the iPhone (P4). Same account, nothing to link.
- **iPhone → Android with a Google sign-in (or Google linked):** same as above.
- **iPhone → Android with Apple only:** the gap, because Android has no native Sign in with Apple.
  Covered by the nudge below. (Sign in with Apple on Android through a web page is possible: an Apple
  Services ID, a return URL on the server and a browser tab. It's more work and more Apple setup, so
  it's left as an option if the nudge turns out not to be enough.)

One account, several sign-ins. A player signed in with Apple on an iPhone links Google in Settings,
and from then on either sign-in, on either phone, opens the same account. This reverses the
9 Nov "no account linking" decision for new links. The schema already allows it:
`td.player_identities` is keyed by (provider, subject) and points at a player, with no limit per player.

### Server (API repo)
1. `POST /v1/me/link/google {idToken, nonce}` and `POST /v1/me/link/apple {identityToken,
   authorizationCode, nonce}` (signed in). Verify the token exactly as sign-in does, then:
   - identity not seen before → `store.LinkIdentity` to the current player;
   - already on this player → OK, nothing changes;
   - on **another** player → new error code (next to 1100) "This Google/Apple account already has its own
     game account." **No merging.** (Merging two accounts' Tether Nodes, inventory and faction state is
     a much larger job; while you are the only player it isn't needed.)
2. `DELETE /v1/me/link/{provider}`: removes that sign-in; refused (new error code) when it is the
   player's only one, so nobody locks themselves out. Unlinking Apple also revokes it once P3 exists.
3. `GET /v1/me` gains `linked: ["apple","google"]`.
4. Tests: link new, link same, link taken, unlink last refused, unlink one of two.
5. API.md documents the three calls and the two error codes.

### App (mobile repo)
1. Settings → new **ACCOUNT** section: one row per sign-in, "Google: linked" / **LINK GOOGLE**, and
   "Apple: linked" / **LINK APPLE** (the Apple button only on iPhone; Android shows Apple's status
   only, since Sign in with Apple has no native Android sheet). An **UNLINK** button on a linked
   row when there are two. Wording in `Ui/Copy.cs`, country-neutral.
2. Linking reuses the sign-in services (`GoogleSignInService`, `AppleSignInService`, and on iOS the
   P4 Google bridge) and posts to the link calls instead of sign-in. Errors go through `ToastService`.
3. Sign-in screen unchanged: Apple and Google on iPhone, Google on Android.
4. **Nudge for Apple-only players** (iPhone): when the account has Apple but not Google, show a short
   notice once after the first Tether Node is built, and keep a line in the ACCOUNT section:
   "Link a Google account to also play on Android." with LINK GOOGLE. Dismissed once, it doesn't come back
   (the Settings line stays).
5. HUD capture adds the Settings ACCOUNT section to `hud-settings.png`.

### Order inside P5
Needs P4 (Google on iOS) first, so an iPhone player can link Google. Server first, deploy to dev, then the app.

## Order

1. You: your-steps 1 (APNs key) and 2 (plist on the build PC). About 15 minutes.
2. Local Claude Code: P1 server (deploy dev) → P1 app → P2 haptics → one `build-ios.bat` + Codemagic run.
3. You: test P1 and the compass on the iPhone. If the compass is weak, local Claude Code does P2 compass.
4. P3 before any App Store submission (needs your-steps 3).
5. P4 (decided 10 Nov) after P1 and P2 work on the iPhone; needs your-steps 3b (iOS OAuth client).
6. P5 account linking right after P4.

## Player-facing text
Any new words (the iOS push prompt reuses the Android one) stay country-neutral and go through `Ui/Copy.cs`.
