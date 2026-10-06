// 后端唯一入口：走与 Web 端相同的 BFF 反代路径。
// 开发期用生产 IP（DevTools 已关合法域名校验）；makerhao.cn 备案通过后
// 整体切换为 https://makerhao.cn/api/v1，只需改这一行。
export const API_BASE = 'http://193.112.128.245/api/v1'

// assetUrl 把后端下发的站内资源路径（/v1/assets/...）拼成小程序可直接
// 加载的完整地址：走与 API 同一条 BFF 反代链路。
export function assetUrl(path?: string): string {
  if (!path) return ''
  return API_BASE.replace(/\/v1$/, '') + path
}
