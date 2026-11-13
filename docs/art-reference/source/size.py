# Size check: how big the nodes stand next to a 1.75 m person, now (0.245 within 10 m) and proposed.
import sys, os, math
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy
import nodes as N, credit as C


def person(x):
    m = N.hull('person', (0.12, 0.12, 0.14), .7, .1, 1)
    def cap(r, h, at):
        bpy.ops.mesh.primitive_cylinder_add(vertices=24, radius=r, depth=h, location=at); o = bpy.context.object
        o.data.materials.append(m); mod = o.modifiers.new('b', 'BEVEL'); mod.width = r * .95; mod.segments = 6; return o
    for s in (-1, 1):
        cap(.075, .86, (x + s * .1, 0, .43))
        cap(.055, .7, (x + s * .27, 0, 1.08))
    cap(.2, .66, (x, 0, 1.17))
    bpy.ops.mesh.primitive_uv_sphere_add(radius=.115, location=(x, 0, 1.63)); bpy.context.object.data.materials.append(m)


def place(build, x, s):
    before = set(bpy.data.objects); build()
    e = bpy.data.objects.new('grp', None); bpy.context.scene.collection.objects.link(e)
    for o in set(bpy.data.objects) - before:
        if o is not e and o.parent is None: o.parent = e
    e.scale = (s, s, s); e.location.x = x


def shot(scale, name, new_credit):
    N.SAMPLES = 48
    sc = N.reset(); N.world(sc); N.ground(sc, 160); N.lights(sc, (0.6, 0.6, 0.8), 1.4)
    person(-3.2)
    M = C.mats()
    builds = [lambda: N.node_crystal(0), lambda: N.node_alloy(0), lambda: N.node_data(0),
              (lambda: C.concept_a(M)) if new_credit else (lambda: N.node_credit(0))]
    for i, b in enumerate(builds):
        place(b, -1.6 + i * 2.1, scale)
    N.cam(sc, (0.4, -9.5, 2.4), (0.4, 0, .8), 35)
    N.render(sc, 1920, 900, os.path.join('out', name))


shot(0.245, 'size-now.jpg', False)
shot(0.6, 'size-proposed.jpg', True)
