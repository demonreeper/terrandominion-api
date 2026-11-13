# Credit node redesign mock-ups (owner, 2026-11-13): three concepts, taller so they read at a distance.
#   python credit.py -- <shot> <samples> <outdir>    shot: a | b | c | lineup | all
import sys, os, math, random
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy
import nodes as N
from nodes import box, cyl, emit, hull, ground_ring, cracks, RES, link

COL = RES['credit']


def mats():
    return dict(body=hull('cr_body', (0.05, 0.06, 0.055), .35, .8, 8),
                gold=hull('cr_coin', (0.05, 0.16, 0.09), .45, .6, 4),
                glow=emit('cr_glow', COL, 7), soft=emit('cr_soft', COL, 2.5, .55))


def glyph(m, at, rot, r, depth):
    """The credit mark on a coin face: hex outline round a small hex with a short bar above and below
    (vertical only, no crossbar)."""
    import bmesh
    from mathutils import Matrix, Euler, Vector
    out = []
    # outline ring = 6 thin bars on a hexagon
    for i in range(6):
        a0 = i * math.pi / 3; a1 = a0 + math.pi / 3; am = (a0 + a1) / 2
        ln = r
        p = Vector((r * .866 * math.cos(am), r * .866 * math.sin(am), 0))
        out.append(((p.x, p.y), (ln * 1.0, r * .11), am + math.pi / 2))
    parts = []
    E = Euler(rot).to_matrix().to_4x4()
    for (x, y), (w, h), a in out:
        o = box('glyph', (1, 1, 1), (0, 0, 0), (0, 0, 0), m, 0)
        o.matrix_world = Matrix.Translation(at) @ E @ Matrix.Translation((x, y, 0)) @ Matrix.Rotation(a, 4, 'Z') @ Matrix.Diagonal((w, h, depth, 1))
        parts.append(o)
    c = cyl('glyph_core', r * .32, depth, (0, 0, -depth / 2), 6, m)
    c.matrix_world = Matrix.Translation(at) @ E; parts.append(c)
    for s in (1, -1):
        b = box('glyph_bar', (1, 1, 1), (0, 0, 0), (0, 0, 0), m, 0)
        b.matrix_world = Matrix.Translation(at) @ E @ Matrix.Translation((0, s * r * .55, 0)) @ Matrix.Diagonal((r * .1, r * .32, depth, 1))
        parts.append(b)
    return parts


def coin(M, r, t, at, rot, lit=False):
    """A hex credit coin: dark green-metal body, glowing rim band, glyph on both faces."""
    from mathutils import Matrix, Euler
    E = Euler(rot).to_matrix().to_4x4(); T = Matrix.Translation(at)
    body = cyl('coin', r, t, (0, 0, -t / 2), 6, M['gold']); body.matrix_world = T @ E
    rim = cyl('coin_rim', r * 1.02, t * .3, (0, 0, -t * .15), 6, M['glow']); rim.matrix_world = T @ E
    for s in (1, -1):
        g = glyph(M['glow'] if lit or r > .3 else M['soft'], tuple((T @ E @ Matrix.Translation((0, 0, s * (t / 2 + .002)))).translation),
                  tuple((Euler(rot).to_matrix() @ Matrix.Rotation(0 if s > 0 else math.pi, 3, 'X')).to_euler()), r * .62, .012)


def concept_a(M):
    """MINT: a big hex coin standing on edge in a cradle of two angled jaws, slowly turning in the game."""
    cyl('base', .8, .08, (0, 0, 0), 6, M['body'])
    cyl('base_seam', .62, .085, (0, 0, 0), 6, M['glow'])
    cyl('plinth', .55, .14, (0, 0, .07), 6, M['body'])
    for s in (1, -1):
        j = box('jaw', (.16, .5, .55), (s * .38, 0, .38), (0, s * .45, 0), M['body'], .02)
        box('jaw_light', (.02, .4, .3), (s * .3, 0, .42), (0, s * .45, 0), M['glow'], 0)
    coin(M, .62, .14, (0, 0, .95), (math.pi / 2, 0, .35), True)
    # a glow column under the coin
    cyl('beam', .07, .45, (0, 0, .2), 12, M['soft'])
    random.seed(3)
    for k, (r, a, n) in enumerate(((1.0, .9, 4), (1.05, 2.4, 2), (.95, 4.0, 5), (1.0, 5.4, 3))):
        x, y = r * math.cos(a), r * math.sin(a)
        for j in range(n):
            cyl('chip', .13, .045, (x, y, j * .05), 6, M['glow'] if j == n - 1 else M['gold'], (0, 0, random.uniform(0, 1)))
    ground_ring('cra_ring', 1.35, COL)


def concept_b(M):
    """COIN STACK: a hex plinth with a tall column of floating coins, gaps between them, held by three rails."""
    cyl('base', .78, .1, (0, 0, 0), 6, M['body'])
    cyl('base_seam', .6, .105, (0, 0, 0), 6, M['glow'])
    cyl('plinth', .5, .22, (0, 0, .1), 6, M['body'])
    cyl('emitter', .36, .03, (0, 0, .32), 6, M['glow'])
    n = 9
    for i in range(n):
        z = .45 + i * .14; r = .42 - i * .018
        coin(M, r, .07, (0, 0, z), (0, 0, i * .22), i == n - 1)
    for i in range(3):
        a = i * 2 * math.pi / 3 + math.pi / 6; x, y = .56 * math.cos(a), .56 * math.sin(a)
        box('rail', (.07, .07, 1.55), (x, y, .3 + .775), (0, 0, a), M['body'], .015)
        box('rail_light', (.012, .02, 1.3), (x * .9, y * .9, .3 + .7), (0, 0, a), M['glow'], 0)
    bpy.ops.mesh.primitive_torus_add(major_radius=.58, minor_radius=.012, location=(0, 0, 1.92)); bpy.context.object.data.materials.append(M['soft'])
    ground_ring('crb_ring', 1.35, COL)


def concept_c(M):
    """VAULT: an armoured hex vault half sunk in the ground, the lid lifted, coins spilling, the credit mark above."""
    cyl('collar', .9, .08, (0, 0, -.02), 6, M['body'])
    cyl('drum', .68, .55, (0, 0, 0), 6, M['body'])
    for i in range(6):
        a = i * math.pi / 3 + math.pi / 6
        box('band', (.05, .02, .42), (.6 * math.cos(a), .6 * math.sin(a), .28), (0, 0, a + math.pi / 2), M['glow'], 0)
    cyl('mouth', .56, .02, (0, 0, .55), 6, M['glow'])
    # lid hinged at the back, lifted 55 degrees
    lid = cyl('lid', .7, .12, (0, 0, 0), 6, M['body'])
    lid.location = (0, .5, 1.18); lid.rotation_euler = (-1.4, 0, 0)
    random.seed(9)
    for i in range(14):
        a = random.uniform(-1.2, 1.2) - math.pi / 2; r = random.uniform(.5, 1.05)
        z = .02 if r > .72 else .56
        cyl('spill', .12, .04, (r * math.cos(a), r * math.sin(a) + (0 if r > .72 else .3), z), 6, M['glow'] if i % 3 == 0 else M['gold'],
            (random.uniform(-.4, .4), random.uniform(-.4, .4), random.uniform(0, 1)))
    coin(M, .4, .06, (0, -.1, 1.7), (math.pi / 2, 0, 0), True)
    cyl('hbeam', .05, .85, (0, -.1, .58), 12, M['soft'])
    ground_ring('crc_ring', 1.35, COL)


CONCEPTS = {'a': concept_a, 'b': concept_b, 'c': concept_c}


def one(k, out, samples):
    N.SAMPLES = samples
    sc = N.reset(); N.world(sc); N.ground(sc); N.lights(sc, COL)
    CONCEPTS[k](mats())
    N.cam(sc, (3.0, -3.9, 1.9), (0, 0, .75), 45)
    N.render(sc, 1080, 1080, os.path.join(out, f'credit-{k}.jpg'))


def lineup(out, samples):
    N.SAMPLES = samples
    sc = N.reset(); N.world(sc); N.ground(sc, 160); N.lights(sc, (0.6, 0.8, 0.65), 1.6)
    M = mats()
    for i, k in enumerate('abc'):
        before = set(bpy.data.objects); CONCEPTS[k](M)
        for o in set(bpy.data.objects) - before:
            if o.parent is None and o.type in ('MESH', 'EMPTY'): o.location.x += (i - 1) * 3.4
    N.cam(sc, (0, -10.5, 3.6), (0, 0, .7), 34)
    N.render(sc, 1920, 900, os.path.join(out, 'credit-concepts.jpg'))


if __name__ == '__main__':
    a = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    shot = a[0] if a else 'all'; samples = int(a[1]) if len(a) > 1 else 64; out = a[2] if len(a) > 2 else 'out'
    os.makedirs(out, exist_ok=True)
    if shot in ('all', 'lineup'): lineup(out, samples)
    for k in 'abc':
        if shot in ('all', k): one(k, out, samples)
