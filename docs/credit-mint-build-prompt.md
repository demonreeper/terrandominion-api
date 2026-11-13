# Build prompt: credit node v2 (the Mint) and bigger resource nodes in AR

Owner decisions, 2026-11-13: credit design **A, Mint** (a large hex coin standing on its edge in a cradle, turning slowly) and resource nodes **2.5x bigger** in AR. Mock-ups: API repo `docs/art-reference/nodes/credit-v2/` and `nodes/node-credit.jpg`.
Use the exported model as it is. Don't rebuild it in code. Work on a branch and install on the phone for the owner to check.

## 1. Pull (the model is already on main)
`git pull` in both repos. Mobile main already has the new `Assets/Art/Models/Node/node_credit/` files (FBX + 4 maps, same .meta GUIDs) and the README note. API main has the sources (`docs/art-reference/source/nodes.py`, `nodes_export.py`) and renders.
After the pull, check in Unity that node_credit reimports, Normal is still a normal map and MetallicSmoothness is linear, as for the other nodes.

## 2. The coin turns
The FBX root `node_credit` has these children: `Hull`, `Glow`, `GlowSoft`, `HullSpin` and `GlowSpin`. The Spin children are the coin. Their pivot is at the node's ground centre, so turning them about their local up axis turns the coin in place.
- ModelDress already dresses them by prefix (HullSpin gets the Hull material and the shared node_credit maps, GlowSpin gets the resource glow), so check that nothing more is needed.
- In `NodeHologram`, after `m_Dress.Place(...)`, find `HullSpin` and `GlowSpin` under the model and add each one with the existing `Spinner` helper: about the up axis, roughly 20 to 25 degrees per second, both at the same speed. Only credit has them; the other nodes are unchanged.

## 3. Resource nodes 2.5x bigger in AR
Today a node within 10 m is drawn at about 0.245 of its model size (ModelScale 0.35 x StreetSize 0.7). That makes the crystal about 40 cm tall and the old credit about 10 cm.
- Near nodes (within NearRealM, 10 m) should be **2.5x** today's size. A crystal then stands about 1 m tall.
- Fade the boost out with distance, so that from about 60 m on a node is the size it is today. Far nodes already grow into beacons (FarGrow), and 2.5x on top of that would be far too big.
- Scale the whole marker together (model, cooldown ring, label height, BodyCenter, harvest burst, tap target), not only the model, so nothing ends up inside the model. The easiest way is a multiplier on the marker root scale in `ArNodeService`.
- Leave alone: the up-high (balcony) view (TrueSizeOfMarker), the salvage drop, the map and holo-table views, and the harvest range.

## 4. Check
- In the editor: HologramCapture shows the four nodes, the credit Mint upright on its plinth, and the coin turning.
- On the phone: walk up to a crystal and a credit node and check that the near size is about knee to waist high, that the label and the cooldown ring sit above and around the bigger model, and that nodes far away look the same as before.
- Watch the build number (the committed AndroidBundleVersionCode may be behind).
