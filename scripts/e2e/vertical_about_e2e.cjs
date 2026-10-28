const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const path = require('node:path')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
async function main() {
  const base = process.env.BLOG_LAYOUT_BASE || 'http://127.0.0.1:13034'
  assert.ok(['127.0.0.1', 'makerhao.cn'].includes(new URL(base).hostname))
  const root = path.resolve(__dirname, '../..')
  let frontend, browser
  const results = []
  const out = path.join(root, 'output/vertical-about')
  try {
    if (new URL(base).hostname === '127.0.0.1') {
      frontend = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: '13034', NUXT_BACKEND_BASE: 'https://makerhao.cn/api' }, stdio: 'ignore' })
      let ready = false
      for (let i = 0; i < 60; i++) { try { ready = (await fetch(base + '/health')).ok } catch {} if (ready) break; await new Promise(r => setTimeout(r, 250)) }
      assert.ok(ready, 'local frontend failed to start')
    }
    await fs.mkdir(out, { recursive: true })
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM })
    for (const [width, motion] of [[1440, 'no-preference'], [390, 'no-preference'], [1440, 'reduce']]) {
      const page = await browser.newPage({ viewport: { width, height: 1000 }, reducedMotion: motion })
      const errors = []; page.on('pageerror', e => errors.push(e.message))
      await page.goto(base + '/projects'); await page.locator('.media-feed .work-card').first().waitFor()
      const portfolio = await page.evaluate(() => ({ width: document.querySelector('main').getBoundingClientRect().width, columns: getComputedStyle(document.querySelector('.media-feed')).gridTemplateColumns.split(' ').length }))
      assert.equal(portfolio.columns, width === 1440 ? 4 : 2)
      if (width === 1440) assert.equal(portfolio.width, 1024)
      await page.goto(base + '/posts'); await page.locator('h1').waitFor()
      assert.equal(await page.locator('main').evaluate(e => e.getBoundingClientRect().width), portfolio.width)
      await page.goto(base + '/about'); await page.locator('.continuous-story').waitFor()
      await page.waitForTimeout(400)
      assert.equal(await page.locator('.journey-controls,.journey-resume,.reading-mode,.chapter-label').count(), 0)
      assert.equal(await page.getByRole('button', { name: /普通阅读|返回连续镜头/ }).count(), 0)
      const geometry = await page.evaluate(() => { const r = document.querySelector('.continuous-story'), s = r.querySelector('.journey-stage'); const nav = document.querySelector('.site-header').getBoundingClientRect().height; return { active: r.classList.contains('is-continuous'), count: r.querySelectorAll('[data-journey-scene]').length, start: scrollY + r.getBoundingClientRect().top - nav, distance: r.offsetHeight - s.clientHeight } })
      if (motion === 'reduce') assert.equal(geometry.active, false)
      if (width === 1440 && motion === 'no-preference') assert.equal(geometry.active, true)
      const translations = []
      if (geometry.active) {
        for (let i = 0; i < geometry.count; i++) {
          await page.evaluate(y => scrollTo({ top: y, behavior: 'instant' }), geometry.start + geometry.distance * i / (geometry.count - 1))
          await page.waitForTimeout(180)
          const pos = await page.locator('.journey-camera').evaluate(e => { const t = new DOMMatrixReadOnly(getComputedStyle(e).transform); return { x: t.m41, y: t.m42 } })
          assert.ok(Math.abs(pos.x) < 1, 'camera moved sideways')
          if (translations.length) assert.ok(pos.y < translations.at(-1).y, 'camera must progress down the page')
          translations.push(pos)
        }
        assert.ok(await page.getByRole('link', { name: /访问 GitHub/ }).isVisible())
      }
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false)
      assert.deepEqual(errors, [])
      await page.screenshot({ path: path.join(out, `${width}-${motion}.png`), fullPage: false })
      results.push({ width, motion, portfolio, geometry, translations, result: 'PASS' }); await page.close()
    }
    await fs.writeFile(path.join(out, 'results.json'), JSON.stringify(results, null, 2) + '\n')
    console.log('Portfolio width/four columns, vertical camera, removed controls, links and reduced motion: PASS')
  } finally { await browser?.close(); frontend?.kill('SIGTERM') }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
