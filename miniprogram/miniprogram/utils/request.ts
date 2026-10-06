import { API_BASE } from './config'

type Query = Record<string, string | number | undefined>

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

function request<T>(method: 'GET' | 'POST', path: string, query?: Query, data?: unknown): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    wx.request({
      url: buildUrl(path, query),
      method,
      data: data as string | AnyObject | undefined,
      timeout: 15000,
      success(res) {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data as T)
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

export const http = {
  get<T>(path: string, query?: Query): Promise<T> {
    return request<T>('GET', path, query)
  },
  post<T>(path: string, data?: unknown): Promise<T> {
    return request<T>('POST', path, undefined, data)
  },
}

// toMessage 把 catch 到的未知错误归一成可展示文本。
export function toMessage(error: unknown): string {
  return error instanceof Error ? error.message : '请求失败，请稍后重试'
}
