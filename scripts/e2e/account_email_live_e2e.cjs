// Local-only real HTTP/BFF email lifecycle; fake mail capture, no real delivery.
const assert = require('node:assert/strict')
const { chromium } = require('playwright')

const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13019'
const target = new URL(base)
assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname) && !target.username && !target.password, 'only credential-free loopback URLs are allowed')
const secrets = new Set()
const originalPassword = 'fixtureOriginal12'
const nextPassword = 'fixtureReplacement34'
const identity = Date.now()
const emailA = `email-live-a-${identity}@example.com`
const emailB = `email-live-b-${identity}@example.com`
const emailC = `email-live-c-${identity}@example.com`

function safeError(error) {
  let result = String(error?.stack || error)
  for (const secret of secrets) if (secret) result = result.split(secret).join('[redacted]')
  return result
}
async function request(context, method, path, token = '', body, expected = 200) {
  const response = await context.request.fetch(base + path, {
    method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { data: body }),
  })
  assert.equal(response.status(), expected, `${method} ${path.split('?')[0]} status`)
  return response
}
async function register(context, email) {
  const reply = await (await request(context, 'POST', '/api/v1/auth/register', '', { email, password: originalPassword, display_name: '邮箱闭环读者' })).json()
  secrets.add(reply.access_token)
  assert.equal(Boolean(reply.user.email_verified), false)
  return reply.access_token
}
async function capturedLink(context, email, purpose) {
  for (let attempt = 0; attempt < 100; attempt++) {
    const response = await context.request.get(base + '/api/fixture/emails/latest', { params: { to: email, purpose } })
    if (response.ok()) {
      const captured = await response.json()
      if (captured.link) {
        const link = new URL(captured.link)
        const token = new URLSearchParams(link.hash.slice(1)).get('token') || ''
        secrets.add(token)
        assert.ok(/^[A-Za-z0-9_-]{43}$/.test(token), 'cryptorandom link token format')
        assert.ok(!link.search.includes('token'), 'token must only be in fragment')
        assert.ok(link.pathname === (purpose === 'verify' ? '/verify-email' : '/reset-password'), 'purpose-specific email link')
        assert.ok(link.protocol === 'https:' || (link.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(link.hostname)), 'trusted HTTPS/loopback link origin')
        return { token, local: base + link.pathname + link.hash }
      }
    }
    await new Promise(resolve => setTimeout(resolve, 50))
  }
  throw new Error('fake mail was not captured within five seconds')
}
async function enterLink(page, link, path) {
  await page.goto(link.local)
  await page.waitForFunction(() => !window.location.hash && !window.location.search)
  assert.ok(new URL(page.url()).pathname === path, 'fragment cleaned without leaving the recovery page')
  assert.equal((await page.locator('body').innerText()).includes(link.token), false, 'raw token must never appear in page text')
  assert.equal(await page.evaluate(token => JSON.stringify(localStorage).includes(token), link.token), false, 'raw token must not enter localStorage')
}

async function main() {
  const browser = await chromium.launch({ headless: true, ...(process.env.BLOG_E2E_CHROMIUM ? { executablePath: process.env.BLOG_E2E_CHROMIUM } : {}) })
  try {
    const accountA = await browser.newContext()
    const accountB = await browser.newContext()
    const accountC = await browser.newContext()
    const guest = await browser.newContext()
    const tokenA = await register(accountA, emailA)
    const tokenB = await register(accountB, emailB)
    const tokenC = await register(accountC, emailC)
    const pageA = await accountA.newPage()
    const pageB = await accountB.newPage()
    const pageGuest = await guest.newPage()
    const networkURLs = []
    for (const page of [pageA, pageB, pageGuest]) page.on('request', request => networkURLs.push(request.url()))

    await request(guest, 'POST', '/api/v1/auth/email-verification/request', '', {}, 401)
    await pageA.goto(base + '/settings')
    await pageA.getByRole('button', { name: '发送验证请求', exact: true }).waitFor()
    await pageA.getByRole('button', { name: '发送验证请求', exact: true }).click()
    await pageA.getByText(/请求已接收/).waitFor()
    const verify = await capturedLink(guest, emailA, 'verify')
    await request(accountA, 'POST', '/api/v1/auth/email-verification/request', tokenA, {}, 429)
    // Confirm A's mailbox while B is logged in. B's state must remain its own.
    await pageB.goto(base + '/settings')
    await pageB.getByRole('button', { name: '发送验证请求', exact: true }).waitFor()
    await enterLink(pageB, verify, '/verify-email')
    await pageB.getByRole('button', { name: '确认验证', exact: true }).click()
    await pageB.getByText('邮箱验证成功。', { exact: true }).waitFor()
    await request(guest, 'POST', '/api/v1/auth/email-verification/confirm', '', { token: verify.token }, 400)
    const meA = await (await request(accountA, 'GET', '/api/v1/auth/me', tokenA)).json()
    const meB = await (await request(accountB, 'GET', '/api/v1/auth/me', tokenB)).json()
    assert.equal(meA.email_verified, true)
    assert.equal(Boolean(meB.email_verified), false)
    await pageB.getByRole('link', { name: '继续', exact: true }).click()
    await pageB.getByRole('button', { name: '发送验证请求', exact: true }).waitFor()
    await pageA.reload()
    await pageA.getByText('已验证', { exact: true }).waitFor()

    const oldDevice = await browser.newContext()
    const oldLogin = await (await request(oldDevice, 'POST', '/api/v1/auth/login', '', { email: emailA, password: originalPassword })).json()
    secrets.add(oldLogin.access_token)
    for (const cookie of await oldDevice.cookies()) if (cookie.name === 'refresh_token') secrets.add(cookie.value)

    await pageGuest.goto(base + '/forgot-password')
    await pageGuest.getByPlaceholder('邮箱', { exact: true }).fill(emailA)
    await pageGuest.getByRole('button', { name: '提交找回请求', exact: true }).click()
    await pageGuest.getByText(/请求已接收，若邮箱已注册且投递可用将收到链接/).waitFor()
    const reset = await capturedLink(guest, emailA, 'reset')
    await request(guest, 'POST', '/api/v1/auth/email-verification/confirm', '', { token: reset.token }, 400)
    await request(guest, 'POST', '/api/v1/auth/password-reset/confirm', '', { token: reset.token, new_password: 'short' }, 400)
    await enterLink(pageGuest, reset, '/reset-password')
    await pageGuest.getByPlaceholder('新密码（至少 8 位）', { exact: true }).fill(nextPassword)
    await pageGuest.getByPlaceholder('确认新密码', { exact: true }).fill(nextPassword)
    await pageGuest.getByRole('button', { name: '确认重置', exact: true }).click()
    await pageGuest.getByText(/新密码已生效/).waitFor()
    await pageGuest.getByText(/默认设置下最长约 15 分钟/).waitFor()
    await request(guest, 'POST', '/api/v1/auth/password-reset/confirm', '', { token: reset.token, new_password: nextPassword }, 400)
    await request(oldDevice, 'POST', '/api/v1/auth/refresh', '', {}, 401)
    await request(guest, 'POST', '/api/v1/auth/login', '', { email: emailA, password: originalPassword }, 401)
    const newLogin = await (await request(guest, 'POST', '/api/v1/auth/login', '', { email: emailA, password: nextPassword })).json()
    secrets.add(newLogin.access_token)
    assert.equal(newLogin.user.email_verified, true)
    const unaffected = await (await request(accountB, 'POST', '/api/v1/auth/refresh', '', {})).json()
    secrets.add(unaffected.access_token)
    assert.equal(unaffected.user.email, emailB)
    assert.equal(Boolean(unaffected.user.email_verified), false)

    // Reset alone proves control of a previously unverified mailbox.
    const known = await (await request(guest, 'POST', '/api/v1/auth/password-reset/request', '', { email: emailC })).text()
    const unknown = await (await request(guest, 'POST', '/api/v1/auth/password-reset/request', '', { email: `unknown-${identity}@example.com` })).text()
    assert.equal(known, unknown, 'existing/non-existing accounts must share the response')
    const resetC = await capturedLink(guest, emailC, 'reset')
    await request(guest, 'POST', '/api/v1/auth/password-reset/confirm', '', { token: resetC.token, new_password: nextPassword })
    assert.equal((await (await request(accountC, 'GET', '/api/v1/auth/me', tokenC)).json()).email_verified, true)
    await request(accountC, 'POST', '/api/v1/auth/refresh', '', {}, 401)

    for (const token of secrets) if (/^[A-Za-z0-9_-]{43}$/.test(token)) assert.ok(networkURLs.every(url => !url.includes(token)), 'mail token must never enter network URL logs')
    for (const path of ['/forgot-password', '/reset-password', '/verify-email']) {
      const html = await (await request(guest, 'GET', path)).text()
      assert.match(html, /name="robots"[^>]*content="[^"]*noindex/)
    }
    await oldDevice.close()
    await accountA.close()
    await accountB.close()
    await accountC.close()
    await guest.close()
    console.log('Account email live PASS: real BFF auth/SQL/fake mail, verification, fragment secrecy, foreign-account isolation, reset single-use/purpose/weak-password, password login, refresh revocation, unverified reset proof, generic response, private noindex')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(safeError(error)); process.exitCode = 1 })
