import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'
type Context = CanvasRenderingContext2D
const tau = Math.PI * 2
function shape(ctx: Context, points: number[][], fill: string) {
  ctx.beginPath(); points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!)); ctx.closePath(); ctx.fillStyle = fill; ctx.fill()
}
function disc(ctx: Context, x: number, y: number, r: number, light: string, dark: string) {
  const gradient = ctx.createRadialGradient(x - r * .35, y - r * .4, r * .05, x + r * .2, y + r * .1, r)
  gradient.addColorStop(0, light); gradient.addColorStop(1, dark)
  ctx.fillStyle = gradient; ctx.beginPath(); ctx.arc(x, y, r, 0, tau); ctx.fill()
}
function joint(ctx: Context, x: number, y: number, radius = 6) {
  disc(ctx, x, y, radius, '#f5e8bb', '#9c8c5e'); disc(ctx, x, y, radius * .26, '#65756b', '#33473f')
}
function wheel(ctx: Context, x: number, y: number, r: number, teeth: number, angle: number, ivory: boolean) {
  ctx.save(); ctx.translate(x, y); ctx.rotate(angle)
  const metal = ctx.createLinearGradient(-r, -r, r, r)
  metal.addColorStop(0, '#d7bd7d'); metal.addColorStop(.4, '#baa06c'); metal.addColorStop(.65, '#d0b47a'); metal.addColorStop(1, '#7c6b44')
  ctx.beginPath()
  for (let i = 0; i < teeth * 4; i++) { const a = i / (teeth * 4) * tau, radius = i % 4 < 2 ? r : r * .91; if (i) ctx.lineTo(Math.cos(a) * radius, Math.sin(a) * radius); else ctx.moveTo(radius, 0) }
  ctx.closePath(); ctx.fillStyle = metal; ctx.fill(); ctx.strokeStyle = '#f5e8b980'; ctx.lineWidth = .7; ctx.stroke()
  disc(ctx, 0, 0, r * .8, ivory ? '#f2eedb' : '#9eb4a4', ivory ? '#c9c7ab' : '#4f6d62')
  for (let i = 0; i < 5; i++) {
    const a = i / 5 * tau
    ctx.save(); ctx.translate(Math.cos(a) * r * .46, Math.sin(a) * r * .46); ctx.rotate(a)
    ctx.fillStyle = '#29483c'; ctx.beginPath(); ctx.ellipse(0, 0, r * .11, r * .22, .25, 0, tau); ctx.fill(); ctx.strokeStyle = '#fbf4d25c'; ctx.stroke(); ctx.restore()
  }
  ctx.strokeStyle = '#9a946a'; ctx.beginPath(); ctx.arc(0, 0, r * .72, 0, tau); ctx.stroke()
  joint(ctx, 0, 0, r * .17)
  ctx.restore()
}
function rod(ctx: Context, ax: number, ay: number, bx: number, by: number, width = 8) {
  ctx.lineCap = 'round'; ctx.strokeStyle = '#324d41'; ctx.lineWidth = width; ctx.beginPath(); ctx.moveTo(ax, ay); ctx.lineTo(bx, by); ctx.stroke(); ctx.strokeStyle = '#bac7ac'; ctx.lineWidth = Math.max(1, width * .23); ctx.stroke()
}
/** Original causal mechanism: motor → belt → meshed shaft → cam → carriage and counterbalanced arm. */
export function paintKineticSculpture(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save()
  const scale = w < 600 ? w * .88 / 840 : Math.min(w / 1300, h / 710) * 1.02
  const centerX = w < 600 ? w * .5 - 100 * scale : Math.min(w * .66, w - 470 * scale - 28)
  ctx.translate(centerX + pointer.x * (w < 600 ? 5 : 16), h * (w < 600 ? .575 : .52) + pointer.y * 9); ctx.scale(scale, scale)
  ctx.transform(1, pointer.x * .015, pointer.x * .04, 1, 0, 0)
  const cycle = Math.floor(t / 8), fraction = Math.min(1, t % 8 / 6)
  const eased = fraction * fraction * (3 - 2 * fraction)
  const drive = labTarget(interaction, 'motor-wheel', 'drive')
  const ball = labTarget(interaction, 'balance-ball', 'pendulum')
  const phase = (cycle + eased) * tau + pointer.x * pointer.strength * .13 + drive.phase
  const cam = -phase * 110 / 72
  const ballRadius = Math.max(31, 24 / scale)
  // Soft contact shadow and a floating, bevelled ceramic plinth.
  ctx.save(); ctx.translate(0, 235); ctx.scale(1, .19)
  const shadow = ctx.createRadialGradient(-5, 0, 25, 0, 0, 330); shadow.addColorStop(0, '#29453448'); shadow.addColorStop(1, '#29453400'); ctx.fillStyle = shadow; ctx.beginPath(); ctx.arc(0, 0, 330, 0, tau); ctx.fill(); ctx.restore()
  shape(ctx, [[-270, 175], [220, 175], [285, 208], [-205, 208]], '#eee8d4')
  shape(ctx, [[-205, 208], [285, 208], [285, 231], [-205, 231]], '#c6ccb5')
  shape(ctx, [[-270, 175], [-205, 208], [-205, 231], [-270, 198]], '#a7b7a0')
  ctx.strokeStyle = '#fff9de'; ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(-263, 176); ctx.lineTo(220, 176); ctx.lineTo(279, 207); ctx.stroke()
  const frame = (offset: number, color: string, width: number) => {
    ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = 'round'; ctx.beginPath(); ctx.moveTo(-160 + offset, 180); ctx.lineTo(-160 + offset, -147); ctx.bezierCurveTo(-160 + offset, -279, 210 + offset, -279, 210 + offset, -147); ctx.lineTo(210 + offset, 180); ctx.stroke()
  }
  frame(9, '#29463724', 15); frame(0, '#49614f', 13); frame(-3, '#9bab87', 3)
  // Rails and mounts remain visible behind moving parts.
  rod(ctx, 119, -185, 119, 44, 5); rod(ctx, 142, -185, 142, 44, 5)
  rod(ctx, -116, 175, -116, -15, 10); rod(ctx, 44, 175, 44, -27, 8)
  ctx.fillStyle = '#8a9b7e'; ctx.fillRect(106, -191, 49, 11); ctx.fillRect(106, 37, 49, 11)
  // A slower rear transmission plane carries torque up through a second belt, behind the main rotor.
  ctx.save(); ctx.globalAlpha = .82
  rod(ctx, -124, 160, -124, -172, 9); rod(ctx, -24, 160, -24, -172, 7)
  for (const side of [-1, 1]) rod(ctx, -70 + side * 28, 22, -85 + side * 54, -155, 4)
  wheel(ctx, -85, -155, 74, 32, phase * .5, false)
  disc(ctx, -85, -155, 23, '#f0e8cb', '#9aab8e')
  ctx.strokeStyle = '#d6bb7d'; ctx.lineWidth = 2; ctx.beginPath(); ctx.arc(-85, -155, 53, phase * .5, phase * .5 + 1.4); ctx.stroke()
  ctx.restore()
  // Drive unit with a ceramic motor housing and bronze end plate.
  const housing = ctx.createLinearGradient(0, 70, 0, 132); housing.addColorStop(0, '#d3aa83'); housing.addColorStop(.5, '#b1805e'); housing.addColorStop(1, '#876449')
  ctx.fillStyle = '#4a614e'; ctx.fillRect(-260, 145, 107, 17); ctx.fillStyle = housing; ctx.fillRect(-255, 84, 92, 62)
  disc(ctx, -209, 115, 32, '#e1c99f', '#9a7750')
  for (let line = 0; line < 6; line++) { ctx.strokeStyle = '#e9c49960'; ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(-251 + line * 6, 90); ctx.lineTo(-251 + line * 6, 138); ctx.stroke() }
  ctx.fillStyle = '#e6d9b7'; ctx.fillRect(-248, 147, 73, 11); ctx.fillStyle = '#51674d'; ctx.font = '6px monospace'; ctx.fillText('DRIVE / SPRING', -244, 155)
  const x1 = -209, y1 = 115, r1 = Math.max(36, 24 / scale), x2 = -70, y2 = 22, r2 = 110
  const direction = Math.atan2(y2 - y1, x2 - x1), tangent = Math.acos((r1 - r2) / Math.hypot(x2 - x1, y2 - y1))
  for (const normal of [direction + tangent, direction - tangent]) {
    const ax = x1 + Math.cos(normal) * r1, ay = y1 + Math.sin(normal) * r1, bx = x2 + Math.cos(normal) * r2, by = y2 + Math.sin(normal) * r2
    rod(ctx, ax, ay, bx, by, 5)
    for (let mark = 0; mark < 4; mark++) { const u = ((phase / tau + mark / 4) % 1 + 1) % 1; ctx.fillStyle = '#d5b570'; ctx.beginPath(); ctx.arc(ax + (bx - ax) * u, ay + (by - ay) * u, 1.7, 0, tau); ctx.fill() }
  }
  wheel(ctx, x1, y1, r1, 16, phase * r2 / r1, false)
  wheel(ctx, x2, y2, r2, 48, phase, true)
  wheel(ctx, 100, -45, 72, 32, cam, false)
  // Eccentric pin traces the shaft phase; its rod lifts a carriage.
  ctx.strokeStyle = '#a9784d'; ctx.lineWidth = 8; ctx.beginPath(); ctx.arc(100, -45, 36, 0, tau); ctx.stroke()
  const pinX = 100 + Math.cos(cam) * 35, pinY = -45 + Math.sin(cam) * 35
  const armAngle = -.12 + Math.sin(cam) * .22
  const leftX = 205 - Math.cos(armAngle) * 84, leftY = -145 - Math.sin(armAngle) * 84
  const rightX = 205 + Math.cos(armAngle) * 84, rightY = -145 + Math.sin(armAngle) * 84
  ctx.fillStyle = '#e4dfc6'; ctx.fillRect(108, leftY - 14, 45, 28); ctx.fillStyle = '#b99a62'; ctx.fillRect(112, leftY - 11, 4, 22); ctx.fillRect(145, leftY - 11, 4, 22)
  rod(ctx, pinX, pinY, leftX, leftY, 8); joint(ctx, pinX, pinY, 7); joint(ctx, leftX, leftY, 6)
  // A broad counterbalanced arm is the sculptural silhouette, with paired hanging ceramic masses.
  rod(ctx, leftX - 15, leftY, rightX, rightY, 10); joint(ctx, 205, -145, 10)
  const ballX = rightX + Math.sin(ball.position) * 125, ballY = rightY + Math.cos(ball.position) * 125
  rod(ctx, rightX, rightY, ballX, ballY - 20, 1.4)
  disc(ctx, ballX + 3, ballY + 4, ballRadius + 1, '#29463815', '#29463800')
  disc(ctx, ballX, ballY, ballRadius, '#faf3dc', '#c0c8ad')
  ctx.strokeStyle = '#b29a65'; ctx.lineWidth = 1.2; ctx.beginPath(); ctx.ellipse(ballX, ballY, ballRadius + 2, ballRadius * .28, -.2 + ball.position, 0, tau); ctx.stroke()
  registerLabHit(ctx, interaction, { id: 'balance-ball', kind: 'pendulum', label: '平衡球：点击推动，或横向拖动后释放', x: ballX, y: ballY, radius: ballRadius + 4, anchorX: rightX, anchorY: rightY, ropeLength: 125 })
  registerLabHit(ctx, interaction, { id: 'motor-wheel', kind: 'drive', label: '动力轮：点击加速，随后缓慢回稳', x: x1, y: y1, radius: r1 + 4 })
  highlightLabHit(ctx, interaction, 'balance-ball', ballX, ballY, ballRadius + 4)
  highlightLabHit(ctx, interaction, 'motor-wheel', x1, y1, r1 + 4)
  // The other output is a small suspended tray rising out of the same cam cycle.
  const trayY = 105 + Math.sin(cam) * 25
  rod(ctx, leftX, leftY + 14, 130, trayY - 13, 1.3)
  ctx.fillStyle = '#b79a67'; ctx.fillRect(95, trayY, 72, 6); ctx.fillStyle = '#e8e6ce'; ctx.beginPath(); ctx.ellipse(131, trayY - 3, 36, 8, 0, 0, tau); ctx.fill()
  disc(ctx, 131, trayY - 18, 16, '#f8f0d6', '#bbc4a4')
  // Calibration details make the scale legible without adding unrelated machinery.
  ctx.strokeStyle = '#8c9a78'; ctx.lineWidth = 1
  for (let i = 0; i < 12; i++) { ctx.beginPath(); ctx.moveTo(159, -175 + i * 17); ctx.lineTo(159 + (i % 3 ? 5 : 10), -175 + i * 17); ctx.stroke() }
  ctx.fillStyle = '#586d51'; ctx.font = '8px monospace'; ctx.fillText('BALANCE / TRANSMISSION / REST', -187, 221)
  ctx.fillStyle = fraction < 1 ? '#c89e5d' : '#8e9d7e'; ctx.beginPath(); ctx.arc(-230, 72, 5, 0, tau); ctx.fill()
  joint(ctx, -160, 169, 5); joint(ctx, 210, 169, 5)
  ctx.restore()
}
