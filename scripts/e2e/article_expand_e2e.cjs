// Local fixture only: real route expansion, native links, and motion fallbacks.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
process.env.BLOG_E2E_FIXTURE_PORT ||= '13007'
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13006'
assert(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname), 'localhost only')
const out = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/article-expand')
const report = { checks: [], errors: [] }
async function main() {
  fs.mkdirSync(out, { recursive: true })
  const fixture = await startFixture({ articles: [makeArticle({ cover_url: '/v1/assets/fixture.png' }), makeArticle({ id: '2', slug: 'second', title: '另一篇文章' })] })
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
    await page.goto(base + '/posts', { waitUntil: 'networkidle' })
    await page.locator('[data-article-card]').first().waitFor()
  }
  async function settled(page) {
    await page.waitForURL('**/posts/fixture-article')
    await page.locator('[data-article-detail]').waitFor()
    await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding'))
  }
  try {
    const { page, context } = await session()
    await list(page)
    assert(await page.evaluate(() => !!document.startViewTransition), 'browser must support native View Transition for this acceptance test')
    await page.evaluate(() => { window.__freezeExpansion = true })
    await page.locator('[data-article-card]').first().locator('p').click()
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
    await page.screenshot({ path: path.join(out, 'article-detail.png'), fullPage: true })
    report.checks.push('Background click: exactly one native 750 ms card-origin clip expansion into real article route')
    await page.goBack({ waitUntil: 'networkidle' })
    await page.locator('[data-article-card]').first().waitFor()
    assert.equal(await page.locator('html[data-article-expanding]').count(), 0)
    report.checks.push('Browser back restores article list without stale expansion')
    // A new document also resets the instrumentation counter and paused state.
    await list(page)
    await page.locator('[data-article-card] h2 a').first().click()
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Title link expands into existing detail route')
    await list(page)
    await page.locator('[data-article-card] > a').first().click()
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Cover link expands into existing detail route')
    await list(page)
    await page.locator('[data-article-card] h2 a').first().focus()
    await page.keyboard.press('Enter')
    await settled(page)
    assert.equal(await page.evaluate(() => window.__expansionCalls), 1)
    report.checks.push('Keyboard Enter on native title link expands')
    await list(page)
    const modified = await page.locator('[data-article-card] h2 a').first().evaluate(link => {
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
    await page.locator('[data-article-card] a[href^="/tags/"]').first().click()
    await page.waitForURL('**/tags/Go')
    assert.equal(await page.evaluate(() => window.__expansionCalls), 0)
    report.checks.push('Tag navigation remains independent without expansion')
    await context.close()
    const slow = await session()
    await list(slow.page)
    let slowRequests = 0
    await slow.page.route('**/api/v1/articles/fixture-article', async route => {
      slowRequests++
      await new Promise(resolve => setTimeout(resolve, 5200))
      await route.continue()
    })
    await slow.page.locator('[data-article-card] h2 a').first().click()
    await settled(slow.page)
    assert.equal(slowRequests, 1, 'slow client detail request was intercepted')
    assert.equal(await slow.page.evaluate(() => window.__expansionCalls), 1)
    await slow.page.getByRole('heading', { name: '并发与一致性', exact: true }).waitFor()
    report.checks.push('Delayed 5200 ms detail response: waits for actual article, then completes expansion and cleanup')
    await slow.context.close()
    for (const mode of ['reduce', 'no-api']) {
      const fallback = await session(mode === 'reduce' ? { reducedMotion: 'reduce' } : {}, mode === 'no-api')
      await list(fallback.page)
      await fallback.page.locator('[data-article-card] h2 a').first().click()
      await settled(fallback.page)
      assert.equal(await fallback.page.evaluate(() => window.__expansionCalls), 0)
      report.checks.push(`${mode}: normal article navigation without transition`)
      await fallback.context.close()
    }
    const mobile = await session({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true })
    await list(mobile.page)
    await mobile.page.locator('[data-article-card] h2 a').first().tap()
    await settled(mobile.page)
    assert.equal(await mobile.page.evaluate(() => window.__expansionCalls), 1)
    assert(await mobile.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    report.checks.push('390px touch: tap expands into detail without horizontal overflow')
    await mobile.context.close()
    const noJs = await browser.newContext({ javaScriptEnabled: false })
    const ssr = await noJs.newPage()
    await ssr.goto(base + '/posts', { waitUntil: 'load' })
    await ssr.locator('[data-article-card] h2 a').first().click()
    await ssr.waitForURL('**/posts/fixture-article')
    await ssr.locator('[data-article-detail]').waitFor()
    report.checks.push('No JavaScript: SSR title anchor reaches article detail')
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
