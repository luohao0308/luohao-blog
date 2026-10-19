import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'

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

export function paintFutureCity(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save()
  const mobile = w < 600
  const scale = mobile ? w * .89 / 941 : Math.min(w * .64 / 941, h * .82 / 620)
  ctx.translate(w * (mobile ? .5 : .69) + pointer.x * (mobile ? 2 : 10), h * (mobile ? .65 : .61) + pointer.y * 6)
  ctx.scale(scale, scale)
  const iso = (x: number, y: number, z = 0) => [(x - y) * 42, (x + y) * 22 - z]
  const quad = (x: number, y: number, width: number, depth: number, z: number, color: string) => shape(ctx, [iso(x, y, z), iso(x + width, y, z), iso(x + width, y + depth, z), iso(x, y + depth, z)], color)
  // Broad layered ground plane, with paved streets between full city blocks.
  quad(-5.6, -5.6, 11.2, 11.2, -14, '#c4cec2')
  quad(-5.6, -5.6, 11.2, 11.2, -5, '#d5ddd1')
  for (let i = -5.2; i <= 5.2; i += 2.6) {
    quad(i - .28, -5.5, .56, 11, 0, '#b2c0b8')
    quad(-5.5, i - .28, 11, .56, 0, '#b2c0b8')
    for (let j = -5; j < 5; j++) {
      quad(i - .014, j + .2, .028, .28, .5, '#f4f0d8')
      quad(j + .2, i - .014, .28, .028, .5, '#f4f0d8')
    }
  }
  // The original city geometry and palette are retained; wider blocks open a civic square.
  quad(-2.13, -2.13, 1.66, 1.66, 1, '#e9e5d4')
  quad(-1.94, -1.94, 1.28, 1.28, 2, '#bdcfac')
  quad(-1.64, -1.64, .68, .68, 3, '#aecac0')
  const buildings = []
  for (const y of [-3.9, -1.3, 1.3, 3.9]) for (const x of [-3.9, -1.3, 1.3, 3.9]) if (Math.abs(x) > 2 || Math.abs(y) > 2) buildings.push({ x, y })
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
    const lane = -5.2 + (i % 5) * 2.6
    const x = i % 2 ? travel : lane + .1
    const y = i % 2 ? lane - .1 : travel
    quad(x, y, .36, .18, 5, i % 3 ? '#eee9d6' : '#bf6f4f')
    shape(ctx, [iso(x, y + .18, 5), iso(x + .36, y + .18, 5), iso(x + .36, y + .18, 1), iso(x, y + .18, 1)], '#4e6660')
  }
  // The same sand-table model has three directly operable pieces.
  const rooftopSeed = 3.7 * 17 + 1.1 * 31
  const rooftopHeight = 44 + random(rooftopSeed) * 115 + Math.sin(t * .8 + rooftopSeed) * 6
  const fan = labTarget(interaction, 'city-fan', 'drive')
  const fanCenter = iso(-1.3, -3.9, rooftopHeight + 28)
  const fanRadius = Math.max(25, 17 / scale)
  ctx.strokeStyle = '#7e9487'; ctx.lineWidth = 3
  ctx.beginPath(); ctx.moveTo(...iso(-1.3, -3.9, rooftopHeight + 5) as [number, number]); ctx.lineTo(fanCenter[0]!, fanCenter[1]!); ctx.stroke()
  ctx.save(); ctx.translate(fanCenter[0]!, fanCenter[1]!)
  ctx.fillStyle = '#dce4d7'; ctx.beginPath(); ctx.arc(0, 0, fanRadius, 0, TAU); ctx.fill()
  ctx.strokeStyle = '#8da498'; ctx.lineWidth = 2.5; ctx.stroke()
  ctx.rotate(t * .65 + fan.phase)
  for (let blade = 0; blade < 3; blade++) {
    ctx.save(); ctx.rotate(blade * TAU / 3)
    shape(ctx, [[0, 0], [fanRadius * .28, -fanRadius * .32], [fanRadius * .83, -fanRadius * .25], [fanRadius * .88, fanRadius * .02], [fanRadius * .25, fanRadius * .2]], '#6e9388')
    ctx.restore()
  }
  ctx.fillStyle = '#cba98c'; ctx.beginPath(); ctx.arc(0, 0, fanRadius * .19, 0, TAU); ctx.fill(); ctx.restore()
  registerLabHit(ctx, interaction, { id: 'city-fan', label: '楼顶风机：点击加速', kind: 'drive', x: fanCenter[0]!, y: fanCenter[1]!, radius: fanRadius })
  highlightLabHit(ctx, interaction, 'city-fan', fanCenter[0]!, fanCenter[1]!, fanRadius)

  const elevator = labTarget(interaction, 'city-elevator', 'toggle')
  const elevatorSeed = 8.9 * 17 + 6.3 * 31
  const elevatorHeight = 44 + random(elevatorSeed) * 115 + Math.sin(t * .8 + elevatorSeed) * 6
  const railTop = iso(4.54, 1.6, elevatorHeight - 4), railBottom = iso(4.54, 1.6, 5)
  ctx.strokeStyle = '#e7ead8'; ctx.lineWidth = 5
  ctx.beginPath(); ctx.moveTo(railTop[0]!, railTop[1]!); ctx.lineTo(railBottom[0]!, railBottom[1]!); ctx.stroke()
  const cabin = iso(4.54, 1.6, 20 + elevator.position * Math.max(50, elevatorHeight - 38))
  const cabinWidth = Math.max(24, 25 / scale), cabinHeight = Math.max(28, 28 / scale)
  ctx.fillStyle = '#ddc6a0'; ctx.fillRect(cabin[0]! - cabinWidth / 2, cabin[1]! - cabinHeight / 2, cabinWidth, cabinHeight)
  ctx.fillStyle = '#789c93'; ctx.fillRect(cabin[0]! - cabinWidth * .39, cabin[1]! - cabinHeight * .37, cabinWidth * .78, cabinHeight * .7)
  ctx.strokeStyle = '#e6e8d1'; ctx.lineWidth = 2
  ctx.beginPath(); ctx.moveTo(cabin[0]!, cabin[1]! - cabinHeight * .35); ctx.lineTo(cabin[0]!, cabin[1]! + cabinHeight * .33); ctx.stroke()
  registerLabHit(ctx, interaction, { id: 'city-elevator', label: '楼侧升降舱：点击升降', kind: 'toggle', x: cabin[0]!, y: cabin[1]!, radius: cabinWidth * .65 })
  highlightLabHit(ctx, interaction, 'city-elevator', cabin[0]!, cabin[1]!, cabinWidth * .65)

  const bus = labTarget(interaction, 'city-bus', 'toggle')
  const busCenter = iso(-2.7 + bus.position * 4.9, 5.2, 9)
  const busWidth = Math.max(58, 46 / scale), busHeight = Math.max(22, 25 / scale)
  ctx.save(); ctx.translate(busCenter[0]!, busCenter[1]!); ctx.rotate(Math.atan2(22, 42))
  ctx.fillStyle = '#c99f76'; ctx.beginPath(); ctx.roundRect(-busWidth / 2, -busHeight / 2, busWidth, busHeight, 4); ctx.fill()
  ctx.fillStyle = '#e3d5b5'; ctx.fillRect(-busWidth * .45, -busHeight * .46, busWidth * .9, busHeight * .15)
  ctx.fillStyle = '#577b72'; ctx.fillRect(-busWidth * .38, -busHeight * .23, busWidth * .66, busHeight * .46)
  for (let pane = 0; pane < 3; pane++) { ctx.fillStyle = '#d6c8a8'; ctx.fillRect(-busWidth * .22 + pane * busWidth * .18, -busHeight * .24, 2, busHeight * .48) }
  ctx.fillStyle = '#46675c'; ctx.fillRect(-busWidth * .31, busHeight * .4, 7, 4); ctx.fillRect(busWidth * .24, busHeight * .4, 7, 4)
  ctx.restore()
  registerLabHit(ctx, interaction, { id: 'city-bus', label: '街区巴士：点击开往下一站或返回', kind: 'toggle', x: busCenter[0]!, y: busCenter[1]!, radius: busWidth * .54 })
  highlightLabHit(ctx, interaction, 'city-bus', busCenter[0]!, busCenter[1]!, busWidth * .54)
  ctx.restore()
}
