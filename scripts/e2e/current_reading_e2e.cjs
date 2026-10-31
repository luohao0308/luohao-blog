// Public-page regression against an isolated API; never connects to a database.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const fs = require('node:fs/promises')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const sliceOne = process.argv.includes('--slice=1')
const out = path.join(root, 'output/current-reading')
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))

async function main() {
  const { server: api, projects, articles: fixture } = require('./current_reading_fixture.cjs').createFixture()
  const published = fixture.filter(article => article.status === 2)
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
      for (const colorScheme of ['light', 'dark']) {
        const context = await browser.newContext({ viewport: { width, height: 900 }, colorScheme })
        const page = await context.newPage()
        const errors = []
        page.on('pageerror', error => errors.push(error.message))
        await page.addInitScript(theme => localStorage.setItem('theme', theme), colorScheme)
        await page.goto(base + '/')
        assert.equal(await page.locator('[data-home-shell]').count(), 1)
        await page.locator('main img').evaluateAll(async images => {
          for (const image of images) { image.loading = 'eager'; await image.decode() }
        })
        await page.screenshot({ path: path.join(out, `home-${width}-${colorScheme}.png`), fullPage: true })
        await page.goto(base + '/projects')
        const projectSavedLink = page.getByRole('link', { name: '我的收藏', exact: true })
        assert.equal(await projectSavedLink.count(), 1)
        assert.ok((await projectSavedLink.boundingBox()).height >= 44)
        const covers = page.locator('[data-project-card] .editorial-cover img')
        assert.equal(await covers.count(), 5)
        await covers.evaluateAll(async images => {
          const paths = new Set()
          for (const img of images) {
            img.loading = 'eager'
            await img.decode()
            if (!img.src.includes('/images/projects/generated/') || !img.src.endsWith('-editorial-v2.webp')) throw new Error('unexpected project cover')
            if (Math.abs(img.naturalWidth / img.naturalHeight - 16 / 9) > .01) throw new Error('cover ratio')
            paths.add(img.src)
          }
          if (paths.size !== 5) throw new Error('shared project cover')
        })
        await page.getByRole('button', { name: '搜索作品集', exact: true }).click()
        const projectSearch = page.getByRole('searchbox', { name: '搜索作品集' })
        await projectSearch.fill('Forge Harness')
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
        await projectSearch.fill('')
        await page.keyboard.press('Escape')
        await page.screenshot({ path: path.join(out, `project-covers-${width}-${colorScheme}.png`), fullPage: true })
        assert.equal(await page.locator('header a[aria-label="搜索文章"]').count(), 0)
        assert.equal(await page.locator('footer a[href="/collections"]').count(), 0)
        assert.equal(await page.locator('header a[href="/collections"]').count(), 0)
        await page.goto(base + '/posts')
        await page.locator('.reading-card img').evaluateAll(async images => {
          if (images.length !== 7) throw new Error('missing original article covers')
          for (const image of images) {
            image.loading = 'eager'; await image.decode()
            if (!image.src.includes('/images/editorial/covers/')) throw new Error('article cover changed')
          }
        })
        const savedLink = page.getByRole('link', { name: '我的收藏', exact: true })
        assert.equal(await savedLink.count(), 1)
        assert.ok((await savedLink.boundingBox()).height >= 44)
        await savedLink.click()
        await page.waitForURL('**/collections')
        await page.getByRole('heading', { name: '收藏', exact: true }).waitFor()
        await page.goto(base + '/posts')
        await page.screenshot({ path: path.join(out, `collections-D-${width}-${colorScheme}.png`) })
        await page.getByRole('button', { name: '搜索文章', exact: true }).click()
        await page.getByRole('searchbox', { name: '搜索文章' }).fill('Ent')
        await page.waitForTimeout(600)
        await page.getByRole('heading', { name: published.find(article => article.slug === 'ent-m2m-edge-case-study').title, exact: true }).waitFor()
        assert.ok(await page.locator('.reading-card').count() < published.length)
        for (const slug of ['dev-workflow', 'forge-harness', 'aigc', 'luohao-blog', 'job-application-workbench']) {
          assert.equal((await page.goto(base + '/projects/' + slug)).status(), 200)
          assert.equal(await page.locator('main figure').count(), 0, 'no repeated template poster in project detail')
          assert.equal(await page.getByText('实践与技术文章', { exact: true }).count(), 0)
          assert.equal(await page.getByRole('heading', { name: '相关项目' }).count(), 0)
          if (['aigc', 'forge-harness'].includes(slug)) {
            await page.screenshot({ path: path.join(out, `${slug}-${width}-${colorScheme}.png`), fullPage: true })
          }
          const source = page.getByRole('link', { name: '查看源码' })
          assert.equal(await source.count(), ['aigc', 'job-application-workbench'].includes(slug) ? 0 : 1)
          if (await source.count()) assert.equal(await source.evaluate(el => !!el.closest('footer')), true)
          for (const back of await page.getByRole('link', { name: '返回作品集' }).all()) {
            assert.ok((await back.boundingBox()).height >= 44)
          }
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
        }
        await page.screenshot({ path: path.join(out, `project-${width}-${colorScheme}.png`), fullPage: true })
        await page.goto(base + '/posts/' + published[0].slug)
        assert.ok((await page.getByRole('link', { name: '返回全部文章' }).boundingBox()).height >= 44)
        assert.equal(await page.getByRole('heading', { name: '相关文章', exact: true }).count(), 0)
        await page.locator('.markdown-body img').evaluateAll(async images => {
          for (const img of images) {
            img.loading = 'eager'
            await img.decode()
            const box = img.getBoundingClientRect()
            if (box.width > 545 || box.height > 385 || box.width > innerWidth) throw new Error('oversized figure')
          }
        })
        await page.screenshot({ path: path.join(out, `article-${width}-${colorScheme}.png`), fullPage: true })
        await page.route('**/api/v1/auth/refresh', route => route.fulfill({ json: {
          access_token: 'fixture-access', token_type: 'Bearer', expires_in: 900,
          user: { id: 'fixture-reader', email: 'reader@example.test', display_name: '测试读者', role: 2 },
        } }))
        await page.goto(base + '/')
        await page.getByRole('button', { name: '测试读者 的账号菜单' }).click()
        await page.keyboard.press('Escape')
        assert.equal(await page.getByRole('navigation', { name: '账号导航' }).count(), 0)
        await page.getByRole('button', { name: '测试读者 的账号菜单' }).click()
        await page.screenshot({ path: path.join(out, `collections-menu-${width}-${colorScheme}.png`) })
        const accountNav = page.getByRole('navigation', { name: '账号导航' })
        assert.equal(await accountNav.getByRole('link', { name: '我的收藏' }).count(), 0)
        await accountNav.getByRole('link', { name: '切换账号' }).waitFor()
        await page.keyboard.press('Escape')
        await page.goto(base + '/posts')
        assert.equal(await page.getByRole('link', { name: '我的收藏', exact: true }).count(), 1)
        await page.getByRole('button', { name: '搜索文章', exact: true }).click()
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
        await page.keyboard.press('Escape')
        assert.ok(await page.getByRole('button', { name: '搜索文章', exact: true }).evaluate(el => el === document.activeElement))
        await page.getByRole('link', { name: '我的收藏', exact: true }).click()
        await page.waitForURL('**/collections')
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
        assert.deepEqual(errors, [])
        results.push(`${width} ${colorScheme}: latest CMS, collections D, original search, five projects, source controls, article figures PASS`)
        await context.close()
      }
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
