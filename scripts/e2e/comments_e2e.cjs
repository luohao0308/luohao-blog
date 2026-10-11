// Run against the disposable TestCommentHTTPModerationLifecycle fixture only.
// NODE_PATH=<bundled modules> BLOG_E2E_BASE_URL=http://127.0.0.1:13000 node scripts/e2e/comments_e2e.cjs
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('This fixture smoke check only supports a local host')
async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const ctx = await browser.newContext({ locale: 'zh-CN' })
    const page = await ctx.newPage()
    const section = page.locator('section[aria-label="评论区"]')
    const initialReply = await ctx.request.get(`${base}/api/v1/articles/ai-delivery-guard/comments?page_size=100`)
    const expectedPublicCount = (await initialReply.json()).comments.length
    assert.ok(expectedPublicCount > 20, 'fixture must exercise public pagination')
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto(`${base}/posts/ai-delivery-guard`)
    await section.getByText('登录后即可发表评论', { exact: false }).waitFor()
    assert.equal(await section.locator('ul.space-y-5 > li').count(), 20)
    await section.getByRole('button', { name: '加载更多评论' }).click()
    await page.waitForFunction(count => document.querySelectorAll('section[aria-label="评论区"] ul.space-y-5 > li').length === count, expectedPublicCount)
    assert.equal(await section.getByRole('button', { name: '加载更多评论' }).count(), 0)
    const login = async email => {
      const response = await ctx.request.post(`${base}/api/v1/auth/login`, { data: { email, password: 'longenough1' } })
      assert.equal(response.status(), 200)
      return (await response.json()).access_token
    }
    await login('reader@example.com')
    await page.reload()
    const input = section.getByPlaceholder('写下你的评论', { exact: false })
    await input.waitFor()
    const marker = `browser-comment-${Date.now()}`
    let failReconciliation = 2
    const publicPageTokens = []
    await page.route('**/api/v1/articles/ai-delivery-guard/comments**', async route => {
      publicPageTokens.push(new URL(route.request().url()).searchParams.get('page_token'))
      if (failReconciliation > 0) {
        failReconciliation--
        return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'fixture unavailable' }) })
      }
      return route.continue()
    })
    await input.fill(marker)
    await section.getByRole('button', { name: '发表评论' }).click()
    await section.getByRole('status').filter({ hasText: '评论已发布' }).waitFor()
    await section.locator('ul.space-y-5').getByText(marker, { exact: true }).waitFor()
    await section.getByText('评论暂时无法加载，请重试。', { exact: true }).waitFor()
    await section.getByRole('button', { name: '重新加载' }).click()
    await section.getByText('评论暂时无法加载，请重试。', { exact: true }).waitFor({ state: 'hidden' })
    assert.equal(publicPageTokens.at(-1), null, 'publication reconciliation retries must reload the first page')
    assert.equal(await section.locator('[aria-label="我的待审核评论"]').count(), 0)
    await input.fill('好👍'.repeat(500))
    assert.equal(Array.from(await input.inputValue()).length, 1000, 'Unicode input must allow 1000 code points')
    await section.getByText('1000 / 1000', { exact: true }).waitFor()
    await input.fill('字'.repeat(1001))
    await section.getByRole('button', { name: '发表评论' }).click()
    await section.getByRole('status').filter({ hasText: '评论最多 1000 字' }).waitFor()
    const pendingMarker = `${marker} https://example.com/docs`
    await input.fill(pendingMarker)
    await section.getByRole('button', { name: '发表评论' }).click()
    await section.locator('[aria-label="我的待审核评论"]').getByText(pendingMarker, { exact: true }).waitFor()
    assert.equal(await section.locator('ul.space-y-5').getByText(pendingMarker, { exact: true }).count(), 0)
    const publicReply = await ctx.request.get(`${base}/api/v1/articles/ai-delivery-guard/comments`)
    assert.equal((await publicReply.json()).comments.some(c => c.content === pendingMarker), false)
    await page.reload()
    assert.equal(await section.getByText(pendingMarker, { exact: true }).count(), 0, 'pending preview is session memory, not public data')
    const adminToken = await login('author@example.com')
    await page.goto(`${base}/admin/comments`, { waitUntil: 'domcontentloaded' })
    await page.locator('.n-data-table tbody tr').first().waitFor()
    await page.waitForFunction(() => document.querySelectorAll('.n-data-table tbody tr').length === 50)
    let failNavigation = 2
    await page.route('**/api/v1/comments?**', async route => {
      if (failNavigation > 0 && new URL(route.request().url()).searchParams.get('page_token')) {
        failNavigation--
        return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'fixture unavailable' }) })
      }
      return route.continue()
    })
    await page.getByRole('button', { name: '下一页' }).click()
    await page.locator('.n-message').filter({ hasText: '评论列表加载失败' }).waitFor()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal(await page.locator('.n-data-table tbody tr').count(), 50, 'failed navigation must preserve current page')
    await page.getByRole('button', { name: '下一页' }).click()
    await page.waitForFunction(() => {
      const rows = document.querySelectorAll('.n-data-table tbody tr')
      return rows.length > 0 && rows.length < 50
    })
    await page.getByRole('button', { name: '上一页' }).click()
    const row = page.locator('.n-data-table tbody tr').filter({ hasText: pendingMarker })
    await row.waitFor()
    await row.getByRole('button', { name: '通过' }).click()
    await page.locator('.n-message').filter({ hasText: '已通过' }).waitFor()
    await page.goto(`${base}/posts/ai-delivery-guard`)
    await section.locator('ul.space-y-5').getByText(pendingMarker, { exact: true }).waitFor()
    await page.screenshot({ path: process.env.BLOG_COMMENT_SCREENSHOT || '/tmp/blog-comment-smoke.png', fullPage: true })
    const all = await ctx.request.get(`${base}/api/v1/comments?page_size=100`, { headers: { Authorization: `Bearer ${adminToken}` } })
    for (const c of (await all.json()).comments.filter(c => c.content.startsWith(marker))) {
      const response = await ctx.request.delete(`${base}/api/v1/comments/${c.id}`, { headers: { Authorization: `Bearer ${adminToken}` } })
      assert.equal(response.status(), 200)
    }
    assert.deepEqual(errors, [], 'browser runtime errors')
    await ctx.close()
    console.log('Comment browser PASS: public pagination, reader identity, immediate publication, private pending preview, admin pagination, approval, cleanup')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
