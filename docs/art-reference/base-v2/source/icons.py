import math, random
from PIL import Image, ImageDraw, ImageFont
FAC=(181,124,255)
SLOTS={1:(4,0,0),2:(8,0,0),3:(8,4,0),4:(8,4,6),5:(8,4,12)}
WALL_R=[14,20,27,33]; NR={1:1,2:1,3:2,4:3,5:4}; BUILT=[1,3,4,5]
PYL={1:0,2:2,3:3,4:4,5:6}; ORDER=[0,3,1,4,2,5]; GATE=4
def hexagon(cx,cy,R,rot=math.pi/6): return [(cx+R*math.cos(rot+k*math.pi/3),cy-R*math.sin(rot+k*math.pi/3)) for k in range(6)]
def col(a): return tuple(int(c*a+14*(1-a)) for c in FAC)
F='/usr/share/fonts/truetype/dejavu/DejaVuSans'
S=4
def fonts(s): return ImageFont.truetype(F+'-Bold.ttf',22*s), ImageFont.truetype(F+'.ttf',17*s)
names=['SPIRE','OUTPOST','STRONGHOLD','CITADEL','CAPITAL']
def layout(t):
    towers=[(0,0,'main')]
    for j in range(PYL[t]):
        a=ORDER[j]*math.pi/3; towers.append((6.2*math.cos(a),6.2*math.sin(a),'pylon'))
    for k in range(NR[t]):
        R=WALL_R[k]; cs=[0,2,4] if (k==0 and t==1) else range(6)
        for c in cs:
            a=math.pi/6+c*math.pi/3; towers.append((R*math.cos(a),R*math.sin(a),'bastion' if k==3 else 'wall'))
        if t==5 and k==2:
            for e in range(6):
                if e==GATE: continue
                a=math.pi/3+e*math.pi/3; ap=R*.866; towers.append((ap*math.cos(a),ap*math.sin(a),'wall'))
    mods=[]
    c1=SLOTS[t][0]
    for i in range(c1): mods.append((1,i,i/c1*math.tau+.2,9.6))
    if SLOTS[t][1]:
        for i,d in enumerate([0,60,180,240]): mods.append((2,i,math.radians(d),16.0))
    c3=SLOTS[t][2]
    if c3:
        angs=[e*60 for e in range(6)] if c3==6 else [e*60+sd*14 for e in range(6) for sd in (-1,1)]
        for i,d in enumerate(angs): mods.append((3,i,math.radians(d),22.6))
    out=[]
    for ring,i,a,R in mods:
        mk=max(1,min(4,t-ring+1-(i%3==2)))
        if t==1: mk=1+i%2
        out.append((R*math.cos(a),R*math.sin(a),mk))
    return towers,out
def draw_base(d,cx,cy,t,sc,S):
    towers,mods=layout(t)
    outer=WALL_R[NR[t]-1]+1.6
    d.ellipse([cx-outer*sc*1.25,cy-outer*sc*1.25,cx+outer*sc*1.25,cy+outer*sc*1.25],fill=col(.09),outline=col(.45),width=2*S)
    for k in range(NR[t]-1,-1,-1):
        d.polygon(hexagon(cx,cy,(WALL_R[k]+1.6)*sc),fill=col(.10+.05*(NR[t]-k)))
    for k in range(NR[t]):  # walls with gate gap
        R=WALL_R[k]
        for e in range(6):
            a1=math.pi/6+e*math.pi/3; a2=a1+math.pi/3
            p1=(cx+R*sc*math.cos(a1),cy-R*sc*math.sin(a1)); p2=(cx+R*sc*math.cos(a2),cy-R*sc*math.sin(a2))
            if e==GATE:
                L=R*sc; g=2.6*sc/L
                q1=(p1[0]+(p2[0]-p1[0])*(.5-g),p1[1]+(p2[1]-p1[1])*(.5-g)); q2=(p1[0]+(p2[0]-p1[0])*(.5+g),p1[1]+(p2[1]-p1[1])*(.5+g))
                d.line([p1,q1],fill=col(1),width=int(1.1*sc)); d.line([q2,p2],fill=col(1),width=int(1.1*sc))
            else: d.line([p1,p2],fill=col(1),width=int(1.1*sc))
    for x,y,mk in mods:
        d.polygon(hexagon(cx+x*sc,cy-y*sc,[1.3,1.7,2.1,2.5][mk-1]*sc),fill=col(1),outline=(255,255,255),width=max(1,S//2))
    for x,y,kind in towers:
        r={'main':3.6+.35*t,'pylon':1.9,'wall':1.7,'bastion':2.2}[kind]*sc
        if kind=='main':
            d.polygon(hexagon(cx,cy,r),fill=(30,30,44),outline=(255,255,255),width=3*S); d.ellipse([cx-1.1*sc,cy-1.1*sc,cx+1.1*sc,cy+1.1*sc],fill=col(1))
        else:
            d.polygon(hexagon(cx+x*sc,cy-y*sc,r),fill=(78,70,110),outline=(235,230,255),width=max(1,S))
    return len(towers),len(mods)
W,H=2400,800
im=Image.new('RGB',(W*S,H*S),(14,16,24)); d=ImageDraw.Draw(im); f,f2=fonts(S)
r=random.Random(2)
for i in range(40):
    x=r.randint(0,W*S); d.line([(x,0),(x+r.randint(-400,400),H*S)],fill=(26,29,40),width=7*S)
for i in range(14):
    y=r.randint(0,H*S); d.line([(0,y),(W*S,y+r.randint(-200,200))],fill=(26,29,40),width=9*S)
for t in range(1,6):
    cx=(240+(t-1)*480)*S; cy=360*S
    nt,nm=draw_base(d,cx,cy,t,4.6*S,S)
    lab='T%d %s'%(t,names[t-1]); w=d.textlength(lab,font=f); d.text((cx-w/2,cy+215*S),lab,font=f,fill=(230,232,240))
    sub='%d towers + %d module towers'%(nt,nm); w=d.textlength(sub,font=f2); d.text((cx-w/2,cy+248*S),sub,font=f2,fill=(150,155,175))
d.text((30*S,24*S),'Map, close zoom: drawn to scale from the server layout. Walls in faction colour, towers pale hexes, modules lit hexes (bigger = higher Mk).',font=f2,fill=(170,175,195))
im.resize((W,H),Image.LANCZOS).save('out/map-close.png')
# far-zoom icons: hex outline rings = wall rings, dots = towers
S2=4; im2=Image.new('RGB',(1200*S2,280*S2),(14,16,24)); d=ImageDraw.Draw(im2); f,f2=fonts(S2)
for t in range(1,6):
    cx=(120+(t-1)*240)*S2; cy=115*S2; R=(30+8*t)*S2
    d.polygon(hexagon(cx,cy,R),fill=col(.22))
    for k in range(NR[t]):
        rr=R*(k+1)/NR[t]
        d.polygon(hexagon(cx,cy,rr),outline=col(1),width=3*S2)
        cs=[0,2,4] if (k==0 and t==1) else range(6)
        for c in cs:
            a=math.pi/6+c*math.pi/3; d.ellipse([cx+rr*math.cos(a)-5*S2,cy-rr*math.sin(a)-5*S2,cx+rr*math.cos(a)+5*S2,cy-rr*math.sin(a)+5*S2],fill=(235,230,255))
    d.polygon(hexagon(cx,cy,R*.22 if NR[t]>1 else R*.3),fill=(30,30,44),outline=(255,255,255),width=3*S2)
    w=d.textlength('T%d'%t,font=f); d.text((cx-w/2,cy+R+14*S2),'T%d'%t,font=f,fill=(230,232,240))
im2.resize((1200,280),Image.LANCZOS).save('out/map-icons.png')
