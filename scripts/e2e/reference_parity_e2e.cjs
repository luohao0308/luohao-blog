// Golden fixture is the user's actual white-layers tab, not the later hybrid.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { PNG } = require('pngjs')
const pixelmatchModule = require('pixelmatch')
const pixelmatch = pixelmatchModule.default || pixelmatchModule
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13008'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname)) throw Error('Local only')
const golden = fs.readFileSync(path.join(__dirname, 'fixtures/white-layers-original.html'), 'utf8')
const output = path.resolve('output/visual-redesign/reference-parity')
async function freeze(page, time) {
  await page.evaluate(time => {
    for (const animation of document.getAnimations()) if (animation.effect?.target?.matches('.ad-piece, .ad-tag')) {
      animation.pause(); animation.currentTime = time
    }
  }, time)
}
async function geometry(page, selector) {
  return page.locator(selector).evaluate(stage => ({
    screen: { width: stage.querySelector('.screen').offsetWidth, height: stage.querySelector('.screen').offsetHeight },
    perspective: getComputedStyle(stage).perspective,
    pieces: [...stage.querySelectorAll('.ad-piece')].map(p => ({ x: p.offsetLeft, y: p.offsetTop, width: p.offsetWidth, height: p.offsetHeight })),
    animations: document.getAnimations().filter(a => a.effect?.target?.matches('.ad-piece, .ad-tag')).map(a => ({ timing: a.effect.getTiming(), frames: a.effect.getKeyframes() })),
  }))
}
async function main() {
  fs.mkdirSync(output, { recursive: true })
  const fixture = await startFixture({ articles: [makeArticle({ title: '给 AI 编码助手上一道锁：415 行的交付守卫' })] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const results = []
  try {
    for (const [width, scheme] of [[1440, 'light'], [657, 'light'], [390, 'light'], [657, 'dark']]) {
      const actualContext = await browser.newContext({ viewport: { width, height: 1400 }, colorScheme: scheme, reducedMotion: 'no-preference' })
      const actual = await actualContext.newPage()
      await actual.goto(base, { waitUntil: 'domcontentloaded' })
      const actualStage = actual.locator('.home-assembly-overlay .stage')
      await actualStage.locator('.ad-piece').first().waitFor({ state: 'attached' })
      await freeze(actual, 0)
      // The user requested fan overlay too; compare the unchanged five-plane
      // scene separately from that explicitly additional presentation layer.
      await actual.addStyleTag({ content: '.assembly-fan-card { visibility: hidden !important; }' })
      const stageBounds = await actualStage.boundingBox()
      const referenceContext = await browser.newContext({ viewport: { width, height: 1400 }, colorScheme: scheme, reducedMotion: 'no-preference' })
      const reference = await referenceContext.newPage()
      await reference.setContent(golden, { waitUntil: 'load' })
      await reference.locator('.ad-piece').first().waitFor({ state: 'attached' })
      await freeze(reference, 0)
      // Stage placement is product chrome, not animation. Align origins so the
      // same reference pixels are compared at the same physical coordinates.
      await reference.locator('.stage').evaluate((stage, bounds) => {
        Object.assign(stage.style, { position: 'fixed', left: `${bounds.x}px`, top: `${bounds.y}px`, width: `${bounds.width}px`, margin: '0' })
      }, stageBounds)
      const actualGeometry = await geometry(actual, '.home-assembly-overlay .stage')
      const referenceGeometry = await geometry(reference, '.stage')
      fs.writeFileSync(path.join(output, `geometry-${width}-${scheme}.json`), JSON.stringify({ reference: referenceGeometry, actual: actualGeometry }, null, 2))
      assert.deepEqual(actualGeometry, referenceGeometry, `original geometry, all ten timelines and native keyframes at ${width}`)
      for (const time of [0, 250, 650, 1100, 1800, 2600, 3100]) {
        await freeze(actual, time); await freeze(reference, time)
        const actualBytes = await actualStage.screenshot({ animations: 'allow' })
        const referenceBytes = await reference.locator('.stage').screenshot({ animations: 'allow' })
        const a = PNG.sync.read(actualBytes), b = PNG.sync.read(referenceBytes)
        assert.equal(a.width, b.width); assert.equal(a.height, b.height)
        const diff = new PNG({ width: a.width, height: a.height })
        const different = pixelmatch(a.data, b.data, diff.data, a.width, a.height, { threshold: 0.1 })
        const ratio = different / (a.width * a.height)
        const name = `${width}-${scheme}-${time}`
        fs.writeFileSync(path.join(output, `actual-${name}.png`), actualBytes)
        fs.writeFileSync(path.join(output, `reference-${name}.png`), referenceBytes)
        fs.writeFileSync(path.join(output, `diff-${name}.png`), PNG.sync.write(diff))
        results.push({ width, scheme, time, different, ratio })
        assert(ratio < 0.005, `reference pixel parity at ${name}: ${(ratio * 100).toFixed(3)}%`)
      }
      await actualContext.close(); await referenceContext.close()
    }
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ status: 'PASS', scope: 'original five planes only; the separately requested fan is hidden for pixel comparison', results }, null, 2))
    console.log(JSON.stringify({ status: 'PASS', comparisons: results.length, maximumDifferentRatio: Math.max(...results.map(r => r.ratio)) }))
  } finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exit(1) })
