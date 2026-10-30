// Public-page regression against an isolated API; never connects to a database.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const sliceOne = process.argv.includes('--slice=1')
const out = path.join(root, 'output/page-differentiation')
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))

async function main() {
  const baseline = path.join(out, 'live-baseline')
  const fixture = JSON.parse(await fs.readFile(path.join(baseline, 'articles.json'), 'utf8')).articles
  const projects = JSON.parse(await fs.readFile(path.join(baseline, 'projects.json'), 'utf8')).projects
  const about = JSON.parse(await fs.readFile(path.join(baseline, 'about.json'), 'utf8'))
  const draft = { ...fixture[0], slug: 'private-draft-fixture', status: 1 }
  const hidden = { ...projects[0], slug: 'private-project-fixture', published: false }
  const published = fixture.filter(article => article.status === 2)
  let mode = 'normal'
  let customCovers = false
  const customCover = '/v1/assets/media/fixture-cover.webp'
  const customImage = await fs.readFile(path.join(root, 'frontend/public/images/editorial/covers/mysql.webp'))
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    const url = new URL(req.url, 'http://fixture')
    const reply = data => res.end(JSON.stringify(data))
    if (url.pathname === customCover) { res.setHeader('Content-Type', 'image/webp'); return res.end(customImage) }
    if (url.pathname.includes('/auth/')) { res.writeHead(401); return reply({}) }
    if (url.pathname.endsWith('/about')) return reply(about)
    if (url.pathname.includes('/collections')) return reply({ collections: [], next_page_token: '' })
    if (url.pathname.endsWith('/projects/list')) return reply({ projects: [...projects.map(p => ({ ...p, cover_url: customCovers ? customCover : p.cover_url })), hidden], next_page_token: '' })
    const projectSlug = url.pathname.match(/\/projects\/([^/]+)$/)?.[1]
    if (projectSlug) {
      const project = projects.find(p => p.slug === decodeURIComponent(projectSlug))
      if (project) return reply({ ...project, cover_url: customCovers ? customCover : project.cover_url })
      res.writeHead(404); return reply({})
    }
    if (url.pathname.endsWith('/articles/list')) {
      if (mode === 'error') { res.writeHead(503); return reply({}) }
      return reply({ articles: mode === 'empty' ? [] : [...fixture.map(a => ({ ...a, cover_url: customCovers ? customCover : a.cover_url })), draft], next_page_token: '' })
    }
    if (url.pathname.includes('/search/articles')) { res.writeHead(503); return reply({}) }
    const slug = decodeURIComponent(url.pathname.split('/articles/')[1]?.split('/')[0] || '')
    if (slug && req.method === 'GET' && !url.pathname.endsWith('/comments')) {
      const article = published.find(item => item.slug === slug)
      if (article) return reply({ ...article, cover_url: customCovers ? customCover : article.cover_url })
      res.writeHead(404); return reply({})
    }
    reply({ articles: [], comments: [], tags: [], categories: [], next_page_token: '' })
  })
  await listen(api)
  const reservation = http.createServer()
  await listen(reservation)
  const port = reservation.address().port
  await new Promise(resolve => reservation.close(resolve))
  const base = `http://127.0.0.1:${port}`
  const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
    cwd: path.join(root, 'frontend'), env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}` }, stdio: ['ignore', 'pipe', 'pipe'],
  })
  let logs = ''
  frontend.stdout.on('data', chunk => { logs += chunk })
  frontend.stderr.on('data', chunk => { logs += chunk })
  let browser
  const results = []
  try {
    let ready = false
    for (let i = 0; i < 100; i++) {
      if (frontend.exitCode !== null) throw new Error(logs)
      try { ready = (await fetch(base + '/about')).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    assert.ok(ready, logs)
    await fs.mkdir(out, { recursive: true })
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    for (const width of [1440, 390]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      const checkPage = async () => {
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'horizontal overflow')
        await page.locator('main img').evaluateAll(async images => Promise.all(images.map(async image => { image.loading = 'eager'; try { await image.decode() } catch { throw new Error(`image decode failed: ${image.src}`) } })))
      }
      for (const theme of ['light', 'dark']) {
        await page.addInitScript(value => localStorage.setItem('theme', value), theme)
        for (const route of ['/', '/posts', '/projects']) {
          assert.equal((await page.goto(base + route)).status(), 200)
          await page.evaluate(value => document.documentElement.classList.toggle('dark', value === 'dark'), theme)
          if (route === '/') {
            assert.equal(await page.locator('section[aria-label="最新文章"] .reading-card').count(), Math.min(3, published.length))
            assert.equal(await page.locator('.reading-card').count(), 3)
            assert.equal(await page.locator('section[aria-label="主推项目"] .work-card').count(), 2)
            await page.getByRole('region', { name: '可互动的模型城' }).waitFor()
            assert.ok(await page.locator('.home-visual canvas').count(), 'latest city retained')
          } else if (route === '/posts') {
            assert.equal(await page.locator('.reading-card').count(), published.length)
            for (const card of await page.locator('.reading-card').all()) {
              const layout = await card.evaluate(item => {
                const cover = item.querySelector('.editorial-cover').getBoundingClientRect()
                const text = item.querySelector('.editorial-body').getBoundingClientRect()
                return { below: text.top >= cover.bottom - 1, ratio: cover.width / cover.height, fit: getComputedStyle(item.querySelector('img')).objectFit }
              })
              assert.ok(layout.below && Math.abs(layout.ratio - 16 / 9) < .02, 'original cover card restored')
              assert.equal(layout.fit, 'contain')
            }
          } else {
            assert.equal(await page.locator('.work-card.poster-card').count(), 5)
            assert.equal(await page.locator('.project-tile, .project-showcase, .article-row').count(), 0)
            assert.equal(await page.locator('a[href="/projects/private-project-fixture"]').count(), 0)
          }
          await checkPage()
          await page.screenshot({ path: path.join(out, `${route === '/' ? 'home' : route.slice(1)}-${theme}-${width}.png`), fullPage: true })
          results.push(`${width} ${theme} ${route}: PASS`)
        }
      }
      customCovers = true
      await page.goto(base + '/posts')
      assert.ok((await page.locator('.reading-card img').first().getAttribute('src')).endsWith(customCover), 'uploaded article cover wins')
      await page.goto(base + '/projects')
      assert.ok((await page.locator('.work-card img').first().getAttribute('src')).endsWith(customCover), 'uploaded project cover wins')
      await page.route('**/api/v1/assets/media/fixture-cover.webp', route => route.abort())
      await page.goto(base + '/projects')
      await page.locator('.work-card:not(.has-cover)').first().waitFor()
      await page.unroute('**/api/v1/assets/media/fixture-cover.webp')
      customCovers = false
      await page.goto(base + '/projects')
      const projectTitle = page.locator('.work-card .editorial-link').first()
      await projectTitle.focus()
      assert.ok(await projectTitle.evaluate(link => link === document.activeElement), 'project keyboard focus')
      await page.locator('.work-card').first().hover()
      assert.equal(await page.locator('.work-card').first().evaluate(card => getComputedStyle(card).transform), 'none', 'reduced motion')
      await page.getByRole('button', { name: '搜索作品集', exact: true }).click()
      await page.getByRole('button', { name: 'React', exact: true }).click()
      assert.equal(await page.locator('.work-card').count(), 1)
      await page.getByRole('link', { name: 'Forge Harness', exact: true }).click()
      await page.waitForURL('**/projects/forge-harness')
      await page.goto(base + '/projects')
      await page.getByRole('button', { name: '搜索作品集', exact: true }).click()
      await page.getByRole('button', { name: 'Python', exact: true }).click()
      assert.equal(await page.locator('.work-card').count(), 4)
      await page.goto(base + '/posts')
      await page.getByRole('button', { name: '搜索文章', exact: true }).click()
      await page.getByRole('searchbox', { name: '搜索文章' }).fill('Ent 多对多')
      await page.getByText('搜索服务暂不可用，已显示本地匹配结果。').waitFor()
      assert.equal(await page.locator('.reading-card').count(), 1)
      assert.equal(await page.locator('.reading-card .editorial-link').getAttribute('href'), '/posts/ent-m2m-edge-case-study')
      await page.getByRole('searchbox', { name: '搜索文章' }).fill('不存在的文章zzzzz')
      await page.getByText('没有找到匹配的文章。').waitFor()
      await page.getByRole('searchbox', { name: '搜索文章' }).fill('')
      await page.waitForFunction(count => document.querySelectorAll('.reading-card').length === count, published.length)
      const articleTitle = page.locator('.reading-card .editorial-link').first()
      await articleTitle.focus()
      assert.ok(await articleTitle.evaluate(link => link === document.activeElement), 'article title keyboard focus')
      await articleTitle.press('Enter')
      await page.waitForURL(`**/posts/${published[0].slug}`)
      await page.goto(base + '/posts')
      const tag = page.locator('.editorial-tags a[href^="/tags/"]').first()
      await tag.focus()
      assert.ok(await tag.evaluate(link => link === document.activeElement), 'independent tag keyboard focus')
      await tag.press('Enter')
      await page.waitForURL('**/tags/**')
      assert.ok(new URL(page.url()).pathname.startsWith('/tags/'))
      for (const route of ['/', '/posts']) {
        mode = 'error'
        await page.goto(base + route)
        await page.getByRole('alert').waitFor()
        mode = 'normal'
        await page.getByRole('button', { name: '重新加载' }).click()
        await page.locator(route === '/' ? '.home-articles .reading-card' : '.reading-card').first().waitFor()
        mode = 'empty'
        await page.goto(base + route)
        await page.getByText('第一篇文章正在路上。').waitFor()
      }
      mode = 'normal'
      await page.route('**/images/editorial/covers/guard.webp', route => route.abort())
      await page.goto(base + '/posts')
      await page.locator('.reading-card img').evaluateAll(images => images.forEach(image => { image.loading = 'eager' }))
      await page.locator('.reading-card:not(.has-cover)').first().waitFor()
      await page.locator('.reading-card').filter({ has: page.locator('a[href="/posts/ai-delivery-guard"]') }).getByRole('link').first().click()
      await page.waitForURL('**/posts/ai-delivery-guard')
      await page.unroute('**/images/editorial/covers/guard.webp')
      if (!sliceOne) {
        await page.locator('h1').filter({ hasText: published.find(article => article.slug === 'ai-delivery-guard').title }).waitFor()
        assert.equal(await page.locator('main figure img[src*="/covers/"]').count(), 0, 'detail repeats promotional cover')
        const projectLinks = page.locator('[aria-label="相关项目"] a[href^="/projects/"]')
        await projectLinks.first().waitFor()
        assert.ok(await projectLinks.count() > 0, 'reverse project association missing')
        await projectLinks.first().click()
        await page.waitForURL('**/projects/**')
        const links = page.locator('[aria-label="项目实践文章"] a[href^="/posts/"]')
        await links.first().waitFor()
        assert.ok(await links.count() > 0, 'published association missing')
        for (const href of await links.evaluateAll(items => items.map(item => item.getAttribute('href'))))
          assert.ok(published.some(article => href === `/posts/${article.slug}`), 'draft association leaked')
        mode = 'error'
        await page.goto(base + '/projects/dev-workflow')
        assert.equal(await page.locator('h1').innerText(), 'dev-workflow', 'static project survives article API failure')
        await page.getByRole('alert').filter({ hasText: '相关文章暂时加载失败' }).waitFor({ timeout: 3000 })
        mode = 'normal'
        await page.getByRole('button', { name: '重新加载' }).click()
        await page.locator('[aria-label="项目实践文章"] a[href^="/posts/"]').first().waitFor()
        for (const draft of fixture.filter(article => article.status !== 2))
          assert.equal(await page.locator(`main a[href="/posts/${draft.slug}"]`).count(), 0, 'draft leaked')
        await page.route('**/images/projects/dev-workflow.svg', route => route.abort())
        await page.goto(base + '/projects')
        await page.locator('.work-card:not(.has-cover)').first().waitFor()
        await page.getByRole('link', { name: 'dev-workflow', exact: true }).click()
        await page.waitForURL('**/projects/dev-workflow')
        await page.getByRole('heading', { name: 'dev-workflow', exact: true, level: 1 }).waitFor()
        await page.waitForFunction(() => !document.querySelector('figure img[src$="projects/dev-workflow.svg"]'))
        await page.unroute('**/images/projects/dev-workflow.svg')
        await page.goto(base + '/projects')
        const sources = await page.locator('.work-card img').evaluateAll(images => images.map(image => new URL(image.src).pathname))
        assert.equal(sources.length, 5)
        assert.equal(new Set(sources).size, 5, 'distinct project covers')
        await checkPage()
        for (const route of ['/projects/dev-workflow', '/projects/forge-harness', '/projects/aigc', '/projects/luohao-blog', '/projects/job-application-workbench', ...published.map(article => `/posts/${article.slug}`)]) {
          assert.equal((await page.goto(base + route)).status(), 200, route)
          await checkPage()
          if (route.startsWith('/projects/')) {
            const shareImage = await page.locator('meta[property="og:image"]').getAttribute('content')
            const assetPath = `/images/projects/${route.split('/').pop()}.png`
            assert.equal(new URL(shareImage).pathname, assetPath, 'distinct project share image')
            assert.equal((await page.request.get(base + assetPath)).status(), 200)
          }
          if (route === '/projects/job-application-workbench') assert.equal(await page.locator('[aria-label="项目实践文章"]').count(), 0, 'empty related section')
          if (route.startsWith('/posts/')) {
            assert.ok(await page.locator('.article-content').count(), 'current reading component retained')
            const image = page.locator('.image-preview-trigger').first()
            if (await image.count()) {
              await image.click()
              await page.getByRole('dialog', { name: '图片预览', exact: true }).waitFor()
              await page.getByRole('button', { name: '关闭预览' }).click()
            }
          }
          if (route.startsWith('/posts/')) assert.equal(await page.locator('main figure img[src*="/covers/"]').count(), 0, 'repeated article cover')
          await page.screenshot({ path: path.join(out, `${route.slice(1).replaceAll('/', '-')}-${width}.png`), fullPage: true })
        }
      }
      if (!sliceOne) {
        for (const route of ['/projects/does-not-exist', '/posts/does-not-exist', `/posts/${draft.slug}`, `/projects/${hidden.slug}`]) {
          assert.equal((await page.goto(base + route)).status(), 404, `404 isolation: ${route}`)
        }
      }
      assert.deepEqual(errors, [])
      results.push(`${width} search/filter/navigation/empty/error/image-fallback${sliceOne ? '' : '/associations/draft-isolation'}: PASS`)
      await context.close()
    }
    await fs.writeFile(path.join(out, sliceOne ? 'slice1-results.txt' : 'results.txt'), results.join('\n') + '\n')
    console.log(results.join('\n'))
  } finally {
    await browser?.close()
    frontend.kill('SIGTERM')
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
