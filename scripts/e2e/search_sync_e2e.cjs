// Isolated admin authoring fixtures: no database, real ES, or model calls.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const out = path.join(root, 'output/search-sync')
const article = { id: '1', version: '1', slug: 'sync-test', title: '搜索同步测试', summary: '', content_md: '原始正文', content_html: '<p>原始正文</p>', tags: [], status: 2 }
let state = 2
let failRetry = true
let deny = false
let retryRequests = 0
let failSave = false
let malformed = false
const messages = { 1: '关键词索引已同步。', 2: '正文已保存，关键词索引缺失。', 3: '正文已保存，搜索索引待更新。', 4: '草稿无需索引。', 5: '搜索服务未配置或暂不可用。', 6: '关键词已同步，语义检索未就绪。' }
const status = () => ({ article_slug: article.slug, state, can_retry: [2, 3, 6].includes(state), message: messages[state], semantic_ready: false, checked_at: '2026-11-01T00:00:00Z' })
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
async function main() {
  const api = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://fixture')
    const send = (code, body) => { res.writeHead(code, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(body)) }
    if (url.pathname.includes('/auth/')) return send(200, { access_token: 'fixture-token', user: { id: 'sync-admin', display_name: '作者', role: 1 } })
    if (url.pathname.endsWith('/search-sync/retry')) {
      retryRequests++
      if (failRetry) return send(503, {})
      state = 1
      return send(200, status())
    }
    if (url.pathname.endsWith('/search-sync')) return send(deny ? 403 : 200, deny ? {} : malformed ? { ...status(), state: '1' } : status())
    if (url.pathname.endsWith('/articles/update')) {
      if (failSave) return send(503, {})
      let body = ''
      req.on('data', chunk => { body += chunk })
      return req.on('end', () => { Object.assign(article, JSON.parse(body)); article.version = String(BigInt(article.version) + 1n); state = 3; send(200, article) })
    }
    if (url.pathname.endsWith('/articles/sync-test')) return send(200, article)
    send(200, { articles: [], categories: [] })
  })
  await listen(api)
  const reservation = http.createServer()
  await listen(reservation)
  const port = reservation.address().port
  await new Promise(resolve => reservation.close(resolve))
  const base = `http://127.0.0.1:${port}`
  const child = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}` }, stdio: 'ignore' })
  let browser
  try {
    let ready = false
    for (let i = 0; i < 100; i++) {
      try { ready = (await fetch(base + '/health')).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 100))
    }
    assert.ok(ready)
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    const results = []
    for (const width of [1440, 390]) {
      state = 2; failRetry = true; deny = false; failSave = false
      const page = await browser.newPage({ viewport: { width, height: 900 } })
      page.on('dialog', dialog => dialog.accept())
      const errors = []
      page.on('pageerror', e => errors.push(e.message))
      await page.goto(base + '/admin/posts/sync-test/edit')
      await page.getByPlaceholder('文章标题').waitFor()
      const panel = page.getByRole('region', { name: '搜索同步' })
      // This assertion fails against the pre-feature production build.
      assert.equal(await panel.count(), 1, 'admin editor exposes search sync separately')
      await panel.getByText('索引缺失', { exact: true }).waitFor()
      await panel.getByRole('button', { name: '重试同步', exact: true }).click()
      await panel.getByRole('alert').filter({ hasText: '本次重试未确认成功' }).waitFor()
      failRetry = false
      await panel.getByRole('button', { name: '刷新状态', exact: true }).click()
      await panel.getByRole('button', { name: '重试同步', exact: true }).click()
      await panel.getByText('索引已同步', { exact: true }).waitFor()
      await fs.mkdir(out, { recursive: true })
      await page.screenshot({ path: path.join(out, `synced-${width}.png`), fullPage: true })
      await page.reload()
      await panel.getByText('索引已同步', { exact: true }).waitFor()
      const title = page.getByPlaceholder('文章标题')
      await title.fill('新保存标题' + width)
      await page.getByRole('button', { name: '保存', exact: true }).click()
      await page.getByText('正文已保存', { exact: true }).last().waitFor()
      await panel.getByText('索引待更新', { exact: true }).waitFor()
      assert.equal(await title.inputValue(), '新保存标题' + width)
      for (const next of [6, 4, 5]) {
        state = next
        await panel.getByRole('button', { name: '刷新状态', exact: true }).click()
        await panel.getByText(messages[next], { exact: true }).waitFor()
        if (next === 6) await page.screenshot({ path: path.join(out, `partial-${width}.png`), fullPage: true })
        if ([4, 5].includes(next)) assert.equal(await panel.getByRole('button', { name: '重试同步' }).count(), 0)
      }
      malformed = true
      await panel.getByRole('button', { name: '刷新状态', exact: true }).click()
      await panel.getByRole('alert').filter({ hasText: '搜索同步状态暂时无法获取' }).waitFor()
      assert.equal(await panel.getByText('索引已同步', { exact: true }).count(), 0)
      malformed = false
      deny = true
      const before = retryRequests
      await panel.getByRole('button', { name: '刷新状态', exact: true }).click()
      await panel.getByText('无权查询或重试搜索同步。', { exact: true }).waitFor()
      assert.equal(retryRequests, before)
      await title.fill('状态不可查也可保存' + width)
      const saved = page.waitForResponse(r => r.url().includes('/articles/update') && r.request().method() === 'PUT')
      await page.getByRole('button', { name: '保存', exact: true }).click()
      assert.equal((await (await saved).json()).title, '状态不可查也可保存' + width)
      await page.getByText('正文已保存', { exact: true }).last().waitFor()
      await panel.getByText('无权查询或重试搜索同步。', { exact: true }).waitFor()
      assert.deepEqual(errors, [])
      await fs.mkdir(out, { recursive: true })
      await page.screenshot({ path: path.join(out, `admin-${width}.png`), fullPage: true })
      results.push(`${width}: missing, retry failure/recovery, reload, save+stale, partial/draft/unavailable, permissions, independent authoring PASS`)
      await page.close()
    }
    await fs.writeFile(path.join(out, 'frontend-results.txt'), results.join('\n') + '\n')
    console.log(results.join('\n'))
  } finally {
    await browser?.close()
    child.kill('SIGTERM')
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
