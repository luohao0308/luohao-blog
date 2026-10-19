import { backgroundStudies, type BackgroundStudyId } from '~/data/backgroundLab'
import { paintKinetic, paintPapercut, paintPoster } from '~/utils/backgroundLabCompositions'
import { paintCity, paintCosmos, paintGarden } from '~/utils/backgroundLabWorlds'
import { paintFutureCity } from '~/utils/backgroundLabFutureCity'
import { paintKineticSculpture } from '~/utils/backgroundLabKineticUpgrade'
import { paintKineticPlayground } from '~/utils/backgroundLabKineticPlayground'
import { paintBalanceSeesaw, paintBalanceOrbit } from '~/utils/backgroundLabBalanceVariants'
import { paintMarbleTower, paintSwitchyard } from '~/utils/backgroundLabTrackVariants'
import { paintBusinessCity, paintHarborCity } from '~/utils/backgroundLabCityVariants'
import type { LabInteraction } from '~/utils/backgroundLabInteraction'
export interface LabPointer { x: number, y: number, strength: number }
type Context = CanvasRenderingContext2D
const tau = Math.PI * 2
const hash = (n: number) => { const value = Math.sin(n * 127.1 + 311.7) * 43758.5453; return value - Math.floor(value) }
function polygon(ctx: Context, points: number[][], fill: string, stroke?: string) {
  ctx.beginPath()
  points.forEach((point, index) => index ? ctx.lineTo(point[0]!, point[1]!) : ctx.moveTo(point[0]!, point[1]!))
  ctx.closePath(); ctx.fillStyle = fill; ctx.fill()
  if (stroke) { ctx.strokeStyle = stroke; ctx.lineWidth = .7; ctx.stroke() }
}
function blocks(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const unit = Math.min(w / 28, 34), rise = Math.min(h / 6, 100)
  const cubes = []
  for (let island = 0; island < 2; island++) for (let row = 0; row < 5; row++) for (let col = 0; col < 5; col++) {
    if (hash(row * 13 + col * 7 + island) < .13) continue
    cubes.push({ row, col, island })
  }
  cubes.sort((a, b) => a.row + a.col - b.row - b.col)
  for (const { row, col, island } of cubes) {
    const seed = row * 11 + col * 19 + island * 37
    const x = w * (island ? .85 : .18) + (col - row) * unit + pointer.x * 35
    const y = h * (island ? .58 : .52) + (col + row - 4) * unit * .5 + pointer.y * 18
    const height = 12 + hash(seed) * rise * .5 + (Math.sin(t * .95 - (row + col) * .5 + island * 2) * .5 + .5) * rise
    const v = unit * .5
    const warm = hash(seed + 9) > .65
    polygon(ctx, [[x - unit, y + v - height], [x, y + 2 * v - height], [x, y + 2 * v], [x - unit, y + v]], warm ? '#b17c62' : '#8b9ea8', '#f1f0ea80')
    polygon(ctx, [[x, y + 2 * v - height], [x + unit, y + v - height], [x + unit, y + v], [x, y + 2 * v]], warm ? '#cba98c' : '#b9c5c8', '#f1f0ea80')
    polygon(ctx, [[x, y - height], [x + unit, y + v - height], [x, y + 2 * v - height], [x - unit, y + v - height]], warm ? '#e0c8ab' : '#d8e0df', '#ffffffb0')
    if (height > 65) { ctx.strokeStyle = '#ffffff70'; ctx.beginPath(); ctx.moveTo(x, y + v * 2 - height + 8); ctx.lineTo(x + unit * .8, y + v * 1.2 - height + 8); ctx.stroke() }
  }
}
function grid(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const vx = w * .5 + pointer.x * w * .12, vy = h * .45 + pointer.y * h * .12
  ctx.lineWidth = 1
  for (let i = 0; i < 19; i++) {
    const depth = (i / 19 + t * .075) % 1, scale = depth * depth
    const rx = 10 + w * scale, ry = 6 + h * .85 * scale
    ctx.strokeStyle = `rgba(151,190,211,${.08 + depth * .3})`
    ctx.strokeRect(vx - rx, vy - ry, rx * 2, ry * 2)
  }
  ctx.strokeStyle = '#86a9bc30'
  for (let i = -8; i <= 8; i++) {
    ctx.beginPath(); ctx.moveTo(vx, vy); ctx.lineTo(w * .5 + i * w / 8, h); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(vx, vy); ctx.lineTo(w * .5 + i * w / 8, 0); ctx.stroke()
  }
  ctx.strokeStyle = '#d2b58b70'; ctx.beginPath(); ctx.moveTo(0, vy); ctx.lineTo(w, vy); ctx.stroke()
}
function orbits(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const cx = w * .52 + pointer.x * 18, cy = h * .52 + pointer.y * 14
  const nodes: { x: number, y: number, label: string }[] = []
  const labels = ['Go', 'Vue', 'Agent', 'Kratos', 'Nuxt', 'TypeScript', 'Redis', 'Docker']
  for (let ring = 0; ring < 4; ring++) {
    const a = Math.min(w * (.23 + ring * .075), h * (.4 + ring * .11))
    const b = a * (.35 + ring * .07), tilt = ring * .74 + pointer.x * .22
    const point = (angle: number) => ({ x: cx + Math.cos(angle) * a * Math.cos(tilt) - Math.sin(angle) * b * Math.sin(tilt), y: cy + Math.cos(angle) * a * Math.sin(tilt) + Math.sin(angle) * b * Math.cos(tilt) })
    ctx.strokeStyle = '#aab6c33a'; ctx.lineWidth = .8; ctx.setLineDash([2, 5]); ctx.beginPath()
    for (let j = 0; j <= 100; j++) { const p = point(j / 100 * tau); if (j) ctx.lineTo(p.x, p.y); else ctx.moveTo(p.x, p.y) }
    ctx.stroke(); ctx.setLineDash([])
    for (let j = 0; j < 2; j++) nodes.push({ ...point(t * (.18 + ring * .045) * (ring % 2 ? -1 : 1) + j * Math.PI + ring), label: labels[ring * 2 + j]! })
  }
  nodes.forEach((node, i) => {
    const next = nodes[(i + 2) % nodes.length]!
    ctx.strokeStyle = '#c4b38d30'; ctx.beginPath(); ctx.moveTo(node.x, node.y); ctx.lineTo(next.x, next.y); ctx.stroke()
    const distance = Math.hypot(node.x - (pointer.x + 1) * w / 2, node.y - (pointer.y + 1) * h / 2)
    const hover = Math.max(0, 1 - distance / 150) * pointer.strength
    ctx.fillStyle = i % 3 ? '#b0c7d2' : '#d6b67e'; ctx.beginPath(); ctx.arc(node.x, node.y, 3 + hover * 7, 0, tau); ctx.fill()
    ctx.strokeStyle = '#d6b67e80'; ctx.beginPath(); ctx.arc(node.x, node.y, 9 + hover * 12, 0, tau); ctx.stroke()
    ctx.fillStyle = '#d8e0e6'; ctx.font = '11px monospace'; ctx.fillText(node.label, node.x + 14, node.y - 12)
  })
}
function type(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const size = Math.max(70, Math.min(w * .14, 170))
  const lines = ['{ BUILD }  < CREATE />  ', 'GO / VUE / AGENT  ', 'const idea = code()  ']
  lines.forEach((line, row) => {
    ctx.save(); ctx.translate(w / 2, h * [.17, .84, 1.13][row]!); ctx.rotate((row % 2 ? .09 : -.1) + pointer.y * .09)
    ctx.font = `600 ${size}px ui-monospace, monospace`; ctx.textBaseline = 'middle'
    const length = ctx.measureText(line).width, direction = row % 2 ? -1 : 1
    const shift = (t * 58 * direction + pointer.x * 70) % length
    for (let copy = -2; copy <= 2; copy++) {
      const x = -length / 2 + copy * length + shift
      ctx.lineWidth = 1; ctx.strokeStyle = row === 1 ? '#a34e3775' : '#343b3b55'; ctx.strokeText(line, x, 0)
      if (row === 1) { ctx.fillStyle = '#a34e3718'; ctx.fillText(line, x, 0) }
    }
    ctx.restore()
  })
  ctx.fillStyle = '#7c827c'; ctx.font = '11px monospace'
  for (let i = 0; i < 6; i++) ctx.fillText(`${String(i + 1).padStart(2, '0')}  ${['func', 'type', 'return', 'async', 'const', 'export'][i]}`, 24, h * .34 + i * 24)
}
function folds(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const count = 11, unit = w / count
  const palette = ['#768e98', '#b4beb9', '#d7c3a7', '#e3d8c4', '#f5f1e8', '#ded3c0', '#ece1cf', '#d8b69c', '#c18f76', '#b1bdb9', '#8099a2']
  for (let i = 0; i < count; i++) {
    const angle = .2 + (Math.sin(t * .8 - i * .48 + pointer.x * .8) * .5 + .5) * 2.7
    const hinge = (i + .3) * unit, width = Math.cos(angle) * unit * 1.25
    const depth = Math.sin(angle) * unit * .33, top = h * .1 + Math.sin(i * .8) * 12 + pointer.y * 12
    const points = [[hinge, top], [hinge + width, top + depth], [hinge + width, h * .9 - depth], [hinge, h * .9]]
    polygon(ctx, points.map(p => [p[0]! + 7, p[1]! + 9]), '#5d4b3b0c')
    polygon(ctx, points, Math.cos(angle) > 0 ? palette[i]! : '#d7cebf', '#ffffff90')
    ctx.strokeStyle = '#776b5440'; ctx.beginPath(); ctx.moveTo(hinge, top); ctx.lineTo(hinge, h * .9); ctx.stroke()
    if (Math.abs(width) > 20) {
      ctx.strokeStyle = '#ffffff60'
      for (let line = 1; line < 6; line++) { const y = top + (h * .9 - top) * line / 6; ctx.beginPath(); ctx.moveTo(hinge + 3, y); ctx.lineTo(hinge + width - 3, y + depth * (1 - line / 3)); ctx.stroke() }
    }
  }
}
function pixels(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  const columns = 58, rows = 34, cellX = w / columns, cellY = h / rows
  const phase = (Math.sin(t * .6 - .4) + 1) / 2
  const blend = phase * phase * (3 - 2 * phase)
  const px = (pointer.x + 1) * w / 2, py = (pointer.y + 1) * h / 2
  for (let y = 0; y < rows; y++) for (let x = 0; x < columns; x++) {
    const seed = y * columns + x
    const bracket = (x >= 7 && x <= 9 && y >= 7 && y <= 26) || (x >= 7 && x <= 17 && (y >= 7 && y <= 9 || y >= 24 && y <= 26)) || (x >= 48 && x <= 50 && y >= 7 && y <= 26) || (x >= 40 && x <= 50 && (y >= 7 && y <= 9 || y >= 24 && y <= 26))
    if (!bracket) { if (seed % 5 === 0) { ctx.fillStyle = '#1f4f6410'; ctx.fillRect(x * cellX, y * cellY, 1.5, 1.5) } continue }
    let dx = (x + .5) * cellX + (1 - blend) * (hash(seed) - .5) * w * .75
    let dy = (y + .5) * cellY + (1 - blend) * (hash(seed + 17) - .5) * h * .8
    const distance = Math.hypot(dx - px, dy - py)
    const repel = Math.max(0, 1 - distance / 140) * 65 * pointer.strength
    dx += (dx - px) / Math.max(1, distance) * repel; dy += (dy - py) / Math.max(1, distance) * repel
    const size = Math.min(cellX, cellY) * (.25 + blend * .55)
    ctx.fillStyle = hash(seed + 7) > .85 ? '#bd8968' : `rgba(41,100,123,${.3 + blend * .5})`
    ctx.fillRect(dx - size / 2, dy - size / 2, size, size)
  }
}
/** Original abstract and scene studies. References inform mechanisms; no external source/assets are bundled. */
export function paintBackgroundStudy(ctx: Context, effect: BackgroundStudyId, w: number, h: number, time: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save(); ctx.fillStyle = backgroundStudies.find(study => study.id === effect)!.paper; ctx.fillRect(0, 0, w, h)
  const painters: Record<BackgroundStudyId, (ctx: Context, w: number, h: number, time: number, pointer: LabPointer, interaction?: LabInteraction) => void> = { blocks, grid, orbits, type, folds, pixels, kinetic: paintKinetic, papercut: paintPapercut, city: paintCity, poster: paintPoster, cosmos: paintCosmos, garden: paintGarden, 'kinetic-sculpture': paintKineticSculpture, 'city-future': paintFutureCity, 'kinetic-playground': paintKineticPlayground, 'balance-seesaw': paintBalanceSeesaw, 'balance-orbit': paintBalanceOrbit, 'marble-tower': paintMarbleTower, switchyard: paintSwitchyard, 'business-city': paintBusinessCity, 'harbor-city': paintHarborCity }
  painters[effect](ctx, w, h, time, pointer, interaction)
  ctx.restore()
}
