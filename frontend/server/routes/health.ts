// 容器健康探针：只验证 Nitro 进程活着，不触碰数据层——backend/ES 抖动时
// frontend 仍应 healthy，caddy 的 depends_on 才不会被绑架（2026-10 技术债）。
export default defineEventHandler(() => ({ status: 'ok' }))
