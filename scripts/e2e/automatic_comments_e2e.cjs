// Isolated automatic-comment browser checks; all accounts, comments and API replies are synthetic.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const http = require('node:http')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const { makeArticle } = require('./ui_fixture.cjs')
const root = path.resolve(__dirname, '../..')
const out = path.resolve(process.env.BLOG_E2E_OUTPUT || path.join(root, 'output/automatic-comments'))
const port = 14350, apiPort = 14351
const base = process.env.BLOG_E2E_BASE_URL || `http://127.0.0.1:${port}`
assert(['127.0.0.1', 'localhost'].includes(new URL(base).hostname), 'localhost only')
async function main() {
  fs.mkdirSync(out, { recursive: true })
  let comments = [], mode = 'ok', seq = 0, release
  const readerA = { id: 'fixture-reader', email: 'reader@example.invalid', role: 2, display_name: '测试读者' }
  const readerB = { ...readerA, id: 'fixture-reader-b', email: 'reader-b@example.invalid', display_name: '测试读者B' }
  let user = readerA, refreshes = 0, commentPosts = 0
  const api = http.createServer(async (req, res) => {
    const url = new URL(req.url, 'http://fixture')
    res.setHeader('Content-Type', 'application/json')
    const reply = (value, status = 200) => { res.statusCode = status; res.end(JSON.stringify(value)) }
    if (url.pathname.startsWith('/v1/auth/')) {
      if (url.pathname.endsWith('/refresh')) refreshes++
      if (url.pathname.endsWith('/login')) {
        let body = ''; for await (const chunk of req) body += chunk
        user = JSON.parse(body).email === readerB.email ? readerB : readerA
      }
      return reply({ access_token: `synthetic-token-${user.id}`, token_type: 'Bearer', expires_in: 900, user })
    }
    if (url.pathname === '/v1/comments' && req.method === 'POST') {
      let body = ''; for await (const chunk of req) body += chunk
      const input = JSON.parse(body)
      commentPosts++
      if (mode === 'delayed401') {
        await new Promise(resolve => { release = resolve })
        return reply({ code: 401, reason: 'AUTH_UNAUTHORIZED' }, 401)
      }
      if (mode === 'rejected') return reply({ code: 400, reason: 'COMMENT_INVALID_ARGUMENT' }, 400)
      if (mode === 'offline') return reply({ code: 503, reason: 'CONTENT_SAFETY_UNAVAILABLE' }, 503)
      if (mode === 'delayed' || mode === 'delayed-pending') await new Promise(resolve => { release = resolve })
      const comment = { id: `fixture-${++seq}`, article_slug: input.article_slug, content: input.content, status: mode === 'pending' || mode === 'delayed-pending' ? 1 : 2, display_name: user.display_name, user_id: user.id, created_at: '2026-11-02T00:00:00Z' }
      if (comment.status === 2) comments.unshift(comment)
      return reply(comment)
    }
    if (url.pathname.endsWith('/comments')) return reply({ comments })
    if (url.pathname === '/v1/articles/list') return reply({ articles: [makeArticle()] })
    if (url.pathname === '/v1/articles/fixture-article') return reply(makeArticle())
    if (url.pathname.includes('/projects')) return reply({ projects: [] })
    if (url.pathname.includes('/collections')) return reply({ collections: [] })
    return reply({})
  })
  await new Promise((resolve, reject) => { api.once('error', reject); api.listen(apiPort, '127.0.0.1', resolve) })
  let server, browser, exited = false
  try {
    if (!process.env.BLOG_E2E_BASE_URL) {
      const log = fs.openSync(path.join(out, 'server.log'), 'w')
      server = spawn(process.execPath, [path.join(root, 'frontend/.output/server/index.mjs')], { cwd: path.join(root, 'frontend'), detached: true, stdio: ['ignore', log, log], env: { ...process.env, HOST: '127.0.0.1', PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${apiPort}`, NUXT_PUBLIC_SITE_URL: base } })
      fs.closeSync(log)
      server.on('exit', () => { exited = true })
    }
    let ready = false
    for (let i = 0; i < 100; i++) {
      assert(!exited, 'owned frontend exited')
      try { ready = (await fetch(`${base}/health`)).ok } catch { /* startup */ }
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 250))
    }
    assert(ready, 'frontend unavailable')
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    for (const width of [1440, 390]) {
      comments = []; mode = 'ok'; user = readerA
      const ctx = await browser.newContext({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
      const page = await ctx.newPage(), errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(`${base}/posts/fixture-article`, { waitUntil: 'networkidle' })
      const section = page.getByRole('region', { name: '评论区' })
      const input = section.getByPlaceholder('写下你的评论')
      const send = section.getByRole('button', { name: '发表评论', exact: true })
      await input.waitFor()
      for (const text of ['普通技术讨论', '参考 https://go.dev/doc/']) {
        await input.fill(text); await send.click()
        await section.getByRole('status').filter({ hasText: '评论已发布' }).waitFor()
        await send.waitFor()
        assert.equal(await input.inputValue(), '')
        assert.equal(await section.locator('ul').first().getByText(text, { exact: true }).count(), 1)
        assert.equal(await section.getByRole('list', { name: '我的待审核评论' }).count(), 0)
      }
      for (const [kind, pattern] of [['rejected', /修改后重新发送/], ['offline', /检测.*不可用.*重试/]]) {
        mode = kind
        await input.fill(`${kind}原稿`); await send.click()
        await page.waitForFunction(() => document.querySelector('section[aria-label="评论区"] button[type="submit"]')?.textContent.includes('发表评论'))
        assert.equal(await input.inputValue(), `${kind}原稿`)
        assert.match(await section.getByRole('status').innerText(), pattern)
        assert.equal(comments.length, 2)
      }
      mode = 'ok'; await input.fill('修改后的正常评论'); await send.click()
      await section.getByRole('status').filter({ hasText: '评论已发布' }).waitFor()
      assert.equal(await input.inputValue(), '')
      release = undefined
      mode = 'delayed'; await input.fill('正在提交的旧稿'); await send.click()
      for (let i = 0; i < 100 && !release; i++) await new Promise(resolve => setTimeout(resolve, 20))
      assert(release, 'delayed request received')
      await input.fill('保存期间新输入的草稿'); release()
      await section.getByRole('status').filter({ hasText: '评论已发布' }).waitFor()
      assert.equal(await input.inputValue(), '保存期间新输入的草稿')
      mode = 'pending'; await send.click()
      await section.getByRole('list', { name: '我的待审核评论' }).waitFor()
      assert.equal(comments.some(c => c.content === '保存期间新输入的草稿'), false, 'legacy pending does not become public')
      assert.deepEqual(errors, [])
      await section.screenshot({ path: path.join(out, `comments-${width}.png`) })
      // A's late 401 must not refresh or replay its comment under B's login.
      release = undefined; mode = 'delayed401'
      await input.fill('账号A的迟到评论不可用B重发'); await send.click()
      for (let i = 0; i < 100 && !release; i++) await new Promise(resolve => setTimeout(resolve, 20))
      assert(release)
      await page.getByRole('button', { name: '测试读者 的账号菜单' }).click()
      await page.getByRole('link', { name: '切换账号', exact: true }).click()
      await page.getByRole('textbox', { name: '邮箱', exact: true }).fill(readerB.email)
      await page.getByLabel('密码', { exact: true }).fill('synthetic-password')
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.getByRole('button', { name: '测试读者B 的账号菜单' }).waitFor()
      await page.locator('a[href="/posts/fixture-article"]').first().click()
      await input.waitFor()
      const beforeRelease = { refreshes, commentPosts }
      mode = 'ok'
      const late401 = page.waitForResponse(response => response.url().endsWith('/api/v1/comments') && response.status() === 401)
      release(); await late401
      // Drain the old request's microtasks without inventing a successful network response.
      await page.waitForLoadState('networkidle')
      assert.deepEqual({ refreshes, commentPosts }, beforeRelease, 'late A 401 must not refresh or rePOST under B')
      assert.equal(comments.some(c => c.content === '账号A的迟到评论不可用B重发'), false)
      release = undefined; mode = 'delayed-pending'
      await input.fill('旧账号私有评论不可跨登录态显示'); await send.click()
      for (let i = 0; i < 100 && !release; i++) await new Promise(resolve => setTimeout(resolve, 20))
      assert(release)
      await page.getByRole('button', { name: '测试读者B 的账号菜单' }).click()
      await page.getByRole('button', { name: '退出登录', exact: true }).click()
      await section.getByRole('link', { name: '登录', exact: true }).waitFor()
      release()
      await page.waitForResponse(response => response.url().endsWith('/api/v1/comments'))
      assert.equal(await section.getByText('旧账号私有评论不可跨登录态显示', { exact: true }).count(), 0)
      assert.equal(await section.getByRole('status').count(), 0)
      await ctx.close()
    }
    console.log('PASS: desktop/mobile ordinary+link publication, 400/503 retained draft, corrected repost, newer input and legacy pending')
  } finally {
    release?.()
    await browser?.close()
    if (server && !exited) { const closed = new Promise(resolve => server.once('exit', resolve)); process.kill(-server.pid, 'SIGTERM'); await closed }
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
