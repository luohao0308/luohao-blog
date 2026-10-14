// Actual local BFF + HTTP auth + SQLite/FS portfolio fixture. No mocked routes.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
const origin = new URL(base)
assert.ok(['localhost', '127.0.0.1'].includes(origin.hostname) && origin.protocol === 'http:', 'local fixture only')
const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2nH0AAAAASUVORK5CYII='
async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const admin = await browser.newContext()
    const publicContext = await browser.newContext({ viewport: { width: 390, height: 844 } })
    const page = await publicContext.newPage()
    await page.goto(base + '/health')
    const request = async (context, method, path, token, data, status = 200) => {
      const reply = await context.request.fetch(base + path, {
        method, headers: token ? { authorization: `Bearer ${token}` } : {}, data,
      })
      if (reply.status() !== status && !path.includes('/auth/')) {
        const error = await reply.json().catch(() => ({}))
        console.error(JSON.stringify({ path, code: error.code, reason: error.reason, message: error.message }))
      }
      assert.equal(reply.status(), status, `${method} ${path}: expected ${status}, got ${reply.status()}`)
      return reply
    }
    const login = await request(admin, 'POST', '/api/v1/auth/login', '', { email: 'author@example.com', password: 'longenough1' })
    const token = (await login.json()).access_token
    assert.equal(typeof token, 'string')
    const originals = await (await request(publicContext, 'GET', '/api/v1/projects/list')).json()
    assert.equal(originals.projects.filter(project => !project.slug.startsWith('portfolio-live-')).length, 3, 'known disposable fixture is required')
    assert.ok(originals.projects.some(project => project.slug === 'luohao-blog'))
    const writable = (value) => {
      const { id, created_at, updated_at, ...editable } = value
      return editable
    }
    const slug = `portfolio-live-${Date.now()}`
    let project = await (await request(admin, 'POST', '/api/v1/admin/projects', token, {
      slug, name: '真实作品整合验证', summary: '本地隔离测试项目', description: '验证作品维护与公开边界。',
      stack: ['Go', 'Nuxt'], role: '测试', repo_url: 'https://example.com/repository', live_url: '',
      emoji: '🧪', year: '2026', published: false, sort: 0, featured: true,
    })).json()
    await request(publicContext, 'GET', `/api/v1/projects/${slug}`, '', undefined, 404)
    await page.goto(base + `/projects/${slug}`)
    await page.getByText('项目不存在', { exact: true }).waitFor()
    const uploaded = await (await request(admin, 'POST', '/api/v1/admin/media/images', token, { data: pixel })).json()
    assert.match(uploaded.url, /^\/v1\/assets\/media\/[0-9a-f]{32}\.png$/)
    project = await (await request(admin, 'PUT', `/api/v1/admin/projects/${slug}?update_mask=published,cover_url,screenshots,sort,problem,approach,outcome`, token, {
      ...writable(project), published: true, cover_url: uploaded.url, screenshots: [uploaded.url],
      problem: '测试数据持久化', approach: '真实接口保存', outcome: '接口与页面读取一致', sort: 0,
    })).json()
    assert.equal(project.cover_url, uploaded.url)
    assert.equal(project.published, true)
    const listed = await (await request(publicContext, 'GET', '/api/v1/projects/list')).json()
    const createdIndex = listed.projects.findIndex(item => item.slug === slug)
    const seededIndex = listed.projects.findIndex(item => item.slug === 'luohao-blog')
    assert.ok(createdIndex >= 0 && createdIndex < seededIndex, 'sort=0 project must be listed before the seeded sort=10 project')
    await request(publicContext, 'GET', `/api/v1/projects/${slug}`)
    await page.getByRole('link', { name: 'Go back home', exact: true }).click()
    await page.getByRole('link', { name: '真实作品整合验证', exact: true }).click()
    try { await page.getByRole('heading', { name: '真实作品整合验证', exact: true }).waitFor({ timeout: 10000 }) }
    catch (error) {
      console.error(JSON.stringify({ url: page.url(), headings: await page.locator('h1').allTextContents(), body: (await page.locator('body').innerText()).slice(0, 800) }))
      throw error
    }
    await page.waitForFunction(() => [...document.images].some(image => image.src.includes('/assets/media/') && image.naturalWidth > 0))
    await page.reload()
    await page.getByText('接口与页面读取一致', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true)
    await request(admin, 'DELETE', `/api/v1/admin/projects/${slug}`, token)
    await request(publicContext, 'GET', `/api/v1/projects/${slug}`, '', undefined, 404)
    const hidden = await (await request(publicContext, 'GET', '/api/v1/projects/list')).json()
    assert.ok(!hidden.projects.some(project => project.slug === slug))
    await page.getByRole('link', { name: '返回作品集', exact: true }).click()
    await page.getByRole('heading', { name: '作品集', exact: true }).waitFor()
    assert.equal(await page.getByRole('link', { name: '真实作品整合验证', exact: true }).count(), 0, 'SPA list refresh must hide the archived project')
    const hiddenDetail = page.waitForResponse(response => response.url().includes(`/api/v1/projects/${slug}`) && response.status() === 404)
    await page.goBack()
    await hiddenDetail
    await page.waitForTimeout(100)
    assert.equal(await page.getByRole('heading', { name: '真实作品整合验证', exact: true }).count(), 0, 'SPA history must not restore archived project content')
    const about = await (await request(publicContext, 'GET', '/api/v1/about')).json()
    await page.goto(base + '/about')
    await page.getByText(about.headline, { exact: true }).waitFor()
    await request(admin, 'PUT', '/api/v1/admin/about', token, {
      ...writable(about), headline: '关于页真实整合验证', intro: '仅修改临时fixture数据。', current_focus: '验证可维护内容', profile_image: uploaded.url,
    })
    const savedAbout = await (await request(publicContext, 'GET', '/api/v1/about')).json()
    assert.equal(savedAbout.headline, '关于页真实整合验证')
    assert.equal(savedAbout.profile_image, uploaded.url)
    await page.getByRole('link', { name: '作品集', exact: true }).click()
    await page.getByRole('heading', { name: '作品集', exact: true }).waitFor()
    await page.getByRole('link', { name: '关于', exact: true }).click()
    await page.getByText('关于页真实整合验证', { exact: true }).waitFor()
    await page.reload()
    await page.getByText('验证可维护内容', { exact: true }).waitFor()
    await request(publicContext, 'POST', '/api/v1/admin/projects', '', { slug: 'denied', name: 'denied' }, 401)
    const reader = await browser.newContext()
    const readerToken = (await (await request(reader, 'POST', '/api/v1/auth/login', '', { email: 'reader@example.com', password: 'longenough1' })).json()).access_token
    await request(reader, 'PUT', '/api/v1/admin/about', readerToken, writable(about), 403)
    await request(reader, 'PUT', `/api/v1/admin/projects/${slug}?update_mask=published`, readerToken, { ...writable(project), published: true }, 403)
    await request(publicContext, 'GET', `/api/v1/projects/${slug}`, '', undefined, 404)
    await reader.close()
    await admin.close()
    await publicContext.close()
    console.log('Portfolio/About live PASS: real auth/BFF, CRUD, image FS/readback, sort, soft archive, public isolation, profile persistence, mobile, Reader/anonymous denial')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
