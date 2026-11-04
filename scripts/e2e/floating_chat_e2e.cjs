// Self-contained local mock acceptance; never calls a model or production API.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const out = path.join(root, 'output/floating-chat')
const article = { id: '1', slug: 'reading', title: '阅读工具测试', summary: '合成测试', content_md: '## 中文标题\n\n正文', content_html: '<h2 id="中文标题">中文标题</h2><p>合成正文</p>', tags: [], status: 2, created_at: '2026-11-01T00:00:00Z' }
const requests = [], pending = []
let mode = 'normal'
const report = { completed: false, cases: [], checks: [], errors: [] }
const listen = server => new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) })
async function main() {
  await fs.mkdir(out, { recursive: true })
  const api = http.createServer((req, res) => {
    const p = new URL(req.url, 'http://fixture').pathname
    const reply = (body, status = 200) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(body)) }
    if (p === '/v1/chat' && req.method === 'POST') {
      let body = ''; req.on('data', chunk => { body += chunk }); req.on('end', () => {
        const input = JSON.parse(body); requests.push(input)
        const current = mode
        const respond = () => {
          if (current === '429' || current === '503') return reply({}, Number(current))
          if (current === 'no-match') return reply({ outcome: 2, answer: '', citations: [], ...(input.article_slug ? { article_slug: input.article_slug } : {}) })
          const answer = { outcome: 1, answer: current === 'delay' ? '迟到的合成回答' : '合成回答：<script>window.floatingInjected=1</script>', citations: [{ slug: input.article_slug || 'reading', title: '合成文章', section_title: '中文标题', anchor: '中文标题', excerpt: '<img src=x onerror="window.floatingInjected=1">引用纯文本' }], ...(input.article_slug ? { article_slug: input.article_slug } : {}) }
          if (current === 'missing-scope') delete answer.article_slug
          reply(answer)
        }
        if (current === 'delay') pending.push(respond); else respond()
      }); return
    }
    if (req.method !== 'GET') return reply({}, 405)
    if (p.includes('/auth/')) return reply({}, 401)
    if (p === '/v1/about') return reply({ name: '作者', headline: '合成作者', intro: '', current_focus: '', skills: [], timeline: [], contacts: [], representative_links: [] })
    if (p.endsWith('/projects/list')) return reply({ projects: [] })
    if (p.endsWith('/articles/list')) return reply({ articles: [article, { ...article, id: '2', slug: 'other', title: '另一篇文章' }] })
    if (p === '/v1/articles/reading') return reply(article)
    if (p === '/v1/articles/other') return reply({ ...article, id: '2', slug: 'other', title: '另一篇文章' })
    if (p === '/v1/articles/private' || p === '/v1/articles/unknown') return reply({}, 404)
    reply({ comments: [], categories: [], collections: [], articles: [] })
  })
  let server, browser, logs = ''
  try {
    await listen(api)
    const reservation = http.createServer(); await listen(reservation)
    const port = reservation.address().port; await new Promise(resolve => reservation.close(resolve))
    const base = `http://127.0.0.1:${port}`
    server = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, HOST: '127.0.0.1', PORT: String(port), NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}`, NUXT_PUBLIC_SITE_URL: base }, stdio: ['ignore', 'pipe', 'pipe'] })
    server.stdout.on('data', chunk => { logs += chunk }); server.stderr.on('data', chunk => { logs += chunk })
    let ready = false
    for (let i = 0; i < 100; i++) { if (server.exitCode !== null) throw Error(logs); try { ready = (await fetch(base + '/health')).ok } catch {} if (ready) break; await new Promise(resolve => setTimeout(resolve, 200)) }
    assert(ready, 'local production build starts')
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    async function session(width, theme) {
      const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' })
      await context.route('**/*', route => ['127.0.0.1', 'localhost'].includes(new URL(route.request().url()).hostname) ? route.continue() : route.abort())
      await context.addInitScript(value => localStorage.setItem('theme', value), theme)
      const page = await context.newPage(); page.setDefaultTimeout(15000); page.on('pageerror', error => report.errors.push(error.message))
      return { page, context }
    }
    const trigger = page => page.getByRole('button', { name: '打开小乌', exact: true })
    const dialog = page => page.getByRole('dialog', { name: /小乌/ })
    const chat = page => dialog(page).getByRole('region', { name: '对话记录' })
    async function visit(page, route) { await page.goto(base + route, { waitUntil: 'networkidle' }); await page.waitForFunction(() => !!document.querySelector('#__nuxt')?.__vue_app__) }
    async function open(page) { await trigger(page).click(); await dialog(page).waitFor() }
    async function ask(page, query) { await chat(page).getByRole('textbox', { name: '输入消息' }).fill(query); await chat(page).getByRole('button', { name: '发送', exact: true }).click() }
    async function requestStarted(query) { for (let i = 0; i < 100; i++) { if (requests.at(-1)?.query === query) return; await new Promise(resolve => setTimeout(resolve, 20)) } throw Error('mock request never reached: ' + query) }
    for (const width of [1440, 1024, 390, 320]) for (const theme of ['light', 'dark']) {
      const { page, context } = await session(width, theme)
      for (const route of ['/', '/posts/reading', '/about']) {
        const before = requests.length; await visit(page, route)
        assert.equal(await page.locator('main [aria-label="对话记录"]').count(), 0, 'article/main has no duplicated embedded chat')
        assert.equal(await trigger(page).count(), 1, 'one global trigger')
        const t = await trigger(page).boundingBox(); assert(t && t.x >= 0 && t.y >= 0 && t.x + t.width <= width + 1, 'trigger in viewport')
        const otherButtons = page.getByRole('button', { name: '打开微信联系与公众号', exact: true })
        if (await otherButtons.isVisible()) { const w = await otherButtons.boundingBox(); assert(t.x + t.width <= w.x || w.x + w.width <= t.x || t.y + t.height <= w.y || w.y + w.height <= t.y, 'AI and WeChat controls do not overlap') }
        await open(page)
        assert.equal(requests.length, before, 'opening never auto-requests')
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'no horizontal overflow')
        const d = await dialog(page).boundingBox(); assert(d && d.x >= -1 && d.x + d.width <= width + 1, 'dialog stays inside viewport')
        if (route === '/posts/reading') await dialog(page).getByRole('button', { name: '聊全站', exact: true, pressed: true }).waitFor()
        else assert.equal(await dialog(page).getByRole('button', { name: '聊本文', exact: true }).count(), 0, 'non-article uses global scope only')
        await trigger(page).focus()
        assert(await dialog(page).evaluate(n => !n.matches(':modal')), 'assistant leaves the page interactive')
        assert(await trigger(page).evaluate(n => n === document.activeElement), 'background trigger remains focusable')
        await page.keyboard.press('Escape'); await dialog(page).waitFor({ state: 'hidden' }); assert(await trigger(page).evaluate(n => n === document.activeElement), 'Escape returns focus')
        await open(page); await page.mouse.click(1, 1); await dialog(page).waitFor({ state: 'hidden' }); assert.equal(requests.length, before, 'close/reopen does not auto-request')
        report.cases.push({ width, theme, route, passed: true })
      }
      if ([1440, 390].includes(width)) { await open(page); await page.screenshot({ path: path.join(out, `dialog-${width}-${theme}.png`) }) }
      await context.close()
    }
    report.checks.push('24 viewport/theme/route cases: global trigger, no embedded duplicate, page remains interactive, outside click and Escape, no overlap/overflow or automatic requests')
    const short = await session(390, 'light')
    await short.page.setViewportSize({ width: 390, height: 480 })
    await visit(short.page, '/posts/reading'); await open(short.page)
    await short.page.waitForFunction(() => document.activeElement?.tagName === 'TEXTAREA')
    const shortBounds = await dialog(short.page).evaluate(n => {
      const d = n.getBoundingClientRect(), t = n.querySelector('textarea').getBoundingClientRect()
      return { dialog: { top: d.top, bottom: d.bottom, left: d.left, right: d.right }, composer: { top: t.top, bottom: t.bottom }, focused: n.querySelector('textarea') === document.activeElement }
    })
    assert(shortBounds.focused)
    assert(shortBounds.dialog.top >= 0 && shortBounds.dialog.bottom <= 480 && shortBounds.dialog.left >= 0 && shortBounds.dialog.right <= 390, 'short-screen dialog fits viewport')
    assert(shortBounds.composer.top >= shortBounds.dialog.top && shortBounds.composer.bottom <= shortBounds.dialog.bottom, 'focused composer is fully visible on a short screen')
    const cdp = await short.context.newCDPSession(short.page)
    await short.page.evaluate(() => {
      window.__imeStarts = 0; window.__imeEnds = 0
      document.addEventListener('compositionstart', () => window.__imeStarts++)
      document.addEventListener('compositionend', () => window.__imeEnds++)
    })
    const beforeIme = requests.length
    await cdp.send('Input.imeSetComposition', { text: '你', selectionStart: 1, selectionEnd: 1 })
    await short.page.waitForFunction(() => window.__imeStarts === 1)
    await short.page.keyboard.press('Escape')
    assert(await dialog(short.page).isVisible(), 'real Chrome IME Escape keeps the dialog open')
    assert.equal(requests.length, beforeIme, 'IME cancellation does not submit a question')
    await cdp.send('Input.imeSetComposition', { text: '', selectionStart: 0, selectionEnd: 0 })
    await short.page.waitForFunction(() => window.__imeEnds >= 1)
    await short.page.keyboard.press('Escape'); await dialog(short.page).waitFor({ state: 'hidden' })
    assert(await trigger(short.page).evaluate(n => n === document.activeElement), 'ordinary Escape after IME closes and restores focus')
    await cdp.detach(); await short.context.close()
    report.checks.push('390x480 dialog/composer bounds and real CDP IME composition Escape protection')
    const failedOpen = await session(390, 'light')
    await visit(failedOpen.page, '/')
    const beforeOpenFailure = requests.length
    await failedOpen.page.evaluate(() => {
      const original = HTMLDialogElement.prototype.show
      HTMLDialogElement.prototype.show = function () {
        HTMLDialogElement.prototype.show = original
        throw new DOMException('Synthetic one-shot show failure', 'InvalidStateError')
      }
    })
    await trigger(failedOpen.page).click()
    await failedOpen.page.getByRole('alert').filter({ hasText: '问答窗口暂时无法打开，请重试。' }).waitFor()
    assert.equal(await dialog(failedOpen.page).isVisible(), false)
    assert(await trigger(failedOpen.page).evaluate(n => n === document.activeElement), 'failed show returns focus')
    assert.equal(requests.length, beforeOpenFailure, 'failed open does not request an answer')
    await open(failedOpen.page)
    assert.equal(await failedOpen.page.getByRole('alert').filter({ hasText: '问答窗口暂时无法打开，请重试。' }).count(), 0, 'successful retry clears the open error')
    assert.equal(requests.length, beforeOpenFailure, 'open retry remains request-free')
    await failedOpen.context.close()
    report.checks.push('One-shot native show failure is visible, focus-safe and retryable without chat requests')
    const { page, context } = await session(1440, 'light')
    await visit(page, '/'); await open(page)
    const input = chat(page).getByRole('textbox', { name: '输入消息' })
    await input.fill('保留草稿'); const beforeDraft = requests.length
    await dialog(page).getByRole('button', { name: '关闭聊天', exact: true }).click(); await open(page)
    assert.equal(await input.inputValue(), '保留草稿'); assert.equal(requests.length, beforeDraft)
    mode = 'normal'; await ask(page, '全站问题'); await chat(page).getByTestId('chat-answer').waitFor(); assert.deepEqual(requests.at(-1), { query: '全站问题' })
    assert.equal(await page.evaluate(() => window.floatingInjected), undefined, 'answer and citation render as plain text')
    assert((await chat(page).innerText()).includes('<img src=x'))
    await dialog(page).getByRole('button', { name: '关闭聊天', exact: true }).click(); await open(page); assert.equal(await chat(page).getByTestId('chat-answer').count(), 1, 'closing retains answer')
    for (const status of [429, 503]) { mode = String(status); await ask(page, '失败' + status); await chat(page).getByRole('alert').last().waitFor(); assert((await chat(page).innerText()).includes(status === 429 ? '提问太频繁' : '问答服务暂时不可用')); mode = 'normal'; await chat(page).getByRole('button', { name: '重试问题', exact: true }).last().click(); await chat(page).getByTestId('chat-answer').last().waitFor(); await page.waitForLoadState('networkidle') }
    mode = 'no-match'; await ask(page, '找不到的主题'); await chat(page).getByText('没有找到相关内容，可换个说法再试。', { exact: true }).waitFor()
    await visit(page, '/posts/reading'); await open(page); assert.equal(await chat(page).getByTestId('chat-question').count(), 0, 'route change clears conversation')
    await dialog(page).getByRole('button', { name: '聊本文', exact: true }).click()
    mode = 'normal'; await ask(page, '本文问题'); await chat(page).getByTestId('chat-answer').waitFor(); assert.deepEqual(requests.at(-1), { query: '本文问题', article_slug: 'reading' })
    await chat(page).getByRole('link', { name: /中文标题/ }).first().click(); await dialog(page).waitFor({ state: 'hidden' }); assert.equal(decodeURIComponent(new URL(page.url()).hash), '#中文标题'); const beforeHash = requests.length; await open(page); assert.equal(await chat(page).getByTestId('chat-answer').count(), 1, 'same-page citation hash preserves record'); assert.equal(requests.length, beforeHash)
    mode = 'missing-scope'; await ask(page, '范围回显缺失'); await chat(page).getByText('当前服务暂不支持问本文，请稍后重试。', { exact: true }).waitFor()
    mode = 'delay'; await ask(page, '切模式前请求'); await requestStarted('切模式前请求'); await dialog(page).getByRole('button', { name: '聊全站', exact: true }).click(); while (pending.length) pending.shift()(); await page.waitForLoadState('networkidle'); assert.equal(await chat(page).getByTestId('chat-question').count(), 0, 'scope switch clears old request and answer'); assert.equal(await chat(page).getByText('迟到的合成回答', { exact: true }).count(), 0)
    mode = 'normal'; await ask(page, '文章页问全站'); await chat(page).getByTestId('chat-answer').waitFor(); assert.deepEqual(requests.at(-1), { query: '文章页问全站' })
    mode = 'delay'; await ask(page, '关闭中继续执行'); await requestStarted('关闭中继续执行'); await dialog(page).getByRole('button', { name: '关闭聊天', exact: true }).click(); while (pending.length) pending.shift()(); await page.waitForLoadState('networkidle'); const beforeReopen = requests.length; await open(page); await chat(page).getByText('迟到的合成回答', { exact: true }).waitFor(); assert.equal(requests.length, beforeReopen, 'reopen does not duplicate pending request')
    mode = 'delay'; await ask(page, '离开文章的请求'); await requestStarted('离开文章的请求')
    await dialog(page).getByRole('button', { name: '关闭聊天', exact: true }).click()
    await page.locator('nav[aria-label="相邻文章"] a[href="/posts/other"]').click()
    await page.waitForURL('**/posts/other'); await page.locator('[data-article-detail] h1').filter({ hasText: '另一篇文章' }).waitFor()
    while (pending.length) pending.shift()()
    await open(page); await dialog(page).getByRole('button', { name: '聊本文', exact: true }).click()
    assert.equal(await chat(page).getByTestId('chat-question').count(), 0, 'SPA navigation to a different article discards old conversation')
    assert.equal(await chat(page).getByText('迟到的合成回答', { exact: true }).count(), 0, 'old article late result cannot appear after SPA navigation')
    mode = 'normal'; await ask(page, '另一篇问题'); await chat(page).getByTestId('chat-answer').waitFor(); assert.deepEqual(requests.at(-1), { query: '另一篇问题', article_slug: 'other' })
    for (const slug of ['private', 'unknown']) { await page.goto(base + '/posts/' + slug); if (await trigger(page).count()) { await open(page); assert.equal(await dialog(page).getByRole('button', { name: '聊本文', exact: true }).count(), 0, '404 never exposes article scope') } }
    report.checks.push('Explicit global/article bodies, draft/answer/pending close persistence, no-match/error retry, scope guard, stale request isolation, safe citation hash and text escaping')
    await context.close(); assert.deepEqual(report.errors, []); report.completed = true
  } catch (error) { report.failure = { message: error.message, stack: error.stack }; throw error }
  finally { while (pending.length) pending.shift()(); await browser?.close(); if (server && server.exitCode === null) { const exited = new Promise(resolve => server.once('exit', resolve)); server.kill('SIGTERM'); await exited } if (api.listening) await new Promise(resolve => api.close(resolve)); await fs.writeFile(path.join(out, 'results.json'), JSON.stringify(report, null, 2)); await fs.writeFile(path.join(out, 'server.log'), logs) }
  console.log(JSON.stringify({ completed: report.completed, cases: report.cases.length, checks: report.checks, errors: report.errors }, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
