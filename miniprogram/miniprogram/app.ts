// 小程序入口。启动时接通登录态桥（401 → 刷新 → 重放）。
import { registerAuthBridge } from './utils/auth'

App({
  onLaunch() {
    registerAuthBridge()
  },
})
