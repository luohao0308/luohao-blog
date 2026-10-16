const fs = require('node:fs')
const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw Error('Local only')
async function main() {
  fs.mkdirSync('output/ui-refinement/s3', { recursive: true })
  const code = '\n  fmt.Println("hello")\n\n'
  // Golden generated and verified by the Go Markdown renderer's own test.
  const renderedCode = fs.readFileSync('backend/internal/biz/article/render/testdata/code-language.html', 'utf8')
  const fixture = await startFixture({ articles: [makeArticle({ content_html: `<h2>请求处理</h2><h2>请求处理</h2><h3>重试</h3><pre><code class="language-go">${code}</code></pre>${renderedCode}<img src="/v1/assets/fixture.png" alt="架构示例"><a href="/posts"><img src="/api/v1/assets/fixture.png" alt="链接图片"></a>` })] })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const html = await (await fetch(`${base}/posts/fixture-article`)).text()
    assert.ok(html.includes('language-go') && html.includes('请求处理') && html.includes('<details'), 'SSR preserves code, headings and static TOC')
    assert.ok(html.includes('src="/api/v1/assets/fixture.png" alt="架构示例"'), 'SSR maps stored image path to BFF')
    const page = await browser.newPage({ viewport: { width: 320, height: 800 } })
    await page.route('**/api/v1/assets/fixture.png', route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#e8eef7"/><rect x="100" y="250" width="350" height="250" rx="20" fill="#3c5d85"/><path d="M450 375H750" stroke="#365f53" stroke-width="12"/><rect x="750" y="250" width="350" height="250" rx="20" fill="#365f53"/></svg>' }))
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.addInitScript(() => {
      window.copied = []
      window.rejectCopy = false
      Object.defineProperty(navigator, 'clipboard', { value: { writeText: async text => {
        if (window.rejectCopy) throw Error('Denied')
        window.copied.push(text)
      } } })
    })
    await page.goto(`${base}/posts/fixture-article`)
    await page.getByRole('heading', { name: '并发与一致性' }).waitFor()
    const copy = page.getByRole('button', { name: '复制代码', exact: true }).first()
    await copy.waitFor()
    assert.equal(await copy.count(), 1, 'code must expose a copy action')
    await copy.click()
    await page.getByRole('button', { name: '已复制', exact: true }).waitFor()
    assert.deepEqual(await page.evaluate(() => window.copied), [code])
    const realCode = page.locator('[data-code-language="go"]')
    assert.equal(await realCode.locator('.code-toolbar > span').textContent(), 'go')
    const expectedGo = 'package main\n\nfunc main() {\n\tprintln("hello <world>")\n}\n'
    assert.equal(await realCode.locator('pre code').textContent(), expectedGo)
    await realCode.getByRole('button', { name: '复制代码', exact: true }).click()
    assert.equal(await page.evaluate(() => window.copied.at(-1)), expectedGo)
    assert.equal(await page.locator('[data-code-language="unknown-language"] .code-toolbar > span').textContent(), 'unknown-language')
    assert.equal(await page.locator('.code-toolbar > span').last().textContent(), '代码')
    await page.evaluate(() => { window.rejectCopy = true })
    await page.getByRole('button', { name: '已复制', exact: true }).first().click()
    await page.getByRole('button', { name: '复制失败，重试', exact: true }).waitFor()
    const mobileTOC = page.locator('details')
    assert.equal(await mobileTOC.getAttribute('open'), null)
    await mobileTOC.locator('summary').click()
    await mobileTOC.getByRole('link', { name: '请求处理', exact: true }).last().click()
    assert.ok(await page.evaluate(() => {
      const id = decodeURIComponent(location.hash.slice(1))
      return document.getElementById(id).getBoundingClientRect().top >= document.querySelector('.site-header').getBoundingClientRect().bottom
    }), 'target heading must remain below the sticky header')
    const ids = await page.locator('.markdown-body h2').evaluateAll(nodes => nodes.map(node => node.id))
    assert.equal(new Set(ids).size, ids.length)
    const imageButton = page.getByRole('button', { name: '放大图片：架构示例' })
    await imageButton.focus()
    await page.keyboard.press('Enter')
    const dialog = page.getByRole('dialog', { name: '图片预览' })
    await dialog.waitFor()
    assert.ok(await dialog.locator('img').evaluate(image => { const rect = image.getBoundingClientRect(); return rect.width <= innerWidth && rect.height <= innerHeight }), 'large image must fit viewport')
    assert.ok(await dialog.getByRole('link', { name: '查看原图' }).getAttribute('href'))
    assert.equal(await page.evaluate(() => document.body.style.overflow), 'hidden')
    await page.screenshot({ path: 'output/ui-refinement/s3/modal-mobile.png' })
    await page.keyboard.press('Tab')
    await page.keyboard.press('Tab')
    assert.ok(await dialog.evaluate(el => el.contains(document.activeElement)), 'focus remains inside modal')
    await page.keyboard.press('Escape')
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await imageButton.evaluate(el => el === document.activeElement), true)
    assert.equal(await page.evaluate(() => document.body.style.overflow), '')
    assert.equal(await page.locator('a[href="/posts"] img[alt="链接图片"]').count(), 1)
    assert.equal(await page.getByRole('button', { name: /链接图片/ }).count(), 0)
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.evaluate(async () => { document.activeElement?.blur(); window.scrollTo(0, 0); await document.fonts.ready })
    await page.waitForFunction(() => scrollY === 0)
    await page.screenshot({ path: 'output/ui-refinement/s3/reading-mobile.png', fullPage: true })
    await page.setViewportSize({ width: 1280, height: 900 })
    assert.ok(await page.getByRole('navigation', { name: '文章目录', exact: true }).filter({ visible: true }).count())
    await page.evaluate(async () => { document.activeElement?.blur(); window.scrollTo(0, 0); await document.fonts.ready })
    await page.waitForFunction(() => scrollY === 0)
    await page.screenshot({ path: 'output/ui-refinement/s3/reading-desktop.png', fullPage: true })
    await imageButton.click()
    // Exercise both unsupported and rejected native-modal paths.
    await page.keyboard.press('Escape')
    for (const mode of ['missing', 'throw']) {
      await page.evaluate(mode => {
        window.originalShowModal ??= HTMLDialogElement.prototype.showModal
        HTMLDialogElement.prototype.showModal = mode === 'missing' ? undefined : function () { throw Error('Unavailable') }
      }, mode)
      await imageButton.click()
      const fallback = page.getByRole('region', { name: '图片预览暂不可用' })
      await fallback.waitFor()
      const original = fallback.getByRole('link', { name: '查看原图' })
      assert.ok(await original.getAttribute('href'))
      assert.equal(await original.evaluate(el => el === document.activeElement), true)
      assert.equal(await page.evaluate(() => document.body.style.overflow), '', 'fallback must not lock scrolling')
      await fallback.getByRole('button', { name: '关闭提示' }).click()
      assert.equal(await imageButton.evaluate(el => el === document.activeElement), true)
    }
    await page.evaluate(() => { HTMLDialogElement.prototype.showModal = window.originalShowModal })
    await imageButton.click()
    await page.goto(`${base}/posts`)
    assert.equal(await page.evaluate(() => document.body.style.overflow), '')
    assert.deepEqual(errors, [])
    console.log('PASS: reading copy whitespace/rejection, responsive TOC, image keyboard/modal/focus/cleanup')
  }
  finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
