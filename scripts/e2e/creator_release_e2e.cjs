// Isolated public-page smoke: starts only its own frontend and mock API.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')

async function main() {
  const articles = process.env.BLOG_E2E_ARTICLE_FIXTURE
    ? JSON.parse(await fs.readFile(process.env.BLOG_E2E_ARTICLE_FIXTURE, 'utf8'))
    : []
  const projects = JSON.parse(await fs.readFile(path.resolve(__dirname, '../../output/profile-content/projects-fixture.json'), 'utf8'))
  const about = JSON.parse(await fs.readFile(path.resolve(__dirname, '../../output/profile-content/about-fixture.json'), 'utf8'))
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    if (req.url.includes('/auth/')) {
      res.writeHead(401)
      return res.end('{}')
    }
    if (req.url.includes('/projects/list')) return res.end(JSON.stringify(projects))
    if (req.url.includes('/about')) return res.end(JSON.stringify(about))
    const project = projects.projects.find(p => p.slug === req.url.split('/projects/')[1]?.split(/[/?]/)[0])
    if (project) return res.end(JSON.stringify(project))
    if (req.url.includes('/articles/list'))
      return res.end(JSON.stringify({ articles: articles.filter(a => a.status === 2), next_page_token: '' }))
    const slug = req.url.split('/articles/')[1]?.split(/[/?]/)[0]
    const article = articles.find(a => a.slug === slug)
    if (article && req.method === 'GET') return res.end(JSON.stringify(article))
    res.end(JSON.stringify({ articles: [], comments: [], tags: [], categories: [], next_page_token: '' }))
  })
  await new Promise(resolve => api.listen(0, '127.0.0.1', resolve))
  const port = 13028
  const base = `http://127.0.0.1:${port}`
  const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
    cwd: path.resolve(__dirname, '../../frontend'),
    env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}` },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  let logs = ''
  frontend.stdout.on('data', data => { logs += data })
  frontend.stderr.on('data', data => { logs += data })
  let browser
  const out = path.resolve(__dirname, '../../output/profile-content')
  const results = []
  try {
    let ready = false
    for (let attempt = 0; attempt < 80; attempt++) {
      if (frontend.exitCode !== null) throw new Error(logs)
      try { ready = (await fetch(`${base}/about`)).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 250))
    }
    assert.ok(ready, `frontend did not start: ${logs}`)
    await fs.mkdir(out, { recursive: true })
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    for (const size of [{ width: 1440, height: 1000 }, { width: 390, height: 844 }]) {
      const context = await browser.newContext({ viewport: size, reducedMotion: 'reduce' })
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      for (const route of ['/', '/about', '/posts', '/projects', '/projects/dev-workflow', '/projects/forge-harness', '/projects/aigc', '/projects/luohao-blog', '/projects/job-application-workbench', ...articles.map(a => `/posts/${a.slug}`)]) {
        const response = await page.goto(base + route)
        assert.equal(response.status(), 200, route)
        await page.locator('h1').waitFor()
        const body = await page.locator('main').innerText()
        assert.ok(!body.includes('Go / Vue 双栈'), route)
        assert.equal(await page.locator('meta[name="description"]').count() > 0 || route.startsWith('/projects/') || route.startsWith('/posts/'), true)
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `overflow ${route} ${size.width}`)
        if (route === '/') {
          for (const name of ['dev-workflow', 'Forge Harness', 'AIGC 应用平台'])
            assert.ok(body.includes(name), `missing featured ${name}`)
        }
        if (route === '/about') {
          assert.ok(body.includes('Django / DRF'))
          assert.ok(body.includes('个人项目为主'))
        }
        if (route === '/projects/aigc') {
          assert.equal(await page.getByRole('link', { name: '查看源码' }).count(), 0)
          assert.ok(body.includes('前端由其他同事负责'))
        }
        // Decode every editorial image, including offscreen lazy images.
        await page.locator('main img[src^="/images/editorial/"]').evaluateAll(async imgs => {
          await Promise.all(imgs.map(async img => { img.loading = 'eager'; await img.decode() }))
        })
        for (const poster of await page.locator('.poster-card').all()) {
          const layout = await poster.evaluate(card => {
            const cover = card.querySelector('.editorial-cover')
            const image = cover.querySelector('img')
            const text = card.querySelector('.editorial-body')
            const rect = cover.getBoundingClientRect()
            return { ratio: rect.width / rect.height, fit: getComputedStyle(image).objectFit,
              overlay: getComputedStyle(card, '::before').content,
              textBelow: text.getBoundingClientRect().top >= rect.bottom - 1 }
          })
          assert.ok(Math.abs(layout.ratio - 16 / 9) < 0.02, `poster ratio ${route} ${JSON.stringify(layout)}`)
          assert.equal(layout.fit, 'contain', 'poster must not crop')
          assert.equal(layout.overlay, 'none', 'poster must not have dark overlay')
          assert.ok(layout.textBelow, 'card title must stay below poster')
        }
        if (route.startsWith('/posts/')) {
          assert.ok(await page.locator('.markdown-body img').count() >= 2, 'article illustrations missing')
          assert.ok(await page.locator('.markdown-body a[href^="https://"]').count() >= 2, 'references missing')
        }
        if (route === '/projects') {
          await page.getByRole('button', { name: '搜索作品集', exact: true }).click()
          await page.getByRole('button', { name: 'React', exact: true }).click()
          assert.equal(await page.locator('.work-card').count(), 1)
          await page.getByRole('link', { name: 'Forge Harness', exact: true }).click()
          await page.waitForURL('**/projects/forge-harness')
          await page.goto(base + route)
          await page.getByRole('button', { name: '搜索作品集', exact: true }).click()
          await page.getByRole('button', { name: 'Python', exact: true }).click()
          assert.equal(await page.locator('.work-card').count(), 4)
          await page.getByRole('button', { name: '全部', exact: true }).click()
          assert.equal(await page.locator('.work-card').count(), 5)
        }
        if (['/about', '/posts', '/projects', '/', '/posts/forge-harness-project', '/posts/ent-m2m-edge-case-study', '/projects/aigc', '/projects/dev-workflow'].includes(route)) {
          if (route === '/projects') await page.goto(base + route)
          await page.locator('main img[src^="/images/editorial/"]').evaluateAll(async imgs => {
            await Promise.all(imgs.map(async img => { img.loading = 'eager'; await img.decode() }))
          })
          await page.screenshot({ path: path.join(out, `${route === '/' ? 'home' : route.slice(1).replaceAll('/', '-')}-${size.width}.png`), fullPage: true })
        }
        results.push(`${size.width} ${route}: PASS`)
      }
      await page.goto(base + '/projects')
      await page.locator('main img[src^="/images/editorial/"]').evaluateAll(async imgs => {
        await Promise.all(imgs.map(async img => { img.loading = 'eager'; await img.decode() }))
      })
      await page.evaluate(() => document.documentElement.classList.add('dark'))
      await page.screenshot({ path: path.join(out, `project-dark-${size.width}.png`), fullPage: true })
      await page.route('**/images/editorial/covers/workflow.webp', route => route.abort())
      await page.goto(base + '/projects')
      await page.locator('.work-card:not(.has-cover)').first().waitFor()
      await page.getByRole('link', { name: 'dev-workflow', exact: true }).click()
      await page.waitForURL('**/projects/dev-workflow')
      await page.waitForFunction(() => !document.querySelector('figure img[src$="covers/workflow.webp"]'))
      assert.deepEqual(errors, [], 'browser errors')
      await context.close()
    }
    await fs.writeFile(path.join(out, 'browser-results.txt'), results.join('\n') + '\n')
    console.log(results.join('\n'))
  }
  finally {
    await browser?.close()
    frontend.kill('SIGTERM')
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
