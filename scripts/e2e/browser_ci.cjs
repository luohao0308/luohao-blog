const assert = require('node:assert/strict')
const { spawn } = require('node:child_process')
const { createServer } = require('node:net')
const { once } = require('node:events')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')

async function availablePort() {
  const server = createServer().listen(0, '127.0.0.1')
  await once(server, 'listening')
  const { port } = server.address()
  await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))
  return port
}

async function main() {
  const port = await availablePort()
  const base = `http://127.0.0.1:${port}`
  const marker = 'CI_ARTICLE_MARKDOWN_BODY_ONLY'
  const article = makeArticle({
    title: '一篇很长的测试文章如何安全发布并被搜索和提问',
    summary: '从保存 Markdown 到返回带引用的回答，顺着一篇文章看各组件如何分工。'.repeat(2),
    content_md: marker,
  })
  const articles = [article, ...[2, 3, 4].map(id => makeArticle({ id: String(id), slug: `fixture-${id}`, title: `测试文章 ${id}` }))]
  const fixture = await startFixture({ articles, port: 0 })
  const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
    cwd: path.resolve(__dirname, '../../frontend'),
    env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${fixture.port}` },
    stdio: 'inherit',
  })
  let browser
  try {
    for (let attempt = 0; ; attempt++) {
      if (frontend.exitCode !== null || attempt === 80) throw new Error('Built frontend did not start')
      const response = await fetch(base + '/posts').catch(() => null)
      if (response?.ok) { await response.text(); break }
      await new Promise(resolve => setTimeout(resolve, 250))
    }
    browser = await chromium.launch({ executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    const page = await browser.newPage({ viewport: { width: 390, height: 844 }, reducedMotion: 'reduce' })
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/search/articles?**', route => route.fulfill({
      json: { articles: [{ ...article, title: '搜索返回的匹配结果' }], search_outcome: 1 },
    }))
    let chatRequest
    await page.route('**/api/v1/chat', route => {
      chatRequest = route.request().postDataJSON()
      return route.fulfill({ json: { outcome: 1, answer: '使用幂等键。', citations: [{ slug: articles[1].slug, title: articles[1].title }] } })
    })
    const homeResponse = await page.goto(base)
    assert.ok(homeResponse?.ok())
    assert.ok(!(await homeResponse.text()).includes(marker), 'homepage SSR payload omits full Markdown from a legacy backend')
    assert.equal(await page.getByRole('region', { name: '最新文章' }).locator('[data-home-card]').count(), 3)
    assert.match(await page.getByLabel('站点概况').innerText(), /4\s*篇文章/)

    await page.goto(base + '/posts')
    await page.waitForFunction(() => document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$nuxt?.isHydrating === false)
    await page.getByRole('button', { name: '搜索文章' }).click()
    const search = page.getByRole('searchbox', { name: '搜索文章' })
    const searchResponse = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/search/articles')
    await search.fill('幂等')
    await searchResponse
    await page.getByRole('link', { name: '搜索返回的匹配结果', exact: true }).click()
    await page.waitForURL(`**/posts/${article.slug}`)
    const heading = page.getByRole('heading', { name: '请求处理', exact: true }).first()
    await heading.waitFor()
    const bodyTop = (await heading.boundingBox()).y
    assert.ok(bodyTop < 650, 'article body starts within the mobile first screen')
    const nav = page.getByRole('navigation', { name: '移动端主导航' })
    assert.deepEqual(await nav.getByRole('link').allTextContents(), ['首页', '文章', '系列', '作品集', '关于', '分类', '标签', '归档'])
    const archive = nav.getByRole('link', { name: '归档' })
    await archive.focus()
    await page.waitForFunction(() => {
      const active = document.activeElement
      if (active?.textContent?.trim() !== '归档') return false
      const rect = active.getBoundingClientRect()
      return rect.left >= -1 && rect.right <= innerWidth + 1
    }, null, { timeout: 3000 })
    const navRows = await nav.getByRole('link').evaluateAll(links => new Set(links.map(link => link.getBoundingClientRect().top)).size)
    assert.equal(navRows, 1, 'mobile navigation occupies one row')
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'no horizontal page overflow')
    await page.getByRole('button', { name: '打开小乌' }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('textbox', { name: '输入消息' }).fill('怎样避免重复处理？')
    await dialog.getByRole('button', { name: '发送', exact: true }).click()
    await dialog.getByTestId('chat-answer').filter({ hasText: '使用幂等键。' }).waitFor()
    assert.equal(chatRequest.query, '怎样避免重复处理？')
    await dialog.getByText('参考文章 · 1', { exact: true }).click()
    await dialog.getByRole('link', { name: articles[1].title, exact: true }).click()
    await page.waitForURL(`**/posts/${articles[1].slug}`)
    await dialog.waitFor({ state: 'hidden' })
    await page.getByRole('button', { name: '打开小乌' }).click()
    await dialog.getByTestId('chat-answer').filter({ hasText: '使用幂等键。' }).waitFor()
    await dialog.getByRole('button', { name: '清空记录', exact: true }).click()
    await dialog.getByTestId('chat-answer').waitFor({ state: 'detached' })
    assert.deepEqual(errors, [])
    console.log(`PASS: mobile homepage, search, reading (body y=${bodyTop}px) and chat`)
  } finally {
    try {
      await browser?.close()
    } finally {
      frontend.kill()
      await fixture.close()
    }
  }
}

main().catch(error => { console.error(error); process.exitCode = 1 })
