// Isolated acceptance for the approved Figma refresh. No CMS/database writes.
const assert = require('node:assert/strict')
const http = require('node:http')
const fs = require('node:fs/promises')
const path = require('node:path')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const { makeArticle, pixelPng } = require('./ui_fixture.cjs')
const root = path.resolve(__dirname, '../..')
const out = path.join(root, 'output/visual-hierarchy/e2e')
const slugs = ['fullstack-smoke-pitfalls', 'ent-m2m-edge-case-study', 'mysql-varchar-191-case-study', 'article-cas-revisions-case-study', 'article-search-sync-case-study', 'dev-workflow-project', 'ai-delivery-guard', 'forge-harness-project', 'aigc-backend-delivery']
const names = ['后台联调与验收', 'Ent 多对多关系', 'MySQL 字符串索引', '两个窗口同时改文章', '正文已保存，搜索为什么还没更新', 'dev-workflow 工程实践', '交付守卫', 'Forge Harness 运行时', 'AIGC 聚合与调度']
const articles = slugs.map((slug, i) => makeArticle({ id: String(i + 1), slug, title: names[i], tags: ['Go', '工程实践'], category: { id: '1', slug: 'engineering', name: '工程实践' }, cover_url: '' }))
const projects = ['dev-workflow', 'forge-harness', 'aigc', 'luohao-blog', 'job-application-workbench'].map((slug, i) => ({ id: String(i + 1), slug, name: ['dev-workflow', 'Forge Harness', 'AIGC 聚合与调度平台', 'luohao-blog', '机会工作台'][i], summary: '合成公开项目，用于本地视觉验收。', description: '项目说明与真实路由回归夹具。', stack: ['Python', 'Go', 'Nuxt'], role: '作者与开发者', emoji: '🧩', year: '2026', featured: i < 2, published: true, sort: i, cover_url: '', screenshots: [], repo_url: 'https://example.com/repository', live_url: '' }))
const about = { name: '罗浩', headline: '全栈开发与 AI 应用', intro: '合成个人资料，仅本地验收。', current_focus: '工程实践', skills: [], timeline: [], contacts: [], representative_links: [] }
const fixture = { mode: 'normal', custom: false, broken: false }
const report = { completed: false, checks: [], errors: [], cases: [] }
const listen = server => new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) })
const check = label => report.checks.push(label)

async function main() {
  await fs.mkdir(path.join(out, 'screenshots'), { recursive: true })
  const api = http.createServer((req, res) => {
    const u = new URL(req.url, 'http://fixture'), p = u.pathname
    const json = (value, code = 200) => { res.writeHead(code, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(value)) }
    if (req.method !== 'GET') return json({ message: 'Read-only fixture' }, 405)
    if (p === '/v1/assets/fixture.png') { res.writeHead(200, { 'Content-Type': 'image/png' }); return res.end(pixelPng) }
    if (p === '/v1/assets/custom.svg') { res.writeHead(200, { 'Content-Type': 'image/svg+xml' }); return res.end('<svg xmlns="http://www.w3.org/2000/svg" width="800" height="450"><rect width="800" height="450" fill="#315ceb"/><circle cx="400" cy="225" r="100" fill="#fff"/></svg>') }
    if (p === '/v1/assets/missing.svg') return json({}, 404)
    if (p.includes('/auth/')) return json({}, 401)
    if (p === '/v1/about') return json(about)
    const current = articles.map((a, i) => ({ ...a, status: fixture.mode === 'private' && i === 3 ? 1 : 2, cover_url: fixture.custom ? '/v1/assets/custom.svg' : fixture.broken ? '/v1/assets/missing.svg' : '' }))
    if (p === '/v1/articles/list') {
      if (fixture.mode === 'error') return json({ message: 'Fixture unavailable' }, 503)
      return json({ articles: fixture.mode === 'empty' ? [] : [...current, makeArticle({ id: '99', slug: 'private-fixture', title: '不应出现的私有文章', status: 1 })], next_page_token: '' })
    }
    if (p === '/v1/search/articles') return json({ articles: current.filter(a => a.title.includes(u.searchParams.get('query') || u.searchParams.get('q') || '')) })
    if (/\/articles\/[^/]+\/comments$/.test(p)) return json({ comments: [], total: 0 })
    if (p.startsWith('/v1/articles/')) { const a = current.find(a => a.slug === decodeURIComponent(p.split('/')[3])); return a ? json(a) : json({}, 404) }
    if (p === '/v1/projects/list') return json({ projects: [...projects, { ...projects[0], slug: 'hidden-project', published: false }], next_page_token: '' })
    if (p.startsWith('/v1/projects/')) { const project = projects.find(v => v.slug === decodeURIComponent(p.split('/')[3])); return project ? json(project) : json({}, 404) }
    if (p.includes('/collections')) return json({ collections: [], next_page_token: '' })
    if (p.includes('/categories')) return json({ categories: [] })
    return json({ tags: [], articles: [], projects: [] })
  })
  let server, browser, base, logs = ''
  try {
    await listen(api)
    const reservation = http.createServer(); await listen(reservation)
    const port = reservation.address().port; await new Promise(resolve => reservation.close(resolve))
    base = `http://127.0.0.1:${port}`
    server = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, HOST: '127.0.0.1', PORT: String(port), NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}`, NUXT_PUBLIC_SITE_URL: base }, stdio: ['ignore', 'pipe', 'pipe'] })
    server.stdout.on('data', chunk => { logs += chunk }); server.stderr.on('data', chunk => { logs += chunk })
    let ready = false
    for (let i = 0; i < 100; i++) {
      if (server.exitCode !== null) throw new Error(logs)
      try { ready = (await fetch(base + '/health')).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    assert(ready, 'isolated frontend started: ' + logs)
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    async function session(width = 1440, theme = 'light', reducedMotion = 'reduce') {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, reducedMotion })
      await context.route('**/*', route => {
        const hostname = new URL(route.request().url()).hostname
        return hostname === '127.0.0.1' || hostname === 'localhost' ? route.continue() : route.abort()
      })
      await context.addInitScript(value => {
        localStorage.setItem('theme', value)
        window.__transitionCalls = 0
        const original = document.startViewTransition?.bind(document)
        if (original) document.startViewTransition = callback => { window.__transitionCalls++; return original(callback) }
      }, theme)
      const page = await context.newPage(); page.setDefaultTimeout(15000)
      page.on('pageerror', error => report.errors.push(error.message))
      return { context, page }
    }
    async function visit(page, route) { const response = await page.goto(base + route, { waitUntil: 'networkidle' }); assert.equal(response.status(), 200, route); await page.waitForFunction(() => !!document.querySelector('#__nuxt')?.__vue_app__); }
    async function imagesAndGeometry(page) {
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'no horizontal overflow at ' + page.url())
      await page.locator('main img').evaluateAll(async images => {
        for (const image of images) { image.loading = 'eager'; await image.decode(); if (!image.naturalWidth) throw new Error('empty image ' + image.src) }
      })
      for (const image of await page.locator('[data-article-card] .editorial-cover img, [data-project-card] .editorial-cover img').all()) {
        const ratio = await image.evaluate(img => ({ natural: img.naturalWidth / img.naturalHeight, box: img.getBoundingClientRect().width / img.getBoundingClientRect().height }))
        assert(Math.abs(ratio.box - 16 / 9) < 0.06, 'cover slot remains 16:9')
        assert(Math.abs(ratio.natural - 16 / 9) < 0.06, 'fixture cover source is 16:9')
      }
    }
    for (const width of [1440, 390, 320]) for (const theme of ['light', 'dark']) {
      const { page, context } = await session(width, theme)
      for (const route of ['/', '/posts', '/projects', '/series', '/series/blog-engineering', '/series/ai-delivery', '/about']) {
        await visit(page, route); await imagesAndGeometry(page)
        const pageCopy = await page.locator('main').innerText()
        for (const phrase of ['记录实现，也记录取舍。把问题拆开，把过程写清楚。', 'WRITING / ENGINEERING NOTES', 'SELECTED WORK / BUILDING IN PUBLIC', 'CODE · CREATE · EXPLORE', '沿着一个主题，读得更深。']) assert(!pageCopy.includes(phrase), 'decorative label or slogan removed: ' + phrase)
        if (route === '/') { assert.equal(await page.locator('[data-home-hero]').count(), 1, 'real home hero retained'); assert(await page.locator('.home-visual canvas').count(), 'city canvas retained') }
        if (route === '/about') assert(pageCopy.includes('Python 后端') && pageCopy.includes('Forge Harness'), 'concrete author and project information retained')
        if (route === '/posts') {
          assert.equal(await page.locator('[data-article-card]').count(), 9)
          for (const slug of slugs.slice(3, 5)) { const card = page.locator('[data-article-card]').filter({ has: page.locator(`a[href="/posts/${slug}"]`) }); assert.equal(await card.locator('img').count(), 1, slug + ' has a cover') }
          const contrast = await page.locator('[data-article-card]').first().evaluate(card => ({ card: getComputedStyle(card).backgroundColor, page: getComputedStyle(document.querySelector('.site-shell')).backgroundColor }))
          assert.notEqual(contrast.card, contrast.page, 'card and page surfaces are distinct')
          const cta = page.locator('a[href="/series"]').filter({ hasText: /系列/ }).filter({ visible: true })
          const colors = await cta.evaluateAll(nodes => nodes.map(n => ({ background: getComputedStyle(n).backgroundColor, text: getComputedStyle(n).color })))
          assert(colors.some(c => c.background !== 'rgba(0, 0, 0, 0)' && c.background !== c.text), 'series action has a visible filled surface')
        }
        if (route === '/projects') { assert.equal(await page.locator('[data-project-card]').count(), 5); const src = await page.locator('[data-project-card] img').evaluateAll(nodes => nodes.map(n => n.getAttribute('src'))); assert.equal(new Set(src).size, 5, 'five distinct project covers') }
        if (route === '/series') assert.equal(await page.locator('[data-series-card]').count(), 2)
        if (route.startsWith('/series/')) {
          assert.equal(await page.locator('[data-series-detail] [data-article-card]').count(), route.endsWith('blog-engineering') ? 5 : 4)
          const contained = await page.locator('[data-series-detail] li').evaluateAll(items => items.every(li => { const card = li.querySelector('[data-article-card]'); return !card || card.getBoundingClientRect().bottom <= li.getBoundingClientRect().bottom + 1 }))
          assert(contained, 'series cards remain within grid list items without overlapping the next row')
        }
        if (width !== 320) await page.screenshot({ path: path.join(out, 'screenshots', `${route.replaceAll('/', '-').slice(1)}-${width}-${theme}.png`), fullPage: true })
        report.cases.push({ route, width, theme, passed: true })
      }
      await context.close()
    }
    check('42 route/viewport/theme combinations: concise public copy, home/about, covers, public counts, surface hierarchy and overflow')
    for (const width of [1440, 390]) {
      const taxonomy = await session(width)
      for (const route of ['/tags', '/tags/Go', '/categories', '/categories/engineering']) {
        await visit(taxonomy.page, route); await imagesAndGeometry(taxonomy.page)
        assert.equal(await taxonomy.page.locator('main h1').count(), 1, 'taxonomy keeps its page title')
        report.cases.push({ route, width, theme: 'light', passed: true })
      }
      await taxonomy.context.close()
    }
    check('Taxonomy titles, article lists and no overflow remain after removing redundant descriptions')
    for (const width of [1440, 390]) {
      const detail = await session(width)
      for (const route of ['/posts/article-cas-revisions-case-study', '/projects/forge-harness']) {
        await visit(detail.page, route); await imagesAndGeometry(detail.page)
        assert.equal(await detail.page.locator('main .detail-cover').count(), 1, 'complete detail cover retained')
        report.cases.push({ route, width, theme: 'light', passed: true })
      }
      await detail.context.close()
    }
    check('Article/project detail at desktop and mobile: cover decode and no overflow')
    const { page, context } = await session()
    await visit(page, '/series')
    const seriesLink = page.locator('[data-series-card] a[href="/series/blog-engineering"]')
    const nativeDefaults = await seriesLink.evaluate(link => [ { ctrlKey: true }, { metaKey: true }, { button: 1 } ].map(init => {
      let prevented; document.addEventListener('click', e => { prevented = e.defaultPrevented; e.preventDefault() }, { once: true }); link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ...init })); return prevented
    }))
    assert.deepEqual(nativeDefaults, [false, false, false], 'modified series clicks remain native')
    await seriesLink.click(); await page.waitForURL('**/series/blog-engineering'); await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding'))
    assert(await page.locator('[data-series-detail] h1').evaluate(n => n === document.activeElement), 'series heading receives focus')
    assert.equal(await page.evaluate(() => window.__transitionCalls), 0, 'reduced-motion navigation avoids view transitions')
    const article = page.locator('[data-article-card]').filter({ has: page.locator('a[href="/posts/article-cas-revisions-case-study"]') })
    await article.locator('h2 a').click(); await page.waitForURL('**/posts/article-cas-revisions-case-study'); await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding'))
    assert(await page.locator('[data-article-detail] h1').evaluate(n => n === document.activeElement), 'article heading receives focus')
    await visit(page, '/projects'); await page.locator('[data-project-card] h2 a').first().click(); await page.waitForURL('**/projects/dev-workflow'); await page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding')); assert(await page.locator('[data-project-detail] h1').evaluate(n => n === document.activeElement))
    check('Series, article and project real navigation/focus; modified links; reduced-motion fallback')
    await visit(page, '/posts')
    const tag = page.locator('[data-article-card] .editorial-tags a').first()
    const beforeTag = await page.evaluate(() => window.__transitionCalls)
    await tag.click(); await page.waitForURL('**/tags/Go')
    assert.equal(await page.evaluate(() => window.__transitionCalls), beforeTag, 'independent tag navigates without card expansion')
    fixture.mode = 'private'; await visit(page, '/series/blog-engineering'); assert.equal(await page.locator('[data-series-detail] [data-article-card]').count(), 4)
    fixture.mode = 'error'; await visit(page, '/series'); await page.getByRole('alert').filter({ hasText: '系列文章暂时无法加载' }).waitFor()
    fixture.mode = 'normal'; await page.getByRole('button', { name: '重新加载', exact: true }).click(); await page.locator('[data-series-card]').first().waitFor()
    fixture.mode = 'empty'; await visit(page, '/series'); assert.equal(await page.locator('[data-series-card]').count(), 0); assert((await page.locator('main').innerText()).includes('暂无'))
    fixture.mode = 'normal'; fixture.custom = true; await visit(page, '/posts'); const custom = page.locator('[data-article-card] img').first(); assert((await custom.getAttribute('src')).includes('/assets/custom.svg')); await custom.evaluate(n => n.decode())
    fixture.custom = false; fixture.broken = true; await visit(page, '/posts'); await page.waitForFunction(() => [...document.querySelectorAll('[data-article-card]')].every(card => !card.querySelector('img'))); assert.equal(await page.locator('[data-article-card] h2 a').count(), 9, 'failed covers retain readable links')
    fixture.broken = false
    check('Independent tags, private exclusion, error/retry, empty series, CMS cover priority and broken-cover readable fallback')
    await context.close()
    const animated = await session(1440, 'light', 'no-preference'); await visit(animated.page, '/series'); assert(await animated.page.evaluate(() => !!document.startViewTransition), 'native transition API available'); await animated.page.locator('[data-series-card] a[href="/series/blog-engineering"]').click(); await animated.page.waitForURL('**/series/blog-engineering'); await animated.page.waitForFunction(() => !document.documentElement.hasAttribute('data-article-expanding')); assert.equal(await animated.page.evaluate(() => window.__transitionCalls), 1, 'series uses the shared real View Transition'); await animated.context.close()
    check('Series entry uses one real native View Transition when motion is enabled')
    assert.deepEqual(report.errors, [], 'no uncaught page errors')
    report.completed = true
  } catch (error) {
    report.failure = { message: error.message, stack: error.stack }
    throw error
  } finally {
    await browser?.close()
    if (server && server.exitCode === null) { const exit = new Promise(resolve => server.once('exit', resolve)); server.kill('SIGTERM'); await exit }
    if (api.listening) await new Promise(resolve => api.close(resolve))
    await fs.writeFile(path.join(out, 'server.log'), logs)
    await fs.writeFile(path.join(out, 'results.json'), JSON.stringify(report, null, 2))
  }
  console.log(JSON.stringify({ passed: report.cases.length, checks: report.checks, errors: report.errors }, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
