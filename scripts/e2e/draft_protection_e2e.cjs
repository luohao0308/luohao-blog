// Isolated HTTP fixtures: no database, production endpoint, or paid model.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const article = { id: '1', version: '1', slug: 'draft-test', title: '服务器标题', summary: '', content_md: '服务器正文', content_html: '<p>服务器正文</p>', tags: [], status: 1 }
let failSave = true
let saveDelay = 0
let saveRequests = 0
let account = 'author'
async function main() {
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    const url = new URL(req.url, 'http://fixture')
    if (url.pathname.includes('/auth/')) return res.end(JSON.stringify({ access_token: 'fixture-token', user: { id: account, display_name: account, email: 'fixture@example.com', role: 1 } }))
    if ((url.pathname.endsWith('/articles/update') || url.pathname.endsWith('/articles/create'))) {
      saveRequests++
      if (failSave) { res.writeHead(503); return res.end('{}') }
      let body = ''
      req.on('data', chunk => { body += chunk })
      return req.on('end', () => setTimeout(() => { Object.assign(article, JSON.parse(body)); article.version = url.pathname.endsWith('/articles/create') ? '1' : String(BigInt(article.version) + 1n); res.end(JSON.stringify(article)) }, saveDelay))
    }
    if (url.pathname.endsWith('/articles/draft-test')) return res.end(JSON.stringify(article))
    res.end(JSON.stringify({ categories: [], articles: [] }))
  })
  await listen(api)
  const reservation = http.createServer()
  await listen(reservation)
  const port = reservation.address().port
  await new Promise(resolve => reservation.close(resolve))
  const base = `http://127.0.0.1:${port}`
  const frontend = spawn(process.execPath, ['.output/server/index.mjs'], { cwd: path.join(root, 'frontend'), env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: String(port), NUXT_BACKEND_BASE: `http://127.0.0.1:${api.address().port}` }, stdio: 'ignore' })
  let browser
  try {
    let ready = false
    for (let i = 0; i < 100; i++) {
      try { ready = (await fetch(base + '/posts')).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    assert.ok(ready, 'built frontend starts')
    browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
    const page = await browser.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('dialog', dialog => dialog.type() === 'beforeunload' ? dialog.accept() : dialog.dismiss())
    await page.goto(base + '/admin/posts/draft-test/edit')
    const title = page.getByPlaceholder('文章标题')
    await title.fill('本地新标题')
    await page.getByRole('status').filter({ hasText: '已暂存到此浏览器' }).waitFor()
    const key = 'blog:article-draft:author:draft-test'
    assert.equal(await page.evaluate(k => JSON.parse(localStorage.getItem(k)).title, key), '本地新标题')
    await page.getByRole('link', { name: '返回列表', exact: true }).click()
    assert.ok(page.url().endsWith('/edit'), 'cancel route leave retains editor')
    await page.reload()
    await page.getByRole('button', { name: '恢复草稿' }).waitFor()
    assert.equal(await title.inputValue(), '服务器标题', 'recovery never overwrites server automatically')
    await page.getByRole('button', { name: '恢复草稿' }).click()
    assert.equal(await title.inputValue(), '本地新标题')
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.getByText('保存失败', { exact: true }).waitFor()
    assert.ok(await page.evaluate(k => localStorage.getItem(k), key), 'failed save retains draft')
    failSave = false
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.getByText('正文已保存', { exact: true }).waitFor()
    assert.equal(await page.evaluate(k => localStorage.getItem(k), key), null)
    saveDelay = 500
    await title.fill('提交时标题')
    const requestsBefore = saveRequests
    const savedResponse = page.waitForResponse(response => response.url().includes('/articles/update'))
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await title.fill('提交后继续编辑')
    await title.press('Enter')
    await savedResponse
    assert.equal(saveRequests, requestsBefore + 1, 'Enter during save cannot submit twice')
    assert.equal(await page.evaluate(k => JSON.parse(localStorage.getItem(k)).title, key), '提交后继续编辑', 'in-flight edits remain in draft')
    saveDelay = 0
    await title.fill('账号 A 私有草稿')
    account = 'other-author'
    await page.reload()
    await title.waitFor()
    assert.equal(await page.getByRole('button', { name: '恢复草稿' }).count(), 0, 'other account cannot restore this draft')
    assert.equal(await title.inputValue(), '提交时标题')
    await page.addInitScript(() => { Storage.prototype.setItem = () => { throw new Error('quota') } })
    await page.reload()
    await title.fill('存储失败也可手动保存')
    await page.getByText('本地暂存失败，请及时手动保存；离开可能丢失修改。', { exact: true }).waitFor()
    await page.getByRole('button', { name: '保存', exact: true }).click()
    await page.getByText('正文已保存', { exact: true }).waitFor()
    const newPage = await browser.newPage()
    newPage.on('pageerror', error => errors.push(error.message))
    newPage.on('dialog', dialog => dialog.type() === 'beforeunload' ? dialog.accept() : dialog.dismiss())
    await newPage.goto(base + '/admin/posts/new')
    await newPage.getByPlaceholder('文章标题').fill('新建草稿标题')
    await newPage.getByPlaceholder('url-safe-slug').fill('draft-test')
    const editor = newPage.locator('.milkdown .ProseMirror[contenteditable]')
    await editor.fill('新建草稿的真实正文')
    const newKey = 'blog:article-draft:other-author:new'
    await newPage.waitForFunction(k => JSON.parse(localStorage.getItem(k) || '{}').content_md?.includes('新建草稿的真实正文'), newKey)
    assert.equal(await newPage.evaluate(k => JSON.parse(localStorage.getItem(k)).title, newKey), '新建草稿标题', 'persisted draft includes title before reload')
    await newPage.reload()
    await newPage.getByRole('button', { name: '恢复草稿' }).waitFor()
    assert.equal(await newPage.getByPlaceholder('文章标题').inputValue(), '')
    assert.equal(await editor.evaluate(el => !!el.closest('[inert]')), true, 'pending recovery editor is inert')
    await newPage.getByRole('button', { name: '恢复草稿' }).click()
    await newPage.waitForFunction(() => document.querySelector('input[placeholder="文章标题"]')?.value === '新建草稿标题')
    await editor.waitFor()
    assert.equal(await newPage.getByPlaceholder('文章标题').inputValue(), '新建草稿标题')
    assert.ok((await editor.innerText()).includes('新建草稿的真实正文'), 'restored markdown remounts into real Milkdown')
    saveDelay = 800
    const creating = newPage.waitForRequest(request => request.url().includes('/articles/create'))
    await newPage.getByRole('button', { name: '创建', exact: true }).click()
    await creating
    assert.equal(await newPage.getByPlaceholder('文章标题').isDisabled(), true, 'new form is disabled while saving')
    assert.equal(await editor.evaluate(el => !!el.closest('[inert]')), true, 'new editor is inert while saving')
    await newPage.waitForURL('**/admin/posts/draft-test/edit')
    assert.equal(await newPage.evaluate(k => localStorage.getItem(k), newKey), null, 'successful creation clears new draft')
    assert.ok(article.content_md.includes('新建草稿的真实正文'), 'create request persists restored markdown')
    await newPage.close()
    assert.deepEqual(errors, [])
    console.log('PASS: draft persistence, explicit recovery, cancelled leave, failed/successful save, account isolation, quota failure, new draft markdown recovery, creation lock and cleanup/navigation')
  } finally {
    await browser?.close()
    frontend.kill('SIGTERM')
    await new Promise(resolve => api.close(resolve))
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
