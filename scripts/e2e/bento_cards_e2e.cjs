// Local-only surface and native-navigation acceptance; uses the shared read-only fixture.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
process.env.BLOG_E2E_FIXTURE_PORT ||= '13007'
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13006'
assert(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname), 'localhost only')
const out = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/bento-cards')
const report = { checks: [], surfaces: {}, gaps: [], errors: [] }
const cardSelector = '.bento-card'
async function surface(card) {
  return card.evaluate(el => {
    const css = getComputedStyle(el), before = getComputedStyle(el, '::before'), after = getComputedStyle(el, '::after')
    return { radius: css.borderRadius, shadow: css.boxShadow, transform: css.transform,
      transition: css.transition, opacity: css.opacity, visibility: css.visibility,
      before: { opacity: before.opacity, pointerEvents: before.pointerEvents, background: before.backgroundImage },
      after: { opacity: after.opacity, pointerEvents: after.pointerEvents, background: after.backgroundImage },
      x: el.style.getPropertyValue('--card-x'), y: el.style.getPropertyValue('--card-y') }
  })
}
async function main() {
  fs.mkdirSync(out, { recursive: true })
  const fixture = await startFixture({ articles: [makeArticle(), makeArticle({ id: '2', slug: 'fixture-nuxt', title: 'Nuxt 服务端渲染', tags: ['Nuxt', 'Go'] })] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  async function session(options = {}) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light', ...options })
    await ctx.addInitScript(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'fixture-article', title: '并发与一致性', saved_at: '2026-10-01T00:00:00Z' }])))
    const page = await ctx.newPage()
    page.setDefaultTimeout(10000)
    page.on('pageerror', err => report.errors.push(err.message))
    return { ctx, page }
  }
  async function enter(page, route) {
    await page.goto(base + route, { waitUntil: 'networkidle' })
    // Entry motion is independent of the card surface under test.
    const skip = page.getByRole('button', { name: '跳过开场', exact: true })
    if (await skip.count()) await skip.click()
    await page.locator('[data-home-live-assembly]').waitFor({ state: 'detached' })
  }
  async function noOverflow(page) {
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'no horizontal document overflow')
  }
  try {
    // Load the downloaded reference without networking. Compare computed surfaces,
    // not pixels across different product content/layouts.
    const referencePath = process.env.BLOG_BENTO_REFERENCE || '/tmp/blog-bento-reference.html'
    assert(fs.existsSync(referencePath), 'download the user reference before running (BLOG_BENTO_REFERENCE)')
    const reference = await browser.newContext({ viewport: { width: 1440, height: 1000 }, javaScriptEnabled: false })
    const refPage = await reference.newPage()
    await refPage.route('**/*', route => route.request().isNavigationRequest()
      ? route.fulfill({ contentType: 'text/html', body: fs.readFileSync(referencePath, 'utf8') }) : route.abort())
    await refPage.goto('http://127.0.0.1/reference', { waitUntil: 'load' })
    const refCard = refPage.locator('.tile').first()
    report.reference = await surface(refCard)
    await refPage.screenshot({ path: path.join(out, 'reference-1440.png'), fullPage: true })
    await reference.close()

    const desktop = await session()
    const page = desktop.page
    for (const route of ['/', '/posts']) {
      await enter(page, route)
      const card = page.locator('article.bento-card').first()
      await card.waitFor()
      await card.scrollIntoViewIfNeeded()
      await page.mouse.move(1, 1)
      const normal = await surface(card)
      assert.equal(normal.radius, report.reference.radius)
      assert.equal(normal.shadow, report.reference.shadow)
      assert.match(normal.transition, /0\.45s/)
      assert.match(normal.transition, /cubic-bezier\(0\.2, 0\.8, 0\.2, 1\)/)
      const box = await card.boundingBox()
      await page.mouse.move(box.x + 40, box.y + 40)
      await page.waitForTimeout(550)
      const hover = await surface(card)
      assert.match(hover.transform, /-4\)/, 'reference hover lifts exactly 4px')
      assert.notEqual(hover.shadow, normal.shadow)
      assert.equal(hover.before.pointerEvents, 'none')
      assert.equal(hover.after.pointerEvents, 'none')
      assert(await page.locator('main[data-card-spotlight="true"]').count())
      await page.mouse.move(box.x + 100, box.y + 60)
      await page.waitForTimeout(50)
      assert.notEqual((await surface(card)).x, hover.x, 'spotlight follows mouse coordinates')
      report.surfaces[route] = { normal, hover }
      await page.screenshot({ path: path.join(out, route === '/' ? 'home-hover.png' : 'posts-hover.png'), fullPage: true })
      await page.mouse.move(1, 1)
      await page.waitForTimeout(50)
      assert.equal(await page.locator('[data-card-spotlight="true"]').count(), 0, 'leave resets lighting')
      assert.equal((await surface(card)).x, '')
      report.checks.push(`${route}: radius/shadow match reference; hover/spotlight/leave`)
    }
    await page.setViewportSize({ width: 1440, height: 400 })
    const resetCard = page.locator('article.bento-card').first()
    await resetCard.hover()
    await page.waitForTimeout(50)
    await page.evaluate(() => window.scrollBy(0, 20))
    await page.waitForTimeout(50)
    assert.equal(await page.locator('[data-card-spotlight="true"]').count(), 0, 'scroll clears stale coordinates')
    await resetCard.hover()
    await page.waitForTimeout(50)
    await page.setViewportSize({ width: 1400, height: 1000 })
    await page.waitForTimeout(50)
    assert.equal(await page.locator('[data-card-spotlight="true"]').count(), 0, 'resize clears stale coordinates')
    report.checks.push('scroll and resize reset spotlight')
    // Actual Tab navigation rather than relying on hover to expose a link.
    await page.mouse.move(1, 1)
    let focused = false
    for (let i = 0; i < 70; i++) {
      await page.keyboard.press('Tab')
      focused = await page.evaluate(() => !!document.activeElement?.closest('article.bento-card'))
      if (focused) break
    }
    assert(focused, 'article title/tag reachable using keyboard')
    await page.waitForTimeout(550)
    const focusedCard = page.locator('article.bento-card').filter({ has: page.locator(':focus') }).first()
    assert.match((await surface(focusedCard)).transform, /-4\)/)
    await page.screenshot({ path: path.join(out, 'keyboard-focus.png'), fullPage: true })
    const article = page.locator('article.bento-card').filter({ has: page.locator('a[href="/posts/fixture-article"]') }).first()
    await article.locator('a[href="/tags/Go"]').click()
    await page.waitForURL('**/tags/Go')
    await page.locator('article.bento-card a[href="/posts/fixture-article"]').first().click()
    await page.waitForURL('**/posts/fixture-article')
    assert.equal(await page.locator('[role="dialog"]').count(), 0, 'details stay native routes, no modal')
    report.checks.push('keyboard focus lift; tag navigation; article detail route preserved')
    for (const route of ['/tags', '/ranking', '/collections']) {
      await enter(page, route)
      assert(await page.locator(cardSelector).count(), `${route} contains fixture-backed cards`)
      assert.equal((await surface(page.locator(cardSelector).first())).radius, '24px')
      await noOverflow(page)
      await page.screenshot({ path: path.join(out, `${route.slice(1)}.png`), fullPage: true })
      report.checks.push(`${route}: real cards present and styled`)
    }
    await page.getByRole('button', { name: '移除', exact: true }).click()
    await page.locator('li.bento-card').waitFor({ state: 'detached' })
    assert.equal(await page.locator('li.bento-card').count(), 0, 'collection delete still works through overlays')
    report.checks.push('collection removal remains operable')
    for (const route of ['/projects', '/categories']) {
      await enter(page, route)
      await noOverflow(page)
      report.gaps.push(`${route}: shared fixture intentionally returns an empty list; populated card styling not covered`)
    }
    // A declared browser fixture exercises populated categories on client navigation;
    // shared server fixture still covers the empty SSR state above.
    await page.route('**/api/v1/categories/list**', route => route.fulfill({ json: {
      categories: [{ id: '1', slug: 'engineering', name: '工程实践', article_count: 2, sort: 1 }],
    } }))
    await enter(page, '/posts')
    await page.locator('main a[href="/categories"]').first().click()
    const category = page.locator('main a.bento-card[href="/categories/engineering"]')
    await category.waitFor()
    assert.equal((await surface(category)).radius, '24px')
    await category.hover()
    await page.waitForTimeout(550)
    assert.match((await surface(category)).transform, /-4\)/)
    report.checks.push('declared category fixture: populated client card style and hover')
    report.gaps.push('/about and project detail: shared fixture has no profile/project detail payload; supplement with anonymous public-data preview')
    await desktop.ctx.close()
    for (const mode of ['dark', 'reduced', 'touch']) {
      const options = mode === 'touch' ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true }
        : mode === 'reduced' ? { reducedMotion: 'reduce' } : { colorScheme: 'dark' }
      const s = await session(options)
      if (mode === 'dark') await s.ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
      await enter(s.page, '/posts')
      const card = s.page.locator('article.bento-card').first()
      await card.scrollIntoViewIfNeeded()
      const box = await card.boundingBox()
      if (mode === 'touch') await s.page.touchscreen.tap(box.x + 12, box.y + 12)
      else await s.page.mouse.move(box.x + 30, box.y + 30)
      await s.page.waitForTimeout(550)
      const style = await surface(card)
      if (mode !== 'dark') {
        assert.equal(await s.page.locator('[data-card-spotlight="true"]').count(), 0)
        assert(['none', 'matrix(1, 0, 0, 1, 0, 0)'].includes(style.transform))
      }
      else assert(await s.page.locator('html.dark').count(), 'dark preference applied')
      await noOverflow(s.page)
      report.surfaces[mode] = style
      await s.page.screenshot({ path: path.join(out, `${mode}.png`), fullPage: true })
      report.checks.push(`${mode}: visible card surface, controls unobstructed, no overflow`)
      await s.ctx.close()
    }
    const ssr = await session({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } })
    await ssr.page.goto(base + '/posts', { waitUntil: 'load' })
    const ssrCard = ssr.page.locator('article.bento-card').first()
    assert(await ssrCard.isVisible())
    assert.equal((await surface(ssrCard)).opacity, '1')
    assert(await ssrCard.locator('a[href="/posts/fixture-article"]').count())
    await noOverflow(ssr.page)
    await ssr.page.screenshot({ path: path.join(out, 'no-js-390.png'), fullPage: true })
    await ssr.ctx.close()
    report.checks.push('no-JS SSR: card/link visible at mobile width')
    assert.deepEqual(report.errors, [])
    report.ok = true
  }
  finally {
    await browser.close()
    await fixture.close()
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify(report, null, 2) + '\n')
  }
  console.log(JSON.stringify(report))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
