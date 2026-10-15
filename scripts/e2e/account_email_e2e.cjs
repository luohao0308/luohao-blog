const assert = require('node:assert/strict')
const { chromium } = require('playwright')
const base = process.env.BLOG_E2E_BASE_URL || 'http://127.0.0.1:13018'
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname)) throw new Error('local mock only')
const verifyToken = 'verify_token_abcdefghijklmnopqrstuvwxyz'
const resetToken = 'reset_token_abcdefghijklmnopqrstuvwxyz'

async function install(page, state) {
  await page.route('**/api/v1/auth/refresh', route => route.fulfill(state.authenticated
    ? { status: 200, contentType: 'application/json', body: JSON.stringify({ access_token: 'reader-token', token_type: 'Bearer', expires_in: 900, user: { id: 'reader', email: 'reader@example.com', display_name: '测试读者', role: 1, email_verified: state.verified } }) }
    : { status: 401, contentType: 'application/json', body: '{}' }))
  await page.route('**/api/v1/auth/me', (route) => { state.meCalls++; return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: 'reader', email: 'reader@example.com', display_name: '测试读者', role: 1, email_verified: false }) }) })
  await page.route('**/api/v1/auth/email-verification/request', (route) => {
    state.verifyRequests++
    if (state.verifyRequestMode === 'success') return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    const cooldown = state.verifyRequestMode === 'cooldown'
    return route.fulfill({ status: cooldown ? 429 : 503, contentType: 'application/json', body: JSON.stringify({ reason: cooldown ? 'AUTH_EMAIL_REQUEST_COOLDOWN' : 'AUTH_MAIL_NOT_CONFIGURED' }) })
  })
  await page.route('**/api/v1/auth/email-verification/confirm', async (route) => {
    state.verifyBodies.push(route.request().postDataJSON())
    if (state.verifyConfirmMode === 'unavailable') return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ reason: 'AUTH_EMAIL_TOKEN_STORE_UNAVAILABLE' }) })
    if (state.verifyConfirmMode === 'invalid') return route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ reason: 'AUTH_EMAIL_VERIFICATION_TOKEN_INVALID' }) })
    state.verified = true
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
  await page.route('**/api/v1/auth/password-reset/request', async (route) => {
    state.resetRequestEmails.push(route.request().postDataJSON().email)
    return route.fulfill({ status: state.resetRequestMode === 'cooldown' ? 429 : 200, contentType: 'application/json', body: state.resetRequestMode === 'cooldown' ? JSON.stringify({ reason: 'AUTH_EMAIL_TOO_MANY_ATTEMPTS' }) : '{}' })
  })
  await page.route('**/api/v1/auth/password-reset/confirm', async (route) => {
    state.resetBodies.push(route.request().postDataJSON())
    if (state.resetConfirmMode === 'unavailable') return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ reason: 'AUTH_EMAIL_TOKEN_STORE_UNAVAILABLE' }) })
    if (state.resetConfirmMode === 'invalid') return route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ reason: 'AUTH_PASSWORD_RESET_TOKEN_INVALID' }) })
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
  await page.route('**/api/v1/auth/logout', (route) => { state.logoutCalls++; return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }) })
  await page.route('**/api/v1/articles/list**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ articles: [] }) }))
  await page.route('**/api/v1/auth/update-password', (route) => { state.passwordChanges++; return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }) })
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BLOG_E2E_CHROMIUM || undefined })
  try {
    const state = { authenticated: false, verified: false, meCalls: 0, verifyRequests: 0, verifyRequestMode: 'not-configured', verifyConfirmMode: 'unavailable', verifyBodies: [], resetRequestMode: 'success', resetRequestEmails: [], resetConfirmMode: 'unavailable', resetBodies: [], logoutCalls: 0, passwordChanges: 0 }
    const context = await browser.newContext({ locale: 'zh-CN' })
    const page = await context.newPage()
    await install(page, state)

    await page.goto(`${base}/login`)
    await page.getByRole('link', { name: '忘记密码？' }).click()
    await page.getByPlaceholder('邮箱').fill('known@example.com')
    await page.getByRole('button', { name: '提交找回请求' }).click()
    const uniform = '请求已接收，若邮箱已注册且投递可用将收到链接；未收到可稍后重试。'
    try { await page.getByText(uniform, { exact: true }).waitFor({ timeout: 8000 }) }
    catch (error) {
      console.error('email request diagnostic', { url: page.url(), requestCount: state.resetRequestEmails.length, body: (await page.locator('body').innerText()).slice(0, 700) })
      throw error
    }
    await page.getByPlaceholder('邮箱').fill('unknown@example.com')
    await page.getByRole('button', { name: '提交找回请求' }).click()
    await page.getByText(uniform, { exact: true }).waitFor()
    assert.deepEqual(state.resetRequestEmails, ['known@example.com', 'unknown@example.com'])

    state.authenticated = true
    await page.goto(`${base}/settings`)
    await page.getByRole('region', { name: '邮箱验证' }).waitFor()
    await page.getByRole('button', { name: '发送验证请求' }).click()
    await page.getByText('邮件服务尚未配置，请稍后再试。', { exact: true }).waitFor()
    state.verifyRequestMode = 'cooldown'
    await page.getByRole('button', { name: '发送验证请求' }).click()
    await page.getByText('请求过于频繁，请稍后重试。', { exact: true }).waitFor()
    state.verifyRequestMode = 'success'
    await page.getByRole('button', { name: '发送验证请求' }).click()
    await page.getByText(/请求已接收；若邮件投递可用/).waitFor()

    await page.goto(`${base}/verify-email#token=${verifyToken}`)
    await page.waitForFunction(() => location.hash === '' && location.search === '')
    assert.ok(!(await page.content()).includes(verifyToken))
    assert.equal(await page.evaluate(token => Object.values(localStorage).includes(token), verifyToken), false)
    await page.getByRole('button', { name: '确认验证' }).click()
    await page.getByText('验证服务暂时不可用，请稍后重试。', { exact: true }).waitFor()
    state.verifyConfirmMode = 'invalid'
    await page.getByRole('button', { name: '确认验证' }).click()
    await page.getByText('验证链接无效、已过期或已经使用。', { exact: true }).waitFor()
    state.verifyConfirmMode = 'success'
    await page.getByRole('button', { name: '确认验证' }).click()
    await page.getByText('邮箱验证成功。', { exact: true }).waitFor()
    assert.equal(state.meCalls, 1, 'successful confirmation must re-read the current account instead of attributing the token')
    assert.deepEqual(state.verifyBodies.map(body => body.token), [verifyToken, verifyToken, verifyToken])
    await page.getByRole('link', { name: '继续' }).click()
    await page.getByRole('region', { name: '邮箱验证' }).getByText('待验证', { exact: true }).waitFor()

    await page.goto(`${base}/reset-password#token=${resetToken}`)
    await page.waitForFunction(() => location.hash === '' && location.search === '')
    assert.ok(!(await page.content()).includes(resetToken))
    await page.getByPlaceholder('新密码（至少 8 位）').fill('new-password-1')
    await page.getByPlaceholder('确认新密码').fill('different-password')
    await page.getByRole('button', { name: '确认重置' }).click()
    await page.getByText('两次输入的新密码不一致', { exact: true }).waitFor()
    await page.getByPlaceholder('确认新密码').fill('new-password-1')
    await page.getByRole('button', { name: '确认重置' }).click()
    await page.getByText('重置服务暂时不可用，请稍后重试。', { exact: true }).waitFor()
    state.resetConfirmMode = 'invalid'
    await page.getByRole('button', { name: '确认重置' }).click()
    await page.getByText('重置链接无效、已过期或已经使用，请重新申请。', { exact: true }).waitFor()
    state.resetConfirmMode = 'success'
    await page.getByRole('button', { name: '确认重置' }).click()
    await page.getByText('新密码已生效。其他设备会在当前登录到期后退出；默认设置下最长约 15 分钟。', { exact: true }).waitFor()
    assert.equal(state.logoutCalls, 0, 'public reset must not logout another signed-in account')
    assert.deepEqual(state.resetBodies.map(body => body.token), [resetToken, resetToken, resetToken])

    for (const path of ['/forgot-password', '/reset-password', '/verify-email']) {
      const html = await (await context.request.get(base + path)).text()
      assert.match(html, /name="robots"[^>]*content="[^"]*noindex/)
    }
    await page.goto(`${base}/admin/posts`)
    await page.getByRole('link', { name: '修改密码', exact: true }).click({ timeout: 5000 })
    await page.waitForURL('**/settings#password')
    await page.getByPlaceholder('当前密码', { exact: true }).fill('old-password-1')
    await page.getByPlaceholder('新密码（至少 8 位）', { exact: true }).fill('new-password-2')
    await page.getByPlaceholder('确认新密码', { exact: true }).fill('new-password-2')
    await page.getByRole('button', { name: '修改密码', exact: true }).click()
    await page.waitForURL('**/login?password_changed=1')
    assert.equal(state.passwordChanges, 1)
    assert.equal(state.logoutCalls, 1, 'admin password entry must reuse settings and sign out only the current account')
    await context.close()
    console.log('Account email mock PASS: uniform recovery, verification status/retry, hash cleanup, invalid/replay recovery, reset confirmation, no unrelated logout, private SSR')
  }
  finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
