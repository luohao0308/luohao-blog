// Isolated homepage sizing regression. Uses local read-only fixtures and owns its frontend process.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const { startFixture } = require('./ui_fixture.cjs')
const root = path.resolve(__dirname, '../..')
const output = path.resolve(process.env.BLOG_E2E_OUTPUT || path.join(root, 'output/home-city-size'))
const port = Number(process.env.BLOG_E2E_PORT || 14340)
const fixturePort = Number(process.env.BLOG_E2E_FIXTURE_PORT || 14341)
const base = `http://127.0.0.1:${port}`
const dev = process.argv.includes('--dev')

async function main() {
  fs.mkdirSync(output, { recursive: true })
  process.env.BLOG_E2E_FIXTURE_PORT = String(fixturePort)
  const fixture = await startFixture()
  const log = fs.openSync(path.join(output, 'server.log'), 'w')
  const command = dev ? path.join(root, 'frontend/node_modules/nuxt/bin/nuxt.mjs') : path.join(root, 'frontend/.output/server/index.mjs')
  const args = dev ? [command, 'dev', '--host', '127.0.0.1', '--port', String(port)] : [command]
  const server = spawn(process.execPath, args, {
    cwd: path.join(root, 'frontend'), detached: true, stdio: ['ignore', log, log],
    env: { ...process.env, HOST: '127.0.0.1', PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${fixturePort}`, NUXT_PUBLIC_SITE_URL: base },
  })
  fs.closeSync(log)
  let exited = false
  server.on('exit', () => { exited = true })
  let browser
  const results = []
  try {
    let ready = false
    for (let i = 0; i < 180; i++) {
      assert(!exited, 'task frontend exited; see server.log')
      try { ready = (await fetch(`${base}/health`)).ok } catch { /* Startup only. */ }
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 500))
    }
    assert(ready, 'task frontend did not become ready')
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    for (const width of [1440, 1024, 768, 640, 390, 320]) for (const theme of ['light', 'dark']) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce' })
      await context.addInitScript(({ theme }) => {
        sessionStorage.setItem('blog:home-entrance:v1', '1')
        localStorage.setItem('theme', theme)
      }, { theme })
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(base, { waitUntil: 'networkidle' })
      await page.locator('.home-visual canvas').waitFor()
      await page.waitForFunction(() => Number(document.querySelector('.home-visual canvas')?.dataset.labFrame) > 0)
      const sizes = await page.evaluate(() => {
        const visual = document.querySelector('.home-visual'), hero = document.querySelector('.home-hero')
        const box = visual.getBoundingClientRect(), parent = hero.getBoundingClientRect()
        const columns = getComputedStyle(hero).gridTemplateColumns.split(' ').map(Number.parseFloat)
        const canvas = visual.querySelector('canvas'), pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data
        let left = canvas.width, right = -1
        for (let y = 0; y < canvas.height; y++) for (let x = 0; x < canvas.width; x++) {
          if (pixels[(y * canvas.width + x) * 4 + 3] > 64) { left = Math.min(left, x); right = Math.max(right, x) }
        }
        return { width: box.width, height: box.height, left: box.left, right: box.right, parentLeft: parent.left, parentRight: parent.right,
          available: columns.length === 2 ? columns[1] : Math.min(parent.width, 440), pageWidth: document.querySelector('.home-page').getBoundingClientRect().width,
          overflow: document.documentElement.scrollWidth > innerWidth, paintedRatio: (right - left + 1) / canvas.width }
      })
      await page.screenshot({ path: path.join(output, `home-${width}-${theme}.png`) })
      assert(Math.abs(sizes.width - sizes.available) < 2, `city must fill available hero column at ${width}/${theme}: ${JSON.stringify(sizes)}`)
      assert(sizes.height >= 319, `city retains original minimum height at ${width}/${theme}`)
      assert(!sizes.overflow && sizes.left >= sizes.parentLeft - 1 && sizes.right <= sizes.parentRight + 1, 'no city or page horizontal overflow')
      assert(sizes.pageWidth <= 841, 'compact page width remains intact')
      assert(sizes.paintedRatio >= .8, 'actual painted city fills its canvas rather than shrinking inside it')
      const sun = page.locator('.home-visual [data-lab-hit="citybiz-sun"]')
      const before = await page.locator('.home-visual canvas').getAttribute('data-lab-actions')
      await sun.focus()
      await page.keyboard.press('Enter')
      await page.waitForFunction(value => Number(document.querySelector('.home-visual canvas')?.dataset.labActions) > Number(value), before)
      assert.deepEqual(errors, [], 'no client runtime errors')
      results.push({ viewportWidth: width, theme, ...sizes, keyboardInteraction: true })
      await context.close()
    }
    // Reflow the same mounted canvas across the breakpoint; desktop motion stays active.
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'no-preference' })
    await context.addInitScript(() => sessionStorage.setItem('blog:home-entrance:v1', '1'))
    const page = await context.newPage()
    await page.goto(base, { waitUntil: 'networkidle' })
    await page.waitForFunction(() => Number(document.querySelector('.home-visual canvas')?.dataset.labFrame) > 0)
    for (const width of [390, 1440]) {
      await page.setViewportSize({ width, height: 1000 })
      await page.waitForFunction(() => {
        const visual = document.querySelector('.home-visual').getBoundingClientRect()
        const canvas = document.querySelector('.home-visual canvas').getBoundingClientRect()
        return Math.abs(visual.width - canvas.width) < 1 && Math.abs(visual.height - canvas.height) < 1
      })
      assert.equal(await page.locator('.home-visual canvas').getAttribute('data-render-state'), 'running')
    }
    await context.close()
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ passed: true, results, resizeAndMotion: true }, null, 2))
    console.log(`PASS: ${results.length} responsive/theme cases, painted coverage, keyboard interaction, resize and motion`)
  } finally {
    await browser?.close()
    if (!exited) {
      const stopped = new Promise(resolve => server.once('exit', resolve))
      process.kill(-server.pid, 'SIGTERM')
      await stopped
    }
    await fixture.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
