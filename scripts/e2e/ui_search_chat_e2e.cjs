const assert = require('node:assert/strict')
const fs = require('node:fs')
const { chromium } = require('playwright')
const { startFixture, makeArticle, pixelPng } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['localhost', '127.0.0.1', '[::1]'].includes(new URL(base).hostname)) throw new Error('Local frontends only')
const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
async function hydrated(page) {
  await page.waitForFunction(() => {
    const app = document.querySelector('#__nuxt')?.__vue_app__
    return app && app.config.globalProperties.$nuxt?.isHydrating === false
  })
}
async function main() {
  fs.mkdirSync('output/ui-refinement/s2', { recursive: true })
  const fixture = await startFixture()
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext({ permissions: [] })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    let oldSearchDone
    const searchDone = new Promise(resolve => { oldSearchDone = resolve })
    await context.route('**/api/v1/search/articles?**', async route => {
      const query = new URL(route.request().url()).searchParams.get('query')
      if (query === '旧查询') { await delay(900); await route.fulfill({ json: { articles: [makeArticle({ title: '过时文章', slug: 'old' })], search_outcome: 1 } }); oldSearchDone(); return }
      await route.fulfill({ json: { articles: query === '新查询' ? [] : fixture.articles, search_outcome: query === '分布式' ? 2 : 1 } })
    })
    await context.route('**/api/v1/auth/refresh', route => route.fulfill({ status: 401, json: { message: 'anonymous fixture' } }))
    await context.route('**/api/v1/assets/fixture.png', route => route.fulfill({ contentType: 'image/png', body: pixelPng }))
    await page.goto(`${base}/posts`)
    await hydrated(page)
    const search = page.getByLabel('搜索文章', { exact: true })
    await search.fill('重试机制')
    await page.waitForResponse(response => response.url().includes('/search/articles?') && response.url().includes(encodeURIComponent('重试机制')))
    await page.getByRole('link', { name: '并发与一致性', exact: true }).first().waitFor()
    await search.fill('旧查询')
    await page.waitForRequest(request => request.url().includes(encodeURIComponent('旧查询')))
    await search.fill('新查询')
    await page.getByText('没有找到匹配的文章。', { exact: true }).waitFor()
    await searchDone
    assert.equal(await page.getByText('过时文章', { exact: true }).count(), 0, 'late search must not replace newer results')
    await search.fill('旧查询')
    await page.waitForRequest(request => request.url().includes(encodeURIComponent('旧查询')))
    await search.fill('')
    await delay(1100)
    assert.equal(await page.getByText('过时文章', { exact: true }).count(), 0, 'clearing search invalidates pending request')
    await search.fill('分布式')
    await page.getByText(/本地匹配/).waitFor()

    let calls = []
    let mode = 'network'
    let outcome = 1
    await context.route('**/api/v1/chat', async route => {
      calls.push(route.request().postDataJSON().query)
      await delay(200)
      if (mode === 'network') return route.abort('failed')
      if (mode === 'legacy') return route.fulfill({ json: { answer: '旧版接口返回的回答', citations: [] } })
      if (mode === 'empty') return route.fulfill({ json: { outcome: 1, answer: '' } })
      await route.fulfill({ json: { outcome, answer: outcome === 1 ? '采用幂等与重试。' : '', citations: outcome === 1 ? [{ slug: 'fixture-article', title: '并发与一致性', summary: '分布式请求处理' }] : [] } })
    })
    await page.goto(`${base}/posts/fixture-article`)
    await hydrated(page)
    const chat = page.locator('section[aria-label="AI 问答"]')
    const input = chat.locator('textarea[aria-label="你的问题"]')
    const ask = chat.getByRole('button', { name: '提问', exact: true })
    await input.fill('   ')
    assert.equal(await ask.isDisabled(), true, 'blank question cannot send')
    await input.fill('中文选词')
    await input.dispatchEvent('compositionstart')
    await input.press('Enter')
    await input.dispatchEvent('compositionend')
    assert.equal(calls.length, 0, 'IME Enter cannot send')
    await input.press('Shift+Enter')
    assert.ok((await input.inputValue()).includes('\n'), 'Shift Enter inserts a newline')
    await input.fill('😀'.repeat(501))
    assert.equal(await ask.isDisabled(), true, '501 codepoints cannot send')
    await input.fill('😀'.repeat(500))
    assert.equal(await ask.isEnabled(), true, '500 codepoints are accepted')
    await input.fill('如何安全重试？')
    await ask.click()
    await input.fill('下一条草稿')
    const retry = chat.getByRole('button', { name: '重试问题', exact: true })
    await retry.waitFor()
    assert.equal(await chat.getByTestId('chat-question').count(), 1)
    mode = 'success'
    await retry.click()
    await chat.getByText('采用幂等与重试。', { exact: true }).waitFor()
    assert.deepEqual(calls, ['如何安全重试？', '如何安全重试？'])
    assert.equal(await chat.getByTestId('chat-question').count(), 1, 'retry preserves the question record')
    assert.equal(await input.inputValue(), '下一条草稿', 'retry preserves next draft')
    const citation = chat.getByRole('link', { name: /并发与一致性/ })
    assert.equal(await citation.getAttribute('href'), '/posts/fixture-article')
    await chat.getByText('分布式请求处理', { exact: true }).waitFor()
    await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async value => { window.fixtureCopied = value } } }))
    await chat.getByRole('button', { name: '复制回答', exact: true }).click()
    await chat.getByText('回答已复制', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => window.fixtureCopied), '采用幂等与重试。')
    await chat.screenshot({ path: 'output/ui-refinement/s2/chat-answer.png' })
    await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: () => Promise.reject(new Error('fixture denied')) } }))
    await chat.getByRole('button', { name: /复制回答/ }).click()
    await chat.getByText(/复制失败/).waitFor()
    for (const [value, pattern] of [[2, /没有找到|暂无匹配|未找到/], [3, /检索.*不可用|检索.*暂时|搜索.*不可用/], [4, /生成.*不可用|生成.*暂时|回答.*暂时/]]) {
      outcome = value
      await input.fill(`结果场景${value}`)
      await ask.click()
      await chat.getByText(pattern).last().waitFor()
    }
    mode = 'legacy'
    await input.fill('旧服务兼容')
    await ask.click()
    await chat.getByText('旧版接口返回的回答', { exact: true }).waitFor()
    await chat.getByText('返回结果', { exact: true }).waitFor()
    mode = 'empty'
    await input.fill('空白生成结果')
    await ask.click()
    await chat.getByRole('alert').filter({ hasText: '回答请求失败' }).waitFor()
    await chat.getByRole('button', { name: '清空记录', exact: true }).click()
    assert.equal(await chat.getByTestId('chat-question').count(), 0)
    await page.getByRole('button', { name: '切换到暗色模式' }).click()
    await page.setViewportSize({ width: 320, height: 800 })
    await page.waitForTimeout(400)
    fs.mkdirSync('output/ui-refinement/s2', { recursive: true })
    await page.screenshot({ path: 'output/ui-refinement/s2/chat-dark-320.png', fullPage: true })
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    assert.deepEqual(errors, [])
    console.log('PASS: semantic search, stale responses, degraded search, chat composition/boundaries/retry/outcomes/copy/clear/mobile')
  }
  finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
