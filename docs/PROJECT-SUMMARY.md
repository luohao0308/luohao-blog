# 项目摘要（AI 快速参考）

_来源：仓库实际内容与最近验证命令输出（build/test/lint/compose/认证冒烟）_
_状态：M0–M5 已交付上线（HTTPS 待域名）；两轮全量 review 完成，修复见 PR #28 起 | 更新：2026-10-04_

> 本文件只保存稳定的项目事实和路径速查，不记录实时任务、临时验证、交接过程或历史证据。当前任务状态以 `TASKS.md` 为准，短期上下文以 `WORKING-CONTEXT.md` 为准。

## 1. 项目概览

个人技术博客，内容方向为全栈开发与 AI Agent。Go（Kratos）后端 + Vue 3（Nuxt）前端的单仓库项目，**已上线 http://193.112.128.245**。M0–M5 全部交付：内容核心、账号后台（JWT+RBAC）、互动统计、ES 混合搜索 + RAG 问答、生产部署（caddy→BFF→backend+mysql/redis/es 六容器）、发布流水线（ghcr）、备份+看门狗。里程碑见根 `README.md`。

## 2. 仓库拓扑与所有权

| 路径 | 类型 | 责任/用途 | 独立 Git 仓库 | 变更边界 |
|---|---|---|---|---|
| `backend/` | Go 服务 | Kratos v2 单体，proto 契约源，模块 `github.com/luohao0308/luohao-blog/backend` | 否 | 分层契约见 `backend/AGENTS.md` |
| `frontend/` | Nuxt 应用 | Vue 3 + Nuxt 4，SSR/SSG，pnpm 11 | 否 | 前端目录内自洽 |
| `deploy/` | 基础设施 | 本地依赖编排（compose）+ 生产反代模板（Caddy，M5 启用） | 否 | 端口/卷变更需同步文档 |
| `.github/` | CI | backend lint+build+test / frontend lint+typecheck+build | 否 | 质量门，慎重修改 |
| `AGENTS.md` / `docs/` | 流程文件 | dev-workflow 0.5.0（9 包，本机排除，不进 Git） | 否 | 规则变更需说明理由 |
| `.dev-workflow/` | 本机元数据 | manifest 与授权记录（Git exclude） | 否 | 仅本机 |

- 远端：`github.com/luohao0308/luohao-blog`（Public），默认分支 `main`；已配置 required CI、conversation resolution、管理员遵守规则、禁止 force push/删除分支；required approving review 数为 0（TASKS.md T-002）。
- 本机 gh CLI 已登录 `luohao0308`；dev-workflow 流程文件被 `.git/info/exclude` 受管排除，新 clone 需重新安装接入。

## 3. 技术与运行时画像

| 层/能力 | 当前方案 | 稳定约束 |
|---|---|---|
| 后端 | Go 1.26（go.mod 声明）/ Kratos v3 | 分层 service→biz→data，DTO/DO/PO 转换，禁跨层 import |
| 认证 | argon2id 密码哈希 + HS256 access JWT（15min）+ Redis refresh 会话（httpOnly cookie，7d 滑动旋转）+ 登录限流（Redis 固定窗口） | 注册关闭，账号只由 `cmd/seed` 创建；JWT 密钥经 `KRATOS_JWT_SECRET` 注入，空则拒绝启动 |
| 前端 | Node ≥20.19（本机 nvm 默认 24.18）+ pnpm 11.7 + Nuxt 4.5.2 + Tailwind CSS 4 + Naive UI 2.45（admin）+ Milkdown Crepe 7.22（编辑器） | `engines.node>=20.19`；pnpm 设置在 `frontend/pnpm-workspace.yaml`（allowBuilds）；admin 面板 naive-ui 组件为 SFC 显式导入（nuxtjs-naive-ui 仅做 SSR 样式收集） |
| 数据（未接入代码） | MySQL 8.4 · Redis 7.4 · ES 8.17.4 · MinIO（compose 提供本地实例） | ES 未来一件两用：BM25 + kNN；compose 账号密码为本地占位 |
| proto 工具链 | buf 1.73 + buf.gen.yaml（go/go-grpc/go-http/openapi 插件 `go run` 固定版本） | buf 模块根：`api`、`internal`；禁止手改生成物 |
| 本地运行 | `go run ./cmd/server -conf ./configs`（backend/）；`pnpm dev`（frontend/） | server 默认 conf 路径 `../../configs` 仅适用 cmd/server 目录 cwd |

## 4. 模块与领域

| 模块/领域 | 目录或入口 | 稳定用途 | 依赖/边界 | 备注 |
|---|---|---|---|---|
| HTTP/gRPC 装配 | `backend/internal/server/` | 服务注册与中间件 | 只依赖 service | |
| 传输适配 | `backend/internal/service/` | DTO↔DO 转换 | service→biz，禁触 data | auth 服务负责 refresh cookie/Header 读写 |
| 业务用例 | `backend/internal/biz/` | DO、usecase、repo 接口 | 禁触 DTO/PO | 依赖倒置接缝；auth.go 声明 TokenIssuer/SessionRepo/RateLimiter 接口 |
| 数据访问 | `backend/internal/data/` | repo 实现、存储客户端、ent | data→biz | `auth.go`：JWT 签发/校验、Redis 会话、限流器 |
| 配置 | `backend/internal/conf/` `backend/configs/` | 配置 proto 与 yaml | 无凭据入库 | `make config` 生成 |
| API 契约 | `backend/api/<domain>/<version>/` | proto 源 + 生成物 | 唯一对外契约 | M1 起按领域新增 |
| 页面 | `frontend/app/pages/` | 首页/归档/关于、文章详情/列表/标签、作品集列表/详情 | 布局 `app/layouts/` | 文章搜索、标签筛选、相邻文章；项目技术栈筛选 |
| 管理后台 | `frontend/app/pages/admin/` + `layouts/admin.vue` | 登录、文章管理表格（状态流转）、Milkdown 编辑器 new/edit | `composables/useAuth.ts` + `middleware/admin-auth.ts` | access token 内存态 + refresh cookie（BFF `cookiePathRewrite` 适配）；admin 带 token 列表可见草稿 |

## 5. 开发、验证与交付入口

| 目的 | 命令或入口 | 适用范围 | 证据/备注 |
|---|---|---|---|
| 后端全量验证 | `go build ./... && go test ./... && golangci-lint run` | backend/ | 2026-09-29 全部通过 |
| proto 生成 | `make api`（api/）/ `make config`（conf） | backend/ | buf 插件走 `go run` 固定版本 |
| 后端启动 | `KRATOS_JWT_SECRET=<密钥> go run ./cmd/server -conf ./configs` | backend/ | 空密钥 fail-fast；conf 默认值 `../../configs` 仅适用 cmd/server cwd |
| 创建作者账号 | `go run ./cmd/seed -conf ./configs -email <email> -password <pw> -name <名>` | backend/ | 唯一建号入口；先幂等执行迁移 |
| 演示文章 seed | `go run ./cmd/seed -conf ./configs -demo-articles`（`-reset` 覆写） | backend/ | 内嵌 markdown（`cmd/seed/demoarticles/`）；幂等补缺，默认不覆盖后台编辑 |
| 认证冒烟 | `./scripts/smoke-auth.sh` | backend/ | 需 compose 依赖服务 + jq；覆盖 login/me/refresh/logout/负向/限流 |
| 前端验证 | `pnpm lint && pnpm typecheck && pnpm build` | frontend/ | 2026-09-29 全部通过 |
| 前端启动 | `pnpm dev` | frontend/ | :3000 |
| 依赖服务 | `docker compose -f deploy/docker-compose.yml up -d` | 仓库根 | `config -q` 已校验；启动冒烟 M1 做 |
| CI | `.github/workflows/ci.yml` | 仓库根 | backend / frontend 两个 job |

## 6. 契约、数据和运行边界

- 对外契约权威源：`backend/api/**/*.proto`（buf 生成 HTTP + gRPC + openapi.yaml）
- 生成物与人工指南：`make api` / `make config`；禁止手改 `*.pb.go` 与 `wire_gen.go`
- Schema/数据变更门禁：M1 引入 golang-migrate 后回填（规划：版本化迁移 + 前向兼容）
- 发布与运行边界：部署目标未定（Unknown）；Caddy 模板在 `deploy/caddy/`
- 敏感信息边界：凭据、Token、私钥永不入库；`.env*` 已被根 .gitignore 忽略；compose 占位口令不得用于任何真实环境

## 7. 技术决策

| 决策 | 当前方案 | 原因/证据 | 影响范围 | 状态 |
|---|---|---|---|---|
| 后端语言/框架 | Go + Kratos v2 | M0 验证：layout 可构建、proto 工具链齐 | 全部后端 | active |
| 前端框架 | Vue 3 + Nuxt 4.5 + Tailwind 4 | M0 验证：SSR 构建通过、typecheck 通过 | 全部前端 | active |
| 架构风格 | 模块化单体 | 个人项目维护成本 | 全部 | active |
| 向量检索 | ES 8 kNN（不引入独立向量库） | M4 落地时验证 | Agent 模块 | planned |
| 内容管道 | MySQL 存储 + 发布事件异步建索引/embedding | M1/M4 落地时验证 | 文章/Agent 模块 | planned |
| proto 生成 | buf（多模块根 api/internal）+ go run 固定插件版本 | M0 验证：两处生成均通过 | backend | active |
| 认证 token 模型 | access JWT（HS256，15min，无状态）+ refresh token（不透明随机串，Redis 会话，httpOnly cookie，7d 滑动旋转，GETDEL 防重放） | M2/S2 冒烟与单测验证；刷新可撤销、登录可限流 | backend auth/S3 RBAC/S4 前端 | active |
| RBAC | casbin（enforcer 挂 HTTP/gRPC 中间件） | S3 已实现；公开文章仅列已发布内容，admin 可管理草稿 | backend auth/article | active |
| 管理后台编辑器 | Milkdown Crepe（所见即所得 Markdown） | S4 冒烟验证：`#`/`**` 实时渲染、markdownUpdated 回传 source；SSR 受阻时降级路径为 Tiptap+tiptap-markdown（未触发） | frontend admin | active |
| BFF refresh cookie | `cookiePathRewrite {'/v1/auth': '/api/v1/auth'}` | S4 冒烟验证：后端契约不变，浏览器经 `/api/v1/auth/*` 自动携带 httpOnly cookie | frontend server/api | active |

## 8. 已知风险与技术债

| 项目 | 风险等级 | 当前影响 | 触发条件 | 后续方向 |
|---|---|---|---|---|
| 本机存在旧 Go 1.17.2（/usr/local/go） | 低 | shell PATH 顺序若先命中旧 go，构建会失败 | 用户交互 shell PATH | 建议删除 /usr/local/go 或确保 brew go 优先 |
| PR review 策略 | 已配置 | main 启用 required CI、conversation resolution、禁止 force push/删除分支；required review 数为 0 | 远端协作 | TASKS.md T-002 |
| layout 自带 todo/helloworld demo | 低 | 与业务无关的演示代码 | M1 已替换契约；生成物残留清理 | 已无行为影响 |
| 登录限流信任 X-Forwarded-For | 低 | 直连时可伪造头绕过每 IP 限流 | 恶意客户端 | 部署后改信任代理链或换身份键；当前威胁模型（个人博客）可接受 |
| conf.proto go_package 仍为 kratos-layout 模板路径 | 低 | 生成物 import 声明与模块名不一致（不影响编译） | conf 变更时 | 后续统一改为本模块路径 |

## 9. 路径速查

| 需要查找的内容 | 路径 |
|---|---|
| 项目规则 | `AGENTS.md`（根）；`backend/AGENTS.md`（分层契约） |
| 后端入口 | `backend/cmd/server/main.go` |
| 前端入口 | `frontend/app/app.vue` |
| 测试入口 | `go test ./...`（backend）；`pnpm typecheck`（frontend） |
| 配置说明 | `backend/configs/config.yaml`（无敏感值） |
| 架构文档 | `docs/architecture/` |
| 设计文档 | `docs/design/`、`DESIGN.md` |
| 本机/Agent 长期记忆 | `docs/project-memory/`（本机忽略，不是团队事实源） |
| 团队共享 Runbook | `docs/operations/runbooks/` |
| 一次性部署证据 | CI/部署平台 artifacts 或受控日志 |
| 任务状态 | `docs/TASKS.md` |
| 当前任务上下文 | `docs/WORKING-CONTEXT.md` |

---

_更新方式：只在稳定事实、仓库拓扑、模块边界、技术决策、命令或路径发生变化并完成验证后更新。_
