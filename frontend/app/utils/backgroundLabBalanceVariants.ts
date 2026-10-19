import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'

type Context = CanvasRenderingContext2D
const TAU = Math.PI * 2
function disc(ctx: Context, x: number, y: number, radius: number, color: string) {
  ctx.beginPath(); ctx.arc(x, y, radius, 0, TAU); ctx.fillStyle = color; ctx.fill()
}
function stroke(ctx: Context, points: number[][], color: string, width: number) {
  ctx.beginPath(); points.forEach((point, index) => index ? ctx.lineTo(point[0]!, point[1]!) : ctx.moveTo(point[0]!, point[1]!))
  ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = 'round'; ctx.stroke()
}
function ceramic(ctx: Context, x: number, y: number, width: number, height: number, color: string) {
  const fill = ctx.createLinearGradient(x, y, x + width, y + height)
  fill.addColorStop(0, '#f2e8d5'); fill.addColorStop(.22, color); fill.addColorStop(1, '#668c7d')
  ctx.fillStyle = fill; ctx.beginPath(); ctx.roundRect(x, y, width, height, Math.min(16, height * .3)); ctx.fill()
}
function world(ctx: Context, w: number, h: number, pointer: LabPointer) {
  const mobile = w < 600
  const scale = mobile ? w * .88 / 660 : Math.min(w * .63 / 660, h * .8 / 580)
  ctx.translate(w * (mobile ? .5 : .69) + pointer.x * 7, h * (mobile ? .64 : .55) + pointer.y * 5)
  ctx.scale(scale, scale)
  return scale
}

/** A broad horizontal balance responds to a sliding counterweight and a struck pan. */
export function paintBalanceSeesaw(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save()
  const scale = world(ctx, w, h, pointer)
  const slider = labTarget(interaction, 'bal-seesaw-weight', 'toggle')
  const pan = labTarget(interaction, 'bal-seesaw-pan', 'pendulum')
  const angle = -.055 + slider.position * .2 + pan.position * .16 + Math.sin(t * .45) * .012
  ctx.fillStyle = '#526e5712'; ctx.beginPath(); ctx.ellipse(0, 214, 250, 24, 0, 0, TAU); ctx.fill()
  ceramic(ctx, -100, 165, 200, 39, '#c6b697')
  ceramic(ctx, -33, 5, 66, 164, '#a6b29a')
  ctx.fillStyle = '#c7a77e'; ctx.beginPath(); ctx.moveTo(-66, 11); ctx.lineTo(0, -60); ctx.lineTo(66, 11); ctx.closePath(); ctx.fill()
  ctx.save(); ctx.translate(0, -25); ctx.rotate(angle)
  ceramic(ctx, -267, -14, 534, 28, '#a1815d')
  stroke(ctx, [[-247, -5], [245, -5]], '#eed9af', 2)
  for (let mark = -210; mark <= 210; mark += 30) stroke(ctx, [[mark, -13], [mark, mark % 60 ? -5 : 0]], '#71634f', 1)
  const weightX = -100 + slider.position * 190
  const weightWidth = Math.max(65, 44 / scale)
  ceramic(ctx, weightX - weightWidth / 2, -79, weightWidth, 65, '#b4c5a4')
  stroke(ctx, [[weightX - 14, -80], [weightX - 14, -95], [weightX + 14, -95], [weightX + 14, -80]], '#8f7353', 5)
  disc(ctx, weightX, -48, 12, '#d8ddc1')
  registerLabHit(ctx, interaction, { id: 'bal-seesaw-weight', label: '移动砝码：点击切换左右配重', kind: 'toggle', x: weightX, y: -50, radius: weightWidth * .58 })
  highlightLabHit(ctx, interaction, 'bal-seesaw-weight', weightX, -50, weightWidth * .58)
  const panX = 220, panRadius = Math.max(49, 24 / scale)
  stroke(ctx, [[panX, 13], [panX - panRadius, 86], [panX + panRadius, 86], [panX, 13]], '#a78c63', 3)
  ctx.fillStyle = '#759782'; ctx.beginPath(); ctx.ellipse(panX, 86, panRadius + 7, 18, 0, 0, Math.PI); ctx.fill()
  ctx.fillStyle = '#b6c7a5'; ctx.beginPath(); ctx.ellipse(panX, 85, panRadius + 7, 12, 0, 0, TAU); ctx.fill()
  disc(ctx, panX - 12, 73, 17, '#d3b184'); disc(ctx, panX + 12, 74, 13, '#ece0c8')
  registerLabHit(ctx, interaction, { id: 'bal-seesaw-pan', label: '平衡盘：点击拨动整根平衡梁', kind: 'pendulum', x: panX, y: 81, radius: panRadius + 8 })
  highlightLabHit(ctx, interaction, 'bal-seesaw-pan', panX, 81, panRadius + 8)
  ctx.restore()
  disc(ctx, 0, -25, 21, '#d6b286'); disc(ctx, 0, -25, 9, '#6a8b76')
  ctx.restore()
}

/** A tall dual-axis orbital sculpture: a moving gimbal and a separate hanging bob. */
export function paintBalanceOrbit(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save()
  const scale = world(ctx, w, h, pointer)
  const rotor = labTarget(interaction, 'bal-orbit-wheel', 'drive')
  const bob = labTarget(interaction, 'bal-orbit-bob', 'pendulum')
  ctx.fillStyle = '#526e5712'; ctx.beginPath(); ctx.ellipse(0, 225, 177, 21, 0, 0, TAU); ctx.fill()
  ceramic(ctx, -100, 187, 200, 30, '#c1b38f')
  ceramic(ctx, -20, 55, 40, 133, '#aab69b')
  // Open upright frame: one tall asymmetrical arch rather than a horizontal beam.
  ctx.beginPath(); ctx.moveTo(-139, 187); ctx.lineTo(-139, -113); ctx.bezierCurveTo(-139, -241, 137, -241, 137, -113); ctx.lineTo(137, 187)
  ctx.strokeStyle = '#9b7954'; ctx.lineWidth = 14; ctx.stroke()
  ctx.beginPath(); ctx.moveTo(-141, 184); ctx.lineTo(-141, -113); ctx.bezierCurveTo(-141, -245, 134, -245, 134, -113)
  ctx.strokeStyle = '#e0c292'; ctx.lineWidth = 3; ctx.stroke()
  const orbitRadius = 103, spin = t * .3 + rotor.phase
  ctx.save(); ctx.translate(0, -60)
  ctx.rotate(Math.sin(t * .27) * .13 + pointer.x * .07)
  ctx.strokeStyle = '#aac0a6'; ctx.lineWidth = 17; ctx.beginPath(); ctx.ellipse(0, 0, orbitRadius, 126, 0, 0, TAU); ctx.stroke()
  ctx.strokeStyle = '#e0e4cc'; ctx.lineWidth = 3; ctx.beginPath(); ctx.ellipse(-2, -2, orbitRadius, 126, 0, 0, TAU); ctx.stroke()
  ctx.save(); ctx.rotate(spin)
  ctx.strokeStyle = '#a58259'; ctx.lineWidth = 9; ctx.beginPath(); ctx.ellipse(0, 0, orbitRadius - 8, 47, .55, 0, TAU); ctx.stroke()
  stroke(ctx, [[-87, -49], [87, 49]], '#b99c72', 5)
  const satelliteRadius = Math.max(34, 23 / scale)
  disc(ctx, orbitRadius - 9, 0, satelliteRadius, '#b58f64'); disc(ctx, orbitRadius - 16, -8, satelliteRadius * .59, '#dfc396')
  registerLabHit(ctx, interaction, { id: 'bal-orbit-wheel', label: '轨道配重：点击加速内环旋转', kind: 'drive', x: orbitRadius - 9, y: 0, radius: satelliteRadius + 4 })
  highlightLabHit(ctx, interaction, 'bal-orbit-wheel', orbitRadius - 9, 0, satelliteRadius + 4)
  ctx.restore(); ctx.restore()
  // The second articulated element hangs below the ring on its own pivot.
  ctx.save(); ctx.translate(0, 19); ctx.rotate(bob.position * .85 + Math.sin(t * .55) * .025)
  const bobRadius = Math.max(37, 23 / scale)
  stroke(ctx, [[0, 0], [0, 107]], '#9b7955', 5)
  ceramic(ctx, -bobRadius, 92, bobRadius * 2, bobRadius * 1.35, '#8aa38c')
  disc(ctx, 0, 92 + bobRadius * .67, bobRadius * .3, '#d4c39c')
  registerLabHit(ctx, interaction, { id: 'bal-orbit-bob', label: '垂直摆锤：点击拨动下方独立摆', kind: 'pendulum', x: 0, y: 92 + bobRadius * .67, radius: bobRadius })
  highlightLabHit(ctx, interaction, 'bal-orbit-bob', 0, 92 + bobRadius * .67, bobRadius)
  ctx.restore()
  disc(ctx, 0, 19, 13, '#c9ae7d')
  ctx.restore()
}
