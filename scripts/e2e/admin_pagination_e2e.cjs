// Browser regression for the admin article cursor pagination UI.
// Build frontend first, then run its preview server on BLOG_E2E_BASE_URL.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')

const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw new Error('This mock smoke check only supports a local host')

let articles = Array.from({ length: 120 }, (_, index) => {
  const number = 120 - index
  return {
    id: String(number),
    slug: `post-${String(number).padStart(3, '0')}`,
    title: `文章 ${String(number).padStart(3, '0')}`,
    summary: '',
    content_md: `# 文章 ${number}`,
    content_html: `<h1>文章 ${number}</h1>`,
    tags: [],
    status: number % 2 === 0 ? 2 : 1,
    created_at: '2026-10-15T00:00:00Z',
    updated_at: '2026-10-15T00:00:00Z',
  }
})

function offsetFromToken(token) {
  if (!token) return 0
  const match = /^offset-(\d+)$/.exec(token)
  if (!match) throw new Error(`unexpected page token: ${token}`)
  return Number(match[1])
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const context = await browser.newContext({ locale: 'zh-CN' })
    const page = await context.newPage()
    const requests = []
    let failNextPage = 0
    let delayAllNextPage = false
    let releaseDelayed
    let delayedStarted
    const delayedStartedPromise = new Promise(resolve => { delayedStarted = resolve })

    await page.route('**/api/v1/auth/refresh', route => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        access_token: 'fixture-admin-token',
        token_type: 'Bearer',
        expires_in: 900,
        user: { id: 'admin', email: 'author@example.com', display_name: 'Author', role: 2 },
      }),
    }))
    await page.route('**/api/v1/articles/list**', async (route) => {
      const url = new URL(route.request().url())
      const token = url.searchParams.get('page_token') || ''
      const filter = url.searchParams.get('filter') || ''
      requests.push({ token, filter, orderBy: url.searchParams.get('order_by') })
      if (failNextPage > 0 && token) {
        failNextPage--
        return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'fixture unavailable' }) })
      }
      if (delayAllNextPage && token && !filter) {
        delayAllNextPage = false
        delayedStarted()
        await new Promise(resolve => { releaseDelayed = resolve })
      }
      const source = filter.includes('PUBLISHED')
        ? articles.filter(article => article.status === 2)
        : filter.includes('DRAFT')
          ? articles.filter(article => article.status === 1)
          : articles
      const offset = offsetFromToken(token)
      const size = Number(url.searchParams.get('page_size') || 50)
      const slice = source.slice(offset, offset + size)
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          articles: slice,
          next_page_token: offset + slice.length < source.length ? `offset-${offset + slice.length}` : '',
        }),
      })
    })
    await page.route('**/api/v1/articles/update**', async (route) => {
      const body = route.request().postDataJSON()
      const article = articles.find(item => item.slug === body.slug)
      assert.ok(article, `updated article ${body.slug} must exist`)
      article.status = body.status
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(article) })
    })
    await page.route('**/api/v1/articles/post-*', async (route) => {
      const slug = decodeURIComponent(new URL(route.request().url()).pathname.split('/').pop())
      articles = articles.filter(article => article.slug !== slug)
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    })

    const rowSlugs = async () => page.locator('.n-data-table tbody tr td:nth-child(2)').allTextContents()
    await page.goto(`${base}/admin/posts`, { waitUntil: 'domcontentloaded' })
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal((await rowSlugs()).length, 50)
    assert.equal(requests.at(-1).orderBy, 'updated_at desc,slug')

    const seen = new Set(await rowSlugs())
    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByText('第 2 页 · 本页 50 条', { exact: true }).waitFor()
    for (const slug of await rowSlugs()) assert.equal(seen.has(slug), false, `duplicate slug ${slug}`), seen.add(slug)
    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByText('第 3 页 · 本页 20 条', { exact: true }).waitFor()
    for (const slug of await rowSlugs()) assert.equal(seen.has(slug), false, `duplicate slug ${slug}`), seen.add(slug)
    assert.equal(seen.size, 120, 'all fixture articles must be reachable')
    assert.equal(await page.getByRole('button', { name: '下一页' }).isDisabled(), true, 'last page must disable next navigation')
    await page.getByRole('button', { name: '上一页' }).click()
    await page.getByText('第 2 页 · 本页 50 条', { exact: true }).waitFor()

    failNextPage = 2
    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByRole('alert').filter({ hasText: '文章列表暂时无法加载' }).waitFor()
    await page.getByText('第 2 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal((await rowSlugs()).length, 50, 'failed navigation must preserve the current page')
    await page.getByRole('button', { name: '重新加载' }).click()
    await page.getByText('第 3 页 · 本页 20 条', { exact: true }).waitFor()

    await page.locator('.n-base-selection').click()
    await page.getByText('已发布', { exact: true }).last().click()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal(requests.at(-1).token, '', 'filter changes must reset the cursor')
    assert.ok((await rowSlugs()).every(slug => Number(slug.slice(-3)) % 2 === 0))

    await page.locator('.n-base-selection').click()
    await page.getByText('全部状态', { exact: true }).last().click()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    delayAllNextPage = true
    await page.getByRole('button', { name: '下一页' }).click()
    await delayedStartedPromise
    await page.locator('.n-base-selection').click()
    await page.getByText('草稿', { exact: true }).last().click()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    releaseDelayed()
    await page.waitForTimeout(100)
    assert.ok((await rowSlugs()).every(slug => Number(slug.slice(-3)) % 2 === 1), 'stale response must not overwrite the new filter')

    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByText('第 2 页 · 本页 10 条', { exact: true }).waitFor()
    const publishedSlug = (await rowSlugs())[0]
    await page.locator('.n-data-table tbody tr').first().getByRole('button', { name: '发布' }).click()
    await page.locator('.n-message').filter({ hasText: '已发布' }).waitFor()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal(requests.at(-1).token, '', 'mutations must refresh from a safe first-page cursor')
    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByText('第 2 页 · 本页 9 条', { exact: true }).waitFor()
    assert.equal((await rowSlugs()).includes(publishedSlug), false, 'published article must leave the draft filter')
    const refillSlug = (await rowSlugs())[0]

    await page.getByRole('button', { name: '上一页' }).click()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    const deletedSlug = (await rowSlugs())[0]
    await page.locator('.n-data-table tbody tr').first().getByRole('button', { name: '删除' }).click()
    await page.locator('.n-popconfirm__action button').last().click()
    await page.locator('.n-message').filter({ hasText: '已删除' }).waitFor()
    await page.getByText('第 1 页 · 本页 50 条', { exact: true }).waitFor()
    assert.equal(requests.at(-1).token, '', 'deletion must refresh from a safe first-page cursor')
    const firstPageAfterDelete = await rowSlugs()
    assert.equal(firstPageAfterDelete.includes(deletedSlug), false, 'deleted article must disappear')
    assert.equal(firstPageAfterDelete.includes(refillSlug), true, 'deleting from the first page must pull the next boundary article into view')
    await page.getByRole('button', { name: '下一页' }).click()
    await page.getByText('第 2 页 · 本页 8 条', { exact: true }).waitFor()

    await context.close()
    console.log('Admin article pagination PASS: 120 rows, stable cursors, last page, retry, filter reset, stale-response guard, safe mutation refresh')
  }
  finally {
    await browser.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
