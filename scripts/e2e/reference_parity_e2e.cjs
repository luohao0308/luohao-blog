// Local completion continuity: real DOM immediately before cleanup vs settled page.
// The old decorative golden scene is intentionally no longer the pixel target.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { PNG } = require('pngjs')
const pixelmatchModule = require('pixelmatch')
const pixelmatch = pixelmatchModule.default || pixelmatchModule
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13008'
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname)) throw Error('Local only')
const output = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/reference-parity')
const scene = '[data-home-live-assembly]'
async function boxes(page) {
  return page.locator('[data-home-shell-layer], [data-home-layer], [data-home-card]').evaluateAll(nodes => nodes.map(node => {
    const { x, y, width, height } = node.getBoundingClientRect()
    return { x, y, width, height }
  }))
}
async function main() {
  fs.mkdirSync(output, { recursive: true })
  const fixture = await startFixture({ articles: [makeArticle(), makeArticle({ id: '2', slug: 'second' }), makeArticle({ id: '3', slug: 'third' })] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const results = []
  try {
    for (const width of [320, 390, 657, 768, 1024, 1440]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: 'light', reducedMotion: 'no-preference' })
      // Capture actual WAAPI handles at creation, including offscreen effects.
      // Pause immediately so slow CI cannot let short card effects finish before
      // the sample is taken. This changes test time, never frames or DOM.
      await context.addInitScript(() => {
        window.__assemblyAnimations = []
        const animate = Element.prototype.animate
        Element.prototype.animate = function (...args) {
          const animation = animate.apply(this, args)
          if (this.matches('[data-home-shell-layer], [data-home-layer], [data-home-card]')) {
            animation.pause()
            window.__assemblyAnimations.push(animation)
          }
          return animation
        }
      })
      const page = await context.newPage()
      await page.goto(base, { waitUntil: 'domcontentloaded' })
      await page.locator(scene).waitFor({ state: 'attached' })
      const timelines = await page.evaluate(() => {
        const animations = window.__assemblyAnimations.filter(a => a.effect?.target?.isConnected)
        window.__assemblyIdentity = [...document.querySelectorAll('[data-home-shell-layer], [data-home-layer], [data-home-card]')]
        const maxEnd = Math.max(...animations.map(a => a.effect.getComputedTiming().endTime))
        for (const animation of animations) {
          animation.pause()
          // Keep every promise pending while sampling its final resting interval.
          animation.currentTime = Math.min(maxEnd - 1, animation.effect.getComputedTiming().endTime - 1)
        }
        return animations.map(a => ({ currentTime: a.currentTime, endTime: a.effect.getComputedTiming().endTime }))
      })
      assert.equal(timelines.length, await page.locator('[data-home-shell-layer], [data-home-layer], [data-home-card]').count(), 'every real layer and card has a captured native timeline')
      await page.addStyleTag({ content: '[data-home-skip] { visibility:hidden !important; }' })
      await page.evaluate(() => document.fonts.ready)
      const beforeBoxes = await boxes(page)
      const before = await page.screenshot({ animations: 'allow', fullPage: true })
      await page.keyboard.press('Escape')
      await page.locator(scene).waitFor({ state: 'detached' })
      const afterBoxes = await boxes(page)
      assert.deepEqual(afterBoxes, beforeBoxes, `no geometry jump at ${width}px`)
      assert(await page.evaluate(() => window.__assemblyIdentity.every(node => node.isConnected)), 'real nodes survive completion')
      assert.equal(await page.locator('.home-assembly-overlay, .ad-piece, .screen-wrap').count(), 0)
      const after = await page.screenshot({ animations: 'allow', fullPage: true })
      const a = PNG.sync.read(before), b = PNG.sync.read(after)
      assert.equal(a.width, b.width); assert.equal(a.height, b.height)
      const diff = new PNG({ width: a.width, height: a.height })
      const different = pixelmatch(a.data, b.data, diff.data, a.width, a.height, { threshold: 0.1 })
      const ratio = different / (a.width * a.height)
      for (const [name, bytes] of [['before', before], ['after', after], ['diff', PNG.sync.write(diff)]]) fs.writeFileSync(path.join(output, `${name}-${width}.png`), bytes)
      results.push({ width, timelines, beforeBoxes, afterBoxes, different, ratio })
      assert(ratio < 0.001, `no final canvas-to-page swap at ${width}: ${ratio}`)
      await context.close()
    }
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ status: 'PASS', scope: 'same real page at final resting interval and after cleanup; skip control hidden only for comparison; not old decorative-scene parity', results }, null, 2))
    console.log(JSON.stringify({ status: 'PASS', comparisons: results.length, maximumDifferentRatio: Math.max(...results.map(r => r.ratio)) }))
  } finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
