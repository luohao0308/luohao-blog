import { assetUrl } from './config'
import { http, setAccessToken, setUnauthorizedHandler, toMessage } from './request'
import { WECHAT_STATUS_BINDING_REQUIRED, WECHAT_STATUS_OK, type Account, type LoginReply, type UserReply } from './types'

// 会话存取：access token 常驻内存 + storage，refresh token 单独存（header
// 通道）。后端 access 15 分钟、refresh 7 天滑动旋转；旋转后的新 refresh
// 经 Set-Cookie 下发，这里解析出来存好，供下一次 X-Refresh-Token 刷新。

const ACCESS_KEY = 'auth:access'
const REFRESH_KEY = 'auth:refresh'
const EXPIRE_KEY = 'auth:expireAt'
const ACCOUNT_KEY = 'auth:account'

function loadAccessToken(): string {
  try {
    return (wx.getStorageSync(ACCESS_KEY) as string) || ''
  } catch {
    return ''
  }
}

export function isLoggedIn(): boolean {
  const expireAt = Number(wx.getStorageSync(EXPIRE_KEY) || 0)
  return Boolean(loadAccessToken()) && Date.now() < expireAt - 30_000
}

// 同时兼容修复前缓存的 wire user；已转换的账号再次读取保持不变。
function toAccount(user: UserReply | Account): Account {
  return {
    id: user.id,
    email: user.email,
    displayName: ('display_name' in user ? user.display_name : user.displayName) || '',
    role: user.role,
    avatarUrl: assetUrl('avatar_url' in user ? user.avatar_url : 'avatarUrl' in user ? user.avatarUrl : ''),
  }
}

export function currentAccount(): Account | null {
  try {
    const stored = wx.getStorageSync(ACCOUNT_KEY) as UserReply | Account | undefined
    return stored ? toAccount(stored) : null
  } catch {
    return null
  }
}

function saveAccount(user: UserReply | Account): Account {
  const account = toAccount(user)
  try {
    wx.setStorageSync(ACCOUNT_KEY, account)
  } catch { /* storage 异常不阻断登录 */ }
  return account
}

function saveSession(reply: LoginReply, refreshToken?: string) {
  const account = toAccount(reply.user)
  setAccessToken(reply.access_token)
  try {
    wx.setStorageSync(ACCESS_KEY, reply.access_token)
    wx.setStorageSync(EXPIRE_KEY, Date.now() + Number(reply.expires_in) * 1000)
    if (refreshToken) {
      wx.setStorageSync(REFRESH_KEY, refreshToken)
    }
    saveAccount(account)
  } catch { /* ignore */ }
  return account
}

export function clearSession() {
  setAccessToken('')
  try {
    wx.removeStorageSync(ACCESS_KEY)
    wx.removeStorageSync(REFRESH_KEY)
    wx.removeStorageSync(EXPIRE_KEY)
    wx.removeStorageSync(ACCOUNT_KEY)
  } catch { /* ignore */ }
}

// extractRefreshCookie 从响应头（wx 会把多个 Set-Cookie 拼接）取 refresh_token。
function extractRefreshCookie(header: Record<string, string | string[] | undefined>): string {
  for (const key of Object.keys(header)) {
    if (key.toLowerCase() !== 'set-cookie') continue
    const raw = String(header[key])
    for (const part of raw.split(',')) {
      const match = /refresh_token=([^;\s]+)/.exec(part)
      if (match) return match[1]
    }
  }
  return ''
}

// refreshSession 单飞刷新一次；成功返回 true 并已保存新会话。
let refreshing: Promise<boolean> | null = null

export function refreshSession(): Promise<boolean> {
  if (refreshing) return refreshing
  refreshing = (async () => {
    const refresh = (wx.getStorageSync(REFRESH_KEY) as string) || ''
    if (!refresh) return false
    try {
      const { res, data } = await http.postRaw<LoginReply>('/auth/refresh', {}, { 'X-Refresh-Token': refresh })
      const newRefresh = extractRefreshCookie(res.header) || refresh
      saveSession(data, newRefresh)
      return true
    } catch {
      clearSession()
      return false
    }
  })()
  const done = refreshing
  const reset = () => { refreshing = null }
  done.then(reset, reset)
  return done
}

// registerAuthBridge 把刷新能力接进请求层：401 → 刷新 → 重放一次。
export function registerAuthBridge() {
  setAccessToken(loadAccessToken())
  setUnauthorizedHandler(refreshSession)
}

// wechatLogin 用 wx.login 的 code 走后端登录；返回账号或绑定票据。
export async function wechatLogin(): Promise<{ kind: 'ok', account: Account } | { kind: 'binding', ticket: string }> {
  const code = await new Promise<string>((resolve, reject) => {
    wx.login({
      success: (res) => (res.code ? resolve(res.code) : reject(new Error('微信登录失败，请重试'))),
      fail: () => reject(new Error('微信登录失败，请重试')),
    })
  })
  const { res, data: reply } = await http.postRaw<import('./types').WechatLoginReply>('/auth/wechat', { code })
  if (reply.status === WECHAT_STATUS_OK && reply.login) {
    const account = saveSession(reply.login, extractRefreshCookie(res.header))
    return { kind: 'ok', account }
  }
  if (reply.status === WECHAT_STATUS_BINDING_REQUIRED && reply.binding_ticket) {
    return { kind: 'binding', ticket: reply.binding_ticket }
  }
  throw new Error('微信登录返回异常，请重试')
}

// bindWechat 用票据把 openid 绑到已有账号并登录。
export async function bindWechat(ticket: string, email: string, password: string): Promise<Account> {
  const { res, data: reply } = await http.postRaw<LoginReply>('/auth/wechat/bind', {
    binding_ticket: ticket,
    email,
    password,
  })
  return saveSession(reply, extractRefreshCookie(res.header))
}

export async function fetchMe(): Promise<Account> {
  const user = await http.get<UserReply>('/auth/me')
  return saveAccount(user)
}

export async function logout(): Promise<void> {
  try {
    await http.post('/auth/logout', {})
  } catch (error) {
    // 后端登出幂等；网络失败也照常清本地态。
    console.warn('logout', toMessage(error))
  }
  clearSession()
}

export type { Account } from './types'
