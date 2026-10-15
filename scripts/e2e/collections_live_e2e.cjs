// Real loopback BFF/SQL collections integration; no intercepted API or production config.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13019'
const url = new URL(base)
assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) && !url.username && !url.password, 'loopback only')
const secrets = new Set()
const nonce = Date.now()
const password = 'collectionsFixture12'
secrets.add(password)
const emailA = `collections-a-${nonce}@example.com`
const emailB = `collections-b-${nonce}@example.com`

function safeError(error) {
  let output = String(error?.stack || error)
  for (const secret of secrets) if (secret) output = output.split(secret).join('[redacted]')
  return output
}
async function request(context, method, path, token = '', body, status = 200) {
  const response = await context.request.fetch(base + path, {
    method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { data: body }),
  })
  assert.equal(response.status(), status, `${method} ${path.split('?')[0]} status`)
  return response
}
async function register(context, email, name) {
  const reply = await (await request(context, 'POST', '/api/v1/auth/register', '', { email, password, display_name: name })).json()
  secrets.add(reply.access_token)
  assert.equal(Boolean(reply.user.email_verified), false, 'unverified legacy-compatible accounts can use collections')
  return reply
}
async function login(context, email, suppliedPassword = password) {
  const reply = await (await request(context, 'POST', '/api/v1/auth/login', '', { email, password: suppliedPassword })).json()
  secrets.add(reply.access_token)
  return reply
}
async function list(context, token, size = 50) {
  const entries = []
  const lengths = []
  const seen = new Set()
  let pageToken = ''
  do {
    const response = await request(context, 'GET', `/api/v1/collections/list?page_size=${size}&page_token=${encodeURIComponent(pageToken)}`, token)
    assert.ok((response.headers()['cache-control'] || '').includes('no-store'), 'private collection response must not be cached')
    const set = await response.json()
    lengths.push(set.collections.length)
    entries.push(...set.collections)
    pageToken = set.next_page_token || ''
    assert.ok(!pageToken || !seen.has(pageToken), 'cursor must progress')
    if (pageToken) seen.add(pageToken)
  } while (pageToken)
  return { entries, lengths }
}
async function guestEntries(page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('blog:collections') || '[]'))
}
async function showAll(page) {
  for (let count = 0; count < 20; count++) {
    const more = page.getByRole('button', { name: '加载更多', exact: true })
    if (!await more.count()) return
    const previous = await page.locator('main a[href^="/posts/collection-fixture-"]').count()
    await more.click()
    await page.waitForFunction(previous =>
      document.querySelectorAll('main a[href^="/posts/collection-fixture-"]').length > previous
      || ![...document.querySelectorAll('main button')].some(button => button.textContent.trim() === '加载更多'), previous)
  }
  throw new Error('visible collection paging did not finish')
}

async function main() {
  const browser = await chromium.launch({ headless: true, ...(process.env.BLOG_E2E_CHROMIUM ? { executablePath: process.env.BLOG_E2E_CHROMIUM } : {}) })
  try {
    const contextA = await browser.newContext()
    const contextA2 = await browser.newContext({ viewport: { width: 390, height: 844 } })
    const contextB = await browser.newContext()
    const guest = await browser.newContext()
    const admin = await browser.newContext()
    const accountA = await register(contextA, emailA, '云收藏用户A')
    const accountB = await register(contextB, emailB, '云收藏用户B')
    const accountA2 = await login(contextA2, emailA)
    const author = await login(admin, 'author@example.com', 'longenough1')
    const published = []
    let pageToken = ''
    do {
      const set = await (await request(guest, 'GET', `/api/v1/articles/list?page_size=100&page_token=${encodeURIComponent(pageToken)}`)).json()
      published.push(...set.articles.filter(article => article.slug.startsWith('collection-fixture-') && article.status === 2))
      pageToken = set.next_page_token || ''
    } while (pageToken)
    published.sort((a, b) => a.slug.localeCompare(b.slug))
    assert.equal(published.length, 105, 'fixture supplies more than one full page')
    const drafts = await (await request(admin, 'GET', `/api/v1/articles/list?page_size=100&filter=${encodeURIComponent('status:"DRAFT"')}`, author.access_token)).json()
    assert.ok(drafts.articles.length >= 1)
    const hiddenSlug = drafts.articles[0].slug
    const first = published[0]
    const last = published.at(-1)
    await request(guest, 'GET', '/api/v1/collections/list', '', undefined, 401)
    await request(guest, 'POST', `/api/v1/collections/${first.slug}`, '', {}, 401)
    await request(guest, 'DELETE', `/api/v1/collections/${first.slug}`, '', undefined, 401)
    await request(contextA, 'POST', `/api/v1/collections/${hiddenSlug}`, accountA.access_token, {}, 404)
    await request(admin, 'POST', `/api/v1/collections/${hiddenSlug}`, author.access_token, {}, 404)
    await request(contextA, 'POST', '/api/v1/collections/collection-hidden-deleted', accountA.access_token, {}, 404)
    await request(admin, 'POST', '/api/v1/collections/collection-hidden-deleted', author.access_token, {}, 404)
    await request(contextA, 'POST', `/api/v1/collections/absent-${nonce}`, accountA.access_token, {}, 404)

    // Unknown user_id may be rejected or ignored, but can never select B.
    const attack = await contextA.request.post(base + `/api/v1/collections/${first.slug}`, { headers: { Authorization: `Bearer ${accountA.access_token}` }, data: { user_id: accountB.user.id } })
    assert.ok([200, 400].includes(attack.status()))
    const original = await (await request(contextA, 'POST', `/api/v1/collections/${first.slug}`, accountA.access_token, {})).json()
    const duplicate = await (await request(contextA, 'POST', `/api/v1/collections/${first.slug}`, accountA.access_token, {})).json()
    assert.deepEqual(original.saved_at, duplicate.saved_at, 'duplicate save must preserve original date')
    assert.equal((await list(contextB, accountB.access_token)).entries.length, 0)
    for (const article of published.slice(1)) await request(contextA, 'POST', `/api/v1/collections/${article.slug}`, accountA.access_token, {})
    const all = await list(contextA, accountA.access_token)
    assert.deepEqual(all.lengths, [50, 50, 5])
    assert.equal(new Set(all.entries.map(entry => entry.article.slug)).size, 105)
    assert.ok(all.entries.every(entry => entry.article.status === 2))
    assert.equal((await list(contextA2, accountA2.access_token)).entries.length, 105, 'same account in another browser sees server state')
    await request(contextB, 'DELETE', `/api/v1/collections/${first.slug}`, accountB.access_token)
    assert.equal((await list(contextA, accountA.access_token)).entries.length, 105, 'B cannot delete A collection')

    const local = [
      { slug: first.slug, title: '原本地收藏一', saved_at: new Date().toISOString() },
      { slug: last.slug, title: '原本地收藏二', saved_at: new Date().toISOString() },
      { slug: hiddenSlug, title: '历史本地记录', saved_at: new Date().toISOString() },
    ]
    await contextA2.addInitScript(entries => { if (localStorage.getItem('blog:collections') === null) localStorage.setItem('blog:collections', JSON.stringify(entries)) }, local)
    const pageA2 = await contextA2.newPage()
    await pageA2.goto(base + '/collections')
    await pageA2.locator('main a[href^="/posts/collection-fixture-"]').first().waitFor()
    await showAll(pageA2)
    await pageA2.getByText(first.title, { exact: true }).waitFor()
    assert.deepEqual((await guestEntries(pageA2)).map(entry => entry.slug), local.map(entry => entry.slug), 'login must not auto-import or erase local guest entries')
    await pageA2.getByRole('button', { name: '导入本地收藏', exact: true }).click()
    await pageA2.waitForFunction(slug => {
      const entries = JSON.parse(localStorage.getItem('blog:collections') || '[]')
      return entries.length === 1 && entries[0].slug === slug
    }, hiddenSlug)
    assert.equal((await list(contextA, accountA.access_token)).entries.length, 105, 'duplicate import is idempotent')
    assert.equal(await pageA2.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true)

    // An update from the first browser is revalidated by SPA detail navigation.
    await request(contextA, 'DELETE', `/api/v1/collections/${first.slug}`, accountA.access_token)
    await pageA2.locator(`a[href="/posts/${first.slug}"]`).first().click()
    await pageA2.getByRole('button', { name: '☆ 收藏', exact: true }).waitFor()
    await pageA2.getByRole('button', { name: '☆ 收藏', exact: true }).click()
    await pageA2.getByRole('button', { name: '★ 已收藏', exact: true }).waitFor()
    assert.equal((await list(contextA, accountA.access_token)).entries.length, 105)

    // Hide an already saved article, changing its private title as well.
    await request(admin, 'PUT', '/api/v1/articles/update?update_mask=status,title,content_md', author.access_token, { slug: last.slug, title: 'PRIVATE_ONLY_COLLECTION_TITLE', content_md: '私密内容只存在于fixture。', status: 1 })
    const visible = await list(contextA, accountA.access_token)
    assert.equal(visible.entries.length, 104)
    assert.ok(!visible.entries.some(entry => entry.article.slug === last.slug))
    assert.ok(!JSON.stringify(visible).includes('PRIVATE_ONLY_COLLECTION_TITLE'))
    await request(contextA, 'POST', `/api/v1/collections/${last.slug}`, accountA.access_token, {}, 404)
    await request(contextA, 'DELETE', `/api/v1/collections/${last.slug}`, accountA.access_token)
    await request(contextA, 'DELETE', `/api/v1/collections/${last.slug}`, accountA.access_token)
    await pageA2.goto(base + '/collections')
    await pageA2.getByText(/收藏已与当前登录账号同步/).waitFor()
    await pageA2.getByText(first.title, { exact: true }).waitFor()
    await showAll(pageA2)
    await pageA2.getByText(first.title, { exact: true }).waitFor()
    assert.equal(await pageA2.getByText('PRIVATE_ONLY_COLLECTION_TITLE', { exact: true }).count(), 0)

    // Signing out exposes only the remaining original guest record.
    await pageA2.getByRole('button', { name: '云收藏用户A 的账号菜单', exact: true }).click()
    await pageA2.getByRole('menuitem', { name: '退出登录', exact: true }).click()
    await pageA2.getByRole('link', { name: '登录', exact: true }).waitFor()
    await pageA2.goto(base + '/collections')
    await pageA2.getByText('历史本地记录', { exact: true }).waitFor()
    assert.equal(await pageA2.getByText(first.title, { exact: true }).count(), 0)
    assert.deepEqual((await guestEntries(pageA2)).map(entry => entry.slug), [hiddenSlug])

    // A separate anonymous browser still collects locally.
    const guestPage = await guest.newPage()
    await guestPage.goto(base + `/posts/${first.slug}`)
    await guestPage.getByRole('button', { name: '☆ 收藏', exact: true }).click()
    await guestPage.getByRole('button', { name: '★ 已收藏', exact: true }).waitFor()
    assert.deepEqual((await guestEntries(guestPage)).map(entry => entry.slug), [first.slug])
    assert.equal((await list(contextB, accountB.access_token)).entries.length, 0)
    assert.equal((await list(contextA, accountA.access_token)).entries.length, 104)
    const html = await (await request(guest, 'GET', '/collections')).text()
    assert.match(html, /name="robots"[^>]*content="[^"]*noindex/)
    for (const context of [contextA, contextA2, contextB, guest, admin]) await context.close()
    console.log('Collections live PASS: real auth/BFF/SQL, unverified account support, 105 records 50/50/5, duplicate dates, tenant isolation, two browsers, explicit partial import, SPA revalidation, hidden privacy/idempotent cancel, logout guest preservation, local anonymous collection, mobile, noindex')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(safeError(error)); process.exitCode = 1 })
