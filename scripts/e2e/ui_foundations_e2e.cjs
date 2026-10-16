// Local-only browser regression. Account APIs are fixtures; no real account mutations.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
const url = new URL(base)
if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || !['http:', 'https:'].includes(url.protocol))
  throw new Error('Only local frontends are supported')
const output = path.resolve('output/ui-refinement/s1')

async function main() {
  fs.mkdirSync(output, { recursive: true })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext({ colorScheme: 'light', viewport: { width: 1280, height: 900 } })
    await context.route('**/api/v1/**', async route => {
      const pathname = new URL(route.request().url()).pathname
      const body = pathname.endsWith('/auth/refresh')
        ? { access_token: 'fixture', user: { id: '2', email: 'reader@example.com', display_name: '测试读者', role: 2, avatar_url: '' } }
        : { articles: [], projects: [], total: 0 }
      await route.fulfill({ json: body })
    })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => {
      // Anonymous session probes may legitimately return 401.
      if (message.type() === 'error' && !/401 \(Unauthorized\)/.test(message.text())) errors.push(message.text())
    })
    await page.goto(`${base}/login?switch=1`)
    await page.getByRole('button', { name: '测试读者 的账号菜单' }).waitFor()
    await page.screenshot({ path: path.join(output, 'initial.png'), fullPage: true })
    const skip = page.getByRole('link', { name: '跳到主要内容', exact: true })
    assert.equal(await skip.count(), 1, 'default layout must expose the skip link')
    await page.keyboard.press('Tab')
    assert.equal(await skip.evaluate(el => el === document.activeElement), true, 'first Tab must reach skip link')
    await page.keyboard.press('Enter')
    await page.waitForFunction(() => document.activeElement?.id === 'main-content')
    await skip.focus()
    await skip.click()
    assert.equal(await page.locator('#main-content').evaluate(el => el === document.activeElement), true, 'click must focus main content')

    const trigger = page.getByRole('button', { name: '测试读者 的账号菜单' })
    await trigger.click()
    assert.equal(await trigger.getAttribute('aria-expanded'), 'true')
    const settings = page.getByRole('link', { name: '个人设置（头像 / 昵称）', exact: true })
    await settings.waitFor()
    assert.equal(await settings.evaluate(el => !!el.closest('nav')), true, 'account links must use navigation disclosure')
    assert.equal(await page.locator('[role=menu], [role=menuitem]').count(), 0)
    await page.keyboard.press('Tab')
    assert.equal(await settings.evaluate(el => el === document.activeElement), true, 'Tab naturally reaches disclosed settings link')
    await page.keyboard.press('Escape')
    assert.equal(await trigger.getAttribute('aria-expanded'), 'false')
    assert.equal(await trigger.evaluate(el => el === document.activeElement), true, 'Escape returns focus to trigger')
    await trigger.click()
    await page.locator('#main-content').click({ position: { x: 5, y: 5 } })
    assert.equal(await trigger.getAttribute('aria-expanded'), 'false', 'outside click closes disclosure')

    async function colors() {
      return page.evaluate(() => {
        const root = getComputedStyle(document.documentElement)
        const main = getComputedStyle(document.querySelector('#main-content'))
        return {
          tokens: ['--surface-page', '--text-primary', '--accent'].map(name => root.getPropertyValue(name).trim()),
          text: main.color,
          background: getComputedStyle(document.querySelector('#main-content').parentElement).backgroundColor,
        }
      })
    }
    const light = await colors()
    assert.ok(light.tokens.every(Boolean), 'all semantic colors must exist')
    await page.getByRole('button', { name: '切换到暗色模式' }).click()
    await page.waitForFunction(() => document.documentElement.classList.contains('dark'))
    // Wait for existing theme transitions to settle before reading painted colors.
    await page.waitForTimeout(400)
    const dark = await colors()
    light.tokens.forEach((token, index) => assert.notEqual(token, dark.tokens[index], 'semantic colors must change between themes'))
    assert.notEqual(light.text, dark.text, 'painted text must change')
    assert.notEqual(light.background, dark.background, 'painted page background must change')
    await page.reload()
    await page.getByRole('button', { name: '切换到亮色模式' }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('dark')), true, 'dark choice survives reload')
    await page.screenshot({ path: path.join(output, 'dark.png'), fullPage: true })
    await page.getByRole('button', { name: '切换到亮色模式' }).click()
    await page.reload()
    await page.getByRole('button', { name: '切换到暗色模式' }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('dark')), false, 'light choice survives reload')

    // Inject plain controls to exercise shared CSS without coupling to a page's form fields.
    await page.evaluate(() => {
      const fixture = document.createElement('div')
      fixture.id = 'foundation-fixture'
      fixture.innerHTML = '<textarea aria-label="focus textarea"></textarea><select aria-label="focus select"><option>One</option></select><div contenteditable="true" aria-label="focus editor" tabindex="0">Editor</div><div class="animate-pulse">Loading</div>'
      document.querySelector('#main-content').append(fixture)
    })
    for (const label of ['focus textarea', 'focus select', 'focus editor']) {
      const control = page.getByLabel(label)
      await control.focus()
      await page.keyboard.press('Tab')
      await page.keyboard.press('Shift+Tab')
      const outline = await control.evaluate(el => {
        const style = getComputedStyle(el)
        return { focused: el === document.activeElement, width: parseFloat(style.outlineWidth), style: style.outlineStyle, color: style.outlineColor }
      })
      assert.equal(outline.focused, true)
      assert.ok(outline.width >= 2 && outline.style !== 'none' && outline.color !== 'rgba(0, 0, 0, 0)', `${label} needs visible keyboard focus`)
    }
    await page.emulateMedia({ reducedMotion: 'reduce' })
    assert.equal(await page.locator('#foundation-fixture .animate-pulse').evaluate(el => getComputedStyle(el).animationName), 'none', 'reduce preference disables pulse')
    await page.locator('#foundation-fixture').evaluate(el => el.remove())
    await page.setViewportSize({ width: 320, height: 800 })
    await page.screenshot({ path: path.join(output, 'mobile-320.png'), fullPage: true })
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), '320px page must not overflow horizontally')
    assert.deepEqual(errors, [], 'browser must have no unexpected console or runtime errors')
    console.log('PASS: skip link, disclosure, themes, focus, reduced motion, 320px layout')
    const adminContext = await browser.newContext({ colorScheme: 'dark', viewport: { width: 1280, height: 900 } })
    await adminContext.route('**/api/v1/**', route => route.fulfill({ json: route.request().url().includes('/auth/refresh')
      ? { access_token: 'fixture', user: { id: '1', email: 'author@example.com', display_name: '作者', role: 1, avatar_url: '' } }
      : { articles: [], categories: [], total: 0 } }))
    const ssrResponse = await adminContext.request.get(`${base}/admin/posts`)
    const ssrHtml = await ssrResponse.text()
    assert.equal(ssrResponse.status(), 200)
    for (const style of ['n-button', 'n-base-icon', 'n-data-table'])
      assert.ok(ssrHtml.includes(`cssr-id="${style}"`), `${style} styles must exist before hydration`)
    const admin = await adminContext.newPage()
    admin.on('pageerror', error => errors.push(error.message))
    admin.on('console', message => {
      if (/hydration.*mismatch|contains mismatches/i.test(message.text())) errors.push(message.text().slice(0, 300))
    })
    await admin.goto(`${base}/admin/posts`)
    await admin.getByRole('heading', { name: '文章管理', exact: true }).waitFor()
    await admin.getByText('作者', { exact: true }).waitFor()
    await admin.waitForFunction(() => {
      const link = document.querySelector('a[href="/admin/posts/new"]')
      return link && getComputedStyle(link).backgroundColor === 'rgb(147, 197, 253)'
    }, undefined, { timeout: 10000 })

    assert.equal(await admin.getByRole('link', { name: '跳到主要内容', exact: true }).count(), 1)
    await admin.getByRole('link', { name: '跳到主要内容', exact: true }).focus()
    await admin.keyboard.press('Enter')
    await admin.waitForFunction(() => document.activeElement?.id === 'main-content')
    await admin.setViewportSize({ width: 320, height: 800 })
    assert.ok(await admin.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'admin at 320px must not overflow')
    await admin.screenshot({ path: path.join(output, 'admin-mobile-320.png'), fullPage: true })
    assert.deepEqual(errors, [], 'admin theme must not cause color parsing or runtime errors')
    await adminContext.close()
    await context.close()
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
