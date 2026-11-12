import bpy, bmesh, math, random, sys
from mathutils import Matrix, Vector, Euler

TEX = '/tmp/claude-0/bl3/tex/'
SHOT = sys.argv[sys.argv.index('--') + 1] if '--' in sys.argv else 'hero'
RS = float(__import__('os').environ.get('RS','1'))
SAMPLES = int(sys.argv[sys.argv.index('--') + 2]) if '--' in sys.argv and len(sys.argv) > sys.argv.index('--') + 2 else 64
import os
_hx = os.environ.get('FACHEX', 'b57cff'); FCODE = os.environ.get('FCODE', 'veil')
FAC = tuple(((int(_hx[i:i+2], 16) / 255) ** 2.2) for i in (0, 2, 4))
_m = max(FAC); FAC = tuple(c / _m for c in FAC)

bpy.ops.wm.read_factory_settings(use_empty=True)
sc = bpy.context.scene

# ---------------- materials ----------------
def node_mat(name):
    m = bpy.data.materials.new(name); m.use_nodes = True
    nt = m.node_tree; nt.nodes.clear()
    out = nt.nodes.new('ShaderNodeOutputMaterial'); return m, nt, out

_IMG = {}
def timg(n, noncolor=True):
    if n not in _IMG:
        im = bpy.data.images.load(TEX + n + '.png')
        if noncolor: im.colorspace_settings.name = 'Non-Color'
        _IMG[n] = im
    return _IMG[n]

def metal(name, col, rough, panel=True, scale=.2, glow=0.0):
    m, nt, out = node_mat(name)
    N_ = nt.nodes.new; L_ = nt.links.new
    b = N_('ShaderNodeBsdfPrincipled'); b.inputs['Metallic'].default_value = .65
    tc = N_('ShaderNodeTexCoord'); mp = N_('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (scale, scale, scale)
    L_(tc.outputs['Object'], mp.inputs[0])
    def tex(n):
        t = N_('ShaderNodeTexImage'); t.image = timg(n); t.projection = 'BOX'; t.projection_blend = .15; L_(mp.outputs[0], t.inputs[0]); return t
    th_, ta, tr, te = tex('sci_height'), tex('sci_albedo'), tex('sci_rough'), tex('sci_emit')
    # albedo: base colour x panel variation
    am = N_('ShaderNodeMapRange'); am.inputs['To Min'].default_value = .55; am.inputs['To Max'].default_value = 1.6
    L_(ta.outputs['Color'], am.inputs['Value'])
    colm = N_('ShaderNodeMixRGB'); colm.blend_type = 'MULTIPLY'; colm.inputs[0].default_value = 1; colm.inputs[1].default_value = (*col, 1)
    L_(am.outputs['Result'], colm.inputs[2])
    # grime: darker toward ground + noise
    nz = N_('ShaderNodeTexNoise'); nz.inputs['Scale'].default_value = 2.5; nz.inputs['Detail'].default_value = 8; L_(tc.outputs['Object'], nz.inputs['Vector'])
    sep = N_('ShaderNodeSeparateXYZ'); L_(tc.outputs['Object'], sep.inputs[0])
    gz = N_('ShaderNodeMapRange'); gz.inputs['From Min'].default_value = 0; gz.inputs['From Max'].default_value = 3; gz.inputs['To Min'].default_value = .45; gz.inputs['To Max'].default_value = 1
    L_(sep.outputs['Z'], gz.inputs['Value'])
    gr = N_('ShaderNodeMath'); gr.operation = 'MULTIPLY'; L_(gz.outputs['Result'], gr.inputs[0])
    nzr = N_('ShaderNodeMapRange'); nzr.inputs['To Min'].default_value = .7; nzr.inputs['To Max'].default_value = 1.15; L_(nz.outputs['Fac'], nzr.inputs['Value']); L_(nzr.outputs['Result'], gr.inputs[1])
    grm = N_('ShaderNodeMixRGB'); grm.blend_type = 'MULTIPLY'; grm.inputs[0].default_value = 1; L_(colm.outputs[0], grm.inputs[1])
    gcol = N_('ShaderNodeCombineColor'); L_(gr.outputs[0], gcol.inputs[0]); L_(gr.outputs[0], gcol.inputs[1]); L_(gr.outputs[0], gcol.inputs[2]); L_(gcol.outputs[0], grm.inputs[2])
    # edge wear via bevel normal
    bv = N_('ShaderNodeBevel'); bv.inputs['Radius'].default_value = .035
    geo = N_('ShaderNodeNewGeometry'); dot = N_('ShaderNodeVectorMath'); dot.operation = 'DOT_PRODUCT'
    L_(bv.outputs[0], dot.inputs[0]); L_(geo.outputs['Normal'], dot.inputs[1])
    ew = N_('ShaderNodeMapRange'); ew.inputs['From Min'].default_value = .985; ew.inputs['From Max'].default_value = .93; L_(dot.outputs['Value'], ew.inputs['Value'])
    nz2 = N_('ShaderNodeTexNoise'); nz2.inputs['Scale'].default_value = 18; L_(tc.outputs['Object'], nz2.inputs['Vector'])
    ewm = N_('ShaderNodeMath'); ewm.operation = 'MULTIPLY'; L_(ew.outputs['Result'], ewm.inputs[0])
    nz2r = N_('ShaderNodeMapRange'); nz2r.inputs['From Min'].default_value = .4; nz2r.inputs['From Max'].default_value = .65; L_(nz2.outputs['Fac'], nz2r.inputs['Value']); L_(nz2r.outputs['Result'], ewm.inputs[1])
    wear = N_('ShaderNodeMixRGB'); L_(ewm.outputs[0], wear.inputs[0]); L_(grm.outputs[0], wear.inputs[1]); wear.inputs[2].default_value = (.55, .57, .62, 1)
    L_(wear.outputs[0], b.inputs['Base Color'])
    # roughness
    rr = N_('ShaderNodeMapRange'); rr.inputs['To Min'].default_value = rough * .55; rr.inputs['To Max'].default_value = rough * 1.5; L_(tr.outputs['Color'], rr.inputs['Value'])
    rw = N_('ShaderNodeMix'); rw.data_type = 'FLOAT'; L_(ewm.outputs[0], rw.inputs['Factor']); L_(rr.outputs['Result'], rw.inputs['A']); rw.inputs['B'].default_value = .22
    L_(rw.outputs['Result'], b.inputs['Roughness'])
    # bump from panel height
    bp = N_('ShaderNodeBump'); bp.inputs['Strength'].default_value = .8; bp.inputs['Distance'].default_value = .02
    L_(th_.outputs['Color'], bp.inputs['Height']); L_(bv.outputs[0], bp.inputs['Normal']); L_(bp.outputs['Normal'], b.inputs['Normal'])
    if glow > 0:
        em = N_('ShaderNodeMath'); em.operation = 'MULTIPLY'; em.inputs[1].default_value = glow; L_(te.outputs['Color'], em.inputs[0])
        b.inputs['Emission Color'].default_value = (*FAC, 1); L_(em.outputs[0], b.inputs['Emission Strength'])
    L_(b.outputs[0], out.inputs[0]); return m

def emit(name, col, strength, alpha=1.0):
    m, nt, out = node_mat(name)
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1); e.inputs[1].default_value = strength
    if alpha < 1:
        tr = nt.nodes.new('ShaderNodeBsdfTransparent'); mx = nt.nodes.new('ShaderNodeMixShader'); mx.inputs[0].default_value = alpha
        nt.links.new(tr.outputs[0], mx.inputs[1]); nt.links.new(e.outputs[0], mx.inputs[2]); nt.links.new(mx.outputs[0], out.inputs[0])
        m.blend_method = 'BLEND'
    else:
        nt.links.new(e.outputs[0], out.inputs[0])
    return m

def img_mat(name, img, emit_strength=0.0, col=(1, 1, 1), alpha_mul=1.0):
    m, nt, out = node_mat(name)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + img)
    tr = nt.nodes.new('ShaderNodeBsdfTransparent'); mx = nt.nodes.new('ShaderNodeMixShader')
    if emit_strength > 0:
        e = nt.nodes.new('ShaderNodeEmission'); e.inputs[1].default_value = emit_strength
        mul = nt.nodes.new('ShaderNodeMixRGB'); mul.blend_type = 'MULTIPLY'; mul.inputs[0].default_value = 1; mul.inputs[2].default_value = (*col, 1)
        nt.links.new(tx.outputs['Color'], mul.inputs[1]); nt.links.new(mul.outputs[0], e.inputs[0]); sh = e
    else:
        sh = nt.nodes.new('ShaderNodeBsdfPrincipled'); sh.inputs['Roughness'].default_value = .85
        nt.links.new(tx.outputs['Color'], sh.inputs['Base Color'])
        e = nt.nodes.new('ShaderNodeEmission'); e.inputs[1].default_value = 1.5
        nt.links.new(tx.outputs['Color'], e.inputs[0]); ad = nt.nodes.new('ShaderNodeAddShader')
        nt.links.new(sh.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); sh = ad
    am = nt.nodes.new('ShaderNodeMath'); am.operation = 'MULTIPLY'; am.inputs[1].default_value = alpha_mul
    nt.links.new(tx.outputs['Alpha'], am.inputs[0]); nt.links.new(am.outputs[0], mx.inputs[0])
    nt.links.new(tr.outputs[0], mx.inputs[1]); nt.links.new(sh.outputs[0], mx.inputs[2]); nt.links.new(mx.outputs[0], out.inputs[0])
    m.blend_method = 'BLEND'; return m

M = {
    'dark': metal('dark', (0.05, 0.055, 0.07), .4, glow=3.0),
    'mid': metal('mid', (0.11, 0.12, 0.15), .45),
    'light': metal('light', (0.28, 0.3, 0.34), .38, scale=.5),
    'emit': emit('emit', FAC, 3.5),
    'emit_soft': emit('emit_soft', FAC, 2, .5),
    'beam': emit('beam', FAC, 2.2, .3),
    'glass': emit('glass', FAC, 1.2, .1),
    'white': emit('white', (0.9, 0.95, 1), 14),
}

# ---------------- geometry builder ----------------
class B:
    def __init__(s): s.bm = {}
    def get(s, mat):
        if mat not in s.bm: s.bm[mat] = bmesh.new()
        return s.bm[mat]
    def box(s, mat, c, size, rz=0.0, rx=0.0, ry=0.0):
        mtx = Matrix.Translation(Vector(c)) @ Euler((rx, ry, rz)).to_matrix().to_4x4() @ Matrix.Diagonal((*size, 1))
        bmesh.ops.create_cube(s.get(mat), size=1.0, matrix=mtx)
    def cyl(s, mat, c, r1, r2, depth, seg=16, rz=0.0, rx=0.0, ry=0.0):
        mtx = Matrix.Translation(Vector(c)) @ Euler((rx, ry, rz)).to_matrix().to_4x4()
        bmesh.ops.create_cone(s.get(mat), cap_ends=True, segments=seg, radius1=r1, radius2=r2, depth=depth, matrix=mtx)
    def sphere(s, mat, c, r):
        bmesh.ops.create_uvsphere(s.get(mat), u_segments=24, v_segments=12, radius=r, matrix=Matrix.Translation(Vector(c)))
    def ring(s, mat, c, R, w, h=.03, seg=96):
        # flat ring from boxes around a circle
        for i in range(seg):
            a = i / seg * math.tau; L = math.tau * R / seg * 1.08
            s.box(mat, (c[0] + R * math.cos(a), c[1] + R * math.sin(a), c[2]), (w, L, h), rz=a)
    def build(s, name, bevel=True):
        objs = []
        for mat, bm in s.bm.items():
            me = bpy.data.meshes.new(name + '_' + mat); bm.to_mesh(me); bm.free()
            ob = bpy.data.objects.new(name + '_' + mat, me); sc.collection.objects.link(ob)
            ob.data.materials.append(M[mat])
            if bevel and mat in ('dark', 'mid', 'light'):
                bv = ob.modifiers.new('bv', 'BEVEL'); bv.width = .025; bv.segments = 2; bv.limit_method = 'ANGLE'
                bv.harden_normals = False
            for p in me.polygons: p.use_smooth = False
            objs.append(ob)
        s.bm = {}; return objs

TIERS = {1: (4.0, 7, 3), 2: (5.5, 10, 6), 3: (7, 13, 9), 4: (8.5, 17, 13), 5: (10, 22, 18)}

def face_greebles(b, rnd, cx, cy, z0, w, d, lh, rz):
    # four faces of a box centred (cx,cy), size w x d, from z0 to z0+lh
    ca, sa = math.cos(rz), math.sin(rz)
    def P(x, y, z): return (cx + x * ca - y * sa, cy + x * sa + y * ca, z)
    for f in range(4):
        nx, ny = [(1, 0), (0, 1), (-1, 0), (0, -1)][f]
        half_n = (w if nx else d) / 2; half_t = (d if nx else w) / 2
        tx, ty = -ny, nx
        for i in range(rnd.randint(10, 18)):
            u = rnd.uniform(-half_t * .8, half_t * .8); v = rnd.uniform(.12, .88) * lh
            sw = rnd.uniform(.08, .32) * w; sh = rnd.uniform(.06, .25) * lh; dp = rnd.uniform(.04, .18)
            x = nx * (half_n + dp / 2) + tx * u; y = ny * (half_n + dp / 2) + ty * u
            b.box(rnd.choice(['mid', 'dark', 'mid', 'light']), P(x, y, z0 + v), (dp if nx else sw, sw if nx else dp, sh), rz=rz)
        # vents
        u0 = rnd.uniform(-half_t * .5, half_t * .5); v0 = rnd.uniform(.2, .6) * lh
        for k in range(5):
            x = nx * (half_n + .03) + tx * u0; y = ny * (half_n + .03) + ty * u0
            b.box('dark', P(x, y, z0 + v0 + k * .09), (.06 if nx else w * .2, w * .2 if nx else .06, .035), rz=rz)
        # emissive strips
        for sgn in (-1, 1):
            u = sgn * half_t * .55
            x = nx * (half_n + .015) + tx * u; y = ny * (half_n + .015) + ty * u
            b.box('emit', P(x, y, z0 + lh * .5), (.03 if nx else .05, .05 if nx else .03, lh * .7), rz=rz)
        # pipe
        u = rnd.uniform(-half_t * .9, half_t * .9)
        x = nx * (half_n + .07) + tx * u; y = ny * (half_n + .07) + ty * u
        b.cyl('light', P(x, y, z0 + lh / 2), .045, .045, lh * .95, seg=8)

def tower(b, rnd, tier, ox, oy, z0):
    pr, th, _ = TIERS[tier]
    lv = tier + 2; lh = th / lv; w = 2.6 + tier * .45
    # foundation skirt
    b.cyl('mid', (ox, oy, z0 + .5), w * 1.0, w * .72, 1.0, seg=4, rz=math.pi / 4)
    z = z0 + 1.0
    for k in range(lv):
        rz = (k % 2) * math.pi / 12
        d = w * .86
        b.box('dark', (ox, oy, z + lh * .47), (w, d, lh * .94), rz=rz)
        face_greebles(b, rnd, ox, oy, z, w, d, lh, rz)
        # corner buttresses
        for cx in (-1, 1):
            for cy in (-1, 1):
                x, y = cx * w * .5, cy * d * .5
                ca, sa = math.cos(rz), math.sin(rz)
                b.box('mid', (ox + x * ca - y * sa, oy + x * sa + y * ca, z + lh * .5), (w * .16, w * .16, lh * 1.02), rz=rz)
        # cantilevered blocks and side masses (jagged silhouette)
        for j in range(3):
            a2 = rnd.uniform(0, math.tau); r2 = w * rnd.uniform(.45, .62)
            cw = w * rnd.uniform(.22, .4); chh = lh * rnd.uniform(.3, .8)
            b.box(rnd.choice(['dark', 'mid']), (ox + math.cos(a2) * r2, oy + math.sin(a2) * r2, z + rnd.uniform(.1, .6) * lh), (cw, cw * .8, chh), rz=a2)
        if k < lv - 1:
            for sd in (-1, 1):
                sw2 = w * .38
                b.box('dark', (ox + sd * w * .62, oy + w * .1, z + lh * .4), (sw2, sw2 * 1.1, lh * .8), rz=rz)
                b.box('emit', (ox + sd * (w * .62 + sw2 * .5 + .01), oy + w * .1, z + lh * .4), (.02, sw2 * .7, .04), rz=rz)
        # ledge + glow band
        b.box('light', (ox, oy, z + lh * .97), (w * 1.1, d * 1.1, .09), rz=rz)
        b.box('emit', (ox, oy, z + lh * .9), (w * 1.02, d * 1.02, .03), rz=rz)
        z += lh; w *= .84
    # crown
    b.box('mid', (ox, oy, z + .3), (w * .9, w * .8, .6))
    for i in range(3 + tier // 2):
        a = i * 2.1; r = w * .3 * (i % 2)
        hgt = rnd.uniform(1.5, 3.2) + tier * .4
        b.cyl('dark', (ox + r * math.cos(a), oy + r * math.sin(a), z + .6 + hgt / 2), .06 + .03 * (i == 0), .02, hgt, seg=8)
        b.sphere('emit', (ox + r * math.cos(a), oy + r * math.sin(a), z + .6 + hgt), .06)
    b.sphere('emit', (ox, oy, z + 1.0), .32)
    return z

def module(b, rnd, c, s):
    x, y, z = c
    b.box('dark', (x, y, z), (s * .78, s * .78, s * .78))
    for f in range(4):
        a = f * math.pi / 2
        b.box('mid', (x + math.cos(a) * s * .4, y + math.sin(a) * s * .4, z), (s * .05 if f % 2 == 0 else s * .55, s * .55 if f % 2 == 0 else s * .05, s * .55))
        b.box('emit', (x + math.cos(a) * s * .43, y + math.sin(a) * s * .43, z + s * .2), (s * .03 if f % 2 == 0 else s * .45, s * .45 if f % 2 == 0 else s * .03, s * .04))
    b.box('light', (x, y, z + s * .41), (s * .5, s * .5, .05))
    for i in range(4):
        b.box('mid', (x + rnd.uniform(-.25, .25) * s, y + rnd.uniform(-.25, .25) * s, z + s * .45), (s * .12, s * .12, s * .1))
    # glowing frame (12 edges)
    e = s * .5; t = .035
    for sx in (-1, 1):
        for sy in (-1, 1):
            b.box('emit', (x + sx * e, y + sy * e, z), (t, t, s))
            b.box('emit', (x + sx * e, y, z + sy * e), (t, s, t))
            b.box('emit', (x, y + sx * e, z + sy * e), (s, t, t))
    b.box('glass', (x, y, z), (s * 1.0, s * 1.0, s * 1.0))

def flag(name, pos, rz, w, h):
    bpy.ops.mesh.primitive_plane_add(size=1)
    ob = bpy.context.object; ob.name = name
    ob.scale = (w, h, 1); bpy.ops.object.transform_apply(scale=True)
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.subdivide(number_cuts=14); bpy.ops.object.mode_set(mode='OBJECT')
    for v in ob.data.vertices:
        u = v.co.x + w / 2
        v.co.z = math.sin(u * 2.4 + v.co.y * .6) * .12 * (u / w + .2)
        v.co.x = u
    ob.rotation_euler = (math.pi / 2, 0, rz); ob.location = pos
    ob.data.materials.append(M['banner'])
    return ob

M['banner'] = img_mat('banner', 'banner_%s.png' % FCODE)
M['holo'] = img_mat('holo', 'holo_%s.png' % FCODE, emit_strength=4, col=FAC, alpha_mul=.9)
M['hexdome'] = img_mat('hexdome', 'hexgrid.png', emit_strength=3, col=FAC)

def base(tier, ox=0.0, oy=0.0, seed=4, holo_face=None, assembling=None):
    rnd = random.Random(seed * 31 + tier)
    pr, th, nm = TIERS[tier]
    b = B()
    # platform: stepped hex
    b.cyl('mid', (ox, oy, .3), pr * 1.08, pr * 1.0, .6, seg=6, rz=math.pi / 6)
    b.cyl('dark', (ox, oy, .75), pr * .8, pr * .76, .3, seg=6, rz=math.pi / 6)
    for k in range(6):  # rim plates
        a = k * math.pi / 3
        b.box('light', (ox + math.cos(a) * pr * .93, oy + math.sin(a) * pr * .93, .62), (pr * .9, .25, .05), rz=a + math.pi / 2)
    b.ring('emit', (ox, oy, .62), pr * .88, .05, .02)
    b.ring('emit', (ox, oy, .91), pr * .55, .04, .02)
    b.ring('emit_soft', (ox, oy, .02), pr * 1.3, .07, .02)
    b.ring('emit_soft', (ox, oy, .02), pr * 1.55, .05, .02)
    # stairs front
    for k in range(3):
        b.box('mid', (ox, oy - pr * 1.0 - .35 + k * .35, .1 + k * .2), (2.2, .4, .2 + k * .4))
    top = tower(b, rnd, tier, ox, oy, .9)
    # modules
    mods = []
    for i in range(nm):
        ring2 = i >= 8; cnt = (nm - 8) if ring2 else min(nm, 8); idx = i - 8 if ring2 else i
        a = idx / cnt * math.tau + (.3 if ring2 else 0) + .2
        rad = pr * (.86 if ring2 else .55); s = .9 if ring2 else 1.2
        h = (2.0 + (idx % 2) * 1.2) if ring2 else th * (.3 + .4 * ((idx % 3) / 2))
        px, py = ox + math.cos(a) * rad, oy + math.sin(a) * rad
        b.cyl('light', (px, py, .97 if not ring2 else .66), .75, .7, .1, seg=6)
        b.ring('emit', (px, py, 1.03 if not ring2 else .72), .6, .04, .02, seg=24)
        if assembling is not None and i == assembling:
            mods.append((px, py, h, s, a)); continue
        module(b, rnd, (px, py, h), s)
        b.cyl('beam', (px, py, (h + 1) / 2), .07, .13, h - 1.0, seg=10)
        if not ring2:
            L = rad - 1.0
            b.box('mid', (ox + math.cos(a) * (L / 2 + .6), oy + math.sin(a) * (L / 2 + .6), h), (L, .16, .16), rz=a)
            b.box('emit', (ox + math.cos(a) * (L / 2 + .6), oy + math.sin(a) * (L / 2 + .6), h + .1), (L, .03, .03), rz=a)
        mods.append((px, py, h, s, a))
    # turrets
    if tier >= 3:
        nt = tier * 2 - 2
        for i in range(nt):
            a = (i + .5) / nt * math.tau; px, py = ox + math.cos(a) * pr * .97, oy + math.sin(a) * pr * .97
            b.cyl('mid', (px, py, .85), .45, .55, .5, seg=8)
            b.box('dark', (px, py, 1.3), (.9, .75, .45), rz=a)
            for sd in (-.18, .18):
                bx = px + math.cos(a) * .85 - math.sin(a) * sd; by = py + math.sin(a) * .85 + math.cos(a) * sd
                b.cyl('light', (bx, by, 1.35), .06, .07, 1.2, seg=8, rz=a, ry=math.pi / 2)
            b.box('emit', (px + math.cos(a) * .46, py + math.sin(a) * .46, 1.4), (.04, .5, .06), rz=a)
    # banner poles
    nb = 2 * tier
    flags = []
    for i in range(nb):
        a = i / nb * math.tau + math.pi / nb; r = pr * 1.18
        px, py = ox + math.cos(a) * r, oy + math.sin(a) * r; ph = 4 + tier * 1.0
        b.cyl('dark', (px, py, ph / 2), .07, .09, ph, seg=8)
        b.cyl('mid', (px, py, ph + .35), .15, .0, .7, seg=6)
        b.box('mid', (px, py, ph - .1), (.1, .1, .1))
        flags.append((px, py, ph, a))
    objs = b.build('base%d' % tier)
    for j, (px, py, ph, a) in enumerate(flags):
        flag('flag%d_%d' % (tier, j), (px, py, ph - 1.55), a + math.pi / 2, 1.1, 2.8)
    # holo banners
    if tier >= 2 and holo_face is not None:
        for sd in (-1, 1):
            bpy.ops.mesh.primitive_plane_add(size=1); hb = bpy.context.object
            hb.scale = (pr * .42, pr * .74, 1); hb.rotation_euler = (math.pi / 2, 0, holo_face)
            hb.location = (ox + sd * pr * 1.35 * math.cos(holo_face), oy + sd * pr * 1.35 * math.sin(holo_face), th * .55 + pr * .25)
            hb.data.materials.append(M['holo'])
    return objs, mods, top

def scaffold_and_module(px, py, h, s, lift):
    b = B(); rnd = random.Random(7)
    module(b, rnd, (px, py, h + lift), s)
    e = s * .68; t = .025; z = h + lift
    for sx in (-1, 1):
        for sy in (-1, 1):
            b.box('emit_soft', (px + sx * e, py + sy * e, z), (t, t, 2 * e))
            b.box('emit_soft', (px + sx * e, py, z + sy * e), (t, 2 * e, t))
            b.box('emit_soft', (px, py + sx * e, z + sy * e), (2 * e, t, t))
            b.box('beam', (px + sx * e, py + sy * e, (z - e + 1) / 2), (t, t, z - e - 1))
    b.cyl('beam', (px, py, (z + 1) / 2), .12, .3, z - 1.0, seg=12)
    b.build('assembling', bevel=False)

# ---------------- world ----------------
w = bpy.data.worlds.new('w'); sc.world = w; w.use_nodes = True
bg = w.node_tree.nodes['Background']; bg.inputs[1].default_value = 1
_tc = w.node_tree.nodes.new('ShaderNodeTexCoord'); _sep = w.node_tree.nodes.new('ShaderNodeSeparateXYZ'); _cr = w.node_tree.nodes.new('ShaderNodeValToRGB')
w.node_tree.links.new(_tc.outputs['Generated'], _sep.inputs[0]); w.node_tree.links.new(_sep.outputs['Z'], _cr.inputs[0])
_cr.color_ramp.elements[0].position = .5; _cr.color_ramp.elements[0].color = (0.006, 0.005, 0.012, 1)
_e = _cr.color_ramp.elements.new(.53); _e.color = (0.035, 0.014, 0.05, 1)
_cr.color_ramp.elements[1].position = .75; _cr.color_ramp.elements[1].color = (0.002, 0.002, 0.008, 1)
w.node_tree.links.new(_cr.outputs[0], bg.inputs[0])

def ground(size=200):
    bpy.ops.mesh.primitive_plane_add(size=size); g = bpy.context.object
    m, nt, out = node_mat('ground')
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); p.inputs['Base Color'].default_value = (0.01, 0.012, 0.02, 1); p.inputs['Roughness'].default_value = .55; p.inputs['Metallic'].default_value = .3
    tc = nt.nodes.new('ShaderNodeTexCoord'); mp = nt.nodes.new('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (size / 6, size / 6, 1)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + 'hexgrid.png')
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (0.25, 0.3, 0.55, 1)
    gr = nt.nodes.new('ShaderNodeTexGradient'); gr.gradient_type = 'SPHERICAL'
    mp2 = nt.nodes.new('ShaderNodeMapping'); mp2.inputs['Scale'].default_value = (1 / 30, 1 / 30, 1)
    nt.links.new(tc.outputs['Object'], mp2.inputs[0]); nt.links.new(mp2.outputs[0], gr.inputs[0])
    mul = nt.nodes.new('ShaderNodeMath'); mul.operation = 'MULTIPLY'
    nt.links.new(tx.outputs['Color'], mul.inputs[0]); nt.links.new(gr.outputs['Fac'], mul.inputs[1])
    mul2 = nt.nodes.new('ShaderNodeMath'); mul2.operation = 'MULTIPLY'; mul2.inputs[1].default_value = .5
    nt.links.new(mul.outputs[0], mul2.inputs[0]); nt.links.new(mul2.outputs[0], e.inputs[1])
    nt.links.new(tc.outputs['UV'], mp.inputs[0]); nt.links.new(mp.outputs[0], tx.inputs[0])
    ad = nt.nodes.new('ShaderNodeAddShader'); nt.links.new(p.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); nt.links.new(ad.outputs[0], out.inputs[0])
    g.data.materials.append(m)

def light(kind, loc, rot, energy, col, size=5):
    ld = bpy.data.lights.new('l', kind); ld.energy = energy; ld.color = col
    if kind == 'AREA': ld.size = size
    o = bpy.data.objects.new('l', ld); o.location = loc; o.rotation_euler = rot; sc.collection.objects.link(o)

def cam(loc, target, lens=35):
    cd = bpy.data.cameras.new('c'); cd.lens = lens; o = bpy.data.objects.new('c', cd); sc.collection.objects.link(o)
    o.location = loc; d = Vector(target) - Vector(loc); o.rotation_euler = d.to_track_quat('-Z', 'Y').to_euler(); sc.camera = o

def render_setup(w_, h_, out):
    sc.render.engine = 'CYCLES'; sc.cycles.device = 'CPU'; sc.cycles.samples = SAMPLES
    sc.cycles.use_denoising = True; sc.cycles.denoiser = 'OPENIMAGEDENOISE'
    sc.cycles.max_bounces = 4; sc.cycles.transparent_max_bounces = 16
    sc.render.resolution_x = w_; sc.render.resolution_y = h_; sc.render.filepath = out
    sc.view_settings.view_transform = 'Filmic'; sc.view_settings.look = 'Medium High Contrast'
    sc.use_nodes = True; nt = sc.node_tree; nt.nodes.clear()
    rl = nt.nodes.new('CompositorNodeRLayers'); gl = nt.nodes.new('CompositorNodeGlare'); gl.glare_type = 'FOG_GLOW'; gl.threshold = 1.2; gl.size = 7; gl.mix = 0.0
    co = nt.nodes.new('CompositorNodeComposite'); nt.links.new(rl.outputs[0], gl.inputs[0]); nt.links.new(gl.outputs[0], co.inputs[0])

def std_lights(scale=1.0):
    light('AREA', (-20 * scale, -18 * scale, 25 * scale), (math.radians(50), 0, math.radians(-45)), 26000 * scale ** 2, (0.75, 0.82, 1.0), 12 * scale)
    light('AREA', (22 * scale, 20 * scale, 10 * scale), (math.radians(70), 0, math.radians(135)), 3500 * scale ** 2, FAC, 15 * scale)
    light('AREA', (0, 30 * scale, 30 * scale), (math.radians(-40), 0, 0), 5000 * scale ** 2, (0.4, 0.5, 1.0), 20 * scale)

ground()
if SHOT == 'hero':
    objs, mods, top = base(4, holo_face=math.pi / 4)
    std_lights()
    cam((21, -21, 5.0), (0, 0, 9.8), lens=30)
    render_setup(int(1080*RS), int(1920*RS), '/tmp/claude-0/bl3/out/hero_%s.png' % FCODE)
elif SHOT == 'lineup':
    xs = [-44, -28, -9, 13, 39]
    for t, x in zip(range(1, 6), xs):
        base(t, ox=x, holo_face=None, seed=t)
    std_lights(1.8)
    cam((-2, -100, 15), (-2, 0, 9), lens=32)
    render_setup(2400, 1100, '/tmp/claude-0/bl3/out/lineup.png')
elif SHOT == 'assemble':
    objs, mods, top = base(3, assembling=1, holo_face=None)
    px, py, h, s, a = mods[1]
    scaffold_and_module(px, py, h, s, 3.2)
    std_lights()
    cam((px + 6 * math.cos(a) + 5, py + 6 * math.sin(a) - 9, h + 2.5), (px, py, h + 2.0), lens=32)
    render_setup(1080, 1350, '/tmp/claude-0/bl3/out/assemble.png')
elif SHOT == 'shield':
    objs, mods, top = base(5, holo_face=0.28)
    bpy.ops.mesh.primitive_uv_sphere_add(radius=19, segments=64, ring_count=32, location=(0, 0, 0)); d = bpy.context.object
    d.scale = (1, 1, 1.25)
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.select_all(action='DESELECT'); bpy.ops.object.mode_set(mode='OBJECT')
    for v in d.data.vertices: v.select = v.co.z < -0.01
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.delete(type='VERT'); bpy.ops.object.mode_set(mode='OBJECT')
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.uv.sphere_project(); bpy.ops.object.mode_set(mode='OBJECT')
    m, nt, out = node_mat('dome')
    tc = nt.nodes.new('ShaderNodeTexCoord'); mp = nt.nodes.new('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (14, 7, 1)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + 'hexgrid.png')
    lw = nt.nodes.new('ShaderNodeLayerWeight'); lw.inputs[0].default_value = .35
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (*FAC, 1)
    mx = nt.nodes.new('ShaderNodeMath'); mx.operation = 'MULTIPLY_ADD'; mx.inputs[2].default_value = .01
    nt.links.new(tx.outputs['Color'], mx.inputs[0]); nt.links.new(lw.outputs['Facing'], mx.inputs[1])
    st = nt.nodes.new('ShaderNodeMath'); st.operation = 'MULTIPLY'; st.inputs[1].default_value = 2.2
    nt.links.new(mx.outputs[0], st.inputs[0]); nt.links.new(st.outputs[0], e.inputs[1])
    tr = nt.nodes.new('ShaderNodeBsdfTransparent'); ad = nt.nodes.new('ShaderNodeAddShader')
    nt.links.new(tr.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); nt.links.new(ad.outputs[0], out.inputs[0])
    nt.links.new(tc.outputs['UV'], mp.inputs[0]); nt.links.new(mp.outputs[0], tx.inputs[0]); m.blend_method = 'BLEND'
    d.data.materials.append(m)
    # impact burst
    bpy.ops.mesh.primitive_uv_sphere_add(radius=.7, location=(-11, -12, 14)); ib = bpy.context.object; ib.data.materials.append(emit('impact', (1, .55, .15), 40))
    std_lights(1.2)
    cam((14, -48, 9), (0, 0, 12), lens=30)
    render_setup(1920, 1080, '/tmp/claude-0/bl3/out/shield.png')

bpy.ops.render.render(write_still=True)
if SHOT == 'hero':
    bpy.ops.export_scene.gltf(filepath='/tmp/claude-0/bl3/out/citadel.glb', export_format='GLB', use_selection=False, export_lights=False, export_cameras=False)
print('DONE', SHOT)
