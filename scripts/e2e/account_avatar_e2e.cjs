// Local browser regression with mocked account APIs; no real accounts or uploads.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('Only local frontends are supported')

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext()
    let user = { id: '1', email: 'reader@example.com', display_name: '测试读者', role: 1, avatar_url: '' }
    let uploads = 0
    let failUpload = false
    let emptyReply = false
    const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=', 'base64')
    await context.route('**/api/v1/**', async route => {
      const path = new URL(route.request().url()).pathname
      let body = {}
      if (path.endsWith('/auth/refresh'))
        body = { access_token: 'fixture', user }
      else if (path.endsWith('/user/avatar')) {
        uploads++
        assert.equal(route.request().headers().authorization, 'Bearer fixture')
        assert.ok(Buffer.from(route.request().postDataJSON().data, 'base64').length > 0)
        if (failUpload)
          return route.fulfill({ status: 400, json: { message: 'fixture failure' } })
        if (emptyReply)
          return route.fulfill({ json: {} })
        user = { ...user, avatar_url: '/v1/assets/avatars/fixture.png' }
        body = user
      }
      else if (path.includes('/assets/avatars/'))
        return route.fulfill({ contentType: 'image/png', body: pixel })
      return route.fulfill({ json: body })
    })
    const page = await context.newPage()
    await page.addInitScript(() => {
      window.avatarEncodes = 0
      const original = HTMLCanvasElement.prototype.toBlob
      HTMLCanvasElement.prototype.toBlob = function (...args) {
        window.avatarEncodes++
        if (window.rejectWebp && args[1] === 'image/webp') {
          args[0](null)
          return
        }
        return original.apply(this, args)
      }
    })
    await page.goto(`${base}/settings`)
    const fileInput = page.locator('input[type=file]')
    await page.getByRole('heading', { name: '个人设置' }).waitFor()
    await fileInput.setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: pixel })
    await page.getByRole('button', { name: '上传新头像' }).click()
    await page.getByRole('status').filter({ hasText: '头像已更新' }).waitFor()
    assert.equal(await page.evaluate(() => window.avatarEncodes), 1, 'one upload should encode once')
    assert.equal(uploads, 1)
    await page.waitForFunction(() => document.querySelector('img[alt="头像预览"]')?.naturalWidth > 0)
    await page.evaluate(() => { window.rejectWebp = true })
    await fileInput.setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: pixel })
    await page.getByRole('button', { name: '上传新头像' }).click()
    await page.getByRole('status').filter({ hasText: '头像已更新' }).waitFor()
    assert.equal(await page.evaluate(() => window.avatarEncodes), 3, 'failed webp encoding should retry once')
    assert.equal(uploads, 2)
    await page.reload()
    await page.waitForFunction(() => document.querySelector('img[alt="头像预览"]')?.naturalWidth > 0)
    await fileInput.setInputFiles({ name: 'invalid.txt', mimeType: 'text/plain', buffer: Buffer.from('not an image') })
    await page.getByRole('status').filter({ hasText: '请选择 jpg / png / webp 图片' }).waitFor()
    assert.equal(uploads, 2, 'invalid input must not upload')
    failUpload = true
    await fileInput.setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: pixel })
    await page.getByRole('button', { name: '上传新头像' }).click()
    await page.getByRole('status').filter({ hasText: '图片不符合要求' }).waitFor()
    assert.equal(uploads, 3)
    assert.equal(await page.locator('img[alt="测试读者"]').getAttribute('src'), '/api/v1/assets/avatars/fixture.png', 'failed upload must preserve account avatar')
    failUpload = false
    await page.getByRole('button', { name: '上传新头像' }).click()
    await page.getByRole('status').filter({ hasText: '头像已更新' }).waitFor()
    assert.equal(uploads, 4, 'failed selection should remain available for retry')
    emptyReply = true
    await fileInput.setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: pixel })
    await page.getByRole('button', { name: '上传新头像' }).click()
    await page.getByRole('status').filter({ hasText: '上传失败' }).waitFor()
    assert.equal(await page.locator('img[alt="测试读者"]').getAttribute('src'), '/api/v1/assets/avatars/fixture.png', 'empty reply must preserve account avatar')
    await page.goto(`${base}/admin/posts`)
    await page.getByRole('link', { name: '头像与个人设置' }).waitFor()
    await page.getByRole('link', { name: '头像与个人设置' }).click()
    await page.waitForURL('**/settings')
    await page.goto(base)
    assert.equal(await page.locator('a[href="/rss.xml"], link[type="application/rss+xml"]').count(), 0)
    assert.equal(await page.getByLabel('邮箱地址').count(), 0)
    assert.equal((await context.request.get(`${base}/rss.xml`)).status(), 404)
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto(`${base}/settings`)
    await page.getByRole('heading', { name: '个人设置' }).waitFor()
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'mobile settings must not overflow')
    console.log('PASS: single encoding, fallback, upload, reload, invalid input, error, admin entry, RSS removal, mobile layout')
  }
  finally {
    await browser.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
