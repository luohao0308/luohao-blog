// Local production-build regression. No real account, model or production calls.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname)) throw new Error('Local frontend only')
const output = path.resolve('output/visual-redesign/implementation')
async function main() {
  fs.mkdirSync(output, { recursive: true })
  const fixture = await startFixture({ articles: [
    makeArticle({ id: '1', slug: 'fixture-article', title: '真实文章：并发与一致性' }),
    makeArticle({ id: '2', slug: 'fixture-go', title: '真实文章：Go 请求处理' }),
    makeArticle({ id: '3', slug: 'fixture-nuxt', title: '真实文章：Nuxt 服务端渲染' }),
  ] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const errors = []
  const checks = []
  async function context(options = {}) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light', ...options })
    // Session restore is a harmless fixture; no real login or POST is sent.
    await ctx.route('**/api/v1/auth/**', route => route.fulfill({ json: { access_token: 'fixture', user: { id: '2', email: 'reader@example.com', display_name: '测试读者', role: 2 } } }))
    const page = await ctx.newPage()
    page.setDefaultTimeout(7000)
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (/hydration/i.test(message.text())) errors.push(message.text()) })
    return { ctx, page }
  }
  function hitPoint(link) {
    return link.evaluate((element) => {
      for (const bounds of element.getClientRects()) {
        for (const ratio of [.1, .25, .5, .75, .9]) {
          const x = bounds.left + bounds.width * ratio
          const y = bounds.top + bounds.height / 2
          const hit = document.elementFromPoint(x, y)
          if (hit && element.contains(hit)) return { x, y }
        }
      }
      return null
    })
  }
  try {
    const { ctx, page } = await context({ reducedMotion: 'no-preference' })
    await page.goto(base, { waitUntil: 'domcontentloaded' })
    assert.equal(await page.getByRole('heading', { level: 1, name: /把想法写成代码/ }).count(), 1, 'approved homepage replaces the old splash')
    await page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
    assert.equal(await page.locator('[data-home-card]').count(), 3, 'real API articles are the animated cards')
    await page.evaluate(() => {
      for (const animation of document.getAnimations()) {
        if (animation.effect?.target?.closest('[data-home-root]')) { animation.pause(); animation.currentTime = 1100 }
      }
    })
    const geometry = await page.locator('[data-home-card]').evaluateAll((elements) => {
      const centers = elements.map(element => { const rect = element.getBoundingClientRect(); return rect.x + rect.width / 2 })
      const resting = elements.map(element => element.offsetLeft + element.offsetWidth / 2)
      return { peakSpread: Math.max(...centers) - Math.min(...centers), restingSpread: Math.max(...resting) - Math.min(...resting) }
    })
    assert(geometry.peakSpread < geometry.restingSpread * .8, 'peak must visibly concentrate real cards into a fan')
    await page.screenshot({ path: path.join(output, 'home-1440-mid.png'), fullPage: true })
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: '跳过开场', exact: true }).waitFor({ state: 'hidden' })
    assert.equal(await page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    assert.equal(await page.locator('[data-home-root] [inert]').count(), 0)
    assert.equal(await page.locator('[data-home-root] #layers, [data-home-root] .fan-card').count(), 0, 'production animates real DOM without preview clones')
    await page.reload({ waitUntil: 'networkidle' })
    assert.equal(await page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0, 'same session reload is static')
    await page.getByRole('link', { name: '阅读文章', exact: true }).click()
    await page.waitForURL('**/posts')
    await page.getByRole('link', { name: '首页', exact: true }).first().click()
    await page.waitForURL(base + '/')
    await page.getByRole('heading', { level: 1, name: /把想法写成代码/ }).waitFor()
    assert.equal(await page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0, 'returning home does not replay')
    await page.screenshot({ path: path.join(output, 'home-1440-settled.png'), fullPage: true })
    checks.push('real content, hybrid, Escape, SSR hydration, no clones, once per session')
    await ctx.close()

    const reduced = await context({ reducedMotion: 'reduce', viewport: { width: 390, height: 844 } })
    await reduced.page.goto(base, { waitUntil: 'networkidle' })
    assert.equal(await reduced.page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0)
    assert.equal(await reduced.page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    assert(await reduced.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await reduced.page.screenshot({ path: path.join(output, 'home-390-reduced.png'), fullPage: true })
    await reduced.ctx.close()
    checks.push('reduced motion remains immediately readable on mobile')

    for (const width of [320, 390, 768, 900, 1024, 1280, 1440]) {
      const mobile = await context({ viewport: { width, height: 900 }, reducedMotion: 'no-preference' })
      await mobile.page.goto(base, { waitUntil: 'domcontentloaded' })
      await mobile.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
      await mobile.page.evaluate(() => {
        for (const animation of document.getAnimations()) {
          if (animation.effect?.target?.closest('[data-home-root]')) { animation.pause(); animation.currentTime = 1100 }
        }
      })
      assert(await mobile.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `no peak overflow at ${width}`)
      assert(await mobile.page.locator('[data-home-card]').evaluateAll(elements => elements.every((element) => {
        const bounds = element.getBoundingClientRect()
        return bounds.left >= -1 && bounds.right <= innerWidth + 1
      })), `all fanned cards remain in view at ${width}`)
      await mobile.page.getByRole('button', { name: '跳过开场', exact: true }).click()
      await mobile.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor({ state: 'hidden' })
      assert(await mobile.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `no page overflow at ${width}`)
      await mobile.page.getByRole('link', { name: '阅读文章', exact: true }).click()
      await mobile.page.waitForURL('**/posts')
      await mobile.page.getByLabel('搜索文章', { exact: true }).fill('一致性')
      await mobile.page.waitForTimeout(600)
      assert.equal(await mobile.page.getByRole('link', { name: /真实文章：并发与一致性/ }).count(), 1)
      await mobile.page.goto(`${base}/posts/fixture-article`, { waitUntil: 'networkidle' })
      assert.equal(await mobile.page.locator('[data-home-root]').count(), 0, 'reading page never mounts the home animation')
      assert.equal(await mobile.page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0)
      await mobile.ctx.close()
    }
    checks.push('320/390/768/900/1024/1280/1440 peak bounds, skip, navigation, search and stable reading')

    const blocked = await context({ reducedMotion: 'no-preference' })
    await blocked.ctx.addInitScript(() => {
      const original = Storage.prototype.setItem
      Storage.prototype.setItem = function (key, value) { if (key.startsWith('blog:home-entrance')) throw new DOMException('Fixture storage denied', 'SecurityError'); return original.call(this, key, value) }
    })
    await blocked.page.goto(base, { waitUntil: 'domcontentloaded' })
    await blocked.page.getByRole('button', { name: '跳过开场', exact: true }).click()
    assert.equal(await blocked.page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    await blocked.ctx.close()
    checks.push('storage write refusal never blocks content')

    const unsupported = await context({ reducedMotion: 'no-preference' })
    await unsupported.ctx.addInitScript(() => { Element.prototype.animate = undefined })
    await unsupported.page.goto(base, { waitUntil: 'networkidle' })
    assert.equal(await unsupported.page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0)
    assert.equal(await unsupported.page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    await unsupported.ctx.close()
    checks.push('unsupported WAAPI shows static content')

    const changed = await context({ reducedMotion: 'no-preference' })
    await changed.page.goto(base, { waitUntil: 'domcontentloaded' })
    await changed.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
    await changed.page.emulateMedia({ reducedMotion: 'reduce' })
    await changed.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor({ state: 'hidden' })
    assert.equal(await changed.page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    await changed.ctx.close()
    checks.push('changing reduced-motion cancels a running entrance')

    const resized = await context({ reducedMotion: 'no-preference' })
    await resized.page.goto(base, { waitUntil: 'domcontentloaded' })
    await resized.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
    await resized.page.setViewportSize({ width: 768, height: 900 })
    await resized.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor({ state: 'hidden' })
    assert.equal(await resized.page.locator('[data-home-root]').getAttribute('data-home-motion'), 'idle')
    assert(await resized.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await resized.ctx.close()
    checks.push('resize cancels obsolete animation geometry')

    // Real coordinates avoid locator.click's retries masking pointer target loss.
    for (const index of [0, 1, 2, 'hero']) {
      const active = await context({ reducedMotion: 'no-preference' })
      await active.page.goto(base, { waitUntil: 'domcontentloaded' })
      await active.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
      await active.page.evaluate(() => {
        for (const animation of document.getAnimations()) {
          if (animation.effect?.target?.closest('[data-home-root]')) { animation.pause(); animation.currentTime = 1100 }
        }
      })
      const link = index === 'hero'
        ? active.page.getByRole('link', { name: '阅读文章', exact: true })
        : active.page.locator('[data-home-card]').nth(index).locator('h2 a')
      const point = await hitPoint(link)
      assert(point, `animated link ${index} has a visible hit area`)
      const target = new URL(await link.getAttribute('href'), base).href
      await active.page.mouse.click(point.x, point.y)
      await active.page.waitForURL(target)
      assert.equal(active.page.url(), target, `first physical click activates ${index}`)
      await active.ctx.close()
    }
    checks.push('first physical mouse click activates all three fanned titles and hero CTA')

    const touch = await context({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 }, reducedMotion: 'no-preference' })
    await touch.page.goto(base, { waitUntil: 'domcontentloaded' })
    await touch.page.getByRole('button', { name: '跳过开场', exact: true }).waitFor()
    await touch.page.evaluate(() => {
      for (const animation of document.getAnimations()) {
        if (animation.effect?.target?.closest('[data-home-root]')) { animation.pause(); animation.currentTime = 1100 }
      }
    })
    const touchPoint = await hitPoint(touch.page.getByRole('link', { name: '阅读文章', exact: true }))
    assert(touchPoint)
    await touch.page.touchscreen.tap(touchPoint.x, touchPoint.y)
    await touch.page.waitForURL('**/posts')
    await touch.ctx.close()
    checks.push('mobile native touch tap activates the moving hero CTA')

    const nojs = await context({ javaScriptEnabled: false })
    await nojs.page.goto(base)
    assert.equal(await nojs.page.getByRole('heading', { level: 1, name: /把想法写成代码/ }).count(), 1)
    assert.equal(await nojs.page.getByRole('link', { name: /真实文章：并发与一致性/ }).count(), 1)
    assert.equal(await nojs.page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0)
    await nojs.ctx.close()
    checks.push('no-JavaScript SSR has real content and links')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify({ checks, errors }, null, 2))
    console.log('PASS', checks)
  } finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
