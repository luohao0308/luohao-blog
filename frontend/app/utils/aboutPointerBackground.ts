export const aboutBackgrounds = [
  { id: 'ripple', name: '01 · 水面涟漪', description: '鼠标像掠过水面，留下缓缓扩散的细波。' },
  { id: 'aurora', name: '02 · 柔光流场', description: '几层透明色光随鼠标移动，交叠成柔软的光晕。' },
  { id: 'field', name: '03 · 磁力线', description: '细线向光标弯曲，像一束被轻轻拨动的丝线。' },
  { id: 'sand', name: '04 · 粒子沙', description: '细小颗粒被光标推散，再慢慢回到自己的位置。' },
  { id: 'mesh', name: '05 · 弹性网格', description: '一张轻薄的网随光标凹陷，离开后舒展回弹。' },
] as const
export type AboutBackgroundId = typeof aboutBackgrounds[number]['id']
export interface AboutBackgroundPointer { x: number, y: number, strength: number }
interface Wake { x: number, y: number, born: number }

// One bounded state per mounted background. No global animation or random seed changes on resize.
export function createAboutBackgroundPainter() {
  let wakes: Wake[] = []
  let previous = { x: -1, y: -1, time: -1 }
  let lastEffect: AboutBackgroundId | undefined
  return (ctx: CanvasRenderingContext2D, width: number, height: number, time: number, pointer: AboutBackgroundPointer, effect: AboutBackgroundId, dark: boolean) => {
    if (lastEffect !== effect) { wakes = []; previous = { x: -1, y: -1, time: -1 }; lastEffect = effect }
    ctx.clearRect(0, 0, width, height)
    const ink = dark ? '166,198,183' : '91,125,117'
    const blue = dark ? '146,179,214' : '113,144,176'
    const gold = dark ? '201,181,135' : '183,151,95'
    const px = pointer.x * width, py = pointer.y * height, strength = pointer.strength
    const line = (alpha: number) => `rgba(${ink},${alpha})`
    const glow = (x: number, y: number, radius: number, color: string, alpha: number) => {
      const gradient = ctx.createRadialGradient(x, y, 0, x, y, radius)
      gradient.addColorStop(0, `rgba(${color},${alpha})`); gradient.addColorStop(1, `rgba(${color},0)`)
      ctx.fillStyle = gradient; ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2)
    }
    // Keep a quiet offset composition even when the pointer is outside the page.
    glow(width * .8, height * .4, Math.max(width, height) * .65, ink, dark ? .06 : .055)
    if (effect === 'ripple') {
      if (strength > .2 && time - previous.time > .09 && Math.hypot(px - previous.x, py - previous.y) > 13) {
        wakes.push({ x: px, y: py, born: time }); previous = { x: px, y: py, time }
      }
      wakes = wakes.filter(wake => time - wake.born < 3.5).slice(-22)
      glow(px, py, 230, blue, strength * .12)
      // Soft resting caustics remain visible without automatic wave emission.
      for (let i = 0; i < 7; i++) {
        ctx.beginPath(); ctx.ellipse(width * .77, height * .5, 90 + i * 44, 48 + i * 27, -.25, 0, Math.PI * 2)
        ctx.strokeStyle = line(.05); ctx.lineWidth = 1; ctx.stroke()
      }
      for (const wake of wakes) {
        const age = time - wake.born
        for (let ring = 0; ring < 3; ring++) {
          const radius = 8 + age * 76 + ring * 17
          ctx.beginPath(); ctx.ellipse(wake.x, wake.y, radius, radius * .65, -.2, 0, Math.PI * 2)
          ctx.strokeStyle = line((1 - age / 3.5) * .18 / (ring + 1)); ctx.lineWidth = 1.1; ctx.stroke()
        }
      }
    }
    else if (effect === 'aurora') {
      for (let i = 0; i < 4; i++) {
        const drift = Math.sin(time * .28 + i * 1.9)
        const x = width * (.27 + i * .19) + (px - width / 2) * strength * (.17 + i * .035)
        const y = height * (.3 + i % 2 * .25) + drift * 24 + (py - height / 2) * strength * .2
        glow(x, y, Math.max(230, width * .38), [ink, blue, gold, ink][i]!, dark ? .21 : .18)
      }
      ctx.save(); ctx.translate(width * .7 + (px - width / 2) * strength * .1, height * .5); ctx.rotate(-.4)
      for (let i = 0; i < 12; i++) {
        ctx.beginPath(); ctx.ellipse(0, 0, 60 + i * 18, 160 + i * 8, 0, 0, Math.PI * 2)
        ctx.strokeStyle = `rgba(${gold},.045)`; ctx.stroke()
      }
      ctx.restore()
    }
    else if (effect === 'field') {
      for (let row = 0; row < 31; row++) {
        ctx.beginPath()
        for (let x = -20; x <= width + 20; x += 14) {
          const base = height * .1 + row * height * .027
          const distance = Math.hypot(x - px, base - py)
          const bend = Math.exp(-distance * distance / 65000) * strength * 100
          const y = base + Math.sin(x / 190 + row * .17 + time * .18) * 15 + bend * (base > py ? 1 : -1)
          if (x === -20) ctx.moveTo(x, y); else ctx.lineTo(x, y)
        }
        ctx.strokeStyle = row % 5 === 0 ? `rgba(${gold},.26)` : line(.14); ctx.lineWidth = .8; ctx.stroke()
      }
      glow(px, py, 150, gold, strength * .12)
    }
    else if (effect === 'sand') {
      const spacing = width < 600 ? 24 : 27
      for (let row = 0; row < height / spacing; row++) for (let col = 0; col < width / spacing; col++) {
        const seed = Math.sin(row * 17.31 + col * 45.17) * 43758.54
        const offset = seed - Math.floor(seed)
        const x = col * spacing + Math.sin(row * 8 + col) * 7, y = row * spacing + offset * 14
        const dx = x - px, dy = y - py, distance = Math.hypot(dx, dy)
        const push = Math.exp(-distance * distance / 18000) * 65 * strength
        const angle = Math.atan2(dy, dx) + Math.sin(time * .45 + offset * 6) * .3
        ctx.beginPath(); ctx.arc(x + Math.cos(angle) * push, y + Math.sin(angle) * push, offset > .75 ? 1.7 : .85, 0, Math.PI * 2)
        ctx.fillStyle = offset > .65 ? `rgba(${gold},.48)` : line(.27); ctx.fill()
      }
      glow(px, py, 140, gold, strength * .1)
    }
    else {
      const spacing = width < 600 ? 36 : 44
      const point = (x: number, y: number) => {
        const dx = px - x, dy = py - y, distance = Math.hypot(dx, dy)
        const pull = Math.exp(-distance * distance / 40000) * .36 * strength
        return { x: x + dx * pull, y: y + dy * pull + Math.sin(time * .7 + x / 140) * 2 * strength }
      }
      ctx.strokeStyle = line(.17); ctx.lineWidth = .7
      for (let row = -1; row <= height / spacing + 1; row++) {
        ctx.beginPath()
        for (let col = -1; col <= width / spacing + 1; col++) {
          const p = point(col * spacing, row * spacing)
          if (col === -1) ctx.moveTo(p.x, p.y); else ctx.lineTo(p.x, p.y)
        }
        ctx.stroke()
      }
      for (let col = -1; col <= width / spacing + 1; col++) {
        ctx.beginPath()
        for (let row = -1; row <= height / spacing + 1; row++) {
          const p = point(col * spacing, row * spacing)
          if (row === -1) ctx.moveTo(p.x, p.y); else ctx.lineTo(p.x, p.y)
        }
        ctx.stroke()
      }
      glow(px, py, 200, blue, strength * .13)
    }
  }
}
