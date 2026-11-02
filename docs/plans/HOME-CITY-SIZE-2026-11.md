# 首页模型城尺寸修复

日期：2026-11-02。用户要求修复城市展示缩小并上线。当前状态：本地实现/验证完成，交付与部署门禁待满足。

## 最小改动

删除首页对HomeVisual的 `max-width:310px; min-height:230px` 覆盖。继续使用组件原有 `width:100%; min-height:320px; aspect-ratio:1.3`，手机保留440px上限；首页840px内容宽度、文案、卡片、城市绘制及交互不变。无API、后端、依赖、数据库或小程序改动。

## 验收

新增独立浏览器回归先在旧实现复现：1440px视口右栏383.8px，但城市宽310px、高238.45px，撑满右栏断言失败。仅移除覆盖后，城市宽383.8px、高320px；实际绘制占画布宽约93%。

- `frontend/`：`pnpm lint`、`pnpm typecheck`、`pnpm build` 全通过；`node --test tests/*.test.cjs` 82项通过。
- 生产构建浏览器专项：320/390/640/768/1024/1440px × 明暗12组、无横向溢出、画布内容占比、键盘太阳交互、断点重排和动画通过；桌面/手机截图已目视检查。
- 脚本及命令见[scripts/e2e说明](../../scripts/e2e/README.md)。原始尺寸结果、截图及日志只保存在本机 `output/home-city-size/`，不提交。

## 交付与恢复边界

从只读核实的main f72e778建立独立 `fix/home-city-size` 分支，保留原目录和其他工作树的用户改动。上线只应更新前端；旧前端固定镜像是恢复点，后端/数据层/微信配置及所有用户数据保持。不得为本修复重跑内容seed、迁移或重建数据服务；恢复同样需准确范围授权。

远端push、PR、merge仍须使用原项目manifest、delivery_guard、准确SHA的一次性授权和新鲜平台证据。CI通过才可合并，不能直接推main；具体SHA由本切片提交记录确定。

本轮只读部署入口检查 `deployment_execute.py status` 返回executionAvailable=false、trusted-host-injection-required；当前未配置可执行可信宿主，不能绕过核心规则改用SSH/Compose更新生产。已完成服务器只读基线采集，未修改生产。旧交付历史不等于本轮执行能力或授权。当前先准备可审查提交，待准确交付授权及可用的受信部署通道后继续，不能声称已上线。
