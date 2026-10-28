const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const path = require('node:path')
const { chromium } = require('playwright')

async function main() {
  const base = process.env.BLOG_LAYOUT_BASE || 'http://127.0.0.1:13035'
  assert.ok(['127.0.0.1', 'makerhao.cn'].includes(new URL(base).hostname))
  const out = path.resolve(process.env.BLOG_E2E_OUTPUT || 'output/about-focus')
  await fs.mkdir(out, { recursive: true })
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM })
  const results = []
  try {
    for (const [width, motion, color, js] of [
      [1440, 'no-preference', 'light', true], [390, 'no-preference', 'light', true],
      [1440, 'no-preference', 'dark', true], [390, 'reduce', 'light', true],
      [1440, 'reduce', 'light', false],
    ]) {
      const page = await browser.newPage({ viewport: { width, height: 900 }, reducedMotion: motion, javaScriptEnabled: js })
      const errors = []; page.on('pageerror', e => errors.push(e.message))
      if (color === 'dark') await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
      assert.equal((await page.goto(base + '/about')).status(), 200)
      const root = page.locator('.about-focus-story')
      await root.waitFor()
      const items = root.locator('[data-reveal]')
      const count = await items.count(); assert.ok(count >= 26)
      if (js && motion === 'no-preference') {
        await page.waitForFunction(() => {
          const h = document.querySelector('.about-focus-story h1')
          return h?.dataset.animation === 'focus' && Number(getComputedStyle(h).opacity) > 0 && Number(getComputedStyle(h).opacity) < 1 && getComputedStyle(h).filter !== 'none'
        })
        for (let i = 0; i < count; i++) {
          await items.nth(i).evaluate(n => n.scrollIntoView({ block: 'center', behavior: 'instant' }))
          await page.waitForFunction(index => document.querySelectorAll('.about-focus-story [data-reveal]')[index]?.dataset.animation === 'focus', i)
        }
        await page.waitForFunction(() => [...document.querySelectorAll('.about-focus-story [data-reveal]')].every(n => n.dataset.revealed === 'true'))
      }
      assert.equal(await page.locator('.journey-thread,.journey-cursor,.journey-camera,.journey-controls,.chapter-label').count(), 0)
      const layout = await root.evaluate(el => ({
        height: el.offsetHeight,
        sections: [...el.querySelectorAll('section')].map(n => ({ top: n.getBoundingClientRect().top, bottom: n.getBoundingClientRect().bottom, padding: parseFloat(getComputedStyle(n).paddingTop) })),
        allVisible: [...el.querySelectorAll('[data-reveal]')].every(n => Number(getComputedStyle(n).opacity) === 1),
        overflow: document.documentElement.scrollWidth > innerWidth + 1,
      }))
      assert.ok(layout.allVisible); assert.ok(!layout.overflow)
      assert.ok(layout.sections.every(s => s.padding <= 48))
      for (let i = 1; i < layout.sections.length; i++) assert.ok(layout.sections[i].top >= layout.sections[i - 1].bottom - 1)
      const github = root.getByRole('link', { name: /访问 GitHub/ })
      assert.equal(await github.getAttribute('href'), 'https://github.com/luohao0308')
      assert.equal(await root.getByRole('link', { name: /给我发邮件/ }).getAttribute('href'), 'mailto:2429260713@qq.com')
      assert.equal(await root.locator('.contact-button').count(), 2)
      assert.ok(await github.evaluate(n => n.offsetHeight >= 100 && getComputedStyle(n).borderRadius === '16px'))
      assert.equal(await root.locator('.selected-group a').count(), 4)
      await github.scrollIntoViewIfNeeded()
      await page.screenshot({ path: path.join(out, `${width}-${motion}-${color}-${js}.png`) })
      assert.deepEqual(errors, [])
      results.push({ width, motion, color, js, textBlocks: count, layout, result: 'PASS' })
      await page.close()
    }
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
    for (const route of ['/posts', '/projects']) {
      await page.goto(base + route); await page.locator('.media-feed').waitFor()
      assert.equal(await page.locator('main').evaluate(n => n.getBoundingClientRect().width), 1024)
      assert.equal(await page.locator('.media-feed').evaluate(n => getComputedStyle(n).gridTemplateColumns.split(' ').length), 4)
    }
    await page.goto(base + '/about')
    const email = page.locator('.contact-button[href^="mailto:"]')
    await email.focus()
    assert.equal(await email.evaluate(n => getComputedStyle(n).opacity), '1')
    await page.close()
    await fs.writeFile(path.join(out, 'results.json'), JSON.stringify(results, null, 2) + '\n')
    console.log('PASS: focus reveal, every text block, compact layout, original contact buttons/links, real content, dark/mobile/no-JS/reduced-motion and unchanged article/portfolio widths')
  } finally { await browser.close() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
