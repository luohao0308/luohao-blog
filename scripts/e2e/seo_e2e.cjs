// Local-only SSR/Nitro SEO integration; never contacts production or loads secrets.
const assert = require('node:assert/strict')
const http = require('node:http')
const net = require('node:net')
const path = require('node:path')
const { spawn, spawnSync } = require('node:child_process')
const { chromium } = require('playwright')

const root = path.resolve(__dirname, '../..')
const origin = 'https://makerhao.cn'
const stamp = '2026-10-12T08:00:00Z'
const media = `/v1/assets/media/${'a'.repeat(32)}.png`
const special = '真实摘要：Go、Vue 与 "引用" <标签> & 内容'
const articles = Array.from({ length: 121 }, (_, index) => ({
  id: `article-id-${index}`, slug: `seo-article-${String(index).padStart(3, '0')}`,
  title: index ? `文章 ${index}` : '文章标题 "引用" & Vue', summary: special,
  content_md: '# 真实正文\n\n技术笔记。', content_html: '<h1 id="content">真实正文</h1><p>技术笔记。</p>',
  status: 2, tags: ['Go & Vue', '中文'], category: { slug: 'engineering', name: '工程' },
  cover_url: media, featured: false, created_at: stamp, updated_at: stamp, published_at: stamp,
}))
articles.push({ ...articles[0], id: 'draft', slug: 'private-draft', status: 1 })
const projects = Array.from({ length: 111 }, (_, index) => ({
  id: `project-id-${index}`, slug: `seo-project-${String(index).padStart(3, '0')}`,
  name: index ? `项目 ${index}` : '项目标题 "引用" & Go', summary: special,
  description: '真实项目说明。', stack: ['Go'], role: '开发', year: '2026', emoji: '🛠',
  cover_url: media, screenshots: [], problem: '', approach: '', outcome: '',
  published: true, sort: index, featured: false, created_at: stamp, updated_at: stamp,
}))
projects.push({ ...projects[0], id: 'hidden-project', slug: 'private-project', published: false })
const about = { name: 'luohao', headline: '全栈开发与 AI Agent', intro: special, current_focus: '工程实践', skills: [], timeline: [], contacts: [], representative_links: [], updated_at: stamp }
let failure = false
let repeatToken = false
let emptyPageLoop = false
let emptyPageCalls = 0
let hangArticles = false
const requests = []

function send(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
  response.end(JSON.stringify(body))
}
function page(items, url, field) {
  if (emptyPageLoop) return { [field]: [], next_page_token: `empty-${++emptyPageCalls}` }
  const token = url.searchParams.get('page_token') || ''
  const offset = token ? Number(token.split('-').at(-1)) : 0
  const size = Math.min(Number(url.searchParams.get('page_size') || 20), 100)
  return { [field]: items.slice(offset, offset + size), next_page_token: repeatToken ? 'page-0' : offset + size < items.length ? `page-${offset + size}` : '' }
}
const backend = http.createServer((request, response) => {
  const url = new URL(request.url, 'http://localhost')
  requests.push({ path: url.pathname, token: url.searchParams.get('page_token') || '', auth: Boolean(request.headers.authorization), cookie: Boolean(request.headers.cookie) })
  if (url.pathname === '/v1/articles/list' && hangArticles) {
    const timer = setTimeout(() => send(response, 503, { code: 503 }), 15000)
    response.once('close', () => clearTimeout(timer))
    return
  }
  if (url.pathname === '/v1/articles/list') return send(response, failure ? 503 : 200, failure ? { code: 503, reason: 'FIXTURE_UNAVAILABLE' } : page(articles, url, 'articles'))
  if (url.pathname === '/v1/projects/list') return send(response, 200, page(projects, url, 'projects'))
  if (url.pathname === '/v1/about') return send(response, 200, about)
  if (url.pathname === '/v1/categories/list') return send(response, 200, { categories: [{ id: 'category', slug: 'engineering', name: '工程', sort: 0, article_count: 121 }] })
  if (url.pathname.startsWith('/v1/articles/')) {
    const item = articles.find(item => item.slug === decodeURIComponent(url.pathname.slice('/v1/articles/'.length)) && item.status === 2)
    return send(response, item ? 200 : 404, item || { code: 404 })
  }
  if (url.pathname.startsWith('/v1/projects/')) {
    const item = projects.find(item => item.slug === decodeURIComponent(url.pathname.slice('/v1/projects/'.length)) && item.published)
    return send(response, item ? 200 : 404, item || { code: 404 })
  }
  return send(response, url.pathname.includes('/auth/') ? 401 : 404, { code: url.pathname.includes('/auth/') ? 401 : 404 })
})

async function freePort() {
  const server = net.createServer()
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const port = server.address().port
  await new Promise(resolve => server.close(resolve))
  return port
}
async function waitReady(base, child) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (child.exitCode !== null) throw new Error(`preview exited: ${child.exitCode}`)
    try { if ((await fetch(base + '/health')).ok) return }
    catch {}
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  throw new Error('local preview did not become ready')
}
function parseSitemap(xml) {
  const result = spawnSync('python3', ['-c', 'import sys,json,xml.etree.ElementTree as E; r=E.fromstring(sys.stdin.read()); n={"s":"http://www.sitemaps.org/schemas/sitemap/0.9"}; print(json.dumps({"tag":r.tag,"locs":[x.text for x in r.findall("s:url/s:loc",n)],"dates":[x.text for x in r.findall("s:url/s:lastmod",n)]}))'], { input: xml, encoding: 'utf8' })
  assert.equal(result.status, 0, `sitemap must parse as XML: ${result.stderr}`)
  return JSON.parse(result.stdout)
}

async function main() {
  await new Promise(resolve => backend.listen(0, '127.0.0.1', resolve))
  const port = await freePort()
  const base = `http://127.0.0.1:${port}`
  const backendBase = `http://127.0.0.1:${backend.address().port}`
  const preview = spawn(process.execPath, ['.output/server/index.mjs'], {
    cwd: path.join(root, 'frontend'), stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, NUXT_BACKEND_BASE: backendBase, NUXT_PUBLIC_SITE_URL: origin, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port) },
  })
  let logs = ''
  for (const stream of [preview.stdout, preview.stderr]) stream.on('data', chunk => { logs = (logs + chunk.toString()).slice(-6000) })
  let browser
  try {
    await waitReady(base, preview)
    const robots = await fetch(base + '/robots.txt', { headers: { 'X-Forwarded-Host': 'evil.example' } })
    assert.equal(robots.status, 200, 'robots endpoint must exist')
    assert.match(robots.headers.get('content-type'), /text\/plain/)
    const robotsText = await robots.text()
    assert.match(robotsText, /Sitemap: https:\/\/makerhao\.cn\/sitemap\.xml/)
    assert.ok(!robotsText.includes('evil.example'))
    assert.ok(!/^Disallow:\s*\/api\/\s*$/m.test(robotsText), 'robots must allow public sharing images')

    requests.length = 0
    const sitemap = await fetch(base + '/sitemap.xml', { headers: { Cookie: 'untrusted=fake', Authorization: 'Bearer fixture-invalid', 'X-Forwarded-Host': 'evil.example' } })
    assert.equal(sitemap.status, 200)
    assert.match(sitemap.headers.get('content-type'), /xml/)
    assert.match(sitemap.headers.get('cache-control'), /no-store|no-cache/)
    const xml = await sitemap.text()
    const parsed = parseSitemap(xml)
    assert.equal(parsed.tag, '{http://www.sitemaps.org/schemas/sitemap/0.9}urlset')
    assert.ok(parsed.locs.includes(origin + '/posts/seo-article-120'), 'last article page must be included')
    assert.ok(parsed.locs.includes(origin + '/projects/seo-project-110'), 'last project page must be included')
    assert.equal(parsed.locs.filter(loc => loc.startsWith(origin + '/posts/')).length, 121)
    assert.equal(parsed.locs.filter(loc => loc.startsWith(origin + '/projects/')).length, 111)
    assert.equal(new Set(parsed.locs).size, parsed.locs.length)
    assert.ok(parsed.locs.every(loc => loc.startsWith(origin + '/') && !/private-|\/admin|\/login|\/register|\/settings|\/collections|reset-password|verify-email|evil\.example/.test(loc)))
    assert.ok(parsed.dates.some(value => value === '2026-10-12' || new Date(value).getTime() === new Date(stamp).getTime()))
    for (const endpoint of ['/v1/articles/list', '/v1/projects/list']) assert.ok(requests.some(item => item.path === endpoint && item.token), `must paginate ${endpoint}`)
    assert.ok(requests.every(item => !item.auth && !item.cookie), 'sitemap must not forward caller session')

    articles[120].status = 1
    projects[110].published = false
    const fresh = parseSitemap(await (await fetch(base + '/sitemap.xml')).text())
    assert.ok(!fresh.locs.includes(origin + '/posts/seo-article-120'))
    assert.ok(!fresh.locs.includes(origin + '/projects/seo-project-110'))
    failure = true
    const unavailable = await fetch(base + '/sitemap.xml')
    assert.equal(unavailable.status, 503, 'backend failure must not become a successful empty map')
    assert.match(unavailable.headers.get('cache-control'), /no-store|no-cache/)
    failure = false
    repeatToken = true
    assert.equal((await fetch(base + '/sitemap.xml')).status, 503, 'repeating page token must terminate safely')
    repeatToken = false
    emptyPageLoop = true
    assert.equal((await fetch(base + '/sitemap.xml', { signal: AbortSignal.timeout(20000) })).status, 503, 'unique tokens with empty pages must be bounded')
    assert.ok(emptyPageCalls <= 1100, 'upstream pagination must have an explicit finite page budget')
    emptyPageLoop = false
    hangArticles = true
    const started = Date.now()
    assert.equal((await fetch(base + '/sitemap.xml', { signal: AbortSignal.timeout(14000) })).status, 503, 'hung upstream must fail within its own timeout')
    assert.ok(Date.now() - started < 14000)
    hangArticles = false

    browser = await chromium.launch({ headless: true, ...(process.env.BLOG_E2E_CHROMIUM ? { executablePath: process.env.BLOG_E2E_CHROMIUM } : {}) })
    const context = await browser.newContext()
    const browserPage = await context.newPage()
    const errors = []
    browserPage.on('dialog', dialog => { errors.push(dialog.message()); dialog.dismiss() })
    for (const [url, title, description, image] of [
      ['/posts/seo-article-000', articles[0].title, special, origin + '/api' + media],
      ['/projects/seo-project-000', projects[0].name, special, origin + '/api' + media],
      ['/about', '', special, ''],
      ['/', '', '', ''],
      ['/posts', '', '', ''],
    ]) {
      const response = await browserPage.goto(base + url + '?tracking=fixture')
      assert.equal(response.status(), 200, `public ${url}`)
      const head = await browserPage.evaluate(() => ({
        title: document.title,
        canonical: document.querySelector('link[rel=canonical]')?.getAttribute('href'),
        description: document.querySelector('meta[name=description]')?.getAttribute('content'),
        ogTitle: document.querySelector('meta[property="og:title"]')?.getAttribute('content'),
        ogDescription: document.querySelector('meta[property="og:description"]')?.getAttribute('content'),
        ogImage: document.querySelector('meta[property="og:image"]')?.getAttribute('content'),
        ogURL: document.querySelector('meta[property="og:url"]')?.getAttribute('content'),
        twitter: document.querySelector('meta[name="twitter:card"]')?.getAttribute('content'),
      }))
      assert.equal(head.canonical, origin + url)
      assert.equal(head.ogURL, origin + url)
      if (title) assert.ok(head.title.includes(title))
      assert.ok(head.ogTitle && head.description && head.ogDescription && head.twitter, `metadata for ${url}`)
      if (description) { assert.equal(head.description, description); assert.equal(head.ogDescription, description) }
      if (image) assert.equal(head.ogImage, image)
      assert.ok(head.ogImage.startsWith(origin + '/'), `absolute share image ${url}`)
      // Check raw SSR HTML too: client-only metadata must not satisfy the gate.
      const html = await (await fetch(base + url)).text()
      assert.match(html, /name="description"/)
      assert.match(html, /property="og:title"/)
      assert.match(html, /property="og:image"/)
    }
    for (const url of ['/login', '/register', '/settings', '/collections', '/admin/login']) {
      const html = await (await fetch(base + url)).text()
      assert.match(html, /name="robots"[^>]*content="[^"]*noindex/, `private SSR ${url}`)
      assert.ok(!/<link[^>]*rel="canonical"/.test(html), `private canonical ${url}`)
    }
    assert.deepEqual(errors, [])
    await context.close()
    console.log('SEO PASS: robots, valid XML, 121 articles/111 projects pagination, public-only/no caller auth, fresh unpublish, failure/repeating-token/empty-page/timeout 503, SSR descriptions/canonical/OG, escaping, private noindex')
  }
  catch (error) { console.error(logs); throw error }
  finally {
    if (browser) await browser.close()
    preview.kill('SIGTERM')
    await new Promise(resolve => preview.exitCode !== null ? resolve() : preview.once('exit', resolve))
    await new Promise(resolve => backend.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
