// Local-only admin fixtures: never writes to an actual backend.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const built = path.resolve('frontend/.output/public/_nuxt')
const editorChunks = fs.existsSync(built) ? fs.readdirSync(built).filter(name => name.endsWith('.js') && /export\{[^}]* as Crepe[,}]/.test(fs.readFileSync(path.join(built, name), 'utf8'))) : []
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname)) throw Error('Local only')
async function main() {
  const browser = await chromium.launch({ executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const context = await browser.newContext()
  const runtimeErrors = []
  context.on('page', current => {
    current.on('pageerror', error => runtimeErrors.push(`pageerror: ${error.message}`))
    current.on('console', message => {
      // Deliberate HTTP 500/module aborts are checked below; hydration is never expected.
      if (/hydration.*(mismatch|contains mismatches)|hydration node/i.test(message.text()))
        runtimeErrors.push(`hydration: ${message.text().slice(0, 600)}`)
    })
  })
  let fail = true
  let release
  let pending
  async function waitForRequest(page) {
    for (let i = 0; i < 200 && !release; i++) await page.waitForTimeout(50)
    assert.ok(release, 'save request must begin')
  }
  const article = { slug: 'fixture', title: '原始标题', summary: '', content_md: '原始正文', tags: [], status: 1, cover_url: '', featured: false }
  await context.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/auth/refresh')) return route.fulfill({ json: { access_token: 'fixture', user: { id: '1', role: 1, display_name: '管理员' } } })
    if (url.pathname.endsWith('/articles/update')) {
      pending = route.request().postDataJSON()
      await new Promise(resolve => { release = resolve })
      return route.fulfill({ status: fail ? 500 : 200, json: fail ? { message: 'failed' } : article })
    }
    if (url.pathname.includes('/categories')) return route.fulfill({ json: { categories: [] } })
    return route.fulfill({ json: url.pathname.includes('/articles/list') ? { articles: [] } : { ...article, slug: url.pathname.split('/').pop(), title: url.pathname.endsWith('/other') ? '另一篇文章' : article.title } })
  })
  const page = await context.newPage()
  try {
    await page.goto(`${base}/admin/posts/fixture/edit`)
    const title = page.getByPlaceholder('文章标题')
    await title.waitFor()
    await page.locator('.milkdown [contenteditable=true]').waitFor()
    assert.equal(await page.getByTestId('save-state').count(), 1, 'persistent save state is required')
    // Transaction and save/unload occur in one browser task: no 200ms listener
    // debounce or Playwright actionability delay can hide a stale source model.
    fail = false
    const prevented = await page.locator('.milkdown .ProseMirror').evaluate(async el => {
      el.focus()
      document.execCommand('insertText', false, '即时正文')
      await Promise.resolve()
      const prevented = !window.dispatchEvent(new Event('beforeunload', { cancelable: true }))
      Array.from(document.querySelectorAll('button')).find(button => button.textContent.trim() === '保存').click()
      return prevented
    })
    assert.equal(prevented, true, 'same-task edit must immediately guard unload')
    await waitForRequest(page)
    assert.ok(pending.content_md.includes('即时正文'), 'same-task save includes last transaction')
    release(); release = null
    await page.getByTestId('save-state').filter({ hasText: '已保存' }).waitFor()
    let immediateDialogs = 0
    page.on('dialog', async dialog => { immediateDialogs++; await dialog.dismiss() })
    await page.locator('.milkdown .ProseMirror').evaluate(async el => {
      el.focus()
      document.execCommand('insertText', false, '离开前最后输入')
      await Promise.resolve()
      return document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$router.push('/admin/posts')
    })
    assert.equal(immediateDialogs, 1, 'same-task edit must guard route leave')
    assert.equal(new URL(page.url()).pathname, '/admin/posts/fixture/edit')
    page.removeAllListeners('dialog')
    fail = true
    console.log('PASS: synchronous final transaction is saved and guarded before debounce')
    await title.fill('第一次修改')
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.waitForFunction(() => document.querySelector('[data-testid=save-state]')?.textContent.includes('保存中'))
    await waitForRequest(page)
    release(); release = null
    await page.getByTestId('save-state').filter({ hasText: '保存失败' }).waitFor()
    assert.equal(await title.inputValue(), '第一次修改')
    fail = false
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await waitForRequest(page)
    await page.locator('.milkdown .ProseMirror').evaluate(async el => {
      el.focus()
      document.execCommand('insertText', false, '保存期间的新正文')
      await Promise.resolve()
    })
    release(); release = null
    await page.getByTestId('save-state').filter({ hasText: '未保存' }).waitFor()
    assert.equal(pending.title, '第一次修改')
    assert.ok(!pending.content_md.includes('保存期间的新正文'), 'in-flight body is detached from later typing')
    let dialogs = 0
    page.on('dialog', async dialog => { dialogs++; await dialog.dismiss() })
    await page.getByRole('link', { name: '返回列表' }).click()
    assert.equal(dialogs, 1)
    assert.equal(new URL(page.url()).pathname, '/admin/posts/fixture/edit')
    await page.evaluate(() => window.__beforeUnloadPrevented = !window.dispatchEvent(new Event('beforeunload', { cancelable: true })))
    assert.equal(await page.evaluate(() => window.__beforeUnloadPrevented), true)
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await waitForRequest(page)
    release(); release = null
    await page.getByTestId('save-state').filter({ hasText: '已保存' }).waitFor()
    await page.getByRole('link', { name: '返回列表' }).click()
    await page.waitForURL('**/admin/posts')
    assert.equal(dialogs, 1, 'saved forms leave without confirmation')
    await page.goto(`${base}/admin/posts/fixture/edit`)
    await title.waitFor()
    await title.fill('拦截切换')
    const routeTo = slug => page.evaluate(slug => document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$router.push(`/admin/posts/${slug}/edit`), slug)
    await routeTo('other')
    assert.equal(new URL(page.url()).pathname, '/admin/posts/fixture/edit')
    page.removeAllListeners('dialog')
    page.on('dialog', dialog => dialog.accept())
    await routeTo('other')
    await page.waitForURL('**/other/edit')
    await page.waitForFunction(() => document.querySelector('input[placeholder="文章标题"]')?.value === '另一篇文章')
    console.log('PASS: same-component slug rejection, acceptance, and reload')
    console.log('PASS: save failure retention, pending edits, guards, successful reset')
    // New-article freezes every field while writing; a late create cannot redirect a departed page.
    await page.goto(`${base}/admin/posts/new`)
    await page.locator('.milkdown [contenteditable=true]').waitFor()
    await title.fill('创建测试')
    await page.getByPlaceholder('url-safe-slug').fill('new-fixture')
    await page.locator('.milkdown [contenteditable=true]').click()
    await page.keyboard.insertText('新文章正文')
    await page.waitForTimeout(500)
    await page.route('**/api/v1/articles/create', async route => {
      await new Promise(resolve => { release = resolve })
      await route.fulfill({ json: { ...article, slug: 'new-fixture' } })
    })
    await page.getByRole('button', { name: '创建', exact: true }).click()
    await waitForRequest(page)
    assert.equal(await title.isDisabled(), true)
    assert.equal(await page.locator('.milkdown .ProseMirror').getAttribute('contenteditable'), 'false')
    await page.getByRole('link', { name: '返回列表' }).click()
    await page.waitForURL('**/admin/posts')
    release(); release = null
    await page.waitForTimeout(500)
    assert.equal(new URL(page.url()).pathname, '/admin/posts')
    console.log('PASS: new save freezes fields, late response cannot pull departed user back')

    await page.goto(`${base}/admin/posts/new`)
    await page.locator('.milkdown [contenteditable=true]').waitFor()
    await title.fill('成功新建')
    await page.getByPlaceholder('url-safe-slug').fill('created-ok')
    await page.locator('.milkdown [contenteditable=true]').click()
    await page.keyboard.insertText('可保存的正文')
    await page.waitForTimeout(500)
    await page.unroute('**/api/v1/articles/create')
    await page.route('**/api/v1/articles/create', route => route.fulfill({ json: { ...article, slug: 'created-ok' } }))
    let createDialogs = 0
    page.removeAllListeners('dialog')
    page.on('dialog', async dialog => { createDialogs++; await dialog.dismiss() })
    await page.getByRole('button', { name: '创建', exact: true }).click()
    await page.waitForURL('**/created-ok/edit')
    assert.equal(createDialogs, 0, 'successful create resets dirty before navigation')
    await page.locator('.milkdown [contenteditable=true]').waitFor()
    await page.evaluate(() => localStorage.setItem('theme', 'dark'))
    await page.reload()
    await page.locator('.milkdown [contenteditable=true]').waitFor()
    const editorColors = await page.locator('.milkdown').evaluate(el => {
      const style = getComputedStyle(el)
      return { scheme: style.colorScheme, background: style.backgroundColor, color: style.color }
    })
    assert.equal(editorColors.scheme, 'dark')
    assert.notEqual(editorColors.background, 'rgb(255, 255, 255)')
    assert.notEqual(editorColors.background, editorColors.color)
    fs.mkdirSync('output/ui-refinement/s4', { recursive: true })
    await page.screenshot({ path: 'output/ui-refinement/s4/editor-dark.png', fullPage: true })
    console.log('PASS: Crepe dark palette is applied')
    console.log('PASS: successful create navigates without discard confirmation')

    // A fresh document with the lazy editor module denied must expose editable source.
    const fallback = await context.newPage()
    const blocked = []
    await fallback.route('**/*', async route => {
      if (/milkdown.*crepe.*\.js/.test(route.request().url()) || editorChunks.includes(new URL(route.request().url()).pathname.split('/').pop())) {
        blocked.push(route.request().url())
        return route.abort('failed')
      }
      return route.fallback()
    })
    await fallback.goto(`${base}/admin/posts/fixture/edit`)
    await fallback.getByRole('button', { name: '重试加载编辑器' }).waitFor()
    assert.ok(blocked.length, 'lazy import actually failed')
    const source = fallback.getByRole('textbox', { name: 'Markdown 正文' })
    await source.fill('失败后继续编辑的正文')
    await fallback.getByRole('button', { name: '重试加载编辑器' }).click()
    await fallback.getByRole('button', { name: '重试加载编辑器' }).waitFor()
    assert.equal(await source.inputValue(), '失败后继续编辑的正文')
    await fallback.screenshot({ path: 'output/ui-refinement/s4/editor-fallback.png', fullPage: true })
    await fallback.getByRole('button', { name: '保存', exact: true }).click()
    await waitForRequest(fallback)
    assert.equal(pending.content_md, '失败后继续编辑的正文', 'fallback source must be sent to the save API')
    release(); release = null
    await fallback.getByTestId('save-state').filter({ hasText: '已保存' }).waitFor()
    assert.equal(await fallback.evaluate(() => window.dispatchEvent(new Event('beforeunload', { cancelable: true }))), true, 'successful fallback save clears dirty')
    console.log('PASS: fallback source saves exact content and clears dirty')
    console.log('PASS: import failure keeps source editable, retry preserves latest input')
    assert.deepEqual(runtimeErrors, [], 'admin pages must hydrate without runtime errors')
    console.log('PASS: no page errors or hydration mismatches')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
