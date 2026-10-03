# M2 账号后台实施计划

_状态：completed | 更新：2026-10-01 | 关联任务：T-004 | 关联设计：M1 计划（completed）_

## 1. 目标、成功标准与停止条件

- 目标结果：只有持有效凭证的作者能写文章——登录/刷新/登出闭环、写操作 RBAC 收紧、管理后台（文章管理与 Markdown 编辑器）、schema 迁移治理上线。
- 可验收成功标准：未登录调用写接口返回 401；登录后可创建/编辑/发布文章；access token 过期后 refresh 闭环；管理后台浏览器端完整走通发布流程；迁移以版本化 SQL 落库并可重复执行。
- 完成后停止条件：不做评论（M3）、不做 ES/embedding（M4）、不做部署变更（M5）。

## 2. 范围与非范围

### 范围

- backend：users 表与迁移基线、密码哈希（argon2id）、认证 API（login/refresh/logout）、JWT 签发与 Kratos 中间件、RBAC 收紧写接口、公开注册关闭（作者账号由 seed/CLI 创建）
- frontend：/admin 登录页、文章管理列表（状态流转按钮）、Markdown 编辑器（textarea + 预览）、token 存储与自动刷新
- 契约：user.proto + article.proto 权限注解更新

### 非范围

- 评论、阅读量、多用户注册开放、OAuth/第三方登录、双因素、Caddy/部署变更

## 3. 当前证据基线

- 代码/配置：M1 完成基线（main `52a2fb7`）；article CRUD 无鉴权；ent auto_migrate 开启；Redis/MySQL 容器可用
- 测试/CI：backend/frontend 全绿流程已稳定（五个 PR 验证）
- 契约/数据：`articles`/`tags` 表已由 auto_migrate 建成；无 users 表
- 运行事实：全栈冒烟通过（SSR 页面含渲染 HTML）
- Unknown：kratos v3 JWT 中间件生态细节（S2 内验证，必要时自写中间件）

## 4. 规模判定与用户确认

- 规模：large
- 触发信号：跨 backend/frontend + 认证（安全边界）+ 四个有序切片
- 确认状态：approved
- 用户确认时间或消息指针：2026-09-30 对话批准 S1–S4；决策选择 golang-migrate / casbin / 富文本编辑器（首次选择作废后重问确认）
- Issue：以 TASKS.md T-004 代替

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | 用户模型 + 迁移治理 | migrations 基线+users（golang-migrate）、ent User schema、biz user、data repo、单测 | M1 | 单测 + 迁移可重复执行 + 基线比对 | main `52a2fb7` | completed + **merged**（PR #6，merge `6a2cbfe`） |
| S2 | 认证 API + JWT 中间件 | auth.proto、login/refresh/logout、JWT 中间件、Redis 会话、登录限流 | S1 | 冒烟：login→受保护探针→refresh→logout | S1 合并点 | completed + **merged**（PR #8，merge `06d97a7`） |
| S3 | RBAC 收紧写接口 | article 写操作接 admin 中间件、契约权限注解、未授权 401/403 用例 | S2 | 冒烟：无 token 401、作者 token 可写、单测 | S2 合并点 | implementation + unit + HTTP smoke verified; delivery pending |
| S4 | 管理后台前端 | /admin 登录页、文章管理+编辑器、token 刷新闭环 | S3 | build + 全栈浏览器级冒烟（curl 级 HTML 断言） | S3 合并点 | completed + **merged**（[PR #11](https://github.com/luohao0308/luohao-blog/pull/11)，squash merge `2c4983a`） |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 密码哈希 | argon2id（golang.org/x/crypto） | 当前抗 GPU 破刻标准 | 无 |
| token 模型 | access JWT（15min，内存态）+ refresh token（Redis 存储、httpOnly cookie、7d 滑动） | 大厂标准；刷新可撤销 | 实现 CRUD 于会话 |
| RBAC | **casbin**：enforcer 挂 Kratos 中间件，策略（sub=role, obj=endpoint, act=method）显式可评审 | 用户指定；策略外置后多角色/资源规则可演进 | 两角色系统引入策略引擎的维护成本 |
| 迁移治理 | **golang-migrate**：SQL 迁移文件进 Git（embed 进二进制、启动时幂等 up）；停用 ent auto_migrate | 用户指定；SQL 显式可评审、面试通用 | **双源风险**：ent schema 与迁移 SQL 需人工对齐，靠迁移演练 + 基线比对缓解 |
| 编辑器 | **富文本 Markdown 编辑器**（Milkdown 为首选，markdown 原生；S4 实现时如 SSR/集成受阻降级 Tiptap+tiptap-markdown） | 用户指定；写作体验优先 | 依赖重、需客户端挂载、SSR 适配成本 |
| 公开注册 | 关闭；作者账号由 seed 命令创建 | 个人博客不需要开放注册 | 需要一个 seed 入口 |

默认开发方法：安全边界变更触发负向用例（无 token/过期 token/伪造 claim）；其余按 M1 轻量闭环。

## 6. 实施切片

### S1：用户模型 + 迁移治理

- 状态：pending
- 修改范围：`backend/migrations/`（新）、ent User schema、`internal/biz/user.go`、`internal/data/user.go`、`internal/data/data.go`（停 auto_migrate）、Makefile
- 步骤：atlas 接入并生成当前 schema 基线 → 新增 users 迁移 → ent User schema + 重建 → biz（密码哈希、邮箱/密码校验）/repo → 单测 → 迁移在真库演练（up/down/up）
- 切片验收：迁移幂等可重放；单测全绿；auto_migrate 已停用且启动正常
- 回退点：main `52a2fb7`

### S2：认证 API + JWT 中间件

- 状态：completed + **merged**（PR #8，merge `06d97a7`，2026-09-30）
- 修改范围：`backend/api/blog/v1/auth.proto`、`error_reason.proto`（USER_*/AUTH_* reason）、`internal/service/auth.go`、`internal/biz/auth.go`、`internal/data/auth.go`（JWT/Redis 会话/限流器）、`internal/server/auth.go`（自研 JWT 中间件）、`internal/conf/conf.proto`（Auth 节）、`cmd/seed/`、`scripts/smoke-auth.sh`
- 步骤执行记录：auth 契约（Login/Refresh/Logout/GetMe）→ argon2 校验（S1 已有）→ JWT 签发（HS256，golang-jwt/v5，claim sub/role/exp，alg 钉死防混淆）→ refresh 会话（Redis GETDEL 原子旋转 + 重放拒绝，httpOnly cookie + X-Refresh-Token header 双通道）→ 自研中间件（kratos v3 无 auth 中间件，按计划降级路径执行；受保护操作清单 + 过期/无效 reason 区分 + 公开接口坏 token 匿名放行）→ seed 建号（先幂等迁移）→ 限流（Redis 固定窗口 10 次/5min/IP，fail-open）→ 冒烟
- 切片验收：冒烟 `backend/scripts/smoke-auth.sh` **30/30 通过**（含：错误密码/未知邮箱同 reason 401、无 token/伪造 401、有效 token 200、refresh 旋转+重放 401、logout 吊销+清 cookie、文章读公开、限流 429、他 IP 不受影响）；过期 token 由单测覆盖（真 JWT 过期路径 + alg-confusion/无 exp 拒绝）；空密钥启动 fail-fast 实测
- 回退点：S1 合并点（未使用）

### S3：RBAC 收紧写接口

- 状态：implementation + unit + HTTP smoke verified; delivery pending
- 修改范围：article service 写方法接 admin 断言、契约注解、测试
- 步骤：写接口（create/update/delete）挂 admin 中间件 → 读保持公开 → 未授权/越权用例 → 契约文档更新
- 切片验收：无 token 401、非 admin 403、admin 可写；现有测试补负向路径
- 回退点：S2 合并点

### S4：管理后台前端

- 状态：completed（implementation + unit + 全栈 curl/浏览器冒烟 verified; delivery pending）
- 修改范围：`frontend/app/pages/admin/`（login、posts/index、posts/new、posts/[slug]/edit）、`app/layouts/admin.vue`、`app/composables/useAuth.ts`、`app/middleware/admin-auth.ts`、`app/components/admin/`（ArticleForm、MarkdownEditor）、`frontend/server/api/[...path].ts`、`frontend/nuxt.config.ts`、**后端** `internal/service/article.go` + `article_test.go`（共享缺陷修复，见 §7）
- 步骤执行记录：auth composable（内存 access token + 401 拦截刷新重试 + refresh cookie 会话恢复，无 body POST 显式 JSON Content-Type）→ admin-auth 路由守卫（client 端 ensureSession 后重定向 login?redirect=）→ admin 布局（NConfigProvider 中文 locale + 跟随站点暗色）→ 登录页（401/429 区分提示）→ 文章管理表格（NDataTable，状态筛选/发布/转草稿/删除确认）→ Milkdown Crepe 富文本编辑器（按 §5 决策，动态 import 客户端挂载，`#`/`**` 所见即所得；未触发降级路径）→ 编辑页（字段回填、slug 锁定、update_mask=title,summary,content_md,tags,status）→ BFF cookiePathRewrite → build + 全栈冒烟
- 切片验收：`pnpm lint/typecheck/build` 全绿；curl 冒烟（经 BFF :3000）：login 200 + Set-Cookie `Path=/api/v1/auth` 重写 ✓、创建 200（强制 DRAFT）、公开列表不含草稿 ✓、`update_mask=status` 发布 200 + published_at 盖章 ✓、公开可见 + SSR HTML 断言 ✓、refresh cookie 旋转 + 旧 token 重放 401 ✓、删除后公开 404 ✓；浏览器 GUI 冒烟（web-gui-tester，10 张截图入 `gui-test-screenshots/`）：错误密码提示、登录跳转、Milkdown 输入渲染、创建/发布/删除、守卫 redirect 全部通过
- 回退点：S3 合并点

## 7. 偏移控制

- 当前允许修改的切片范围：各切片列出的路径；跨切片共享前置（如统一 401/403 错误码）记入本节
- 跨切片共享前置修改（S4 期间发现并修复）：
  1. **后端 update 接口自 M1/S2 起不可用**：`service.convertArticle` 不透传 status，biz 层拒绝 UNSPECIFIED 导致任何 update 恒 400（此前冒烟只覆盖 create/read/delete，未暴露）。修复：convertArticle 透传 status（create 仍由 biz 强制 DRAFT），新增 `service/article_test.go` 锁定行为
  2. **BFF refresh cookie 路径重写**：后端契约 `Path=/v1/auth` 在 `/api` 代理前缀下永不匹配，`server/api/[...path].ts` 增加 `cookiePathRewrite: {'/v1/auth': '/api/v1/auth'}`，保住 httpOnly 主通道，后端契约不变
  3. **无 body POST 需要 JSON Content-Type**：Kratos 对未声明 Content-Type 的请求返回 400 CODEC，前端 refresh/logout POST 显式带 `application/json`
  4. **nuxtjs-naive-ui 模块不注册组件**（仅 SSR 样式收集）：admin 页面 naive-ui 组件全部改为 SFC 显式导入
  5. **Nuxt 嵌套路由**：`posts.vue` + `posts/new.vue` 形成父布局嵌套导致子页不渲染，列表页移至 `posts/index.vue`（NUXT_E4011 印证）
- 需要重新确认的变化：范围、顺序、接口、迁移或风险发生实质变化
- 不需要重新确认的变化：已确认切片内部的一般实现细节调整（编辑器按 §5 决策执行 Milkdown，§6 旧文 textarea 描述作废）

## 8. 契约、迁移与发布

- 兼容策略：auth 为新增契约；article 写接口新增鉴权是**破坏性变更**（M1 期间无外部消费者，可接受；PR 描述标注）
- 数据迁移/回填：Atlas 版本化 SQL 进 `backend/migrations/`；users 表全新；现有 articles/tags 不动
- 发布顺序：S1→S2→S3→S4 各自独立 PR
- 回滚/恢复：revert PR；迁移回滚按 down 文件；refresh 会话可全量吊销（Redis flush 按需）

## 9. 测试与验证矩阵

| 层级 | 要证明的声明/场景 | Test/Eval/Check | 命令/入口 | 通过条件 |
|---|---|---|---|---|
| 单元 | 密码哈希往返、slug/邮箱校验、JWT 签发/过期/伪造、会话旋转 | Test | `go test ./internal/...` | 全绿 |
| 集成/契约 | login/refresh/logout 全链路；RBAC 401/403 | Test + 冒烟 | curl 脚本 | 断言通过 |
| 迁移 | 基线 + users 迁移可重复执行 | 冒烟演练 | atlas migrate apply ×2 | 幂等 |
| E2E/冒烟 | 登录→创建→发布→前台可见 | SSR HTML 断言 | preview 冒烟 | 断言通过 |
| 安全 | 无 token/过期/伪造/限流触发 | 负向用例 | 冒烟 | 全部拒绝 |

## 10. 风险与缓解

| 风险 | 概率/影响 | 早期信号 | 缓解/恢复 |
|---|---|---|---|
| Atlas 与 ent 版本兼容 | 中/中 | 生成报错 | 兜底方案：golang-migrate 手写 SQL（决策降级路径） |
| JWT 中间件与 kratos v3 API 不匹配 | 中/中 | 编译/路由失败 | 自写极简中间件（claims 断言 + 白名单） |
| 刷新令牌旋转的并发竞态 | 低/高 | 单测重放用例 | Redis 原子 GETDEL + 旧 token 宽限窗口 |
| 迁移与 auto_migrate 双轨残留 | 中/中 | 启动日志出现 DDL | S1 即停用 auto_migrate，配置项移除 |

## 11. 交付状态与 PR 证据

- 当前状态：S1 **merged**（PR #6，merge `6a2cbfe`）；S2 **merged**（PR #8，merge `06d97a7`，2026-09-30）；S3 **merged**（PR #9，merge `b2af83a`，2026-09-30）；S4 **merged**（[PR #11](https://github.com/luohao0308/luohao-blog/pull/11)，head `71cb60b0e57ec8dbf68c0fd5657f8fc0a816ec31`，squash merge `2c4983aac1bc8e340d86baf8b2b621d89c143458`，2026-10-01）
- repo / remote：luohao-blog / origin
- PR 编号或链接：S4 = [PR #11](https://github.com/luohao0308/luohao-blog/pull/11)（S2 = [PR #8](https://github.com/luohao0308/luohao-blog/pull/8)，S1 见 PR #6）
- source ref / target ref：S4 = `feat/m2-s4-admin-frontend` → `main`
- exact head SHA：S4 = `71cb60b0e57ec8dbf68c0fd5657f8fc0a816ec31`
- required CI 结果与时间：S4 两 job pass（run 36754607113，2026-10-01，merge 前实时核验）
- S3 PR：[#9](https://github.com/luohao0308/luohao-blog/pull/9)，head `e0a955b1ba4ae62a1a6dc92bfb7d094eb783d0cd`，CI run `36743731504` 两 job pass；squash merge `b2af83ab56b28f33b5fa7e92ec16997da027ab30`。
- 独立 Review 门禁：用户于 2026-09-30 要求移除；manifest 已关闭该门禁，GitHub `main` protection 已移除 required approving review。
- merge commit：S1 = `6a2cbfe`；S2 = `06d97a7a16de0b90801fedff74de43c51b997df0`；S4 = `2c4983aac1bc8e340d86baf8b2b621d89c143458`
- guard 证据：S4 push/PR/merge 三次 `--consume` 全 allow（grantId m2s4-{push,pr,merge}-20261001，授权与平台证据存 `.dev-workflow/authorizations/`）

## 12. 文档同步

- [x] `TASKS.md` / 上下文（2026-10-01）
- [x] `PROJECT-SUMMARY.md`（状态、技术画像、模块、决策表已更新）
- [x] 架构/ADR（ADR-0001 认证 token 模型、ADR-0002 迁移治理已在 `docs/architecture/DECISIONS.md` 落档，2026-09-30；本次核验后勾选）
- [x] 设计/契约/生成物（无契约变更；§7 共享前置修复已记录）
- [x] Runbook/工作日志（GUI 冒烟截图 `gui-test-screenshots/`，不入库）

## 13. 完成定义

- [x] 大型计划已获得用户确认并记录切片版本。
- [x] 所有切片验收通过，且过程状态按顺序更新（S1–S4 全部 merged）。
- [x] 适用测试、构建、迁移、重启和冒烟通过。
- [x] 契约、文档和长期知识已同步（ADR 两条待补，见 §12）。
- [x] 最终证据、SHA/产物身份和剩余风险已记录。
- [x] 如已进入远端交付，PR 和 CI 证据完整；merge 经 fail-closed 门禁授权。
