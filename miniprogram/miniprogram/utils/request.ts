import { API_BASE } from './config'

type Query = Record<string, string | number | undefined>
type HeaderBag = Record<string, string>

// accessToken 由 auth 模块维护；登录后自动附加到所有请求。
export let accessToken = ''

export function setAccessToken(token: string) {
  accessToken = token
}

// unauthorizedHandler 由 auth 模块注册：401 → 刷新 → 重放原请求一次。
let unauthorizedHandler: (() => Promise<boolean>) | null = null

export function setUnauthorizedHandler(handler: () => Promise<boolean>) {
  unauthorizedHandler = handler
}

function buildUrl(path: string, query?: Query): string {
  const url = `${API_BASE}${path}`
  if (!query) return url
  const pairs = Object.entries(query)
    .filter(([, value]) => value !== undefined && value !== '')
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`)
  return pairs.length ? `${url}?${pairs.join('&')}` : url
}

// Kratos 错误体为 protojson 形状：{ code, message, details }。
function errorMessage(statusCode: number, data: unknown): string {
  const message = (data as { message?: string } | null)?.message
  return message || `请求失败（${statusCode}）`
}

// UnauthorizedError 标记 401 响应，触发一次刷新重试。
export class UnauthorizedError extends Error {}

function rawRequest<T>(method: 'GET' | 'POST', path: string, query?: Query, data?: unknown, extraHeaders?: HeaderBag, timeoutMs?: number): Promise<{ res: WechatMiniprogram.RequestSuccessCallbackResult, data: T }> {
  return new Promise((resolve, reject) => {
    const header: HeaderBag = { 'content-type': 'application/json' }
    if (accessToken) {
      header.Authorization = `Bearer ${accessToken}`
    }
    if (extraHeaders) {
      for (const [key, value] of Object.entries(extraHeaders)) {
        header[key] = value
      }
    }
    wx.request({
      url: buildUrl(path, query),
      method,
      header,
      data: data as string | AnyObject | undefined,
      timeout: timeoutMs ?? 15000,
      success(res) {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve({ res, data: res.data as T })
          return
        }
        if (res.statusCode === 401) {
          reject(new UnauthorizedError(errorMessage(res.statusCode, res.data)))
          return
        }
        reject(new Error(errorMessage(res.statusCode, res.data)))
      },
      fail() {
        reject(new Error('网络不可用，请稍后重试'))
      },
    })
  })
}

async function request<T>(method: 'GET' | 'POST', path: string, query?: Query, data?: unknown, timeoutMs?: number): Promise<T> {
  try {
    const { data: body } = await rawRequest<T>(method, path, query, data, undefined, timeoutMs)
    return body
  } catch (error) {
    if (error instanceof UnauthorizedError && unauthorizedHandler) {
      // 刷新成功后重放原请求一次；仍 401 则把错误抛给调用方。
      const retried = await unauthorizedHandler()
      if (retried) {
        const { data: body } = await rawRequest<T>(method, path, query, data, undefined, timeoutMs)
        return body
      }
    }
    throw error
  }
}

export const http = {
  get<T>(path: string, query?: Query): Promise<T> {
    return request<T>('GET', path, query)
  },
  post<T>(path: string, data?: unknown): Promise<T> {
    return request<T>('POST', path, undefined, data)
  },
  // postRaw 暴露完整响应（refresh token 经 Set-Cookie 下发）。
  postRaw<T>(path: string, data?: unknown, extraHeaders?: HeaderBag): Promise<{ res: WechatMiniprogram.RequestSuccessCallbackResult, data: T }> {
    return rawRequest<T>('POST', path, undefined, data, extraHeaders)
  },
  // postTimeout 长超时 POST（AI 问答这类慢端点用；其余请求保持 15s 默认）。
  postTimeout<T>(path: string, data?: unknown, timeoutMs?: number): Promise<T> {
    return request<T>('POST', path, undefined, data, timeoutMs)
  },
}

// toMessage 把 catch 到的未知错误归一成可展示文本。
export function toMessage(error: unknown): string {
  return error instanceof Error ? error.message : '请求失败，请稍后重试'
}
