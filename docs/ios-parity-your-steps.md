# Your steps: iOS push and the rest

Only the things you do yourself. The code side is in `ios-parity-plan.md`.
Secrets rule as before: `.p8` keys go in `D:\Development\Github\keys\terrandominion\`, never in chat or git.

## Order
1. Steps 1, 2 and 3b below (about 25 minutes).
2. Copy `ios-parity-plan.md` and this file into `terrandominion-api\docs\` on the PC that runs
   `build-ios.bat`, and send your local Claude Code the prompt from the thread.
3. When its build is in TestFlight, do step 4 (tests).
4. Step 3 only before the App Store.

## 1. One Apple key for push and Sign in with Apple, about 10 minutes
1. developer.apple.com → Certificates, Identifiers & Profiles → **Keys → +** → name `TD APNs and SIWA`,
   tick **Apple Push Notifications service (APNs)** and **Sign in with Apple** → next to Sign in with
   Apple click **Configure** → primary App ID `com.hisstudios.terrandominion` → Save → Continue →
   Register → **Download** the `.p8` (only possible once) to the keys folder. ☐ Note the **Key ID**.
2. Firebase console → project **terran-dominion** → ⚙ Project settings → **Cloud Messaging** →
   **Apple app configuration** → your iOS app → **APNs Authentication Key → Upload** → the `.p8`,
   Key ID, and your Team ID.
3. Check Identifiers → `com.hisstudios.terrandominion` still has **Push Notifications** ticked
   (you ticked it in the TestFlight steps).

## 2. GoogleService-Info.plist on the build PC
The file must be in `terrandominion-mobile\Assets\Firebase\` on **whichever PC runs `build-ios.bat`**
(you put it on the office PC). If you build at home, copy it there too. It's git-ignored.

## 3. Sign in with Apple key on the server (only before the App Store, part P3)
Use the same `.p8` from step 1. Your local Claude Code puts it on the dev server (it moves the file,
never reads it).

## 3b. Google iOS client (for Google sign-in on iPhone, part P4)
1. <https://console.cloud.google.com> → project **Terran Dominion** → Google Auth Platform → **Clients →
   Create client** → type **iOS**, name `TD iOS`, bundle id `com.hisstudios.terrandominion`, your Team ID → Create.
2. ☐ Note the **iOS client ID** (`…apps.googleusercontent.com`) and the **iOS URL scheme**
   (`com.googleusercontent.apps.…`). Neither is secret; give both to your local Claude Code.
3. The Google consent screen is published (owner, 10 Nov), so any Google account can sign in.

## 4. Test on the iPhone (after the new TestFlight build)
1. Open Field Ops, turn on the spot: does the arrow follow smoothly? Tell me yes or no.
2. Build a Tether Node, say yes to notifications.
3. Close the game, attack that Tether Node from the Android phone: a banner should arrive. Tap it:
   the game should open on that Tether Node.
4. In the iPhone's Settings turn notifications off for the game, open the game's Settings: a
   "notifications are off" row with a button to the iPhone settings should show.
5. Feel for haptics on a harvest and an attack.
6. On the iPhone, sign in with **Google** (your Android account): your Tether Nodes and inventory
   should be there.
7. Settings → ACCOUNT → **LINK APPLE**. Sign out, sign in with Apple: same account.
8. On Android, Settings → ACCOUNT should show Apple as linked.

## Send back in the thread (none secret)
☐ "APNs key uploaded" · ☐ iOS client ID and URL scheme · ☐ plist on the build PC · ☐ compass result · ☐ push test result · ☐ switching test result

## Never send
Any `.p8` file.
