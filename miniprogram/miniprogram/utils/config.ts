// 后端唯一入口：走与 Web 端相同的 BFF 反代路径。
export const API_BASE = 'https://makerhao.cn/api/v1'

// assetUrl 把后端下发的站内资源路径（/v1/assets/...）拼成小程序可直接
// 加载的完整地址：走与 API 同一条 BFF 反代链路。
export function assetUrl(path?: string): string {
  if (!path) return ''
  return API_BASE.replace(/\/v1$/, '') + path
}
