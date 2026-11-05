"""Builds Planetiler's two missing support files for a regional (Maldives) map, offline.

1. water-polygons-split-3857.zip: the sea as one EPSG:3857 polygon per grid cell. Taken from the
   natural=coastline ways in the OSM extract: they are joined into closed rings (OSM draws land on
   the left, so an island ring runs counter-clockwise) and every island becomes a hole in its cell.
2. natural_earth_vector.sqlite.zip: an empty Natural Earth database. Natural Earth only feeds the
   world-scale zooms (0-6), which the game never shows.

Pure Python (no GDAL/osmium): a minimal OSM PBF reader and a minimal shapefile writer.
Usage: python3 make_ocean.py maldives.osm.pbf outdir
"""
import math
import os
import sqlite3
import struct
import sys
import zipfile
import zlib

# ---- minimal protobuf -----------------------------------------------------------------------


def varint(buf, i):
    shift = result = 0
    while True:
        b = buf[i]
        i += 1
        result |= (b & 0x7F) << shift
        if b < 0x80:
            return result, i
        shift += 7


def fields(buf):
    """Yields (field_number, wire_type, value) for one message."""
    i, n = 0, len(buf)
    while i < n:
        key, i = varint(buf, i)
        fn, wt = key >> 3, key & 7
        if wt == 0:
            v, i = varint(buf, i)
        elif wt == 2:
            ln, i = varint(buf, i)
            v = buf[i:i + ln]
            i += ln
        elif wt == 1:
            v = buf[i:i + 8]
            i += 8
        elif wt == 5:
            v = buf[i:i + 4]
            i += 4
        else:
            raise ValueError("wire type %d" % wt)
        yield fn, wt, v


def packed(buf):
    out, i, n = [], 0, len(buf)
    while i < n:
        v, i = varint(buf, i)
        out.append(v)
    return out


def zigzag(v):
    return (v >> 1) ^ -(v & 1)


def delta(values):
    out, acc = [], 0
    for v in values:
        acc += zigzag(v)
        out.append(acc)
    return out

# ---- OSM PBF ----------------------------------------------------------------------------------


def read_pbf(path):
    """Returns ({node_id: (lon, lat)}, [ (way_id, [node refs]) for natural=coastline ], bbox)."""
    data = open(path, "rb").read()
    i = 0
    nodes = {}
    coast = []
    bbox = None
    while i < len(data):
        (hlen,) = struct.unpack(">I", data[i:i + 4])
        i += 4
        header = dict((fn, v) for fn, _, v in fields(data[i:i + hlen]))
        i += hlen
        size = header[3]
        blob = dict((fn, v) for fn, _, v in fields(data[i:i + size]))
        i += size
        raw = zlib.decompress(blob[3]) if 3 in blob else blob[1]
        kind = header[1].decode()

        if kind == "OSMHeader":
            for fn, _, v in fields(raw):
                if fn == 1:  # HeaderBBox, nanodegrees
                    b = dict((f, zigzag(x)) for f, _, x in fields(v))
                    bbox = (b[1] / 1e9, b[4] / 1e9, b[2] / 1e9, b[3] / 1e9)  # left, bottom, right, top
            continue

        strings, groups = [], []
        gran, lat_off, lon_off = 100, 0, 0
        for fn, _, v in fields(raw):
            if fn == 1:
                strings = [s for f, _, s in fields(v) if f == 1]
            elif fn == 2:
                groups.append(v)
            elif fn == 17:
                gran = v
            elif fn == 19:
                lat_off = v
            elif fn == 20:
                lon_off = v

        natural = coastline = None
        for idx, s in enumerate(strings):
            if s == b"natural":
                natural = idx
            elif s == b"coastline":
                coastline = idx

        for group in groups:
            for fn, _, v in fields(group):
                if fn == 2:  # DenseNodes
                    d = dict((f, x) for f, _, x in fields(v))
                    ids = delta(packed(d.get(1, b"")))
                    lats = delta(packed(d.get(8, b"")))
                    lons = delta(packed(d.get(9, b"")))
                    for nid, la, lo in zip(ids, lats, lons):
                        nodes[nid] = ((lon_off + gran * lo) / 1e9, (lat_off + gran * la) / 1e9)
                elif fn == 1:  # plain Node
                    d = dict((f, x) for f, _, x in fields(v))
                    nodes[zigzag(d[1])] = ((lon_off + gran * zigzag(d[9])) / 1e9,
                                           (lat_off + gran * zigzag(d[8])) / 1e9)
                elif fn == 3:  # Way
                    d = {}
                    for f, _, x in fields(v):
                        d[f] = x
                    keys = packed(d.get(2, b""))
                    vals = packed(d.get(3, b""))
                    if natural is not None and coastline is not None and any(
                            k == natural and val == coastline for k, val in zip(keys, vals)):
                        coast.append((d[1], delta(packed(d.get(8, b"")))))
    return nodes, coast, bbox

# ---- rings ------------------------------------------------------------------------------------


def join_rings(coast):
    """Chains coastline ways end to start into closed rings; returns (rings, open_chains)."""
    chains = [list(refs) for _, refs in coast if len(refs) >= 2]
    rings = [c for c in chains if c[0] == c[-1]]
    open_ = [c for c in chains if c[0] != c[-1]]
    by_start = {}
    for c in open_:
        by_start.setdefault(c[0], []).append(c)
    used = set()
    leftovers = []
    for c in open_:
        if id(c) in used:
            continue
        used.add(id(c))
        chain = list(c)
        while chain[0] != chain[-1]:
            nxt = [n for n in by_start.get(chain[-1], []) if id(n) not in used]
            if not nxt:
                break
            used.add(id(nxt[0]))
            chain.extend(nxt[0][1:])
        if chain[0] == chain[-1]:
            rings.append(chain)
        else:
            leftovers.append(chain)
    return rings, leftovers


def signed_area(pts):
    a = 0.0
    for (x1, y1), (x2, y2) in zip(pts, pts[1:]):
        a += x1 * y2 - x2 * y1
    return a / 2.0


R = 6378137.0


def merc(lon, lat):
    lat = max(-85.0511, min(85.0511, lat))
    return (math.radians(lon) * R, math.log(math.tan(math.pi / 4 + math.radians(lat) / 2)) * R)

# ---- shapefile ----------------------------------------------------------------------------------


def write_shapefile(base, polygons):
    """polygons: list of [ring, ...] in metres; outer rings clockwise, holes counter-clockwise."""
    records = []
    for rings in polygons:
        pts = [p for r in rings for p in r]
        xs = [p[0] for p in pts]
        ys = [p[1] for p in pts]
        parts, offset = [], 0
        for r in rings:
            parts.append(offset)
            offset += len(r)
        content = struct.pack("<i4d2i", 5, min(xs), min(ys), max(xs), max(ys), len(rings), len(pts))
        content += struct.pack("<%di" % len(parts), *parts)
        content += b"".join(struct.pack("<2d", x, y) for x, y in pts)
        records.append((content, (min(xs), min(ys), max(xs), max(ys))))

    allx = [b for _, bb in records for b in (bb[0], bb[2])]
    ally = [b for _, bb in records for b in (bb[1], bb[3])]
    box = (min(allx), min(ally), max(allx), max(ally))

    def header(length_words):
        return (struct.pack(">7i", 9994, 0, 0, 0, 0, 0, length_words) +
                struct.pack("<2i", 1000, 5) + struct.pack("<4d", *box) + struct.pack("<4d", 0, 0, 0, 0))

    shp, shx, offset = [], [], 50
    for n, (content, _) in enumerate(records, 1):
        words = len(content) // 2
        shp.append(struct.pack(">2i", n, words) + content)
        shx.append(struct.pack(">2i", offset, words))
        offset += 4 + words
    with open(base + ".shp", "wb") as f:
        f.write(header(offset))
        f.write(b"".join(shp))
    with open(base + ".shx", "wb") as f:
        f.write(header(50 + 4 * len(records)))
        f.write(b"".join(shx))

    # dBASE III with one numeric field, FID.
    with open(base + ".dbf", "wb") as f:
        f.write(struct.pack("<BBBBIHH20x", 3, 126, 11, 6, len(records), 32 + 32 + 1, 1 + 10))
        f.write(struct.pack("<11sc4xBB14x", b"FID", b"N", 10, 0))
        f.write(b"\r")
        for n in range(len(records)):
            f.write(b" " + str(n).rjust(10).encode())
        f.write(b"\x1a")
    with open(base + ".prj", "w") as f:
        f.write('PROJCS["WGS 84 / Pseudo-Mercator",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563]],'
                'PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433]],PROJECTION["Mercator_1SP"],'
                'PARAMETER["central_meridian",0],PARAMETER["scale_factor",1],PARAMETER["false_easting",0],'
                'PARAMETER["false_northing",0],UNIT["metre",1],AUTHORITY["EPSG","3857"]]')

# ---- main ---------------------------------------------------------------------------------------


def main(pbf, outdir):
    nodes, coast, bbox = read_pbf(pbf)
    if bbox is None:
        lons = [c[0] for c in nodes.values()]
        lats = [c[1] for c in nodes.values()]
        bbox = (min(lons), min(lats), max(lons), max(lats))
    print("nodes %d, coastline ways %d, extract bbox %s" % (len(nodes), len(coast), bbox))
    rings, leftovers = join_rings(coast)
    print("closed rings %d, unclosed chains %d (dropped)" % (len(rings), len(leftovers)))

    islands = []
    for ring in rings:
        pts = [nodes[r] for r in ring if r in nodes]
        if len(pts) < 4 or pts[0] != pts[-1]:
            continue
        m = [merc(lon, lat) for lon, lat in pts]
        if signed_area(m) < 0:   # holes must run counter-clockwise
            m.reverse()
        islands.append(m)

    # The sea: one frame half a degree larger than the extract, with every island as a hole.
    # Islands never overlap each other, so no polygon clipping is needed.
    left, bottom, right, top = bbox
    x0, y0 = merc(left - 0.5, bottom - 0.5)
    x1, y1 = merc(right + 0.5, top + 0.5)
    outer = [(x0, y0), (x0, y1), (x1, y1), (x1, y0), (x0, y0)]  # clockwise
    polygons = [[outer] + islands]
    print("sea polygon with %d islands" % len(islands))

    folder = os.path.join(outdir, "water-polygons-split-3857")
    os.makedirs(folder, exist_ok=True)
    base = os.path.join(folder, "water_polygons")
    write_shapefile(base, polygons)
    with zipfile.ZipFile(os.path.join(outdir, "water-polygons-split-3857.zip"), "w", zipfile.ZIP_DEFLATED) as z:
        for ext in (".shp", ".shx", ".dbf", ".prj"):
            z.write(base + ext, "water-polygons-split-3857/water_polygons" + ext)

    ne = os.path.join(outdir, "natural_earth_vector.sqlite")
    if os.path.exists(ne):
        os.remove(ne)
    con = sqlite3.connect(ne)
    con.execute("CREATE TABLE meta (name TEXT)")
    con.commit()
    con.close()
    with zipfile.ZipFile(os.path.join(outdir, "natural_earth_vector.sqlite.zip"), "w", zipfile.ZIP_DEFLATED) as z:
        z.write(ne, "packages/natural_earth_vector.sqlite")
    print("written to", outdir)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
