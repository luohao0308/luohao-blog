const assert = require('node:assert/strict')
const { chromium } = require('playwright')

const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('This mock smoke only supports a local frontend')
const pixelPng = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2nH0AAAAASUVORK5CYII=', 'base64')

function project(overrides = {}) {
  return {
    id: 'p1', slug: 'luohao-blog', name: 'luohao-blog', summary: '本站全栈个人站', description: '已核实的项目说明',
    stack: ['Go', 'Nuxt 4'], role: '独立设计与开发', repo_url: 'https://github.com/luohao0308/luohao-blog', live_url: '',
    emoji: '🛠️', year: '2026', featured: true, cover_url: '', screenshots: [], problem: '内容与工程实践分散',
    approach: '用同一仓库建设内容和交付流程', outcome: '站点与流程持续迭代', published: true, sort: 10,
    updated_at: '2026-10-15T00:00:00Z', ...overrides,
  }
}

async function installRoutes(page, state) {
  await page.route('**/api/v1/auth/refresh', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ access_token: 'admin-token', token_type: 'Bearer', expires_in: 900, user: { id: 'admin', email: 'author@example.com', display_name: 'Author', role: 2 } }) }))
  await page.route('**/api/v1/admin/media/images', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ url: '/v1/assets/media/abcdefabcdefabcdefabcdefabcdefab.jpg' }) }))
  await page.route('**/api/v1/assets/media/*', route => route.fulfill({ status: 200, contentType: 'image/png', body: pixelPng }))
  await page.route('**/api/v1/projects/list**', (route) => {
    state.projectListCalls++
    if (state.failProjects > 0) {
      state.failProjects--
      return route.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
    }
    const admin = route.request().headers().authorization === 'Bearer admin-token'
    const published = new URL(route.request().url()).searchParams.get('published')
    let projects = admin ? state.projects : state.projects.filter(item => item.published)
    if (published === 'true') projects = projects.filter(item => item.published)
    if (published === 'false') projects = projects.filter(item => !item.published)
    projects = [...projects].sort((a, b) => a.sort - b.sort || a.slug.localeCompare(b.slug))
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ projects }) })
  })
  await page.route('**/api/v1/admin/projects', async (route) => {
    const created = { ...route.request().postDataJSON(), id: `p${state.projects.length + 1}`, updated_at: new Date().toISOString() }
    state.projects.push(created)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(created) })
  })
  await page.route('**/api/v1/admin/projects/*', async (route) => {
    const pathname = new URL(route.request().url()).pathname
    if (pathname.endsWith('/api/v1/admin/projects')) return route.fallback()
    const slug = decodeURIComponent(pathname.split('/').pop())
    const existing = state.projects.find(item => item.slug === slug)
    if (!existing) return route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
    if (route.request().method() === 'DELETE') existing.published = false
    else Object.assign(existing, route.request().postDataJSON(), { slug, updated_at: new Date().toISOString() })
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(existing) })
  })
  await page.route('**/api/v1/projects/*', (route) => {
    const slug = decodeURIComponent(new URL(route.request().url()).pathname.split('/').pop())
    if (slug === 'list') return route.fallback()
    const admin = route.request().headers().authorization === 'Bearer admin-token'
    const found = state.projects.find(item => item.slug === slug && (admin || item.published))
    return route.fulfill({ status: found ? 200 : 404, contentType: 'application/json', body: JSON.stringify(found || {}) })
  })
  await page.route('**/api/v1/admin/about', async (route) => {
    state.about = { ...route.request().postDataJSON(), updated_at: new Date().toISOString() }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(state.about) })
  })
  await page.route('**/api/v1/about', route => route.fulfill({ status: state.failAbout ? 503 : 200, contentType: 'application/json', body: JSON.stringify(state.failAbout ? {} : state.about) }))
  await page.route('**/api/v1/articles/list**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ articles: [{ id: 'a1', slug: 'article', title: '代表文章', summary: '', content_md: '正文', content_html: '<p>正文</p>', tags: [], status: 2, created_at: '2026-10-15T00:00:00Z', updated_at: '2026-10-15T00:00:00Z' }] }) }))
}

const field = (page, label) => page.locator('.n-form-item').filter({ hasText: label }).locator('input, textarea').first()

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const state = {
      projects: [project(), project({ id: 'p2', slug: 'draft-tool', name: '内部草稿', published: false, featured: false, sort: 20 }), project({ id: 'p3', slug: 'dev-workflow', name: 'dev-workflow', problem: '流程协作问题', featured: true, sort: 30 })],
      about: { id: 'about', name: 'luohao', headline: '全栈工程师', intro: '保留的个人简介', profile_image: '', current_focus: 'Go 与 AI Agent 工程实践', skills: [{ group: '后端', items: ['Go'] }], timeline: [{ year: '2026', text: '搭建 luohao-blog。' }], contacts: [{ label: 'GitHub', href: 'https://github.com/luohao0308', text: 'github.com/luohao0308' }], representative_links: [] },
      failProjects: 0, failAbout: false, projectListCalls: 0,
    }
    const context = await browser.newContext({ locale: 'zh-CN', viewport: { width: 1280, height: 900 } })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await installRoutes(page, state)

    await page.goto(`${base}/admin/projects`, { waitUntil: 'domcontentloaded' })
    await page.getByRole('heading', { name: '项目管理' }).waitFor()
    await page.locator('.n-data-table tbody tr').first().waitFor()
    assert.equal(await page.locator('.n-data-table tbody tr').count(), 3, 'admin must see drafts')
    await page.locator('.n-data-table tbody tr').filter({ hasText: '内部草稿' }).getByRole('button', { name: '上架' }).click()
    await page.locator('.n-message').filter({ hasText: '已上架' }).waitFor()
    assert.equal(state.projects.find(item => item.slug === 'draft-tool').published, true)
    assert.equal(state.projects.find(item => item.slug === 'luohao-blog').sort, 10, 'status update must not alter another project')

    await page.locator('.n-data-table tbody tr').filter({ hasText: 'luohao-blog' }).getByRole('button', { name: '编辑' }).click()
    await page.getByRole('heading', { name: '编辑项目' }).waitFor()
    await page.locator('[data-testid="project-image-input"]').setInputFiles({ name: 'project.png', mimeType: 'image/png', buffer: pixelPng })
    await page.getByTestId('project-media-notice').filter({ hasText: '截图已上传' }).waitFor()
    await page.getByRole('button', { name: '设为封面' }).click()
    await field(page, '排序').fill('5')
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.locator('.n-message').filter({ hasText: '项目已保存' }).waitFor()
    assert.equal(state.projects[0].sort, 5)
    assert.equal(state.projects[0].cover_url, '/v1/assets/media/abcdefabcdefabcdefabcdefabcdefab.jpg')
    assert.deepEqual(state.projects[0].screenshots, ['/v1/assets/media/abcdefabcdefabcdefabcdefabcdefab.jpg'])
    await page.reload()
    await page.getByAltText('项目封面预览').waitFor()

    await page.goto(`${base}/admin/projects/new`, { waitUntil: 'domcontentloaded' })
    await page.getByRole('heading', { name: '新增项目' }).waitFor()
    await page.waitForTimeout(100)
    await page.locator('.n-form-item input').first().fill('新项目')
    await field(page, 'Slug').fill('new-project')
    await field(page, '摘要').fill('真实的新项目摘要')
    await field(page, '角色').fill('独立开发')
    await page.getByRole('button', { name: '创建', exact: true }).click()
    await page.waitForURL('**/admin/projects/new-project/edit')
    assert.ok(state.projects.some(item => item.slug === 'new-project'))

    await page.goto(`${base}/admin/about`, { waitUntil: 'domcontentloaded' })
    await page.getByRole('heading', { name: '关于页管理' }).waitFor()
    await field(page, '当前方向').fill('当前专注真实系统交付')
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.locator('.n-message').filter({ hasText: '关于页已保存' }).waitFor()
    assert.equal(state.about.current_focus, '当前专注真实系统交付')
    await page.reload()
    await field(page, '当前方向').waitFor()
    assert.equal(await field(page, '当前方向').inputValue(), '当前专注真实系统交付')

    await page.getByRole('link', { name: 'luohao.blog', exact: true }).click()
    await page.getByRole('link', { name: '作品集', exact: true }).click()
    await page.getByRole('heading', { name: '作品集' }).waitFor()
    assert.equal(await page.getByText('内部草稿', { exact: true }).count(), 1, 'newly published project must be public')
    await page.getByRole('link', { name: 'luohao-blog', exact: true }).click()
    await page.getByText('内容与工程实践分散', { exact: true }).waitFor()
    await page.getByAltText('luohao-blog 项目封面').waitFor()
    await page.getByRole('heading', { name: '相关项目' }).locator('..').getByRole('link', { name: 'dev-workflow', exact: true }).click()
    await page.locator('article > header h1').filter({ hasText: 'dev-workflow' }).waitFor()
    assert.equal(await page.locator('article > header h1').textContent(), 'dev-workflow', 'dynamic detail navigation must replace the previous project')
    await page.getByText('流程协作问题', { exact: true }).waitFor()
    state.projects.find(item => item.slug === 'dev-workflow').published = false
    await page.getByRole('link', { name: '返回作品集', exact: true }).click()
    await page.getByRole('heading', { name: '作品集', exact: true }).waitFor()
    assert.equal(await page.getByRole('link', { name: 'dev-workflow', exact: true }).count(), 0)
    await page.goBack()
    await page.waitForURL('**/projects/dev-workflow')
    await page.getByText('流程协作问题', { exact: true }).waitFor({ state: 'hidden' })
    assert.equal(await page.getByText('流程协作问题', { exact: true }).count(), 0, 'unpublished route must not retain the previous project content')

    state.about.contacts.push({ label: '危险', href: 'javascript:alert(1)', text: '不可见' })
    await page.getByRole('link', { name: '关于', exact: true }).click()
    await page.getByText('当前专注真实系统交付', { exact: true }).waitFor()
    await page.getByText('代表文章', { exact: true }).waitFor()
    assert.equal(await page.getByText('不可见', { exact: true }).count(), 0, 'unsafe contact must not render')

    await page.setViewportSize({ width: 390, height: 844 })
    await page.evaluate(() => localStorage.setItem('theme', 'dark'))
    await page.reload()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true)
    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('dark')), true)
    assert.deepEqual(errors, [])
    await context.close()

    const retryContext = await browser.newContext()
    const retryPage = await retryContext.newPage()
    state.failProjects = 2
    const callsBeforeRetryScenario = state.projectListCalls
    await installRoutes(retryPage, state)
    await retryPage.goto(`${base}/admin/projects`, { waitUntil: 'domcontentloaded' })
    await retryPage.getByRole('alert').filter({ hasText: '项目列表暂时无法加载' }).waitFor()
    assert.equal(state.projectListCalls - callsBeforeRetryScenario, 2, 'ofetch must exhaust its one automatic GET retry before showing the error')
    await retryPage.getByRole('button', { name: '重新加载' }).click()
    await retryPage.locator('.n-data-table tbody tr').first().waitFor()
    assert.equal(state.projectListCalls - callsBeforeRetryScenario, 3, 'manual retry must issue one fresh successful request')
    await retryContext.close()

    console.log('Portfolio/About mock PASS: admin CRUD, media persistence, sorting, publish visibility, public detail/about, unsafe-link filtering, retry, dark mobile')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
