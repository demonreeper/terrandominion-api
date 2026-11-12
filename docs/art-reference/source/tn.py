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

    return ob


# ================= Tether Node (hex pylon) =================
def lin(hx):
    c = tuple(((int(hx[i:i+2], 16) / 255) ** 2.2) for i in (0, 2, 4)); m_ = max(c); return tuple(x / m_ for x in c)

RES = {'crystal': lin('59d9ff'), 'alloy': lin('ffb840'), 'data': lin('b885ff'), 'credit': lin('66f28c')}
FACS = {'free': '3a8cff', 'lumen': '4fd8ff', 'forge': 'ffb52e', 'veil': 'b57cff', 'verdant': '4cf08a', 'slip': 'ff5d6c', 'bastion': 'dbe8f5', 'fa': '5b78ff', 'fb': 'ff5fd2', 'fc': 'd4ff3a'}

def fmats(key):
    global FAC
    col = lin(FACS[key]); old = FAC; FAC = col
    M['dark@' + key] = metal('dark@' + key, (0.05, 0.055, 0.07), .4, glow=3.0)
    M['emit@' + key] = emit('emit@' + key, col, 4)
    M['hot@' + key] = emit('hot@' + key, tuple(.55 + .45 * c for c in col), 9)
    M['soft@' + key] = emit('soft@' + key, col, 2.2, .5)
    M['beam@' + key] = emit('beam@' + key, col, 2.5, .3)
    M['glass@' + key] = emit('glass@' + key, col, 1.0, .12)
    M['plate@' + key] = img_mat('plate@' + key, 'hexgrid.png', emit_strength=1.6, col=col, alpha_mul=.8)
    if key not in ('free', 'fa', 'fb', 'fc'):
        M['banner@' + key] = img_mat('banner@' + key, 'banner_%s.png' % key)
    FAC = old
    return col

for r, c in RES.items():
    M['r_' + r] = emit('r_' + r, c, 5)
    M['rs_' + r] = emit('rs_' + r, c, 2.2, .45)
M['dmg'] = emit('dmg', (1, .12, .08), 3)
M['dmg_dim'] = emit('dmg_dim', (1, .12, .08), .8, .5)

def hex_ring(b, mat, c, R, w, h=.03, rz=0.0, tilt=(0, 0)):
    for i in range(6):
        a0 = rz + i * math.pi / 3; a1 = a0 + math.pi / 3
        p0 = Vector((R * math.cos(a0), R * math.sin(a0), 0)); p1 = Vector((R * math.cos(a1), R * math.sin(a1), 0))
        mid = (p0 + p1) / 2; ln = (p1 - p0).length
        ang = math.atan2(p1.y - p0.y, p1.x - p0.x)
        rot = Euler((tilt[0], tilt[1], 0)).to_matrix()
        mm = rot @ mid
        mtx = Matrix.Translation(Vector(c) + mm) @ rot.to_4x4() @ Euler((0, 0, ang)).to_matrix().to_4x4() @ Matrix.Diagonal((ln + w, w, h, 1))
        bmesh.ops.create_cube(b.get(mat), size=1.0, matrix=mtx)

def seg(b, mat, p0, p1, r, sides=6):
    p0 = Vector(p0); p1 = Vector(p1); d = p1 - p0
    q = d.to_track_quat('Z', 'Y')
    mtx = Matrix.Translation((p0 + p1) / 2) @ q.to_matrix().to_4x4()
    bmesh.ops.create_cone(b.get(mat), cap_ends=True, segments=sides, radius1=r, radius2=r, depth=d.length, matrix=mtx)

def pylon(key, tier, ox=0.0, oy=0.0, health=1.0, banners=True, beam=True):
    col = fmats(key); b = B(); o = Vector((ox, oy, 0)); rnd = random.Random(tier * 7 + len(key))
    D, E, H, S, BM, G = 'dark@' + key, 'emit@' + key, 'hot@' + key, 'soft@' + key, 'beam@' + key, 'glass@' + key
    PR = 2.0 if tier == 1 else 2.6
    # --- hex plate: stepped base, inset, rim lights, corner brackets
    b.cyl('mid', o + Vector((0, 0, .09)), PR + .25, PR + .1, .18, seg=6, rz=math.pi / 6)
    b.cyl(D, o + Vector((0, 0, .21)), PR, PR - .12, .1, seg=6, rz=math.pi / 6)
    b.cyl('mid', o + Vector((0, 0, .29)), PR * .55, PR * .5, .1, seg=6, rz=math.pi / 6)
    hex_ring(b, E, o + Vector((0, 0, .265)), PR * .97, .04, .02, rz=math.pi / 6)
    hex_ring(b, S, o + Vector((0, 0, .265)), PR * .72, .03, .015, rz=math.pi / 6)
    for i in range(6):
        a = i * math.pi / 3 + math.pi / 6; R = PR + .17
        cx, cy = ox + R * math.cos(a), oy + R * math.sin(a)
        b.box('light', (cx, cy, .2), (.28, .28, .26), rz=a)
        b.box(H, (ox + (R + .15) * math.cos(a), oy + (R + .15) * math.sin(a), .2), (.05, .12, .05), rz=a)
        # survey ticks between corners
        for t in range(1, 4):
            at = a + t * math.pi / 12
            b.box(E, (ox + (PR + .02) * math.cos(at), oy + (PR + .02) * math.sin(at), .27), (.18, .025, .02), rz=at)
    # --- buttress fins (three, leaning in)
    for i in range(3):
        a = i * math.tau / 3 + .3
        for k in range(4):
            t = k / 3
            r_ = .62 + .55 * (1 - t); z = .45 + 1.1 * t
            b.box(D, (ox + r_ * math.cos(a), oy + r_ * math.sin(a), z), (.38, .12, .38), rz=a, ry=-.5)
        b.box(E, (ox + .95 * math.cos(a), oy + .95 * math.sin(a), .75), (.03, .14, .5), rz=a, ry=-.6)
    # --- tapered hex spire in three stacked segments with glowing bands and slits
    z = .34; r = .62 if tier == 1 else .7; segs = [(1.15, .86), (1.0, .84), (.85, .8)] if tier == 1 else [(1.25, .86), (1.1, .86), (.95, .84), (.7, .8)]
    for i, (h, k) in enumerate(segs):
        r2 = r * k
        b.cyl(D if i % 2 == 0 else 'mid', o + Vector((0, 0, z + h / 2)), r, r2, h, seg=6)
        for f in range(6):
            a = f * math.pi / 3 + math.pi / 6; rr = (r + r2) / 2 * .87 + .01
            b.box(E, (ox + rr * math.cos(a), oy + rr * math.sin(a), z + h * .5), (.025, .07, h * .55), rz=a)
            if rnd.random() < .5:
                b.box('light', (ox + (rr + .02) * math.cos(a + .25), oy + (rr + .02) * math.sin(a + .25), z + h * rnd.uniform(.2, .8)), (.06, .14, .1), rz=a)
        z += h
        b.cyl(E, o + Vector((0, 0, z + .03)), r2 + .05, r2 + .05, .06, seg=6)
        b.cyl('light', o + Vector((0, 0, z + .1)), r2 + .02, r2 * .92, .08, seg=6)
        z += .12; r = r2 * .95
    top = z
    # crown prongs holding the core
    for i in range(3):
        a = i * math.tau / 3
        p0 = o + Vector((r * .8 * math.cos(a), r * .8 * math.sin(a), top)); p1 = o + Vector((.42 * math.cos(a), .42 * math.sin(a), top + .7))
        seg(b, 'mid', p0, p1, .055, 6)
        b.sphere(H, p1, .05)
    # --- floating cut core
    cz = top + 1.05
    b.cyl(H, o + Vector((0, 0, cz + .22)), .3, 0.0, .45, seg=6)
    b.cyl(H, o + Vector((0, 0, cz - .17)), 0.0, .3, .35, seg=6)
    b.sphere(G, o + Vector((0, 0, cz)), .55)
    # orbit rings (more with tier)
    hex_ring(b, E, o + Vector((0, 0, cz)), .8, .03, .03, rz=.2, tilt=(.62, .15))
    if tier >= 2:
        hex_ring(b, E, o + Vector((0, 0, cz)), 1.0, .03, .03, rz=.6, tilt=(-.5, .6))
        hex_ring(b, S, o + Vector((0, 0, cz - .2)), 1.25, .02, .02, rz=0, tilt=(.3, -.35))
    # --- sky beam with rising signal rings
    if beam:
        b.cyl(BM, o + Vector((0, 0, cz + 8)), .14, .02, 15, seg=12)
        b.cyl(E, o + Vector((0, 0, cz + 6)), .02, .008, 11, seg=8)
    # --- perimeter posts, lattice (tier 2), banners
    posts = []
    n = 3 if tier == 1 else 6
    for i in range(n):
        a = i * math.tau / n + math.pi / 6; R = PR + .55
        p = o + Vector((R * math.cos(a), R * math.sin(a), 0)); posts.append((p, a))
        ph = 1.6 if tier == 1 else 2.0
        b.cyl('mid', p + Vector((0, 0, .1)), .16, .14, .2, seg=6)
        b.cyl(D, p + Vector((0, 0, ph / 2)), .06, .05, ph, seg=6)
        b.cyl(E, p + Vector((0, 0, ph + .06)), .07, .07, .12, seg=6)
        b.sphere(H, p + Vector((0, 0, ph + .18)), .045)
    if tier >= 2:
        for i in range(n):
            p0, _ = posts[i]; p1, _ = posts[(i + 1) % n]
            for zz in (.7, 1.35):
                seg(b, S, p0 + Vector((0, 0, zz)), p1 + Vector((0, 0, zz)), .018, 4)
    objs = b.build('tn_' + key)
    # hex-cell glow inset on the plate
    bpy.ops.mesh.primitive_circle_add(vertices=6, radius=PR * .95, fill_type='NGON', location=(ox, oy, .262), rotation=(0, 0, math.pi / 6))
    pl = bpy.context.object; bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.uv.cube_project(cube_size=1.2); bpy.ops.object.mode_set(mode='OBJECT')
    pl.data.materials.append(M['plate@' + key])
    if banners and key in M_BANNER_KEYS:
        for i, (p, a) in enumerate(posts):
            if tier == 1 or i % 2 == 0:
                bn = flag('bn', p + Vector((0, 0, (1.6 if tier == 1 else 2.0) - .1)), a + math.pi / 2, .55, 1.0)
                bn.data.materials.clear(); bn.data.materials.append(M['banner@' + key])
                bn.location = p + Vector((.0, .0, 1.05 if tier == 1 else 1.45))
    # health ring on the ground
    hb = B(); RR = PR + 1.15; N_ = 72
    for i in range(N_):
        if i % 12 in (0,): continue
        a = i / N_ * math.tau + math.pi / 2
        mat = E if i / N_ < health else 'dmg_dim'
        hb.box(mat, (ox + RR * math.cos(a), oy + RR * math.sin(a), .02), (.09, math.tau * RR / N_ * .8, .02), rz=a)
    hb.build('hp_' + key, bevel=False)
    return o, top, cz

def node_at(kind, p, rnd):
    b = B(); p = Vector(p)
    E, S = 'r_' + kind, 'rs_' + kind
    b.cyl('mid', p + Vector((0, 0, .06)), .6, .55, .12, seg=6)
    hex_ring(b, E, p + Vector((0, 0, .13)), .5, .03, .015)
    hex_ring(b, S, p + Vector((0, 0, .015)), .95, .03, .015)
    if kind == 'crystal':
        for i in range(7):
            a = rnd.uniform(0, math.tau); r = 0 if i == 0 else rnd.uniform(.12, .32); h = (1.3 if i == 0 else rnd.uniform(.45, .9))
            tilt = 0 if i == 0 else .35
            b.cyl(E if i % 2 == 0 else S, p + Vector((r * math.cos(a), r * math.sin(a), .12 + h / 2)), .12 if i == 0 else .08, 0.0, h, seg=6, rx=tilt * math.sin(a), ry=-tilt * math.cos(a))
    elif kind == 'alloy':
        for i in range(5):
            b.box('light', p + Vector((0, 0, .2 + i * .14)), (.62 - i * .06, .4 - i * .03, .1), rz=i * .25)
            b.box(E, p + Vector((0, 0, .26 + i * .14)), (.64 - i * .06, .42 - i * .03, .02), rz=i * .25)
    elif kind == 'data':
        c = p + Vector((0, 0, .75)); s = .3
        for sx in (-1, 1):
            for sy in (-1, 1):
                b.box(E, c + Vector((sx * s, sy * s, 0)), (.03, .03, 2 * s))
                b.box(E, c + Vector((sx * s, 0, sy * s)), (.03, 2 * s, .03))
                b.box(E, c + Vector((0, sx * s, sy * s)), (2 * s, .03, .03))
        b.box(S, c, (.3, .3, .3), rz=.6, rx=.6)
        for i in range(4):
            a = i * math.tau / 4 + .4
            b.box(E, c + Vector((.62 * math.cos(a), .62 * math.sin(a), .15 * math.sin(3 * a))), (.09, .09, .09), rz=a)
    elif kind == 'credit':
        for i in range(8):
            a = i * math.tau / 8
            b.cyl(E if i % 2 else S, p + Vector((.42 * math.cos(a), .42 * math.sin(a), .7)), .14, .14, .025, seg=12, ry=math.pi / 2, rz=a + math.pi / 2)
        b.cyl(E, p + Vector((0, 0, .7)), .17, .17, .04, seg=6, rx=math.pi / 2)
    b.build('node_' + kind, bevel=False)

def link(kind, a, bpt, lift=1.6, packets=4, rnd=None):
    b = B(); a = Vector(a); bpt = Vector(bpt); mid = (a + bpt) / 2 + Vector((0, 0, lift))
    pts = [(1 - t) ** 2 * a + 2 * (1 - t) * t * mid + t ** 2 * bpt for t in [i / 40 for i in range(41)]]
    for i in range(40):
        seg(b, 'r_' + kind, pts[i], pts[i + 1], .022, 6)
        seg(b, 'rs_' + kind, pts[i], pts[i + 1], .06, 6)
    for k in range(packets):
        t = (k + .5 + (rnd.uniform(-.2, .2) if rnd else 0)) / packets
        q = (1 - t) ** 2 * a + 2 * (1 - t) * t * mid + t ** 2 * bpt
        b.cyl('r_' + kind, q, .09, .09, .04, seg=6, rx=math.pi / 2)
    b.build('link_' + kind, bevel=False)

def links_for(o, cz, kinds, rnd, dist=7.5, start=0.4):
    for i, k in enumerate(kinds):
        a = start + i * math.tau / max(4, len(kinds)) + rnd.uniform(-.25, .25)
        d = dist + rnd.uniform(-1.2, 1.5)
        np_ = o + Vector((d * math.cos(a), d * math.sin(a), 0))
        node_at(k, np_, rnd)
        link(k, o + Vector((0, 0, cz - .9)), np_ + Vector((0, 0, .8)), lift=.6 + d * .08, rnd=rnd)

def dome(o, R, col, strength=2.0):
    bpy.ops.mesh.primitive_uv_sphere_add(radius=R, segments=48, ring_count=24, location=o); d = bpy.context.object
    d.scale = (1, 1, 1.1)
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.select_all(action='DESELECT'); bpy.ops.object.mode_set(mode='OBJECT')
    for v in d.data.vertices: v.select = v.co.z < -0.01
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.delete(type='VERT'); bpy.ops.object.mode_set(mode='OBJECT')
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.uv.sphere_project(); bpy.ops.object.mode_set(mode='OBJECT')
    m, nt, out = node_mat('dome')
    tc = nt.nodes.new('ShaderNodeTexCoord'); mp = nt.nodes.new('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (10, 5, 1)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + 'hexgrid.png')
    lw = nt.nodes.new('ShaderNodeLayerWeight'); lw.inputs[0].default_value = .35
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1)
    mx = nt.nodes.new('ShaderNodeMath'); mx.operation = 'MULTIPLY_ADD'; mx.inputs[2].default_value = .01
    nt.links.new(tx.outputs['Color'], mx.inputs[0]); nt.links.new(lw.outputs['Facing'], mx.inputs[1])
    st = nt.nodes.new('ShaderNodeMath'); st.operation = 'MULTIPLY'; st.inputs[1].default_value = strength
    nt.links.new(mx.outputs[0], st.inputs[0]); nt.links.new(st.outputs[0], e.inputs[1])
    tr = nt.nodes.new('ShaderNodeBsdfTransparent'); ad = nt.nodes.new('ShaderNodeAddShader')
    nt.links.new(tr.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); nt.links.new(ad.outputs[0], out.inputs[0])
    nt.links.new(tc.outputs['UV'], mp.inputs[0]); nt.links.new(mp.outputs[0], tx.inputs[0]); m.blend_method = 'BLEND'
    d.data.materials.append(m)

def tn_lights(scale=.35, col=None):
    global FAC
    if col: FAC = col
    std_lights(scale)

OUT = '/tmp/claude-0/bl3/out/'
M_BANNER_KEYS = ('lumen', 'forge', 'veil', 'verdant', 'slip', 'bastion')
ground(120)
rnd = random.Random(11)
if SHOT == 'tn1':
    o, top, cz = pylon('free', 1)
    links_for(o, cz, ['crystal'], rnd, dist=5.5, start=3.1)
    tn_lights(.35, lin('3a8cff'))
    cam((6.2, -7.8, 2.6), (-.4, 1.0, 2.4), lens=30)
    render_setup(int(1080 * RS), int(1350 * RS), OUT + 'tn-tier1.png')
elif SHOT == 'tn2':
    o, top, cz = pylon('free', 2)
    links_for(o, cz, ['crystal', 'alloy', 'data', 'credit'], rnd, dist=6.5, start=.15)
    tn_lights(.4, lin('3a8cff'))
    cam((12.5, -14.5, 5.6), (0, 1.2, 2.9), lens=30)
    render_setup(int(1920 * RS), int(1080 * RS), OUT + 'tn-tier2.png')
elif SHOT == 'tnshield':
    key = os.environ.get('FCODE', 'slip')
    o, top, cz = pylon(key, 2, health=.62)
    links_for(o, cz, ['crystal', 'alloy', 'data', 'credit'], rnd, dist=7.8, start=.2)
    dome((0, 0, 0), 4.6, lin(FACS[key]), 2.2)
    bpy.ops.mesh.primitive_uv_sphere_add(radius=.28, location=(-2.6, -3.0, 2.6)); ib = bpy.context.object; ib.data.materials.append(emit('impact', (1, .55, .15), 40))
    tn_lights(.45, lin(FACS[key]))
    cam((8.5, -12.5, 3.6), (0, 0, 2.6), lens=30)
    render_setup(int(1920 * RS), int(1080 * RS), OUT + 'tn-shield.png')
elif SHOT == 'tnline':
    keys = ['lumen', 'forge', 'veil', 'verdant', 'slip', 'bastion']
    for i, k in enumerate(keys):
        o, top, cz = pylon(k, 2, ox=(i - 2.5) * 8.0, beam=True)
    tn_lights(1.0, (0.6, 0.7, 1.0))
    cam((0, -46, 11), (0, 0, 3.4), lens=34)
    render_setup(int(2400 * RS), int(900 * RS), OUT + 'tn-six-factions.png')

elif SHOT == 'freecol':
    keys = ['lumen', 'free']
    for i, k in enumerate(keys):
        pylon(k, 2, ox=(i - .5) * 9.0)
    tn_lights(.8, (0.6, 0.7, 1.0))
    cam((0, -22, 6.5), (0, 0, 2.8), lens=34)
    render_setup(int(1920 * RS), int(1080 * RS), OUT + 'tn-freelancer-colours.png')

bpy.ops.render.render(write_still=True)
if SHOT == 'tn2':
    bpy.ops.export_scene.gltf(filepath=OUT + 'tether-node-t2.glb', export_format='GLB', use_selection=False, export_lights=False, export_cameras=False)
print('DONE', SHOT)
