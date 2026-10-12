// Local production frontend against TestOfficialAccountHTTPLifecycle only.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('Only local fixture frontends are supported')
async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext({ locale: 'zh-CN' })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const login = await context.request.post(`${base}/api/v1/auth/login`, { data: { email: 'author@example.com', password: 'longenough1' } })
    assert.equal(login.status(), 200)
    await page.goto(`${base}/admin/wechat-sync`)
    await page.getByRole('heading', { name: '自动同步已启用' }).waitFor()
    await page.getByRole('cell', { name: '同步成功', exact: true }).waitFor()
    await page.getByLabel('已发布文章的 slug').fill('wechat-integration')
    await page.getByRole('button', { name: '同步文章', exact: true }).click()
    await page.getByText('已加入同步任务，同一文章版本不会重复创建任务。', { exact: true }).waitFor()
    assert.equal(await page.getByRole('cell', { name: '同步验证', exact: true }).count(), 1, 'same version must not duplicate')
    assert.equal(await page.getByRole('button', { name: '重试', exact: true }).count(), 0, 'successful publication must not retry')
    await page.reload()
    await page.getByRole('cell', { name: '同步成功', exact: true }).waitFor()
    let failListing = true
    await page.route('**/api/v1/admin/wechat-sync/tasks*', async route => {
      if (failListing)
        return route.fulfill({ status: 503, json: { message: 'fixture unavailable' } })
      return route.continue()
    })
    await page.getByRole('button', { name: '刷新状态' }).click()
    await page.getByRole('alert').filter({ hasText: '公众号同步状态加载失败' }).waitFor()
    failListing = false
    await page.getByRole('button', { name: '刷新状态' }).click()
    await page.getByRole('alert').filter({ hasText: '公众号同步状态加载失败' }).waitFor({ state: 'hidden' })
    await page.setViewportSize({ width: 390, height: 844 })
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'mobile admin page must not overflow')
    await page.setViewportSize({ width: 1280, height: 900 })
    if (process.env.BLOG_E2E_SCREENSHOT)
      await page.screenshot({ path: process.env.BLOG_E2E_SCREENSHOT, fullPage: true })
    await page.goto(base)
    await page.getByRole('heading', { name: '在微信里找到我' }).waitFor()
    assert.equal(await page.locator('a[href="/rss.xml"], link[type="application/rss+xml"]').count(), 0)
    await page.getByRole('button', { name: '复制公众号微信号' }).click()
    await page.getByRole('status').filter({ hasText: /微信号已复制|请在微信中搜索/ }).waitFor()
    await page.setViewportSize({ width: 390, height: 844 })
    await page.reload()
    await page.getByRole('heading', { name: '在微信里找到我' }).waitFor()
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'mobile homepage must not overflow')
    assert.deepEqual(errors, [])
    const readerContext = await browser.newContext()
    const readerPage = await readerContext.newPage()
    await readerPage.goto(`${base}/admin/wechat-sync`)
    await readerPage.waitForURL('**/admin/login?redirect=**')
    await readerContext.close()
    console.log('PASS: real auth/BFF, durable success, idempotent enqueue, reload, fetch recovery, public account entry, mobile, anonymous guard')
  }
  finally {
    await browser.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
