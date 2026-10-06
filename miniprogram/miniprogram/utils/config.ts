// 后端唯一入口：走与 Web 端相同的 BFF 反代路径。
// 开发期用生产 IP（DevTools 已关合法域名校验）；makerhao.cn 备案通过后
// 整体切换为 https://makerhao.cn/api/v1，只需改这一行。
export const API_BASE = 'http://193.112.128.245/api/v1'
