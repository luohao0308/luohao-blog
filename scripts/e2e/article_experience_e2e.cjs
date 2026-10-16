// Browser regression for S2 article media, cover, featured, reading time, and TOC UI.
// Build frontend, run its preview locally, then execute with bundled Playwright.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')

const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('This mock smoke check only supports a local host')

const pixelPng = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2nH0AAAAASUVORK5CYII=', 'base64')

function makeArticle(overrides = {}) {
  return {
    id: 'article-1',
    slug: 'demo',
    title: '目录与媒体演示',
    summary: '这是一段独立摘要，用于验证详情页展示。',
    content_md: '# 开始\n\n短正文\n\n![原有配图](/api/v1/assets/media/initial.jpg)',
    content_html: '<h2>你好 世界</h2><p>正文</p><h2>你好 世界</h2><h3>API / 边界</h3>',
    tags: ['Go'],
    category: { slug: 'engineering', name: '工程' },
    status: 2,
    featured: false,
    cover_url: '',
    published_at: '2026-10-15T00:00:00Z',
    created_at: '2026-10-15T00:00:00Z',
    updated_at: '2026-10-15T00:00:00Z',
    view_count: 3,
    like_count: 1,
    ...overrides,
  }
}

async function installRoutes(page, state) {
  await page.route('**/api/v1/auth/refresh', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      access_token: 'fixture-admin-token',
      token_type: 'Bearer',
      expires_in: 900,
      user: { id: 'admin', email: 'author@example.com', display_name: 'Author', role: 1 },
    }),
  }))
  await page.route('**/api/v1/categories/list**', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ categories: [{ id: 'cat-1', slug: 'engineering', name: '工程', sort: 1 }] }),
  }))
  await page.route('**/api/v1/admin/media/images', async (route) => {
    state.uploadRequests.push(route.request().postDataJSON())
    if (state.uploadMode === 'error')
      return route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ message: 'invalid image' }) })
    const body = state.uploadMode === 'empty' ? {} : { url: '/v1/assets/media/0123456789abcdef0123456789abcdef.jpg' }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
  })
  await page.route('**/api/v1/articles/update**', async (route) => {
      state.updateCalls++
    state.updateRequest = {
      url: route.request().url(),
      body: route.request().postDataJSON(),
    }
    Object.assign(state.article, state.updateRequest.body)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(state.article) })
  })
  await page.route('**/api/v1/articles/demo/view', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))
  await page.route('**/api/v1/articles/demo/like', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))
  await page.route('**/api/v1/articles/demo', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(state.article),
  }))
  await page.route('**/api/v1/articles/list**', (route) => {
    const url = new URL(route.request().url())
    const featured = url.searchParams.get('filter') === 'featured'
    const source = featured ? state.homeArticles.filter(article => article.featured) : state.homeArticles
    state.listRequests.push({ filter: url.searchParams.get('filter'), orderBy: url.searchParams.get('order_by') })
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ articles: source }) })
  })
  await page.route('**/api/v1/assets/media/*', route => route.fulfill({ status: 200, contentType: 'image/png', body: pixelPng }))
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext({ locale: 'zh-CN', viewport: { width: 1280, height: 900 } })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => {
      if (message.type() === 'error' && /RangeError|attribute caption|attribute title/.test(message.text())) errors.push(message.text())
    })
    const state = {
      article: makeArticle({ status: 1 }),
      homeArticles: [
        makeArticle({ id: 'draft-featured', slug: 'draft-featured', title: '草稿不可见', featured: true, status: 1 }),
        makeArticle({ id: 'featured', slug: 'featured', title: '真正精选', featured: true, status: 2, cover_url: '/v1/assets/media/0123456789abcdef0123456789abcdef.jpg' }),
        makeArticle({ id: 'latest', slug: 'latest', title: '普通最新', featured: false, status: 2 }),
      ],
      uploadMode: 'success',
      uploadRequests: [],
      listRequests: [],
      updateRequest: null,
      updateCalls: 0,
    }
    await installRoutes(page, state)

    await page.goto(`${base}/admin/posts/demo/edit`, { waitUntil: 'domcontentloaded' })
    await page.getByRole('heading', { name: '编辑文章' }).waitFor()
    const initialImage = page.locator('.milkdown img[src$="/initial.jpg"]')
    await initialImage.waitFor()
    const imageInput = page.locator('[data-testid="article-image-input"]')

    await imageInput.setInputFiles({ name: 'invalid.txt', mimeType: 'text/plain', buffer: Buffer.from('not an image') })
    await page.getByTestId('media-notice').filter({ hasText: '仅支持' }).waitFor()
    assert.equal(state.uploadRequests.length, 0, 'invalid type must not call the API')

    await imageInput.setInputFiles({ name: 'broken.png', mimeType: 'image/png', buffer: Buffer.from('not an image') })
    await page.getByTestId('media-notice').filter({ hasText: '上传失败' }).waitFor()
    assert.equal(state.uploadRequests.length, 0, 'invalid image bytes must not call the API')

    await imageInput.setInputFiles({ name: 'too-large.png', mimeType: 'image/png', buffer: Buffer.alloc(2 * 1024 * 1024 + 1) })
    await page.getByTestId('media-notice').filter({ hasText: '超过 2MB' }).waitFor()
    assert.equal(state.uploadRequests.length, 0, 'oversized source must not call the API')

    state.uploadMode = 'error'
    await imageInput.setInputFiles({ name: 'server-error.png', mimeType: 'image/png', buffer: pixelPng })
    await page.getByTestId('media-notice').filter({ hasText: '图片不符合要求' }).waitFor()

    state.uploadMode = 'empty'
    await imageInput.setInputFiles({ name: 'empty.png', mimeType: 'image/png', buffer: pixelPng })
    await page.getByTestId('media-notice').filter({ hasText: '上传结果无效' }).waitFor()

    state.uploadMode = 'success'
    await imageInput.setInputFiles({ name: 'success.png', mimeType: 'image/png', buffer: pixelPng })
    await page.getByTestId('media-notice').filter({ hasText: '已插入正文' }).waitFor()
    assert.ok(state.uploadRequests.at(-1).data.length > 0, 'encoded image payload must be non-empty')
    const encoded = Buffer.from(state.uploadRequests.at(-1).data, 'base64')
    assert.deepEqual([...encoded.subarray(0, 3)], [0xFF, 0xD8, 0xFF], 'browser upload must normalize to JPEG for WeChat compatibility')
    assert.ok(encoded.length <= 1024 * 1024, 'browser upload must stay within the 1 MiB target')
    const insertedImage = page.locator('.milkdown img[src$="/0123456789abcdef0123456789abcdef.jpg"]')
    await insertedImage.waitFor()
    await initialImage.waitFor()
    await page.locator('.milkdown .ProseMirror').focus()
    await page.keyboard.press('ControlOrMeta+End')
    await page.keyboard.insertText('插图后继续编辑')
    await insertedImage.waitFor()
    await page.keyboard.press('ControlOrMeta+z')
    await page.keyboard.press('ControlOrMeta+Shift+z')
    await insertedImage.waitFor()
    await initialImage.waitFor()
    await page.getByRole('button', { name: '设为封面' }).click()
    await page.getByRole('checkbox', { name: '首页精选' }).check()
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.locator('.n-message').filter({ hasText: '已保存' }).waitFor()
    assert.match(new URL(state.updateRequest.url).searchParams.get('update_mask'), /cover_url/)
    assert.match(new URL(state.updateRequest.url).searchParams.get('update_mask'), /featured/)
    assert.equal(state.updateRequest.body.cover_url, '/v1/assets/media/0123456789abcdef0123456789abcdef.jpg')
    assert.equal(state.updateRequest.body.featured, true)
    assert.match(state.updateRequest.body.content_md, /!\[[^\]]*\]\(\/api\/v1\/assets\/media\/0123456789abcdef0123456789abcdef\.jpg\)/)

    assert.match(state.updateRequest.body.content_md, /插图后继续编辑/)
    assert.match(state.updateRequest.body.content_md, /initial\.jpg/)
    // Crepe serializes block-image alt as its aspect ratio (e.g. 1.00).
    // Verify actual nodes, URLs, editing history and persisted content instead.
    await page.reload({ waitUntil: 'domcontentloaded' })
    await insertedImage.waitFor()
    await initialImage.waitFor()
    await page.getByAltText('文章封面预览').waitFor()
    assert.equal(await page.getByRole('checkbox', { name: '首页精选' }).isChecked(), true)
    const updateCallsBeforeEnter = state.updateCalls
    await page.getByPlaceholder('文章标题').press('Enter')
    await page.locator('.n-message').filter({ hasText: '已保存' }).waitFor()
    assert.equal(state.updateCalls, updateCallsBeforeEnter + 1, 'Enter must submit one update without a native reload')
    assert.match(page.url(), /\/admin\/posts\/demo\/edit$/)

    state.article.status = 2
    state.article.content_md = '短正文'
    await page.getByRole('link', { name: '查看前台' }).click()
    await page.getByText('约 1 分钟阅读', { exact: true }).waitFor()
    await page.getByText(state.article.summary, { exact: true }).waitFor()
    await page.getByRole('navigation', { name: '文章目录' }).waitFor()
    const tocLinks = page.getByRole('navigation', { name: '文章目录' }).getByRole('link')
    assert.deepEqual(await tocLinks.allTextContents(), ['你好 世界', '你好 世界', 'API / 边界'])
    assert.deepEqual(await tocLinks.evaluateAll(links => links.map(link => link.getAttribute('href'))), ['#你好-世界', '#你好-世界-2', '#api-边界'])
    assert.equal(await page.locator('#你好-世界').count(), 1)
    assert.equal(await page.locator('#你好-世界-2').count(), 1)
    assert.equal(await page.locator('#api-边界').count(), 1)
    await page.getByAltText('目录与媒体演示封面').waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('dark')), false, 'default light theme must stay light')

    await page.locator('header a[href="/"]').first().click()
    await page.getByRole('region', { name: '首页文章推荐' }).getByRole('heading', { name: '精选文章' }).waitFor()
    await page.getByRole('region', { name: '首页文章推荐' }).getByText('真正精选', { exact: true }).waitFor()
    await page.getByAltText('真正精选封面').waitFor()
    assert.equal(await page.getByRole('region', { name: '首页文章推荐' }).getByText('普通最新', { exact: true }).count(), 0)
    assert.equal(await page.getByRole('region', { name: '首页文章推荐' }).getByText('草稿不可见', { exact: true }).count(), 0)
    assert.ok(state.listRequests.some(request => request.filter === 'featured' && request.orderBy === 'published_at desc,slug'))

    await context.close()

    const fallbackContext = await browser.newContext({ locale: 'zh-CN', viewport: { width: 390, height: 844 }, colorScheme: 'dark' })
    const fallbackPage = await fallbackContext.newPage()
    fallbackPage.on('pageerror', error => errors.push(error.message))
    const fallbackState = {
      article: makeArticle({ content_html: '<p>没有标题的正文</p>', content_md: '', cover_url: '' }),
      homeArticles: [makeArticle({ id: 'latest', slug: 'latest', title: '普通最新', featured: false, status: 2 })],
      uploadMode: 'success', uploadRequests: [], listRequests: [], updateRequest: null, updateCalls: 0,
    }
    await installRoutes(fallbackPage, fallbackState)
    await fallbackPage.goto(`${base}/admin/posts/demo/edit`, { waitUntil: 'domcontentloaded' })
    await fallbackPage.getByRole('heading', { name: '编辑文章' }).waitFor()
    await fallbackPage.locator('.milkdown [contenteditable=true]').waitFor()
    assert.equal(await fallbackPage.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true, '390px admin editor must not overflow')
    await fallbackPage.locator('header a[href="/"]').first().click()
    await fallbackPage.getByRole('region', { name: '首页文章推荐' }).getByRole('heading', { name: '最新文章' }).waitFor()
    await fallbackPage.getByText('普通最新', { exact: true }).waitFor()
    assert.equal(await fallbackPage.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true, '390px homepage must not overflow')
    await fallbackPage.goto(`${base}/admin/posts/demo/edit`, { waitUntil: 'domcontentloaded' })
    await fallbackPage.locator('.milkdown [contenteditable=true]').waitFor()
    await fallbackPage.getByRole('link', { name: '查看前台' }).click()
    assert.equal(await fallbackPage.getByRole('navigation', { name: '文章目录' }).count(), 0, 'heading-free article must omit the TOC')
    assert.equal(await fallbackPage.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true, '390px detail must not overflow')
    assert.equal(await fallbackPage.evaluate(() => document.documentElement.classList.contains('dark')), true, 'dark preference must be applied')
    await fallbackContext.close()

    assert.deepEqual(errors, [], 'browser runtime errors')
    console.log('Article experience PASS: upload validation, insertion, persisted cover/featured, featured fallback, TOC uniqueness, reading time, dark mobile layout')
  }
  finally {
    await browser.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
