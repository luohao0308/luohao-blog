// Read-only local production-build checks for the approved original assembly scene.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname)) throw new Error('Local frontend only')
const output = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/visual-redesign/reference-assembly')
const overlay = '.home-assembly-overlay'
async function main() {
  fs.mkdirSync(output, { recursive: true })
  const fixture = await startFixture({ articles: [
    makeArticle({ id: '1', slug: 'fixture-article', title: '真实文章：并发与一致性' }),
    makeArticle({ id: '2', slug: 'fixture-go', title: '真实文章：Go 请求处理' }),
    makeArticle({ id: '3', slug: 'fixture-nuxt', title: '真实文章：Nuxt 服务端渲染' }),
  ] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const errors = [], checks = []
  async function session(options = {}, init) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light', reducedMotion: 'no-preference', ...options })
    if (init) await ctx.addInitScript(init)
    const page = await ctx.newPage()
    page.setDefaultTimeout(7000)
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (/hydration/i.test(message.text())) errors.push(message.text()) })
    return { ctx, page }
  }
  async function enter(page) {
    await page.goto(base, { waitUntil: 'domcontentloaded' })
    await page.locator(`${overlay} .ad-piece`).first().waitFor({ state: 'attached' })
  }
  async function restored(page) {
    await page.locator(overlay).waitFor({ state: 'detached' })
    assert.equal(await page.locator('.site-shell [inert]').count(), 0, 'real page inert states restored')
    assert.equal(await page.getByRole('button', { name: '跳过开场', exact: true }).count(), 0)
  }
  async function freeze(page, time) {
    await page.evaluate(({ selector, time }) => {
      for (const animation of document.getAnimations()) if (animation.effect?.target?.closest(selector)) {
        animation.pause(); animation.currentTime = time
      }
    }, { selector: overlay, time })
  }
  try {
    const first = await session()
    await enter(first.page)
    await freeze(first.page, 1100)
    assert.equal(await first.page.locator(`${overlay} .ad-piece`).count(), 5)
    assert.equal(await first.page.locator(`${overlay} .ad-tag`).count(), 5)
    assert.equal(await first.page.locator(`${overlay} .assembly-fan-card`).count(), 3, 'approved fan overlays the unchanged original scene')
    assert.equal(await first.page.locator(`${overlay} .screen-wrap > .screen > .hero`).count(), 1)
    assert(await first.page.locator('[data-home-root] h1').textContent(), 'real SSR homepage remains in DOM')
    assert.equal(await first.page.locator('[data-home-card]').count(), 3)
    assert(await first.page.locator(overlay).evaluate(el => el.inert && el.getAttribute('aria-hidden') === 'true'))
    assert(await first.page.locator(`${overlay} a`).evaluateAll(links => links.every(link => link.closest('[inert]'))), 'all decorative cloned links are inert')
    assert.deepEqual(await first.page.evaluate(() => {
      const ids = Array.from(document.querySelectorAll('[id]'), node => node.id)
      return ids.filter((id, index) => ids.indexOf(id) !== index)
    }), [], 'no repeated ids')
    const timings = await first.page.evaluate(selector => document.getAnimations()
      .filter(animation => animation.effect?.target?.matches(`${selector} .ad-piece`))
      .map(animation => ({ timing: animation.effect.getTiming(), frames: animation.effect.getKeyframes() })), overlay)
    assert.equal(timings.length, 5)
    timings.forEach(({ timing, frames }, index) => {
      assert.equal(timing.duration, 2900)
      assert.equal(timing.delay, index * 65)
      assert.equal(timing.easing, 'cubic-bezier(0.25, 0.55, 0.25, 1)')
      assert.deepEqual(frames.map(frame => frame.computedOffset), [0, .24, .6, .92, 1])
      assert.match(frames[0].transform, /rotateX\(54deg\)/)
      assert.match(frames[1].transform, /rotateX\(36deg\)/)
      assert.match(frames[2].transform, /rotateX\(29deg\)/)
    })
    await first.page.screenshot({ path: path.join(output, 'home-1440-reference-peak.png'), fullPage: false })
    await first.page.getByRole('button', { name: '跳过开场', exact: true }).click()
    await restored(first.page)
    await first.page.reload({ waitUntil: 'networkidle' })
    assert.equal(await first.page.locator(overlay).count(), 0, 'same session reload does not replay')
    await first.page.getByRole('link', { name: '阅读文章', exact: true }).click()
    await first.page.waitForURL('**/posts')
    await first.page.getByRole('link', { name: '首页', exact: true }).first().click()
    await first.page.waitForURL(base + '/')
    assert.equal(await first.page.locator(overlay).count(), 0, 'same session route return does not replay')
    await first.page.locator('[data-home-card] a[href="/posts/fixture-article"]').first().click()
    await first.page.waitForURL('**/posts/fixture-article')
    assert.equal(await first.page.locator(overlay).count(), 0)
    await first.ctx.close()
    checks.push('five original pieces, original timing/keyframes, inert decorative scene, SSR content, skip, once per session, list/article navigation')

    const natural = await session()
    await enter(natural.page)
    await natural.page.getByRole('button', { name: '跳过开场', exact: true }).focus()
    assert.equal(await natural.page.locator(overlay).count(), 1, 'skip focus does not dismiss the scene')
    const started = Date.now()
    await restored(natural.page)
    assert.equal(await natural.page.evaluate(() => document.activeElement.id), 'main-content', 'focused skip returns to real content on completion')
    const elapsed = Date.now() - started
    assert(elapsed > 2300 && elapsed < 4000, `natural completion about 3.2s: ${elapsed}ms`)
    await natural.ctx.close()
    checks.push(`natural completion ${elapsed}ms after scene observed`)

    for (const key of ['Enter', 'Space']) {
      const keyboard = await session()
      await enter(keyboard.page)
      await keyboard.page.getByRole('button', { name: '跳过开场', exact: true }).focus()
      await keyboard.page.keyboard.press(key)
      await restored(keyboard.page)
      assert.equal(await keyboard.page.evaluate(() => document.activeElement.id), 'main-content')
      await keyboard.ctx.close()
    }
    checks.push('keyboard Enter/Space skip retains focus and returns to content')

    const dark = await session({ colorScheme: 'dark' })
    await enter(dark.page)
    assert(await dark.page.locator('html').evaluate(element => element.classList.contains('dark')))
    assert.equal(await dark.page.locator(`${overlay} .screen-wrap > .screen .site-header`).evaluate(element => getComputedStyle(element).backgroundColor), 'rgba(0, 0, 0, 0)', 'site theme does not recolor reference artwork')
    await dark.page.keyboard.press('Escape')
    await restored(dark.page)
    assert(await dark.page.locator('html').evaluate(element => element.classList.contains('dark')))
    await dark.ctx.close()
    checks.push('reference artwork remains white and the real dark preference is preserved')

    for (const action of ['escape', 'pointer', 'resize', 'dynamic-reduce', 'hidden', 'route']) {
      const current = await session()
      await enter(current.page)
      if (action === 'escape') await current.page.keyboard.press('Escape')
      if (action === 'pointer') await current.page.mouse.click(20, 100)
      if (action === 'resize') await current.page.setViewportSize({ width: 1024, height: 900 })
      if (action === 'dynamic-reduce') await current.page.emulateMedia({ reducedMotion: 'reduce' })
      if (action === 'hidden') await current.page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
        document.dispatchEvent(new Event('visibilitychange'))
      })
      if (action === 'route') {
        await current.page.evaluate(async () => {
          const app = document.querySelector('#__nuxt').__vue_app__
          const router = app?.config.globalProperties.$router
          if (!router) throw new Error('Nuxt router unavailable for route-dispose check')
          await router.push('/posts')
        })
        await current.page.waitForURL('**/posts')
      }
      await restored(current.page)
      await current.ctx.close()
      checks.push(`${action} restores the real page`)
    }

    for (const width of [320, 390, 657, 768, 1024, 1440]) {
      const current = await session({ viewport: { width, height: 1000 } })
      await enter(current.page)
      for (const time of [0, 500, 1100, 1800, 2700, 3100]) {
        await freeze(current.page, time)
        assert(await current.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `no document overflow ${width}px at ${time}ms`)
      }
      await freeze(current.page, 1100)
      if (width === 390) await current.page.screenshot({ path: path.join(output, 'home-390-reference-peak.png'), fullPage: false })
      await current.page.keyboard.press('Escape')
      await restored(current.page)
      assert(await current.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `no settled overflow ${width}px`)
      await current.ctx.close()
    }
    checks.push('320/390/657/768/1024/1440 stage clipping without document overflow')

    for (const variant of ['reduce', 'unsupported', 'no-js', 'storage-denied']) {
      const options = variant === 'reduce' ? { reducedMotion: 'reduce' } : variant === 'no-js' ? { javaScriptEnabled: false } : {}
      const init = variant === 'unsupported' ? () => { Element.prototype.animate = undefined } : variant === 'storage-denied' ? () => {
        const get = Storage.prototype.getItem, set = Storage.prototype.setItem
        Storage.prototype.getItem = function (key) { if (key.startsWith('blog:home-entrance')) throw new DOMException('Fixture denied', 'SecurityError'); return get.call(this, key) }
        Storage.prototype.setItem = function (key, value) { if (key.startsWith('blog:home-entrance')) throw new DOMException('Fixture denied', 'SecurityError'); return set.call(this, key, value) }
      } : undefined
      const current = await session(options, init)
      if (variant === 'storage-denied') {
        await enter(current.page)
        await current.page.keyboard.press('Escape')
        await restored(current.page)
        await current.page.getByRole('link', { name: '阅读文章', exact: true }).click()
        await current.page.waitForURL('**/posts')
        await current.page.getByRole('link', { name: '首页', exact: true }).first().click()
        await current.page.waitForURL(base + '/')
        assert.equal(await current.page.locator(overlay).count(), 0, 'memory fallback prevents route replay')
      } else {
        await current.page.goto(base, { waitUntil: 'networkidle' })
        assert.equal(await current.page.locator(overlay).count(), 0)
        assert.equal(await current.page.locator('[data-home-root] [inert]').count(), 0)
        assert(await current.page.locator('[data-home-root] h1').textContent())
        assert.equal(await current.page.locator('[data-home-card]').count(), 3)
      }
      await current.ctx.close()
      checks.push(`${variant} fallback`)
    }
    assert.deepEqual(errors, [], 'no runtime or hydration errors')
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, errors, base, reference: 'output/visual-redesign/motion/white-layers-original.html' }, null, 2))
    console.log(JSON.stringify({ ok: true, checks, output }))
  } finally {
    await browser.close()
    await fixture.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
