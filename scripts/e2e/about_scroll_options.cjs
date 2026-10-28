const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const { chromium } = require('playwright')
async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM })
  const results = []
  try {
    for (const width of [1440, 390]) {
      for (const effect of ['float', 'focus', 'wipe']) {
        const page = await browser.newPage({ viewport: { width, height: 900 }, reducedMotion: 'no-preference' })
        const errors = []; page.on('pageerror', error => errors.push(error.message))
        await page.goto(`http://127.0.0.1:13035/preview/about-scroll?effect=${effect}`)
        await page.locator('.scroll-demo h1').waitFor()
        await page.waitForFunction(() => { const h = document.querySelector('.scroll-demo h1'); const o = Number(getComputedStyle(h).opacity); return o > 0 && o < 1 && h.getAnimations().length > 0 })
        const items = page.locator('.scroll-demo [data-reveal]')
        const count = await items.count()
        assert.ok(count >= 20)
        for (let i = 0; i < count; i++) {
          await items.nth(i).evaluate(node => node.scrollIntoView({ block: 'center', behavior: 'instant' }))
          await page.waitForFunction(index => {
            const node = document.querySelectorAll('.scroll-demo [data-reveal]')[index]
            return Boolean(node.dataset.animation)
          }, i)
          assert.equal(await items.nth(i).getAttribute('data-animation'), effect)
        }
        await page.waitForTimeout(1600)
        assert.equal(await page.locator('.scroll-demo [data-reveal]:not([data-revealed="true"])').count(), 0)
        assert.equal(await page.locator('.journey-thread,.journey-cursor,.journey-controls').count(), 0)
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
        const sequence = await page.locator('.scroll-demo section').evaluateAll(sections => sections.map(section => ({ top: section.getBoundingClientRect().top, bottom: section.getBoundingClientRect().bottom })))
        for (let i = 1; i < sequence.length; i++) assert.ok(sequence[i].top >= sequence[i - 1].bottom - 1, 'sections overlap')
        await page.getByRole('button', { name: '重新播放' }).click()
        await page.waitForFunction(() => scrollY === 0)
        await page.waitForTimeout(1700)
        await page.screenshot({ path: `output/scroll-options/${effect}-${width}.png`, fullPage: false })
        assert.deepEqual(errors, [])
        results.push({ effect, width, animatedTextBlocks: count, result: 'PASS' })
        await page.close()
      }
    }
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
    await page.goto('http://127.0.0.1:13035/preview/about-scroll?effect=focus')
    await page.locator('.scroll-demo').waitFor()
    await page.waitForFunction(() => [...document.querySelectorAll('.scroll-demo [data-reveal]')].every(node => node.dataset.revealed === 'true'))
    await page.getByRole('link', { name: /给我发邮件/ }).focus()
    assert.equal(await page.getByRole('link', { name: /给我发邮件/ }).evaluate(node => getComputedStyle(node).opacity), '1')
    await page.close()
    const switcher = await browser.newPage()
    await switcher.goto('http://127.0.0.1:13035/preview/about-scroll')
    await switcher.waitForFunction(() => document.querySelector('.scroll-demo h1')?.dataset.animation)
    for (const label of ['景深渐显', '遮罩揭示', '逐段上浮']) {
      await switcher.getByRole('button', { name: label, exact: true }).click()
      await switcher.getByRole('button', { name: label, exact: true, pressed: true }).waitFor()
    }
    await switcher.close()
    await fs.writeFile('output/scroll-options/results.json', JSON.stringify(results, null, 2) + '\n')
    console.log('PASS: 3 effects × desktop/mobile, every text block animated, no overlaps/guide line, replay/switching, keyboard and reduced motion')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
