# Map tiles

The Field Ops map is drawn from raster tiles of the Maldives, rendered by TileServer GL on the API
server from OpenStreetMap data, in a dark unlabelled style (`styles/terran.json`) that matches the
game. Served at `https://td-dev.demonreeper.xyz/tiles/{z}/{x}/{y}@2x.png`.

Map data © OpenStreetMap contributors (ODbL); the game must show that credit on the map.

1. **Make the map data (PC, Java 21+).** If osmdata.openstreetmap.de or naciscdn.org can't be
   reached (as from the Maldives), skip `--download` and make those two inputs from the extract
   itself with `python3 make_ocean.py data/sources/maldives.osm.pbf data/sources` (no libraries
   needed): the sea is built from the extract's coastline, and Natural Earth (only used at world
   zoom levels) is left empty. The extract itself is mirrored at
   `https://download.openstreetmap.fr/extracts/asia/maldives-latest.osm.pbf`.
   Otherwise: Planetiler downloads the Maldives extract plus ocean
   polygons (about 1 GB the first time) and writes `maldives.mbtiles` (tens of MB):
   ```powershell
   mkdir D:\Development\Github\ProjectAR\References\maps; cd D:\Development\Github\ProjectAR\References\maps
   curl.exe -L -o planetiler.jar https://github.com/onthegomap/planetiler/releases/latest/download/planetiler.jar
   java -Xmx8g -jar planetiler.jar --download --area=maldives --output=maldives.mbtiles
   ```
2. **Install TileServer GL (server, arm64 Ubuntu, Node 20-24).**
   ```bash
   sudo apt install -y nodejs npm xvfb build-essential pkg-config libglfw3-dev libuv1-dev \
     libjpeg-turbo8 libicu-dev libcairo2-dev libpango1.0-dev libpng-dev libjpeg-dev libgif-dev \
     librsvg2-dev libcurl4-openssl-dev libpixman-1-dev
   sudo npm install -g tileserver-gl
   mkdir -p ~/terrandominion-maps/data ~/terrandominion-maps/styles
   ```
3. **Copy the files (PC).**
   ```powershell
   scp D:\Development\Github\ProjectAR\References\maps\maldives.mbtiles ubuntu@sshmvepoc.monocity.xyz:/home/ubuntu/terrandominion-maps/data/
   cd D:\Development\Github\ProjectAR\TerranDominion-API\deploy\maps
   scp config.json terrandominion-maps.service ubuntu@sshmvepoc.monocity.xyz:/home/ubuntu/terrandominion-maps/
   scp styles/terran.json ubuntu@sshmvepoc.monocity.xyz:/home/ubuntu/terrandominion-maps/styles/
   ```
4. **Run it (server).**
   ```bash
   sudo mv ~/terrandominion-maps/terrandominion-maps.service /etc/systemd/system/
   sudo systemctl daemon-reload && sudo systemctl enable --now terrandominion-maps
   curl -s -o /tmp/t.png -w '%{http_code} %{content_type}\n' http://127.0.0.1:8095/styles/terran/16/46149/32007.png
   ```
   `200 image/png` means it renders (that tile is Malé). Check `journalctl -u terrandominion-maps -n 30` if not.
5. **Publish through nginx (server).** Add the block in `nginx-tiles.conf` to the td-dev HTTPS
   server block, then `sudo nginx -t && sudo systemctl reload nginx` and open
   `https://td-dev.demonreeper.xyz/tiles/16/46149/32007@2x.png` in a browser.

**Updating the map:** rerun step 1 (monthly is plenty), copy the new `maldives.mbtiles`, and
`sudo systemctl restart terrandominion-maps`. Cached tiles expire within a week; purge Cloudflare's
cache to see changes at once.
