import type { LabPointer } from '~/utils/backgroundLab'

type Context = CanvasRenderingContext2D
const TAU = Math.PI * 2
function shape(ctx: Context, points: number[][], fill: string) {
  ctx.beginPath()
  points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!))
  ctx.closePath()
  ctx.fillStyle = fill
  ctx.fill()
}
function random(seed: number) { const value = Math.sin(seed * 127.1 + 19.7) * 43758.5453; return value - Math.floor(value) }

export function paintCity(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const scale = Math.min(w / 1050, h / 720) * (w < 600 ? 1.72 : 1.12)
  ctx.translate(w * .71 + pointer.x * 16, h * .57 + pointer.y * 9)
  ctx.scale(scale, scale)
  const iso = (x: number, y: number, z = 0) => [(x - y) * 42, (x + y) * 22 - z]
  const quad = (x: number, y: number, width: number, depth: number, z: number, color: string) => shape(ctx, [iso(x, y, z), iso(x + width, y, z), iso(x + width, y + depth, z), iso(x, y + depth, z)], color)
  // Broad layered ground plane, with paved streets between full city blocks.
  quad(-5.3, -5.3, 10.6, 10.6, -14, '#c4cec2')
  quad(-5.3, -5.3, 10.6, 10.6, -5, '#d5ddd1')
  for (let i = -5; i <= 5; i += 2) {
    quad(i - .22, -5.2, .44, 10.4, 0, '#b2c0b8')
    quad(-5.2, i - .22, 10.4, .44, 0, '#b2c0b8')
    for (let j = -5; j < 5; j++) {
      quad(i - .014, j + .2, .028, .28, .5, '#f4f0d8')
      quad(j + .2, i - .014, .28, .028, .5, '#f4f0d8')
    }
  }
  const buildings = []
  for (let y = -4; y <= 4; y += 2) for (let x = -4; x <= 4; x += 2) buildings.push({ x, y })
  buildings.sort((a, b) => a.x + a.y - b.x - b.y)
  for (const { x, y } of buildings) {
    const seed = (x + 5) * 17 + (y + 5) * 31
    const variation = random(seed)
    const height = 44 + variation * 115 + Math.sin(t * .8 + seed) * 6
    const size = 1.22
    const bx = x - .61, by = y - .61
    const warm = variation > .63
    quad(bx - .16, by - .16, size + .32, size + .32, 1, '#e9ebe1')
    shape(ctx, [iso(bx, by + size, height), iso(bx + size, by + size, height), iso(bx + size, by + size, 0), iso(bx, by + size, 0)], warm ? '#b58265' : '#78958b')
    shape(ctx, [iso(bx + size, by, height), iso(bx + size, by + size, height), iso(bx + size, by + size, 0), iso(bx + size, by, 0)], warm ? '#d2ac8b' : '#a9bcb0')
    quad(bx, by, size, size, height, warm ? '#ecdbc2' : '#dce4d7')
    quad(bx + .12, by + .12, .98, .98, height + 2, warm ? '#c4a187' : '#bfd0c1')
    quad(bx + .4, by + .3, .35, .3, height + 6, '#81968d')
    for (let floor = 12; floor < height - 12; floor += 15) for (let column = 0; column < 3; column++) {
      const lit = Math.sin(t * .75 + seed + floor + column * 3) > .35
      const color = lit ? '#f3dba1' : '#456b65'
      shape(ctx, [iso(bx + .16 + column * .34, by + size + .002, floor), iso(bx + .34 + column * .34, by + size + .002, floor), iso(bx + .34 + column * .34, by + size + .002, floor + 7), iso(bx + .16 + column * .34, by + size + .002, floor + 7)], color)
      shape(ctx, [iso(bx + size + .002, by + .16 + column * .34, floor), iso(bx + size + .002, by + .34 + column * .34, floor), iso(bx + size + .002, by + .34 + column * .34, floor + 7), iso(bx + size + .002, by + .16 + column * .34, floor + 7)], lit ? '#f9e8bc' : '#6e8c80')
    }
    // Street tree with a trunk and two rounded foliage layers.
    const tree = iso(bx - .28, by + 1.2)
    ctx.fillStyle = '#776f51'; ctx.fillRect(tree[0]! - 1.5, tree[1]! - 17, 3, 18)
    ctx.fillStyle = '#60836a'; ctx.beginPath(); ctx.ellipse(tree[0]!, tree[1]! - 25, 11, 16, -.2, 0, TAU); ctx.fill()
    ctx.fillStyle = '#91aa7d'; ctx.beginPath(); ctx.ellipse(tree[0]! - 3, tree[1]! - 29, 7, 11, -.2, 0, TAU); ctx.fill()
  }
  for (let i = 0; i < 6; i++) {
    const travel = ((t * (i % 2 ? .48 : -.38) + i * 2 + 50) % 10) - 5
    const lane = -5 + (i % 5) * 2
    const x = i % 2 ? travel : lane + .1
    const y = i % 2 ? lane - .1 : travel
    quad(x, y, .36, .18, 5, i % 3 ? '#eee9d6' : '#bf6f4f')
    shape(ctx, [iso(x, y + .18, 5), iso(x + .36, y + .18, 5), iso(x + .36, y + .18, 1), iso(x, y + .18, 1)], '#4e6660')
  }
  ctx.restore()
}

export function paintCosmos(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const gradient = ctx.createRadialGradient(w * .76, h * .4, 0, w * .76, h * .4, w * .85)
  gradient.addColorStop(0, '#23334a'); gradient.addColorStop(1, '#0c1426')
  ctx.fillStyle = gradient; ctx.fillRect(0, 0, w, h)
  for (let i = 0; i < 95; i++) {
    const rx = random(i + 23), ry = random(i * 13 + 3)
    const x = (rx * w + pointer.x * (i % 3 + 1) * 3 + w) % w
    const y = (ry * h + t * (i % 3 + 1) * .8 + h) % h
    ctx.globalAlpha = .2 + (.5 + .5 * Math.sin(t * .65 + i)) * .45
    ctx.fillStyle = '#e5e8ed'; ctx.beginPath(); ctx.arc(x, y, i % 8 ? .8 : 1.5, 0, TAU); ctx.fill()
  }
  ctx.globalAlpha = 1
  const radius = Math.min(w * .23, h * .29) * (w < 600 ? 1.7 : 1)
  const cx = w * .77 + pointer.x * 14, cy = h * .46 + Math.sin(t * .24) * h * .025 + pointer.y * 9
  const angle = t * .22 + 1.7
  const mx = cx + Math.cos(angle) * radius * 1.85, my = cy + Math.sin(angle) * radius * .72
  const moonRadius = radius * .13
  const drawMoon = () => {
    const moon = ctx.createRadialGradient(mx - moonRadius * .4, my - moonRadius * .4, 0, mx, my, moonRadius)
    moon.addColorStop(0, '#dddcd3'); moon.addColorStop(.6, '#969eab'); moon.addColorStop(1, '#34415a')
    ctx.fillStyle = moon; ctx.beginPath(); ctx.arc(mx, my, moonRadius, 0, TAU); ctx.fill()
  }
  if (Math.sin(angle) < 0) drawMoon()
  const ring = (front: boolean) => {
    ctx.save(); ctx.translate(cx, cy); ctx.rotate(-.3)
    for (let i = 0; i < 17; i++) {
      ctx.strokeStyle = `rgba(${i % 3 ? '192,180,158' : '223,211,179'},${i % 4 ? '.38' : '.7'})`
      ctx.lineWidth = radius * .018
      ctx.beginPath(); ctx.ellipse(0, 0, radius * (1.43 + i * .025), radius * (.35 + i * .006), 0, front ? 0 : Math.PI, front ? Math.PI : TAU); ctx.stroke()
    }
    ctx.restore()
  }
  ring(false)
  ctx.save(); ctx.beginPath(); ctx.arc(cx, cy, radius, 0, TAU); ctx.clip()
  const globe = ctx.createLinearGradient(cx - radius, cy - radius, cx + radius, cy + radius)
  globe.addColorStop(0, '#eee1b9'); globe.addColorStop(.35, '#bca78f'); globe.addColorStop(.65, '#857b80'); globe.addColorStop(1, '#253144')
  ctx.fillStyle = globe; ctx.fillRect(cx - radius, cy - radius, radius * 2, radius * 2)
  for (let i = 0; i < 25; i++) {
    const y = cy - radius + i * radius * .085
    ctx.strokeStyle = i % 3 ? '#6d666c38' : '#f1d9b74d'; ctx.lineWidth = radius * (.012 + (i % 4) * .007)
    ctx.beginPath(); ctx.moveTo(cx - radius, y); ctx.bezierCurveTo(cx - radius * .4, y + Math.sin(t * .26 + i) * radius * .09, cx + radius * .4, y + radius * .12, cx + radius, y - radius * .05); ctx.stroke()
  }
  const shade = ctx.createRadialGradient(cx - radius * .5, cy - radius * .5, radius * .1, cx - radius * .3, cy - radius * .4, radius * 1.65)
  shade.addColorStop(0, '#ffffff15'); shade.addColorStop(.55, '#00000000'); shade.addColorStop(1, '#071225e8')
  ctx.fillStyle = shade; ctx.fillRect(cx - radius, cy - radius, radius * 2, radius * 2); ctx.restore()
  ring(true)
  if (Math.sin(angle) >= 0) drawMoon()
  const smallX = w * .18 + pointer.x * 5, smallY = h * .82
  ctx.fillStyle = '#8295a74a'; ctx.beginPath(); ctx.arc(smallX, smallY, radius * .16, 0, TAU); ctx.fill()
  ctx.restore()
}

function leaf(ctx: Context, x: number, y: number, length: number, rotation: number, color: string) {
  ctx.save(); ctx.translate(x, y); ctx.rotate(rotation)
  ctx.beginPath(); ctx.moveTo(0, 0); ctx.bezierCurveTo(length * .4, -length * .42, length * .88, -length * .29, length, 0); ctx.bezierCurveTo(length * .6, length * .26, length * .22, length * .3, 0, 0)
  ctx.fillStyle = color; ctx.fill()
  ctx.strokeStyle = '#f0efd144'; ctx.lineWidth = .8; ctx.beginPath(); ctx.moveTo(0, 0); ctx.quadraticCurveTo(length * .5, -length * .04, length * .92, 0); ctx.stroke(); ctx.restore()
}

export function paintGarden(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const wash = ctx.createLinearGradient(0, 0, w, h)
  wash.addColorStop(0, '#edf0e3'); wash.addColorStop(1, '#d7dfca'); ctx.fillStyle = wash; ctx.fillRect(0, 0, w, h)
  const unit = Math.min(w / 1000, h / 700) * (w < 600 ? 1.65 : 1)
  ctx.translate(w * .73 + pointer.x * 8, h * 1.03)
  ctx.scale(unit, unit)
  // Complete plants share a root line; back stems stay paler, front stems articulate in wind.
  for (let plant = 0; plant < 8; plant++) {
    const height = 310 + (plant * 137 % 260)
    const rootX = (plant - 3.5) * 62
    const wind = Math.sin(t * .65 + plant * .7) * 19 + Math.sin(t * 1.1 + plant) * 6 + pointer.x * pointer.strength * 28
    const lean = (plant % 2 ? 1 : -1) * (45 + plant * 4) + wind
    const back = plant < 3
    ctx.globalAlpha = back ? .45 : .95
    ctx.strokeStyle = back ? '#82977a' : '#5c795b'; ctx.lineWidth = back ? 2 : 3
    ctx.beginPath(); ctx.moveTo(rootX, 15); ctx.bezierCurveTo(rootX - 22, -height * .3, rootX + lean, -height * .68, rootX + lean * .65, -height); ctx.stroke()
    for (let branch = 1; branch < 8; branch++) {
      const f = branch / 9
      const stemX = rootX + lean * Math.sin(f * 1.5) * .65 - 8 * Math.sin(f * Math.PI)
      const stemY = -height * f
      const side = branch % 2 ? -1 : 1
      const length = (60 + (plant * 13 + branch * 7) % 35) * (1 - f * .45)
      const angle = side < 0 ? -Math.PI + .38 + Math.sin(t * .8 + branch + plant) * .08 : -.48 + Math.sin(t * .8 + branch + plant) * .08
      leaf(ctx, stemX, stemY, length, angle + wind * .002, back ? '#8b9e79' : plant % 2 ? '#668460' : '#416a50')
    }
    const flowerX = rootX + lean * .65, flowerY = -height
    ctx.save(); ctx.translate(flowerX, flowerY); ctx.rotate(Math.sin(t * .5 + plant) * .08)
    const petalSize = 14 + (plant % 3) * 6
    for (let petal = 0; petal < 7; petal++) {
      const angle = petal / 7 * TAU + t * .016
      const bloom = 1 + Math.sin(t * .75 + plant) * .1
      ctx.save(); ctx.rotate(angle); ctx.scale(bloom, bloom)
      const petalGradient = ctx.createLinearGradient(0, 0, 0, -petalSize * 1.8)
      petalGradient.addColorStop(0, '#c6a66c'); petalGradient.addColorStop(1, plant % 3 ? '#f7edda' : '#c9957e')
      ctx.fillStyle = petalGradient; ctx.beginPath(); ctx.ellipse(0, -petalSize * .7, petalSize * .48, petalSize, -.15, 0, TAU); ctx.fill(); ctx.restore()
    }
    ctx.fillStyle = '#9d7848'; ctx.beginPath(); ctx.arc(0, 0, petalSize * .35, 0, TAU); ctx.fill()
    ctx.fillStyle = '#e0c487'; ctx.beginPath(); ctx.arc(-2, -2, petalSize * .18, 0, TAU); ctx.fill(); ctx.restore()
  }
  ctx.globalAlpha = 1
  // Low foreground leaves frame the scene without obscuring the introduction.
  for (let i = 0; i < 5; i++) leaf(ctx, -210 + i * 95, -12, 100 + i * 12, -1.8 + i * .28 + Math.sin(t * .55 + i) * .035, '#4b6f5355')
  ctx.restore()
}
