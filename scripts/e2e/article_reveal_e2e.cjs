// Real local BFF/SSR, read-only fixture, and client-only search outcomes.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const { startFixture, makeArticle } = require('./ui_fixture.cjs')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13000'
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname)) throw new Error('Local frontend only')
async function main() {
  const articles = Array.from({ length: 8 }, (_, index) => makeArticle({ id: String(index + 1), slug: `list-${index}`, title: `列表文章 ${index}`, summary: '可检索的真实fixture内容' }))
  const fixture = await startFixture({ articles })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  const errors = []
  const checks = []
  try {
    for (const reducedMotion of ['no-preference', 'reduce']) {
      const context = await browser.newContext({ viewport: { width: 390, height: 844 }, reducedMotion })
      await context.route('**/api/v1/auth/**', route => route.fulfill({ json: { access_token: 'fixture', user: { id: '2', role: 2, display_name: '测试读者' } } }))
      await context.addInitScript(() => {
        window.__articleRevealStarts = 0
        const original = Element.prototype.animate
        Element.prototype.animate = function (...args) {
          if (this.hasAttribute('data-article-reveal')) window.__articleRevealStarts++
          return original.apply(this, args)
        }
      })
      const page = await context.newPage()
      page.setDefaultTimeout(7000)
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (/hydration/i.test(message.text())) errors.push(message.text()) })
      await page.goto(`${base}/posts`, { waitUntil: 'networkidle' })
      assert.equal(await page.locator('[data-article-reveal]').count(), 8)
      const expected = reducedMotion === 'reduce' ? 0 : 6
      assert.equal(await page.evaluate(() => window.__articleRevealStarts), expected, 'list animation is bounded and respects preference')
      await page.waitForFunction(() => document.getAnimations().every(animation => !animation.effect?.target?.hasAttribute('data-article-reveal')))
      await context.route('**/api/v1/search/articles?**', route => route.fulfill({ json: { articles: [articles[5], articles[1]], search_outcome: 1 } }))
      await page.getByLabel('搜索文章', { exact: true }).fill('semantic-only-query')
      await page.waitForFunction(() => document.querySelectorAll('[data-article-reveal]').length === 2)
      assert.deepEqual((await page.locator('[data-article-reveal] h2').allTextContents()).map(title => title.trim()), ['列表文章 5', '列表文章 1'])
      assert.equal(await page.evaluate(() => window.__articleRevealStarts), expected, 'semantic updates never replay the entry')
      await page.getByLabel('搜索文章', { exact: true }).fill('')
      await page.waitForFunction(() => document.querySelectorAll('[data-article-reveal]').length === 8)
      assert.equal(await page.evaluate(() => window.__articleRevealStarts), expected)
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      checks.push(`${reducedMotion}: bounded entry, ranked results, clear query, no replay, no overflow`)
      await context.close()
    }
    assert.deepEqual(errors, [])
    const output = path.resolve('output/visual-redesign/implementation')
    fs.mkdirSync(output, { recursive: true })
    fs.writeFileSync(path.join(output, 'article-reveal.json'), JSON.stringify({ checks, errors }, null, 2))
    console.log('PASS', checks)
  } finally { await browser.close(); await fixture.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
