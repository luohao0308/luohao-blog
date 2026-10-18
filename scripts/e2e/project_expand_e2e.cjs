// Local fixture only: real route expansion, native links, and motion fallbacks.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const http = require('node:http')
const projects = ['fixture-project', 'second-project'].map((slug, index) => ({
  id: String(index + 1), slug, name: index ? '探索工具' : '个人博客', summary: '项目卡片展开浏览器验证',
  description: '真实项目详情内容。', stack: ['Nuxt', 'Go'], role: '独立开发', emoji: '🧩', year: '2026',
  featured: true, published: true, sort: index, cover_url: '/v1/assets/fixture.svg', screenshots: [],
  problem: '记录探索过程', approach: '构建个人博客', outcome: '公开分享',
  repo_url: 'https://github.com/example/fixture', live_url: 'https://example.com/demo',
}))
async function startFixture() {
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, 'http://127.0.0.1').pathname
    res.setHeader('Content-Type', 'application/json')
    const json = (value, status = 200) => { res.statusCode = status; res.end(JSON.stringify(value)) }
    if (req.method !== 'GET') return json({ message: 'Read-only fixture' }, 405)
    if (pathname === '/v1/assets/fixture.svg') {
      res.setHeader('Content-Type', 'image/svg+xml')
      return res.end('<svg xmlns="http://www.w3.org/2000/svg" width="800" height="450"><rect width="800" height="450" fill="#e5edf5"/><circle cx="400" cy="225" r="100" fill="#3c5d85"/></svg>')
    }
    if (pathname === '/v1/projects/list') return json({ projects })
    if (pathname.startsWith('/v1/projects/')) {
      const project = projects.find(item => item.slug === decodeURIComponent(pathname.split('/')[3]))
      return project ? json(project) : json({ message: 'Not found' }, 404)
    }
    if (pathname.includes('/auth/')) return json({ message: 'Anonymous fixture' }, 401)
    if (pathname.includes('/articles')) return json({ articles: [] })
    if (pathname.includes('/categories')) return json({ categories: [] })
    return json({ message: 'Unknown fixture route' }, 404)
  })
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(Number(process.env.BLOG_E2E_FIXTURE_PORT || 13007), '127.0.0.1', resolve) })
  return { close: () => new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve())) }
}
process.env.BLOG_E2E_FIXTURE_PORT ||= '13007'
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13006'
assert(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname), 'localhost only')
const out = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/project-expand')
const report = { checks: [], errors: [] }
async function main() {
  fs.mkdirSync(out, { recursive: true })
  const fixture = await startFixture()
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  async function session(options = {}, noApi = false) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, ...options })
    await context.addInitScript(({ noApi }) => {
      window.__expansionCalls = 0
      if (noApi) { document.startViewTransition = undefined; return }
      const original = document.startViewTransition?.bind(document)
      if (!original) return
      document.startViewTransition = function (callback) {
        window.__expansionCalls++
        const transition = original(callback)
        transition.ready.then(() => {
          window.__expansionReady = true
          const allAnimations = document.getAnimations()
          const animations = allAnimations.filter(a => a.animationName === 'article-expand')
          window.__expansionEvidence = animations.map(a => ({ name: a.animationName, duration: a.effect.getTiming().duration, keyframes: a.effect.getKeyframes() }))
          window.__sharedEvidence = allAnimations.filter(a => a.effect?.pseudoElement?.includes('article-content')).map(a => ({ name: a.animationName, pseudo: a.effect.pseudoElement, duration: a.effect.getTiming().duration }))
          if (window.__freezeExpansion) allAnimations.forEach(a => { a.pause(); a.currentTime = 375 })
        }).catch(() => {})
        return transition
      }
    }, { noApi })
    const page = await context.newPage()
    page.on('pageerror', err => report.errors.push(err.message))
    page.setDefaultTimeout(12000)
    return { page, context }
  }
  async function list(page) {
    await page.goto(base + '/projects', { waitUntil: 'networkidle' })
    await page.locator('[data-project-card]').first().waitFor()
  }
  async function settled(page) {
    await page.waitForURL('**/projects/fixture-project')
    await page.locator('[data-project-detail]').waitFor()
    await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding'))
  }
  try {
    const { page, context } = await session()
    await list(page)
    assert(await page.evaluate(() => !!document.startViewTransition), 'browser must support native View Transition for this acceptance test')
    await page.evaluate(() => { window.__freezeExpansion = true })
    await page.locator('[data-project-card]').first().locator('p').first().click()
    await page.waitForFunction(() => window.__expansionReady)
    const middle = await page.evaluate(() => ({ calls: window.__expansionCalls, animations: window.__expansionEvidence, expanding: document.documentElement.hasAttribute('data-article-expanding'), shared: window.__sharedEvidence, origin: document.documentElement.style.getPropertyValue('--article-expand-from') }))
    assert.equal(middle.calls, 1)
    assert(middle.expanding)
    assert.match(middle.origin, /inset\(.+round 24px\)/)
    assert.equal(middle.animations.length, 1, 'real pseudo-element expansion exists')
    assert.equal(middle.animations[0].duration, 750)
    assert(middle.animations[0].keyframes.some(frame => frame.clipPath?.includes('inset')))
    report.middle = middle
    await page.screenshot({ path: path.join(out, 'expansion-middle.png') })
    await page.evaluate(() => document.getAnimations().forEach(a => a.play()))
    await settled(page)
    await page.screenshot({ path: path.join(out, 'project-detail.png'), fullPage: true })
    report.checks.push('Background click: exactly one native 750 ms card-origin clip expansion into real project route')
    await page.goBack({ waitUntil: 'networkidle' })
    await page.locator('[data-project-card]').first().waitFor()
    assert.equal(await page.locator('html[data-article-expanding]').count(), 0)
    report.checks.push('Browser back restores project list without stale expansion')
    // A new document also resets the instrumentation counter and paused state.
    await list(page)
    await page.locator('[data-project-card] h2 a').first().click()
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Title link expands into existing detail route')
    await list(page)
    await page.locator('[data-project-card] img').first().click()
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Cover link expands into existing detail route')
    await list(page)
    await page.locator('[data-project-card] h2 a').first().focus()
    await page.keyboard.press('Enter')
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Keyboard Enter on native title link expands')
    await list(page)
    const modified = await page.locator('[data-project-card] h2 a').first().evaluate(link => {
      const results = []
      for (const init of [{ ctrlKey: true }, { metaKey: true }, { button: 1 }]) {
        document.addEventListener('click', event => { results.push(event.defaultPrevented); event.preventDefault() }, { once: true })
        link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ...init }))
      }
      return results
    })
    assert.deepEqual(modified, [false, false, false])
    assert.equal(await page.evaluate(() => window.__expansionCalls), 0)
    report.checks.push('Ctrl/Meta/middle clicks are not intercepted (native default sampled before test cancels navigation)')
    await page.locator('[data-project-card] h2 a').first().click()
    await settled(page)
    const external = await page.locator('[data-project-detail] header a[target="_blank"]').evaluateAll(links => links.map(link => ({ href: link.href, rel: link.rel, target: link.target, tag: link.tagName })))
    assert.equal(external.length, 2)
    assert(external.every(link => link.tag === 'A' && link.rel.includes('noopener') && link.target === '_blank' && link.href.startsWith('https://')))
    // Sample interception on outgoing controls without making external requests.
    const externalPrevented = await page.locator('[data-project-detail] header a[target="_blank"]').first().evaluate(link => {
      let prevented
      document.addEventListener('click', event => { prevented = event.defaultPrevented; event.preventDefault() }, { once: true })
      link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
      return prevented
    })
    assert.equal(externalPrevented, false)
    report.checks.push('Source/demo remain secure external anchors; outgoing click is not intercepted')
    const beforeRelated = await page.evaluate(() => window.__expansionCalls)
    await page.locator('[data-project-detail] [data-project-card] h2 a').first().click()
    await page.waitForURL('**/projects/second-project')
    await page.getByRole('heading', { name: '探索工具', exact: true }).waitFor()
    await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding'))
    assert.equal(await page.evaluate(() => window.__expansionCalls), beforeRelated + 1)
    report.checks.push('Related project card expands into different project route and updates detail')
    await context.close()
    for (const mode of ['reduce', 'no-api']) {
      const fallback = await session(mode === 'reduce' ? { reducedMotion: 'reduce' } : {}, mode === 'no-api')
      await list(fallback.page)
      await fallback.page.locator('[data-project-card] h2 a').first().click()
      await settled(fallback.page)
      assert.equal(await fallback.page.evaluate(() => window.__expansionCalls), 0)
      report.checks.push(`${mode}: normal project navigation without transition`)
      await fallback.context.close()
    }
    const mobile = await session({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true })
    await list(mobile.page)
    await mobile.page.locator('[data-project-card] img').first().tap()
    await settled(mobile.page)
    assert(await mobile.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
    report.checks.push('Mobile touch cover opens detail without horizontal overflow')
    await mobile.page.screenshot({ path: path.join(out, 'project-mobile.png'), fullPage: true })
    await mobile.context.close()
    const noJs = await browser.newContext({ javaScriptEnabled: false })
    const ssr = await noJs.newPage()
    await ssr.goto(base + '/projects', { waitUntil: 'load' })
    await ssr.locator('[data-project-card] h2 a').first().click()
    await ssr.waitForURL('**/projects/fixture-project')
    await ssr.locator('[data-project-detail]').waitFor()
    report.checks.push('No JavaScript: SSR title anchor reaches project detail')
    await noJs.close()
    assert.deepEqual(report.errors, [])
  } finally {
    fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2))
    await browser.close()
    await fixture.close()
  }
  console.log(JSON.stringify(report, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
