// Isolated article reading controls; no database or production requests.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const out = path.join(root, 'output/article-chat')
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const article = { id: '1', slug: 'reading', title: '阅读工具测试', summary: '测试', content_md: '## 中文标题\n\n正文内容', tags: [], status: 2, created_at: '2026-11-01T00:00:00Z', content_html: '<h2 id="中文标题">中文标题</h2><p>正文内容</p>' }
const requests = []
let mode = 'normal'
async function main() {
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    const url = new URL(req.url, 'http://fixture')
    if (url.pathname === '/v1/about') return res.end(JSON.stringify({ name: '作者', intro: '', skills: [], timeline: [], contacts: [], representative_links: [] }))
    if (url.pathname.endsWith('/projects/list')) return res.end(JSON.stringify({ projects: [] }))
    if (url.pathname.includes('/auth/')) { res.writeHead(401); return res.end('{}') }
    if (url.pathname.endsWith('/articles/list')) return res.end(JSON.stringify({ articles: [article, { ...article, slug: 'other', title: '另一篇' }] }))
    if (url.pathname === '/v1/chat') {
      let body = ''
      req.on('data', c => { body += c })
      req.on('end', () => {
        const input = JSON.parse(body)
        requests.push(input)
        if (Number(mode)) { res.writeHead(Number(mode)); return res.end('{}') }
        const result = { article_slug: input.article_slug, answer: mode === 'delay' ? '旧文章迟到回答' : '当前文章的回答', citations: [{ slug: input.article_slug, title: '阅读工具测试', section_title: '中文标题', anchor: mode === 'unknown' ? '不存在' : '中文标题', excerpt: '<script>window.chatInjected=1</script> 真实段落' }] }
        if (mode === 'old-backend') { delete result.article_slug; result.answer = '不应展示的全站回答' }
        if (mode === 'legacy') result.citations = [{ slug: input.article_slug, title: '旧版引用' }]
        setTimeout(() => res.end(JSON.stringify(result)), mode === 'delay' ? 1000 : 0)
      })
      return
    }
    if (req.method === 'GET' && /\/articles\/(reading|other)$/.test(url.pathname)) return res.end(JSON.stringify({ ...article, slug: url.pathname.split('/').at(-1) }))
    res.end(JSON.stringify({ comments: [], categories: [], articles: [] }))
  })
  await listen(api)
  const reservation = http.createServer()
  await listen(reservation)
  const port = reservation.address().port
  await new Promise(resolve => reservation.close(resolve))
  const base = `http://127.0.0.1:${port}`
  const frontend = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}` }, stdio: ['ignore', 'pipe', 'pipe'] })
  let logs = ''
  frontend.stdout.on('data', chunk => { logs += chunk })
  frontend.stderr.on('data', chunk => { logs += chunk })
  let browser
  try {
    let ready = false
    for (let i = 0; i < 100; i++) {
      if (frontend.exitCode !== null) throw new Error(logs)
      try { ready = (await fetch(base + '/about')).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    assert.ok(ready, logs)
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    const results = []
    for (const width of [1440, 390]) {
      const page = await browser.newPage({ viewport: { width, height: 900 }, colorScheme: width === 390 ? 'dark' : 'light' })
      await page.addInitScript(theme => localStorage.setItem('theme', theme), width === 390 ? 'dark' : 'light')
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(base + '/posts/reading')
      const chat = page.getByRole('region', { name: 'AI 问答' })
      // section aria-label is a named region.
      await chat.getByRole('heading', { name: '问本文', exact: true }).waitFor()
      const input = chat.getByRole('textbox', { name: '你的问题' })
      async function ask(query) {
        await input.fill(query)
        await chat.getByRole('button', { name: '提问', exact: true }).click()
      }
      mode = 'normal'
      await ask('这一节讲什么？')
      await chat.getByText('当前文章的回答', { exact: true }).waitFor()
      assert.deepEqual(requests.at(-1), { query: '这一节讲什么？', article_slug: 'reading' })
      const source = chat.getByRole('link', { name: /中文标题/ }).first()
      assert.equal(decodeURIComponent(await source.getAttribute('href')), '/posts/reading#中文标题')
      await source.click()
      assert.equal(decodeURIComponent(new URL(page.url()).hash), '#中文标题')
      assert.equal(await page.evaluate(() => window.chatInjected), undefined)
      await fs.mkdir(out, { recursive: true })
      await page.evaluate(() => window.scrollTo(0, 0))
      await page.screenshot({ path: path.join(out, `article-chat-answer-${width}.png`), fullPage: true })
      mode = 'unknown'
      await ask('未知锚点')
      await page.waitForFunction(() => [...document.querySelectorAll('[aria-label="AI 问答"] a')].some(a => a.getAttribute('href') === '/posts/reading'))
      mode = 'legacy'
      await ask('旧版引用格式')
      await chat.getByRole('link', { name: '旧版引用', exact: true }).waitFor()
      assert.equal(await chat.getByRole('link', { name: '旧版引用', exact: true }).getAttribute('href'), '/posts/reading')
      mode = 'old-backend'
      await ask('旧服务器范围检查')
      await chat.getByText('当前服务暂不支持问本文，请稍后重试。', { exact: true }).waitFor()
      assert.ok((await chat.innerText()).includes('旧服务器范围检查'))
      assert.equal(await chat.getByText('不应展示的全站回答', { exact: true }).count(), 0)
      for (const status of [400, 404, 429, 503]) {
        mode = String(status)
        await ask('失败问题' + status)
        await page.waitForFunction(() => ![...document.querySelectorAll('[aria-label="AI 问答"] [role="status"]')].some(el => el.textContent === '正在生成回答…'))
        assert.ok((await chat.innerText()).includes('失败问题' + status))
        const expected = { 400: '问题不符合要求', 404: '这篇文章已不可用', 429: '提问太频繁', 503: '问答服务暂时不可用' }[status]
        await chat.getByText(new RegExp(expected)).waitFor()
      }
      mode = 'normal'
      await chat.getByRole('button', { name: '重试问题', exact: true }).last().click()
      await page.waitForFunction(() => ![...document.querySelectorAll('[aria-label="AI 问答"] [role="status"]')].some(el => el.textContent === '正在生成回答…'))
      mode = 'delay'
      await ask('等待期间离开')
      await page.waitForFunction(() => [...document.querySelectorAll('[aria-label="AI 问答"] [role="status"]')].some(el => el.textContent === '正在生成回答…'))
      for (let i = 0; i < 50 && requests.at(-1)?.query !== '等待期间离开'; i++) await page.waitForTimeout(20)
      assert.equal(requests.at(-1)?.query, '等待期间离开', 'old response is already in flight')
      await page.locator('nav[aria-label="相邻文章"] a[href="/posts/other"]').click()
      await page.waitForURL('**/posts/other')
      await page.waitForTimeout(1200)
      assert.equal(await page.getByText('旧文章迟到回答', { exact: true }).count(), 0)
      assert.equal(await page.getByText('等待期间离开', { exact: true }).count(), 0)
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
      assert.deepEqual(errors, [])
      await fs.mkdir(out, { recursive: true })
      await page.screenshot({ path: path.join(out, `article-chat-${width}.png`), fullPage: true })
      results.push(`${width}: scoped request, real/missing anchor, excerpt escaping, error retry, navigation isolation PASS`)
      await page.close()
    }
    await fs.writeFile(path.join(out, 'article-chat-results.txt'), results.join('\n') + '\n')
    console.log(results.join('\n'))
  } finally {
    await browser?.close()
    frontend.kill('SIGTERM')
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
