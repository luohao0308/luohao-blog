const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
if (!['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw new Error('local mock only')

const article = slug => ({ id: slug, slug, title: `文章 ${slug}`, summary: '', content_md: '正文', content_html: '<p>正文</p>', tags: [], status: 2, created_at: '2026-10-15T00:00:00Z', updated_at: '2026-10-15T00:00:00Z' })
const server = new Map([['A', []], ['B', []]])

async function install(page, state) {
  await page.route('**/api/v1/auth/login', async route => {
    const email = route.request().postDataJSON().email
    state.account = email.startsWith('b@') ? 'B' : 'A'
    state.loginRequestAt = Date.now()
    return route.fulfill({ status: 200, headers: { 'Set-Cookie': `session=${state.account}; Path=/api/v1/auth` }, contentType: 'application/json', body: JSON.stringify({ access_token: `token-${state.account}`, user: { id: state.account, email, display_name: state.account, role: 2, email_verified: false } }) })
  })
  await page.route('**/api/v1/auth/refresh', async (route) => {
    if (state.refreshDelay) await new Promise(resolve => setTimeout(resolve, state.refreshDelay))
    if (state.holdRefresh) {
      state.refreshStartedAt = Date.now()
      await new Promise(resolve => { state.releaseRefresh = resolve })
      state.holdRefresh = false
    }
    state.refreshResolvedAt = Date.now()
    return state.account ? route.fulfill({ status: 200, headers: { 'Set-Cookie': `session=${state.account}; Path=/api/v1/auth` }, contentType: 'application/json', body: JSON.stringify({ access_token: `token-${state.account}`, user: { id: state.account, email: `${state.account}@example.com`, display_name: state.account, role: 2 } }) }) : route.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
  })
  await page.route('**/api/v1/auth/logout', route => { state.account = ''; return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }) })
  await page.route('**/api/v1/collections/list**', async (route) => {
    if (state.listFailures > 0) { state.listFailures--; return route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }) }
    const requestAccount = route.request().headers().authorization?.endsWith('token-B') ? 'B' : 'A'
    const rows = [...(server.get(requestAccount) || [])]
    if (state.listDelay) await new Promise(resolve => setTimeout(resolve, state.listDelay))
    const token = Number(new URL(route.request().url()).searchParams.get('page_token') || 0)
    const slice = rows.slice(token, token + 100)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ collections: slice, next_page_token: token + slice.length < rows.length ? String(token + slice.length) : '' }) })
  })
  await page.route('**/api/v1/articles/list**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ articles: [article('status-target')] }) }))
  await page.route('**/api/v1/articles/status-target/view', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))
  await page.route('**/api/v1/articles/status-target/comments**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ comments: [] }) }))
  await page.route('**/api/v1/articles/status-target', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(article('status-target')) }))
  await page.route('**/api/v1/collections/*', async route => {
    const slug = decodeURIComponent(new URL(route.request().url()).pathname.split('/').pop())
    if (slug === 'list') return route.fallback()
    const requestAccount = route.request().headers().authorization?.endsWith('token-B') ? 'B' : 'A'
    state.calls.push({ account: requestAccount, method: route.request().method(), slug })
    if (slug === 'refresh-race' && route.request().method() === 'POST' && requestAccount === 'A')
      return route.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
    if (slug === 'slow-import' && route.request().method() === 'POST' && requestAccount === 'A') {
      await new Promise(resolve => setTimeout(resolve, 700))
      return route.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
    }
    if (slug === 'bad-import') return route.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
    const rows = server.get(requestAccount) || []
    if (route.request().method() === 'DELETE') server.set(requestAccount, rows.filter(row => row.article.slug !== slug))
    else if (!rows.some(row => row.article.slug === slug)) rows.unshift({ article: article(slug), saved_at: new Date().toISOString() })
    return route.fulfill({ status: 200, contentType: 'application/json', body: route.request().method() === 'DELETE' ? '{}' : JSON.stringify((server.get(requestAccount) || []).find(row => row.article.slug === slug)) })
  })
}

async function login(page, email) {
  await page.goto(`${base}/login`)
  await page.getByPlaceholder('邮箱').fill(email)
  await page.getByPlaceholder('密码').fill('longenough1')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL(`${base}/`)
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    const stateA = { account: '', calls: [] }
    await install(pageA, stateA)
    await pageA.goto(`${base}/collections`)
    await pageA.evaluate(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'good-import', title: '本地成功', saved_at: new Date().toISOString() }, { slug: 'bad-import', title: '本地失败', saved_at: new Date().toISOString() }])))
    await pageA.reload()
    await pageA.getByText('本地成功', { exact: true }).waitFor()
    await login(pageA, 'a@example.com')
    await pageA.goto(`${base}/collections`)
    await pageA.getByRole('button', { name: '导入本地收藏' }).click()
    await pageA.getByText(/已导入 1 条，1 条失败/).waitFor()
    assert.deepEqual((await pageA.evaluate(() => JSON.parse(localStorage.getItem('blog:collections')))).map(item => item.slug), ['bad-import'])
    assert.ok(server.get('A').some(row => row.article.slug === 'good-import'))

    const ctxA2 = await browser.newContext()
    const pageA2 = await ctxA2.newPage()
    const stateA2 = { account: '', calls: [], listDelay: 0 }
    await install(pageA2, stateA2)
    await login(pageA2, 'a@example.com')
    await pageA2.goto(`${base}/collections`)
    await pageA2.getByText('文章 good-import', { exact: true }).waitFor()

    const ctxB = await browser.newContext()
    const pageB = await ctxB.newPage()
    await install(pageB, { account: '', calls: [] })
    await login(pageB, 'b@example.com')
    await pageB.goto(`${base}/collections`)
    assert.equal(await pageB.getByText('文章 good-import', { exact: true }).count(), 0, 'accounts must be isolated')

    server.set('A', Array.from({ length: 105 }, (_, i) => ({ article: article(`many-${i}`), saved_at: new Date(Date.now() - i).toISOString() })))
    await pageA2.reload()
    await pageA2.getByRole('button', { name: '加载更多' }).waitFor()
    assert.equal(await pageA2.locator('ul > li').count(), 20)
    await pageA2.getByRole('button', { name: '加载更多' }).click()
    assert.equal(await pageA2.locator('ul > li').count(), 40)
    stateA2.listDelay = 700
    const delayedList = pageA2.waitForRequest(request => request.url().includes('/api/v1/collections/list'))
    await pageA2.getByRole('button', { name: '刷新云端' }).click()
    await delayedList
    await pageA2.locator('ul > li').first().getByRole('button', { name: '移除' }).click()
    await pageA2.waitForTimeout(900)
    assert.equal(server.get('A').some(row => row.article.slug === 'many-0'), false)
    assert.equal(await pageA2.getByText('文章 many-0', { exact: true }).count(), 0, 'older delayed list must not overwrite a successful same-owner delete')

    await pageA.getByLabel('A 的账号菜单').click()
    await pageA.getByRole('menuitem', { name: '退出登录' }).click()
    await pageA.goto(`${base}/collections`)
    await pageA.getByText('本地失败', { exact: true }).waitFor()

    const ctxSwitch = await browser.newContext()
    const pageSwitch = await ctxSwitch.newPage()
    const stateSwitch = { account: '', calls: [] }
    await install(pageSwitch, stateSwitch)
    await pageSwitch.goto(`${base}/collections`)
    await pageSwitch.evaluate(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'slow-import', title: '延迟导入', saved_at: new Date().toISOString() }])))
    await login(pageSwitch, 'a@example.com')
    await pageSwitch.goto(`${base}/collections`)
    const slowRequest = pageSwitch.waitForRequest(request => request.url().includes('/api/v1/collections/slow-import'))
    await pageSwitch.getByRole('button', { name: '导入本地收藏' }).click()
    await slowRequest
    await pageSwitch.getByLabel('A 的账号菜单').click()
    await pageSwitch.getByRole('menuitem', { name: '退出登录' }).click()
    await login(pageSwitch, 'b@example.com')
    await pageSwitch.waitForTimeout(900)
    assert.equal(stateSwitch.calls.some(call => call.slug === 'slow-import' && call.account === 'B'), false, 'A 401 must not retry the save as B')
    assert.equal(server.get('B').some(row => row.article.slug === 'slow-import'), false)
    assert.ok((await pageSwitch.evaluate(() => JSON.parse(localStorage.getItem('blog:collections')))).some(item => item.slug === 'slow-import'), 'uncertain owner-switched import must remain local')

    const ctxInit = await browser.newContext()
    const pageInit = await ctxInit.newPage()
    const stateInit = { account: 'A', calls: [], refreshDelay: 700 }
    await install(pageInit, stateInit)
    await pageInit.goto(`${base}/collections`)
    await pageInit.evaluate(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'hand-fast', title: '恢复期间', saved_at: new Date().toISOString() }])))
    await pageInit.reload()
    await pageInit.waitForTimeout(150)
    assert.equal(stateInit.calls.some(call => call.slug === 'hand-fast'), false, 'session recovery must finish before any cloud mutation is enabled')
    assert.equal((await pageInit.getByRole('button', { name: '导入本地收藏' }).count()), 0)

    await login(pageA, 'a@example.com')
    await pageA.goto(`${base}/collections`)
    await pageA.evaluate(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'quota-import', title: '配额失败', saved_at: new Date().toISOString() }])))
    await pageA.reload()
    await pageA.evaluate(() => {
      const original = Storage.prototype.setItem
      Storage.prototype.setItem = function (key, value) {
        if (key === 'blog:collections') throw new DOMException('quota', 'QuotaExceededError')
        return original.call(this, key, value)
      }
    })
    await pageA.getByRole('button', { name: '导入本地收藏' }).click()
    await pageA.getByText(/0 条失败|1 条失败/).waitFor()
    assert.ok((await pageA.evaluate(() => JSON.parse(localStorage.getItem('blog:collections')))).some(item => item.slug === 'quota-import'), 'localStorage write failure must preserve the retryable guest item')

    server.set('A', [{ article: article('status-target'), saved_at: new Date().toISOString() }])
    const ctxMutation = await browser.newContext()
    const pageMutation = await ctxMutation.newPage()
    const stateMutation = { account: '', calls: [], listDelay: 0 }
    await install(pageMutation, stateMutation)
    await login(pageMutation, 'a@example.com')
    await pageMutation.goto(`${base}/collections`)
    await pageMutation.getByText('文章 status-target', { exact: true }).waitFor()
    stateMutation.listDelay = 700
    const delayedDetailList = pageMutation.waitForRequest(request => request.url().includes('/api/v1/collections/list'))
    await pageMutation.getByRole('link', { name: '文章 status-target' }).click()
    await delayedDetailList
    const mutationButton = pageMutation.getByRole('button', { name: '★ 已收藏' })
    await mutationButton.click()
    await pageMutation.waitForTimeout(900)
    const uncollectedButton = pageMutation.getByRole('button', { name: '☆ 收藏' })
    await uncollectedButton.waitFor()
    assert.equal(await uncollectedButton.isDisabled(), false, 'superseded initialize must preserve ready after a successful mutation')
    assert.equal(server.get('A').some(row => row.article.slug === 'status-target'), false)

    const ctxRefreshRace = await browser.newContext()
    const pageRefreshRace = await ctxRefreshRace.newPage()
    const stateRefreshRace = { account: '', calls: [], holdRefresh: false, releaseRefresh: null, refreshStartedAt: 0, loginRequestAt: 0, refreshResolvedAt: 0 }
    await install(pageRefreshRace, stateRefreshRace)
    await login(pageRefreshRace, 'a@example.com')
    await pageRefreshRace.evaluate(() => localStorage.setItem('blog:collections', JSON.stringify([{ slug: 'refresh-race', title: '刷新竞态', saved_at: new Date().toISOString() }])))
    await pageRefreshRace.goto(`${base}/collections`)
    await pageRefreshRace.getByRole('button', { name: '导入本地收藏' }).waitFor()
    stateRefreshRace.holdRefresh = true
    stateRefreshRace.loginRequestAt = 0
    const heldRefresh = pageRefreshRace.waitForRequest(request => request.url().includes('/api/v1/auth/refresh'))
    await pageRefreshRace.getByRole('button', { name: '导入本地收藏' }).click()
    await heldRefresh
    await pageRefreshRace.getByLabel('A 的账号菜单').click()
    await pageRefreshRace.getByRole('menuitem', { name: '切换账号' }).click()
    await pageRefreshRace.getByPlaceholder('邮箱').fill('b@example.com')
    await pageRefreshRace.getByPlaceholder('密码').fill('longenough1')
    await pageRefreshRace.getByRole('button', { name: '登录', exact: true }).click()
    await pageRefreshRace.waitForTimeout(150)
    assert.equal(stateRefreshRace.loginRequestAt, 0, 'B login HTTP must wait while the A refresh is in flight')
    stateRefreshRace.releaseRefresh()
    await pageRefreshRace.waitForURL(`${base}/`)
    assert.ok(stateRefreshRace.loginRequestAt >= stateRefreshRace.refreshResolvedAt)
    assert.equal((await ctxRefreshRace.cookies()).find(cookie => cookie.name === 'session')?.value, 'B')
    await pageRefreshRace.getByRole('button', { name: 'B 的账号菜单', exact: true }).waitFor()
    assert.equal(server.get('B').some(row => row.article.slug === 'refresh-race'), false)
    assert.equal(stateRefreshRace.calls.some(call => call.slug === 'refresh-race' && call.account === 'B'), false)

    server.set('A', [{ article: article('status-target'), saved_at: new Date().toISOString() }])
    const ctxFailure = await browser.newContext()
    const pageFailure = await ctxFailure.newPage()
    const stateFailure = { account: '', calls: [], listFailures: 0 }
    await install(pageFailure, stateFailure)
    await login(pageFailure, 'a@example.com')
    stateFailure.listFailures = 2
    await pageFailure.goto(`${base}/collections`)
    await pageFailure.getByRole('alert').filter({ hasText: '云端收藏加载失败' }).waitFor()
    assert.equal(await pageFailure.getByRole('button', { name: '移除' }).count(), 0, 'unknown cloud state must not expose mutation buttons')
    await pageFailure.getByRole('button', { name: '重新加载' }).click()
    await pageFailure.getByText('文章 status-target', { exact: true }).waitFor()
    stateFailure.listFailures = 20
    await pageFailure.getByRole('link', { name: '文章 status-target' }).click()
    const collectButton = pageFailure.getByRole('button', { name: /收藏/ }).first()
    await pageFailure.getByRole('alert').filter({ hasText: /收藏.*无法加载|云端收藏加载失败/ }).waitFor()
    assert.equal(await collectButton.isDisabled(), true)
    stateFailure.listFailures = 0
    await pageFailure.getByRole('button', { name: '重试' }).click()
    await pageFailure.getByRole('button', { name: '★ 已收藏' }).waitFor()

    await Promise.all([ctxA.close(), ctxA2.close(), ctxB.close(), ctxSwitch.close(), ctxInit.close(), ctxFailure.close(), ctxMutation.close(), ctxRefreshRace.close()])
    console.log('Collections mock PASS: explicit partial import, guest retention, same-account sync, account isolation, pagination, logout preservation')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
