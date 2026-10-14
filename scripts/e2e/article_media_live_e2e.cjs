// Real local BFF + TestOfficialAccountHTTPLifecycle media/article smoke.
// Never point this script at production: both the frontend and its fixture
// backend must resolve to localhost/127.0.0.1.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')

const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
const baseURL = new URL(base)
if (!['127.0.0.1', 'localhost'].includes(baseURL.hostname))
  throw new Error('This live fixture smoke only supports a local frontend')

const pixelPng = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2nH0AAAAASUVORK5CYII=', 'base64')
const articleSlug = process.env.BLOG_E2E_ARTICLE_SLUG || 'wechat-integration'

async function login(context, email) {
  const response = await context.request.post(`${base}/api/v1/auth/login`, {
    data: { email, password: 'longenough1' },
  })
  assert.equal(response.status(), 200, `${email} fixture login must succeed`)
  return (await response.json()).access_token
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const adminContext = await browser.newContext({ locale: 'zh-CN', viewport: { width: 1280, height: 900 } })
    const adminPage = await adminContext.newPage()
    const errors = []
    adminPage.on('pageerror', error => errors.push(error.message))
    const adminToken = await login(adminContext, 'author@example.com')
    const authHeaders = { Authorization: `Bearer ${adminToken}` }
    const seedResponse = await adminContext.request.get(`${base}/api/v1/articles/${articleSlug}`, { headers: authHeaders })
    assert.equal(seedResponse.status(), 200, `fixture article ${articleSlug} must exist`)
    const seed = await seedResponse.json()
    if (!/^##\s/m.test(seed.content_md || '')) {
      const seededContent = `${String(seed.content_md || '').trimEnd()}\n\n## 集成目录\n\n真实 BFF 上传验证。`
      const seedUpdate = await adminContext.request.put(`${base}/api/v1/articles/update?update_mask=content_md`, {
        headers: authHeaders,
        data: {
          slug: seed.slug,
          title: seed.title,
          summary: seed.summary,
          content_md: seededContent,
          tags: seed.tags,
          status: seed.status,
        },
      })
      assert.equal(seedUpdate.status(), 200, 'fixture heading seed must succeed')
    }

    await adminPage.goto(`${base}/admin/posts/${articleSlug}/edit`, { waitUntil: 'domcontentloaded' })
    await adminPage.getByRole('heading', { name: '编辑文章' }).waitFor()
    const imageInput = adminPage.locator('[data-testid="article-image-input"]')
    await imageInput.setInputFiles({ name: 'fixture.png', mimeType: 'image/png', buffer: pixelPng })
    await adminPage.getByTestId('media-notice').filter({ hasText: '已插入正文' }).waitFor()
    await adminPage.getByRole('button', { name: '设为封面' }).click()
    await adminPage.getByRole('checkbox', { name: '首页精选' }).check()

    const coverPreview = adminPage.getByAltText('文章封面预览')
    await coverPreview.waitFor()
    const coverSrc = await coverPreview.getAttribute('src')
    assert.match(coverSrc, /^\/api\/v1\/assets\/media\/[0-9a-f]{32}\.jpg$/)

    await adminPage.getByRole('button', { name: '保存', exact: true }).click()
    await adminPage.locator('.n-message').filter({ hasText: '已保存' }).waitFor()

    const storedResponse = await adminContext.request.get(`${base}/api/v1/articles/${articleSlug}`)
    assert.equal(storedResponse.status(), 200)
    const stored = await storedResponse.json()
    assert.equal(stored.cover_url, coverSrc.slice('/api'.length))
    assert.equal(stored.featured, true)
    assert.match(stored.content_md, new RegExp(`!\\[文章图片\\]\\(${coverSrc.replaceAll('/', '\\/')}\\)`))

    await adminPage.reload({ waitUntil: 'domcontentloaded' })
    await adminPage.getByAltText('文章封面预览').waitFor()
    assert.equal(await adminPage.getByRole('checkbox', { name: '首页精选' }).isChecked(), true)

    const mediaResponse = await adminContext.request.get(`${base}${coverSrc}`)
    assert.equal(mediaResponse.status(), 200)
    assert.match(mediaResponse.headers()['content-type'] || '', /^image\/jpeg(?:;|$)/)
    const mediaBytes = Buffer.from(await mediaResponse.body())
    assert.ok(mediaBytes.length > 3)
    assert.deepEqual([...mediaBytes.subarray(0, 3)], [0xFF, 0xD8, 0xFF])

    const anonymousContext = await browser.newContext({ locale: 'zh-CN', viewport: { width: 390, height: 844 } })
    const anonymousPage = await anonymousContext.newPage()
    anonymousPage.on('pageerror', error => errors.push(error.message))
    await anonymousPage.goto(`${base}/posts/${articleSlug}`, { waitUntil: 'domcontentloaded' })
    const publicCover = anonymousPage.getByAltText(`${stored.title}封面`, { exact: true })
    await publicCover.waitFor()
    assert.ok(await publicCover.evaluate(image => image.naturalWidth > 0 && image.naturalHeight > 0), 'public cover must decode')
    await anonymousPage.getByRole('navigation', { name: '文章目录' }).waitFor()
    await anonymousPage.getByText(/约 \d+ 分钟阅读/).waitFor()
    assert.equal(await anonymousPage.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true)

    const anonymousUpload = await anonymousContext.request.post(`${base}/api/v1/admin/media/images`, {
      data: { data: pixelPng.toString('base64') },
    })
    assert.equal(anonymousUpload.status(), 401)
    await anonymousContext.close()

    const readerContext = await browser.newContext()
    const readerToken = await login(readerContext, 'reader@example.com')
    const readerUpload = await readerContext.request.post(`${base}/api/v1/admin/media/images`, {
      data: { data: pixelPng.toString('base64') },
      headers: { Authorization: `Bearer ${readerToken}` },
    })
    assert.equal(readerUpload.status(), 403)
    await readerContext.close()

    assert.deepEqual(errors, [])
    await adminContext.close()
    console.log('Article media live PASS: real auth/BFF, JPEG upload, FS readback, metadata persistence, public render, anonymous/reader denial')
  }
  finally {
    await browser.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
