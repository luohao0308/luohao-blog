# M5 上线部署实施计划

_状态：awaiting_user_confirmation | 更新：2026-10-02 | 关联任务：M5（待立项）_

## 1. 目标、成功标准与停止条件

- 目标结果：博客在公网域名下可访问（HTTPS），内容/搜索/问答全功能可用，有发布流水线和备份。
- 可验收成功标准：浏览器打开 `https://<域名>` 走完「阅读 → 搜索 → 登录 → 发布 → 问答」；服务器重启后服务自恢复；备份文件可恢复。
- 完成后停止条件：不做 CDN/多节点/监控告警平台等扩展；不做对象存储业务功能（MinIO 仍闲置）。

## 2. 现状与缺口（2026-10-02 盘点）

| 资产 | 状态 |
|---|---|
| backend/Dockerfile | ✅ 已有（golang builder → debian slim，配置经 /data/conf 挂载） |
| frontend Dockerfile | ❌ 缺 |
| 生产 compose（app + 依赖 + 反代） | ❌ 缺（现有 compose 仅本地开发依赖） |
| Caddyfile | 注释模板（/api/* → :8000，其余 → :3000） |
| frontend runtime config | ✅ `NUXT_BACKEND_BASE` 可覆盖 |
| 生产密钥管理 | ❌ 缺（JWT/DB/ES/API keys） |
| 发布流水线 | ❌ 缺（CI 仅 lint/build/test） |
| 备份 | ❌ 缺 |

## 3. 切片拆分（2-6 片，确认后生效）

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 |
|---|---|---|---|---|---|
| S1 本地生产化 | 一条命令在本机跑起「生产形态」全栈 | frontend Dockerfile、`deploy/compose.prod.yml`（caddy+frontend+backend+mysql+redis+es）、Caddyfile 启用、生产 secrets 模板、备份脚本 | 无（纯本地） | compose 全服务 healthy；公网形态冒烟（阅读/搜索/问答/登录/发布）；备份→恢复演练 | 纯新增文件 + 少量配置，git revert 即回退 | ✅ merged via PR #23（merge `a975bfc`，required CI 两项通过，2026-10-02）。本地验证全通过：双镜像构建；六容器 healthy；Caddy :8080 全功能冒烟（页面/列表/登录/发布/公开读/混合搜索/真实 DeepSeek 问答 1.3s）；备份→drop 库→恢复演练通过；容器 restart 策略自恢复顺带验证。踩坑记录：Dockerfile golang 1.25→1.26（go.mod 要求）；pnpm-workspace.yaml 未进 pnpm install 前的 COPY 层导致 allowBuilds 失效（musl 平台二进制回退）；本机 80 端口被自带 Apache 占用（HTTP_PORT 参数化，8080 冒烟）；compose 端口默认值语法 ${VAR:-default} |
| S2 服务器首次上线 | 公网全功能可用 | ~~域名/HTTPS~~（用户选择纯 IP）＋ Docker 已装 → clone + 密钥文件 + compose up --build + seed + reindex | 用户指定服务器 193.112.128.245（S1 完成） | 公网全功能冒烟通过（含真实 DeepSeek 问答）；restart 自恢复由容器策略保障 | compose down；密钥可轮换 | ✅ completed（2026-10-02，IP 直访 http://193.112.128.245 ；HTTPS 需域名后启用） |
| S3 发布流水线 | 合并 main → 自动构建镜像 → 服务器拉取更新 | GitHub Actions build/push 镜像（ghcr）、服务器发布脚本、发布/回滚 Runbook | S2 | 一次合并触发发布；回滚演练 | 镜像 tag 回滚 |
| S4 运维基线 | 数据可恢复、故障可感知 | MySQL 备份 cron（本机 + 可选远端）、探活、日志约定 | S2 | 备份恢复演练通过；宕机可发现 | 关闭 cron 即回退 |

## 4. 关键决策与未知（阻塞 S2，不阻塞 S1）

1. 服务器：已有还是待购？地域（国内访问速度 vs 备案要求——国内云 + .cn/备案 vs 海外免备案）？
2. 域名：已有还是待购？DNS 可控？
3. 服务器操作执行方式：AI 准备脚本与文档、用户亲手执行（推荐，生产操作权限边界清晰）；或 AI 经 SSH 直接执行（需提供访问方式并逐次授权）
4. 镜像仓库：ghcr（公开仓库默认可用）

## 5. 风险与缓解

| 风险 | 缓解 |
|---|---|
| 生产密钥泄露 | secrets 只进服务器文件（同本地 secrets 模式），不入 Git；出错即轮换 |
| ES 单机内存压力（512MB heap + MySQL + app 同机） | 2C4G 起步的 VPS；实测后调优 |
| 国内云 Docker Hub 拉取受限 | 镜像走 ghcr 或配置可用镜像源（MinIO 教训已记录） |
| 数据丢失 | S1 起内建备份脚本，S4 上 cron |

## 6. 偏移与确认

- 本计划确认前不修改产品代码、不创建交付 PR、不触碰任何服务器/域名。
- 范围、顺序或执行方式实质变化时重新确认。
