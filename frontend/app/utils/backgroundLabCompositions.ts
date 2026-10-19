import type { LabPointer } from '~/utils/backgroundLab'
type Context = CanvasRenderingContext2D
const tau = Math.PI * 2
const smooth = (v: number) => v * v * (3 - 2 * v)
function poly(ctx: Context, points: number[][], color: string) {
  ctx.beginPath(); points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!)); ctx.closePath(); ctx.fillStyle = color; ctx.fill()
}
function gear(ctx: Context, x: number, y: number, r: number, angle: number, gold: boolean) {
  ctx.save(); ctx.translate(x, y); ctx.rotate(angle)
  const metal = ctx.createLinearGradient(-r, -r, r, r)
  metal.addColorStop(0, gold ? '#d8b66f' : '#bcc6c2'); metal.addColorStop(.42, gold ? '#a9844c' : '#879997'); metal.addColorStop(1, gold ? '#6e5334' : '#485c5e')
  ctx.fillStyle = '#423e3420'; ctx.beginPath(); ctx.arc(5, 9, r, 0, tau); ctx.fill()
  ctx.beginPath()
  for (let i = 0; i < 96; i++) { const a = i / 96 * tau, radius = i % 4 < 2 ? r : r * .89; if (i) ctx.lineTo(Math.cos(a) * radius, Math.sin(a) * radius); else ctx.moveTo(radius, 0) }
  ctx.closePath(); ctx.fillStyle = metal; ctx.fill(); ctx.strokeStyle = '#f4ead180'; ctx.lineWidth = 1; ctx.stroke()
  ctx.beginPath(); ctx.arc(0, 0, r * .72, 0, tau); ctx.fillStyle = gold ? '#9b7d4b' : '#6e8280'; ctx.fill(); ctx.strokeStyle = '#e0e0ca90'; ctx.stroke()
  for (let i = 0; i < 6; i++) { const a = i / 6 * tau; ctx.beginPath(); ctx.arc(Math.cos(a) * r * .48, Math.sin(a) * r * .48, r * .14, 0, tau); ctx.fillStyle = '#344644'; ctx.fill(); ctx.strokeStyle = '#e8e1c840'; ctx.stroke() }
  ctx.beginPath(); ctx.arc(0, 0, r * .19, 0, tau); ctx.fillStyle = '#d5d1b9'; ctx.fill(); ctx.beginPath(); ctx.arc(0, 0, r * .055, 0, tau); ctx.fillStyle = '#3f4a46'; ctx.fill(); ctx.restore()
}
/** A single motor drives meshed wheels, an eccentric linkage and a linear piston. */
export function paintKinetic(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const scale = Math.min(w / 1120, h / 650), cx = w * .73 + pointer.x * 18, cy = h * .5 + pointer.y * 12
  ctx.translate(cx, cy); ctx.scale(scale, scale)
  const shadow = ctx.createRadialGradient(0, 190, 5, 0, 190, 300); shadow.addColorStop(0, '#263b3540'); shadow.addColorStop(1, '#263b3500'); ctx.fillStyle = shadow; ctx.save(); ctx.scale(1, .26); ctx.beginPath(); ctx.arc(0, 700, 300, 0, tau); ctx.fill(); ctx.restore()
  poly(ctx, [[-230, 165], [165, 165], [255, 210], [-150, 210]], '#acb6a8'); poly(ctx, [[-150, 210], [255, 210], [255, 230], [-150, 230]], '#687b70'); poly(ctx, [[-230, 165], [-150, 210], [-150, 230], [-230, 185]], '#879a8a')
  ctx.fillStyle = '#697d72'; ctx.fillRect(-118, -120, 18, 302); ctx.fillRect(90, -155, 16, 337)
  ctx.strokeStyle = '#a7b5a7'; ctx.lineWidth = 5; ctx.strokeRect(-120, -125, 228, 285)
  const a = t * .8 + pointer.x * .12
  gear(ctx, -58, 35, 100, a, true); gear(ctx, 110, -7, 74, -a * 100 / 74 + .1, false); gear(ctx, -125, -118, 67, -a * 100 / 67 + .18, false)
  const pinX = -58 + Math.cos(a) * 53, pinY = 35 + Math.sin(a) * 53
  const pistonX = 110 + Math.cos(a) * 38
  ctx.strokeStyle = '#354c46'; ctx.lineWidth = 12; ctx.lineCap = 'round'; ctx.beginPath(); ctx.moveTo(pinX, pinY); ctx.lineTo(pistonX, -132); ctx.stroke(); ctx.strokeStyle = '#c4ccbc'; ctx.lineWidth = 4; ctx.stroke()
  ctx.fillStyle = '#4b6258'; ctx.fillRect(58, -159, 140, 42); ctx.fillStyle = '#cfb47b'; ctx.fillRect(pistonX - 18, -157, 36, 38)
  for (const [x, y] of [[pinX, pinY], [pistonX, -132]]) { ctx.beginPath(); ctx.arc(x!, y!, 8, 0, tau); ctx.fillStyle = '#d7d5b8'; ctx.fill(); ctx.beginPath(); ctx.arc(x!, y!, 2, 0, tau); ctx.fillStyle = '#3c4d43'; ctx.fill() }
  ctx.strokeStyle = '#566d62'; ctx.lineWidth = 2
  for (let i = 0; i < 12; i++) { ctx.beginPath(); ctx.moveTo(-203 + i * 23, 175); ctx.lineTo(-190 + i * 23, 182); ctx.stroke() }
  ctx.font = '11px monospace'; ctx.fillStyle = '#40564b'; ctx.fillText('KINETIC / NO. 07', -205, 245); ctx.fillText('1 : 1.35 : 1.49', 5, 245)
  ctx.restore()
  ctx.strokeStyle = '#677b6125'; ctx.lineWidth = 1
  for (let i = 0; i < 7; i++) { ctx.beginPath(); ctx.moveTo(w * .46 + i * 34, h * .14); ctx.lineTo(w * .46 + i * 34, h * .17); ctx.stroke() }
}
function arch(ctx: Context, x: number, y: number, width: number, height: number) {
  const r = width / 2
  ctx.beginPath(); ctx.moveTo(x, y + height); ctx.lineTo(x, y + r); ctx.arc(x + r, y + r, r, Math.PI, tau); ctx.lineTo(x + width, y + height); ctx.closePath()
}
export function paintPapercut(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const aw = Math.min(w * .55, h * .83), ah = h * .83, ax = w * .72 - aw / 2, ay = h * .085
  for (let layer = 4; layer >= 0; layer--) {
    const offset = layer * 11, px = pointer.x * layer * 3, py = pointer.y * layer * 2
    arch(ctx, ax - offset + px + 5, ay - offset + py + 7, aw + offset * 2, ah + offset * 1.4); ctx.fillStyle = '#5747360a'; ctx.fill()
    arch(ctx, ax - offset + px, ay - offset + py, aw + offset * 2, ah + offset * 1.4); ctx.fillStyle = ['#d7d2bd', '#e1d9c3', '#e8dfca', '#eee7d8', '#f5f0e6'][layer]!; ctx.fill()
  }
  arch(ctx, ax, ay, aw, ah); ctx.clip()
  ctx.fillStyle = '#d6e0d2'; ctx.fillRect(ax, ay, aw, ah)
  ctx.fillStyle = '#c88e69'; ctx.beginPath(); ctx.arc(ax + aw * .68 + pointer.x * 5, ay + ah * .26, aw * .12, 0, tau); ctx.fill()
  const mountains = ['#a7b9ad', '#809e91', '#5c8171', '#3e695a', '#244e45']
  mountains.forEach((color, layer) => {
    const shift = pointer.x * (layer + 1) * 8, base = ay + ah * (.39 + layer * .115), float = Math.sin(t * .2 + layer) * 3
    ctx.beginPath(); ctx.moveTo(ax - 60, ay + ah + 30)
    for (let point = 0; point <= 12; point++) { const x = ax - 50 + point * (aw + 100) / 12 + shift, y = base + Math.sin(point * 1.5 + layer * .9) * ah * .075 + (point % 3 - 1) * ah * .025 + float; ctx.lineTo(x, y) }
    ctx.lineTo(ax + aw + 60, ay + ah + 30); ctx.closePath(); ctx.fillStyle = color; ctx.fill(); ctx.strokeStyle = '#fff8e82b'; ctx.lineWidth = 1.4; ctx.stroke()
  })
  for (let i = 0; i < 5; i++) {
    const x = ax + aw * (.17 + i * .13) + pointer.x * 22, y = ay + ah * (.69 + Math.sin(i) * .06)
    ctx.fillStyle = '#efe8d0'; ctx.fillRect(x, y - 23 - i * 2, 15, 30 + i * 2); poly(ctx, [[x - 3, y - 23 - i * 2], [x + 7, y - 34 - i * 2], [x + 18, y - 23 - i * 2]], '#b88360'); ctx.fillStyle = '#556d5b'; ctx.fillRect(x + 6, y - 12, 4, 9)
  }
  ctx.strokeStyle = '#faf2d270'; ctx.lineWidth = 2; ctx.beginPath(); ctx.moveTo(ax + aw * .15, ay + ah); ctx.bezierCurveTo(ax + aw * .75, ay + ah * .95, ax + aw * .17, ay + ah * .77, ax + aw * .65, ay + ah * .73); ctx.stroke()
  ctx.strokeStyle = '#355e4c'; ctx.lineWidth = 1.5
  for (let bird = 0; bird < 3; bird++) {
    const bx = ax + aw * (.14 + bird * .15) + Math.sin(t * .3 + bird) * 12, by = ay + ah * .22 + bird * 13
    ctx.beginPath(); ctx.moveTo(bx - 7, by + Math.sin(t * 2 + bird) * 2); ctx.quadraticCurveTo(bx - 3, by - 4, bx, by); ctx.quadraticCurveTo(bx + 4, by - 5, bx + 8, by + Math.sin(t * 2 + bird) * 2); ctx.stroke()
  }
  ctx.restore()
}
function rounded(ctx: Context, x: number, y: number, w: number, h: number, r: number) {
  const radius = Math.min(r, w / 2, h / 2)
  ctx.beginPath(); ctx.moveTo(x + radius, y); ctx.arcTo(x + w, y, x + w, y + h, radius); ctx.arcTo(x + w, y + h, x, y + h, radius); ctx.arcTo(x, y + h, x, y, radius); ctx.arcTo(x, y, x + w, y, radius); ctx.closePath()
}
export function paintPoster(ctx: Context, w: number, h: number, t: number, pointer: LabPointer) {
  ctx.save()
  const u = Math.min(w / 1120, h / 650), cx = w * .72 + pointer.x * 8, cy = h * .5 + pointer.y * 7
  ctx.translate(cx, cy); ctx.scale(u, u)
  ctx.save(); ctx.translate(-50, -8); ctx.rotate(-.12 + Math.sin(t * .2) * .025)
  ctx.fillStyle = '#292f2920'; ctx.fillRect(-173, -243, 380, 510); ctx.fillStyle = '#d8d4c5'; ctx.fillRect(-182, -254, 380, 510)
  ctx.strokeStyle = '#555e4930'; ctx.lineWidth = 1
  for (let i = 0; i < 10; i++) { ctx.beginPath(); ctx.moveTo(-167, -230 + i * 48); ctx.lineTo(183, -230 + i * 48); ctx.stroke() }
  ctx.restore()
  ctx.save(); ctx.rotate(.055 + pointer.x * .06)
  ctx.fillStyle = '#4b3c2d20'; ctx.fillRect(-174, -250, 390, 520); ctx.fillStyle = '#f4efdf'; ctx.fillRect(-184, -262, 390, 520)
  ctx.fillStyle = '#31362f'; ctx.font = '11px monospace'; ctx.fillText('IDEAS / FORM / MOTION', -160, -230); ctx.fillText('2026', 142, -230)
  const phase = t % 12 / 4, index = Math.floor(phase), f = smooth(Math.max(0, Math.min(1, ((phase % 1) - .42) / .58)))
  const keys = [{ x: -143, y: -184, w: 295, h: 295, r: 148 }, { x: -153, y: -180, w: 150, h: 350, r: 9 }, { x: -148, y: -90, w: 290, h: 125, r: 2 }]
  const a = keys[index]!, b = keys[(index + 1) % 3]!, lerp = (key: keyof typeof a) => a[key] + (b[key] - a[key]) * f
  rounded(ctx, lerp('x'), lerp('y'), lerp('w'), lerp('h'), lerp('r')); ctx.fillStyle = '#c36f46'; ctx.fill()
  ctx.save(); ctx.beginPath(); ctx.rect(-158, -200, 336, 370); ctx.clip()
  ctx.fillStyle = '#31483c'; ctx.font = '700 79px Arial, sans-serif'; ctx.translate(0, -12 + Math.sin(t * .32) * 5)
  ctx.fillText('IDEA', -155, -90); ctx.fillText('BUILD', -155 + (index === 1 ? f * 20 : 0), -9); ctx.strokeStyle = '#f5efda'; ctx.lineWidth = 1.3; ctx.strokeText('SHARE', -155, 74)
  ctx.restore()
  ctx.save(); ctx.translate(128, 157); ctx.rotate(t * .18)
  for (let i = 0; i < 12; i++) { ctx.rotate(tau / 12); ctx.fillStyle = '#344a3b'; ctx.fillRect(-5, 17, 10, 36) }
  ctx.beginPath(); ctx.arc(0, 0, 13, 0, tau); ctx.fillStyle = '#d7b465'; ctx.fill(); ctx.restore()
  ctx.fillStyle = '#52634b'; ctx.font = '10px monospace'; ctx.fillText('FORM FOLLOWS EXPLORATION', -157, 208)
  ctx.strokeStyle = '#52634b50'; ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(-157, 187); ctx.lineTo(62, 187); ctx.stroke()
  ctx.restore(); ctx.restore()
}
