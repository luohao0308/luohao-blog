// Mock APIs only; never reads or writes a real account.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13830'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname)) throw new Error('local mock only')
const shots = process.env.BLOG_E2E_SCREENSHOTS
const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=', 'base64')
async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const page = await context.newPage()
  let user = { id: 'fixture', display_name: '示例读者', email: 'reader@example.com', role: 1, email_verified: false, avatar_url: '' }
  let authenticated = true
  let failName = false
  const writes = { name: 0, password: 0, verify: 0, logout: 0 }
  const errors = []
  page.on('pageerror', e => errors.push(e.message))
  await context.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/auth/refresh')) return route.fulfill(authenticated ? { json: { access_token: 'fixture', user } } : { status: 401, json: {} })
    if (path.endsWith('/user/profile')) {
      writes.name++
      if (failName) return route.fulfill({ status: 503, json: {} })
      user = { ...user, display_name: route.request().postDataJSON().display_name }
      return route.fulfill({ json: user })
    }
    if (path.endsWith('/auth/email-verification/request')) { writes.verify++; return route.fulfill({ status: 429, json: { reason: 'AUTH_EMAIL_REQUEST_COOLDOWN' } }) }
    if (path.endsWith('/auth/update-password')) {
      writes.password++
      assert.deepEqual(route.request().postDataJSON(), { old_password: 'old-password', new_password: 'new-password' })
      return route.fulfill({ json: {} })
    }
    if (path.endsWith('/auth/logout')) { writes.logout++; authenticated = false; return route.fulfill({ json: {} }) }
    return route.fulfill({ json: {} })
  })
  async function select(name) {
    await page.getByRole('tab', { name, exact: true }).click()
    assert.equal(await page.getByRole('tabpanel').count(), 1, 'only active panel is accessible')
  }
  async function shot(name) {
    if (shots) { fs.mkdirSync(shots, { recursive: true }); await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true }) }
  }
  try {
    await page.goto(`${base}/settings?new_account=1`)
    await page.getByRole('tab', { name: '邮箱验证', exact: true, selected: true }).waitFor({ timeout: 8000 })
    await page.getByText(/账号已创建。/).waitFor()
    await page.goto(`${base}/settings`)
    await page.getByRole('tab', { name: '个人资料', exact: true, selected: true }).waitFor({ timeout: 8000 })
    assert.equal(await page.locator('main').evaluate(el => el.getBoundingClientRect().width), 896)
    assert.equal(await page.locator('.settings-page').evaluate(el => el.getBoundingClientRect().width), 848)
    await shot('settings-desktop')
    await page.getByRole('textbox', { name: '昵称', exact: true }).fill('尚未保存')
    await page.locator('input[type=file]').setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: pixel })
    const preview = await page.locator('img[alt="头像预览"]').getAttribute('src')
    await select('账号安全')
    await page.getByPlaceholder('当前密码', { exact: true }).fill('old-password')
    await page.getByPlaceholder('新密码（至少 8 位）', { exact: true }).fill('new-password')
    await page.getByPlaceholder('确认新密码', { exact: true }).fill('mismatch')
    await page.getByRole('button', { name: '修改密码', exact: true }).click()
    await page.getByText('两次输入的新密码不一致', { exact: true }).waitFor()
    assert.equal(writes.password, 0)
    await shot('settings-desktop-security')
    await select('邮箱验证')
    await page.getByRole('button', { name: '发送验证请求' }).click()
    await page.getByText('请求过于频繁，请稍后重试。', { exact: true }).waitFor()
    assert.equal(writes.verify, 1)
    await shot('settings-desktop-email')
    await select('个人资料')
    assert.equal(await page.getByRole('textbox', { name: '昵称', exact: true }).inputValue(), '尚未保存')
    assert.equal(await page.locator('img[alt="头像预览"]').getAttribute('src'), preview)
    failName = true
    await page.getByRole('button', { name: '保存昵称', exact: true }).click()
    await page.getByText('保存失败，请稍后再试', { exact: true }).waitFor()
    assert.equal(await page.getByRole('textbox', { name: '昵称', exact: true }).inputValue(), '尚未保存')
    failName = false
    await page.getByRole('button', { name: '保存昵称', exact: true }).click()
    await page.getByText('昵称已更新', { exact: true }).waitFor()
    await page.getByRole('region', { name: '账号概览' }).getByRole('heading', { name: '尚未保存' }).waitFor()
    assert.equal(writes.name, 2)
    await page.getByRole('tab', { name: '个人资料', exact: true }).focus()
    await page.keyboard.press('ArrowLeft')
    assert.equal(await page.getByRole('tab', { name: '账号安全', exact: true }).getAttribute('aria-selected'), 'true')
    await page.keyboard.press('Home')
    await page.keyboard.press('Tab')
    assert.equal(await page.getByRole('button', { name: '选择图片' }).evaluate(el => el === document.activeElement), true)
    await page.getByRole('tab', { name: '个人资料', exact: true }).focus()
    await page.keyboard.press('End')
    assert.equal(await page.getByPlaceholder('当前密码', { exact: true }).inputValue(), 'old-password')
    assert.equal(await page.getByPlaceholder('确认新密码', { exact: true }).inputValue(), 'mismatch')
    for (const width of [320, 390, 768]) {
      await page.setViewportSize({ width, height: 844 })
      for (const name of ['个人资料', '邮箱验证', '账号安全']) { await select(name); assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${width} ${name}`) }
    }
    user = { ...user, display_name: '示例读者' }
    await page.goto(`${base}/settings`)
    await page.getByRole('tab', { name: '个人资料', exact: true }).waitFor()
    await page.setViewportSize({ width: 390, height: 844 })
    await shot('settings-mobile')
    await select('账号安全'); await shot('settings-mobile-security')
    await select('邮箱验证'); await shot('settings-mobile-email')
    await page.evaluate(() => {
      const bar = document.querySelector('[role=tablist]')
      for (let i = 0; i < 8; i++) { const item = document.createElement('button'); item.textContent = `测试分组${i}`; bar.append(item) }
      bar.lastElementChild.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    })
    assert.ok(await page.getByRole('tablist').evaluate(el => el.scrollLeft > 0 && el.scrollWidth > el.clientWidth))
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    user = { ...user, email_verified: true, display_name: '很长的昵称用于检查布局'.repeat(2), email: 'long-reader-name-for-layout-check@example.com' }
    await page.goto(`${base}/settings#email`)
    await page.reload()
    await page.getByRole('tab', { name: '邮箱验证', exact: true, selected: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: '发送验证请求' }).count(), 0)
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.evaluate(() => document.documentElement.classList.add('dark'))
    await shot('settings-mobile-dark-email')
    await page.setViewportSize({ width: 1440, height: 1000 })
    await select('个人资料'); await shot('settings-desktop-dark')
    await page.goto(`${base}/settings#password`)
    await page.getByRole('tab', { name: '账号安全', exact: true, selected: true }).waitFor()
    await page.getByPlaceholder('当前密码', { exact: true }).fill('old-password')
    await page.getByPlaceholder('新密码（至少 8 位）', { exact: true }).fill('new-password')
    await page.getByPlaceholder('确认新密码', { exact: true }).fill('new-password')
    await page.getByRole('button', { name: '修改密码', exact: true }).click()
    await page.waitForURL('**/login?password_changed=1')
    assert.equal(writes.password, 1); assert.equal(writes.logout, 1)
    await page.goto(`${base}/settings`)
    await page.waitForURL(url => url.pathname === '/login' && url.searchParams.get('redirect') === '/settings')
    assert.deepEqual(errors, [])
    console.log('PASS settings: width/groups/drafts/file/keyboard/deep-links/name-retry/email/password/logout/320-390-768/dark/long-content/tab-scroll/anonymous')
  }
  finally { await context.close(); await browser.close() }
}
main().catch(err => { console.error(err); process.exitCode = 1 })
