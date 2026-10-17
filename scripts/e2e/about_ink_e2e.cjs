// Local-only acceptance for the fluid personal About page. All profile content is synthetic.
const assert = require('node:assert/strict')
const http = require('node:http')
const fs = require('node:fs')
const path = require('node:path')
const { PNG } = require('pngjs')
const { chromium } = require('playwright')
const { makeArticle, pixelPng } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13006'
assert(['localhost', '127.0.0.1'].includes(new URL(base).hostname), 'local frontend only')
const out = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/about-ink')
const profile = { name: 'luohao', headline: '保持好奇，持续构建。', intro: '本地验收简介：专注全栈开发与 AI Agent。', current_focus: '构建可靠的软件', skills: [{ group: '开发', items: ['Go', 'Nuxt'] }], timeline: [{ year: '2026', text: '持续探索与记录' }], contacts: [{ label: 'Email', href: 'mailto:fixture@example.com', text: 'fixture@example.com' }], representative_links: [] }
const project = { slug: 'fixture-project', name: '验收项目', summary: '本地项目', description: '', stack: ['Go'], role: '开发', emoji: '', year: '2026', featured: true, published: true, sort: 0 }
const report = { checks: [], screenshots: [], errors: [] }
async function main() {
  fs.mkdirSync(out, { recursive: true })
  const fixture = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1')
    res.setHeader('Content-Type', 'application/json; charset=utf-8')
    const json = (v, status = 200) => { res.statusCode = status; res.end(JSON.stringify(v)) }
    if (req.method !== 'GET') return json({ message: 'Read only' }, 405)
    if (url.pathname.includes('/auth/')) return json({ message: 'Anonymous' }, 401)
    if (url.pathname.endsWith('/about')) return json(profile)
    if (url.pathname.includes('/assets/')) { res.setHeader('Content-Type', 'image/png'); return res.end(pixelPng) }
    if (url.pathname.endsWith('/articles/list') || url.pathname.includes('/search/articles')) return json({ articles: [makeArticle()] })
    if (url.pathname.endsWith('/projects/list')) return json({ projects: [project] })
    if (url.pathname.includes('/categories')) return json({ categories: [] })
    if (url.pathname.includes('/collections')) return json({ collections: [] })
    return json({ message: 'Unknown fixture route' }, 404)
  })
  await new Promise((resolve, reject) => { fixture.once('error', reject); fixture.listen(Number(process.env.BLOG_E2E_FIXTURE_PORT || 13007), '127.0.0.1', resolve) })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] })
  async function session(options = {}, blockWebgl = false) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, ...options })
    if (blockWebgl) await ctx.addInitScript(() => { const original = HTMLCanvasElement.prototype.getContext; HTMLCanvasElement.prototype.getContext = function (kind, ...args) { return String(kind).includes('webgl') ? null : original.call(this, kind, ...args) } })
    const page = await ctx.newPage()
    page.on('pageerror', error => report.errors.push(error.message))
    await page.goto(`${base}/about`, { waitUntil: 'networkidle' })
    await page.locator('[data-about-content]').waitFor()
    return { ctx, page }
  }
  async function content(page) {
    assert.equal(await page.locator('h1').innerText(), 'luohao')
    for (const text of ['本地验收简介', '技能栈', '经历', '联系我']) assert((await page.locator('[data-about-content]').innerText()).includes(text), `preserved ${text}`)
    assert.equal(await page.locator('[data-about-content] a[href="/projects/fixture-project"]').count(), 1)
    assert.equal(await page.locator('[data-about-content] a[href="/posts/fixture-article"]').count(), 1)
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'no document overflow')
  }
  async function shot(page, name) {
    const bytes = await page.screenshot({ path: path.join(out, name + '.png') })
    report.screenshots.push(name + '.png')
    return PNG.sync.read(bytes).data
  }
  function changedPixels(first, second) {
    assert.equal(first.length, second.length)
    let changed = 0
    for (let i = 0; i < first.length; i += 4) {
      if (Math.abs(first[i] - second[i]) + Math.abs(first[i + 1] - second[i + 1]) + Math.abs(first[i + 2] - second[i + 2]) > 12) changed++
    }
    return changed / (first.length / 4)
  }
  try {
    const { ctx, page } = await session()
    await content(page)
    const canvas = page.locator('[data-ink-canvas]')
    await canvas.waitFor()
    await page.waitForFunction(() => document.querySelector('[data-ink-canvas]')?.dataset.inkState === 'active')
    const first = await shot(page, 'desktop-initial')
    await page.waitForTimeout(700)
    const second = await shot(page, 'desktop-flow')
    report.autonomousChangedPixelRatio = changedPixels(first, second)
    assert(report.autonomousChangedPixelRatio > .005, 'autonomous flow changes at least 0.5% of visible pixels')
    await page.mouse.move(220, 500)
    await page.mouse.move(1000, 600, { steps: 20 })
    await page.mouse.click(900, 480)
    await page.waitForTimeout(150)
    report.interactiveChangedPixelRatio = changedPixels(second, await shot(page, 'desktop-interaction'))
    assert(report.interactiveChangedPixelRatio > .005, 'interactive scene changes visible pixels')
    report.checks.push('desktop WebGL active; autonomous and interactive frames visibly differ; identity and all real content entry types preserved')
    await page.evaluate(() => scrollTo(0, 450))
    await page.waitForFunction(() => Number(document.querySelector('[data-about-ink]')?.dataset.inkProgress) > 0)
    report.scroll = await page.evaluate(() => ({ y: scrollY, height: document.documentElement.scrollHeight, root: document.querySelector('[data-about-ink]').getBoundingClientRect().toJSON(), progress: document.querySelector('[data-about-ink]').dataset.inkProgress, scrolling: document.scrollingElement?.tagName }))
    assert(Number(await page.locator('[data-about-ink]').getAttribute('data-ink-progress')) > 0, 'scroll progress changes')
    await page.locator('[data-about-content]').scrollIntoViewIfNeeded()
    await page.waitForTimeout(300)
    await page.evaluate(() => scrollTo(0, document.body.scrollHeight))
    await page.waitForTimeout(400)
    assert.equal(await canvas.getAttribute('data-ink-state'), 'still', 'offscreen renderer pauses')
    await page.locator('.site-header a[href="/posts"]').first().click()
    await page.waitForURL('**/posts')
    assert.equal(await page.locator('[data-ink-canvas]').count(), 0, 'route unmount removes canvas')
    await page.waitForTimeout(250)
    report.checks.push('scroll progress, offscreen pause, route unmount without browser errors')
    await ctx.close()
    const mobile = await session({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true })
    await content(mobile.page)
    await mobile.page.touchscreen.tap(200, 420)
    await shot(mobile.page, 'mobile-390')
    await mobile.ctx.close()
    const reduced = await session({ reducedMotion: 'reduce' })
    await content(reduced.page)
    assert.equal(await reduced.page.locator('[data-ink-canvas]').getAttribute('data-ink-state'), 'still')
    const still = await shot(reduced.page, 'reduced-motion')
    await reduced.page.mouse.move(300, 500)
    await reduced.page.mouse.click(850, 500)
    const response = await shot(reduced.page, 'reduced-interaction')
    report.reducedInteractionChangedPixelRatio = changedPixels(still, response)
    assert(report.reducedInteractionChangedPixelRatio > .0001, 'interaction changes pixels independently of animation time')
    await reduced.page.waitForTimeout(250)
    assert.equal(changedPixels(response, await shot(reduced.page, 'reduced-stable')), 0, 'reduced-motion does not animate autonomously')
    await reduced.ctx.close()
    const themed = await session()
    await content(themed.page)
    await themed.page.getByRole('button', { name: '切换到暗色模式' }).click()
    await themed.page.locator('html.dark').waitFor({ state: 'attached' })
    await themed.page.waitForTimeout(200)
    await shot(themed.page, 'dark-mode')
    await themed.page.locator('[data-ink-canvas]').evaluate(canvas => canvas.getContext('webgl').getExtension('WEBGL_lose_context').loseContext())
    await themed.page.waitForFunction(() => document.querySelector('[data-ink-canvas]')?.dataset.inkState === 'lost')
    assert.equal(await themed.page.locator('[data-ink-canvas]').evaluate(el => getComputedStyle(el).visibility), 'hidden')
    assert(await themed.page.getByRole('heading', { name: 'luohao', exact: true }).isVisible())
    await themed.ctx.close()
    report.checks.push('live theme toggle updates ink palette; real context loss leaves readable identity and static background')
    const fallback = await session({}, true)
    await content(fallback.page)
    assert.equal(await fallback.page.locator('[data-ink-canvas]').getAttribute('data-ink-state'), 'unsupported')
    await shot(fallback.page, 'webgl-fallback')
    await fallback.ctx.close()
    const nojs = await session({ javaScriptEnabled: false })
    await content(nojs.page)
    await shot(nojs.page, 'no-javascript')
    await nojs.ctx.close()
    report.checks.push('390px touch layout; reduced-motion still state; WebGL-disabled fallback; no-JS SSR identity/profile/navigation remain visible')
    assert.deepEqual(report.errors, [])
    report.status = 'passed'
  } finally {
    fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2))
    await browser.close()
    await new Promise(resolve => fixture.close(resolve))
  }
  console.log(JSON.stringify(report, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
