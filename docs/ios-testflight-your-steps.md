# Your steps: Terran Dominion on iOS TestFlight

Only the things you do yourself, in order. About 30 minutes, no cost (your Apple account and the
Codemagic key from MonoCity already exist). The code side is in `ios-testflight-plan.md` (Part 2)
and is done by your local Claude Code.

Steps 1–7 can be done now. The last section (running the build) waits until the code changes are merged.

Secrets rule as before: `.p8` keys and passwords go in `D:\Development\Github\keys\terrandominion\`
and your password manager, never in chat or git.

## 1. Apple Developer Program (already have it)
MonoCity is already on TestFlight, so your developer account and team are in place. Terran Dominion
goes under the same team. ☐ Note your **Team ID** (developer.apple.com → Membership details) if you
don't have it to hand. Not secret.

## 2. App ID (bundle id)
developer.apple.com → Certificates, Identifiers & Profiles → **Identifiers → +** → App IDs → App →
- Description `Terran Dominion`, **Explicit** bundle id `com.hisstudios.terrandominion`.
- Tick **Sign in with Apple** (Enable as a primary App ID) and **Push Notifications** (Firebase links
  its push code into the build, and ticking it now avoids a signing error and saves a step for S3).
- Register. No certificates or profiles by hand: Codemagic makes them (step 5).

## 3. App Store Connect app record
<https://appstoreconnect.apple.com> → Apps → **+ → New App**
- Platform iOS, Name `Terran Dominion` (must be unique on the App Store; if taken, try `Terran Dominion AR`),
  primary language English, bundle id from step 2, SKU `terrandominion-ios`, access Full.
- Name and subtitle stay country-neutral.
- ☐ Note the **Apple ID** number of the app (App Information page). Not secret.

## 4. App Store Connect API key (reuse MonoCity's)
Your Codemagic team already has the App Store Connect integration named `codemagic` (MonoCity's
`codemagic.yaml` uses it). A team key covers every app in the team, so nothing new is needed, as long
as the key has **App Manager** or **Admin** access (App Store Connect → Users and Access →
Integrations). App Manager can create the new bundle id's certificate and profile.

## 5. Codemagic
1. **Add the app:** Codemagic → Add application → GitHub → `demonreeper/terrandominion-mobile` →
   "codemagic.yaml" project type. The yaml lands with the mobile changes (Part 2, item 7).
2. **No Unity on Codemagic** (you have Unity Personal, which Codemagic can't activate). Your PC does
   the Unity part and uploads the Xcode project; Codemagic only signs, builds and uploads to TestFlight.
   - If you made the `unity` group with UNITY_EMAIL / PASSWORD / SERIAL, delete those variables.
   - Make a GitHub token so Codemagic can download the Xcode project: github.com → Settings →
     Developer settings → **Fine-grained tokens → Generate new token** → name `codemagic-td`,
     expiry 1 year, Repository access **Only select repositories** → `terrandominion-mobile`,
     Permissions → Repository → **Contents: Read-only** → Generate. Copy it.
   - Codemagic → Terran Dominion → Environment variables: name `GITHUB_TOKEN`, value the token,
     group `github` (create it), **Secret** ticked → Add.
3. **Unity Hub on your PC:** Installs → 6000.6.7f1 → ⚙ → **Add modules** → tick **iOS Build Support**
   → Install. (Windows can export an iOS Xcode project; only the Xcode build needs a Mac.)
4. Optional now, needed for S3: Firebase console → **Add app → Apple** with the same bundle id →
   download `GoogleService-Info.plist`, then in Codemagic add it to an environment group `firebase` as
   `GOOGLE_SERVICE_INFO_PLIST` (paste the file's **base64**, Secure). Skipping it is fine for this build,
   since push is off on iOS.

## 6. TestFlight testers
- **Internal** (up to 100 people who are users of your App Store Connect team, no review, builds
  appear about 10–30 minutes after upload): App Store Connect → your app → TestFlight → **Internal
  Testing → +** → group `Core`, tick "automatic distribution", add yourself.
- **External** (anyone by email or public link, up to 10,000) needs Beta App Review of the first
  build (usually under a day), plus Test Information: feedback email, short description, privacy
  policy URL `https://demonreeper.xyz/terrandominion/privacy.html`. Keep the description country-neutral.
- On the iPhone: install **TestFlight** from the App Store, sign in with the tester's Apple ID, accept the invite.

## 7. Export compliance
Nothing to do: the build sets `ITSAppUsesNonExemptEncryption = NO` (HTTPS only), so App Store Connect
won't ask about encryption for each build.

## Send back in the thread (none secret)
☐ Team ID · ☐ App Store Connect app Apple ID (step 3) · ☐ "GitHub token added in Codemagic" ·
☐ "iOS Build Support installed" · ☐ "testers group made"

## Never send
The App Store Connect `.p8`, the GitHub token, any Apple `.p8`.

## When the code is merged: run the build
1. On your PC, your local Claude Code runs `References\temp\build-ios.bat` (Unity editor closed). It
   exports the Xcode project, zips it and uploads it to a GitHub pre-release named `ios-b<build number>`.
2. Codemagic → Terran Dominion → **Start new build** → branch `main`, workflow
   **iOS TestFlight - Terran Dominion**. It picks up the newest `ios-b…` upload.
3. Wait for the email (about 15–25 minutes). If it fails, send the failing step's name in the thread.
4. 10–30 minutes after success the build shows in the TestFlight app on your iPhone. Install it.
5. Sign in with Apple, then check the footer build number. The build locks itself at the end of the
   5th day after it was built; to test that, set the phone's clock past that date once.
