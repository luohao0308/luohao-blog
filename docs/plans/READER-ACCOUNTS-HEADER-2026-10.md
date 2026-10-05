# 前台账号体系与头部改版 实施计划

_状态：approved | 更新：2026-10-05 | 关联任务：T-010 | 关联设计：无独立设计文档，设计决定见第 5 节_

## 1. 目标、成功标准与停止条件

- 目标结果：博客前台支持访客注册/登录（READER 账号），头部展示登录态与头像（超管可跳后台），用户可上传头像、改昵称、改密码，评论要求登录且身份取自账号。
- 可验收成功标准：
  - 未登录访客可注册（邮箱+密码+昵称）并自动登录；重复邮箱 409、弱密码 400。
  - 登录后头部显示头像下拉（个人设置/进入后台[仅超管]/退出），刷新页面会话可恢复。
  - 用户可上传头像（≤2MB，jpeg/png/webp）并在头部/评论区生效；可改昵称与密码。
  - 未登录提交评论 401；登录后评论身份取自账号，先审后显审核流不变；旧匿名评论正常展示。
  - 头部导航 8 项收拢为 5 项（分类/标签/归档进「文章」下拉），logo 保持左侧。
- 完成后停止条件：S1–S4 全部验收合并且生产部署随后续发布流程生效；不扩展做后台账号管理界面。

## 2. 范围与非范围

### 范围

- `backend/`：auth/user proto 契约、注册/资料/头像 API、`users.avatar_url` 迁移、头像磁盘存储与静态服务、注册限流、契约与单元测试。
- `frontend/`：头部布局与 UserMenu、`/login`、`/register`、`/settings` 页面、公开布局会话恢复、评论区登录门禁。
- `deploy/compose.prod.yml`：backend 头像目录命名卷；`deploy/backup.sh` 补头像目录备份。

### 非范围

- 邮箱验证 / SMTP 发信（订阅发信另行立项）。
- 后台读者账号管理界面（邀请制管理页）。
- 移动端汉堡菜单（移动端导航维持平铺收拢后条目）。
- 旧匿名评论数据回填 user_id（保留 NULL，展示不受影响）。
- 头像裁剪、多尺寸生成。

## 3. 当前证据基线

- 代码/配置：注册关闭（`biz/user.go` 仅 seed 建号）；`UserRoleReader=2` 已预留未使用；评论为匿名昵称制先审后显（`comment.proto`）；头部为 logo 左 + 8 导航项右（`layouts/default.vue`）；useAuth 客户端恢复会话已具备（refresh cookie + 单飞刷新）。
- 测试/CI：`go build/vet/test`（SQLite 内存库）+ `pnpm lint/typecheck/build` 为 required CI；Playwright E2E 16 用例与本改动交集为评论区（S4 需同步更新）。
- 契约/数据：migrations 000001–000007；ent schema 与手写 SQL 双源（已知漂移风险，两处必须同步改）。
- 运行事实：生产六容器（caddy/frontend/backend/mysql/redis/es），**无 MinIO**；dev compose 的 MinIO 从未被后端使用；本机当前无 docker 守护进程，集成冒烟能力受限。
- Unknown：BFF 对大 JSON body（头像 base64）的透传无既有先例，S1 冒烟验证；Kratos 静态文件挂载方式以 `server/http.go` 实际结构为准。

## 4. 规模判定与用户确认

- 规模：large
- 触发信号：跨 backend/frontend/deploy 三边界；四个有序切片；多个独立可验收结果。
- 确认状态：approved
- 用户确认时间或消息指针：2026-10-05 会话，用户对切片计划回复「确认」；头部布局选「保持 logo 在左（导航收 5 项 + 最右头像）」，账号策略选「开放注册」。
- 用户调整：无（按原拆分批准）。
- Issue：未创建（个人项目按 manifest 流程要求走 PR + required CI，Issue 保持 manual + user）。

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | 开放注册 + 头像/资料 API + 磁盘存储 | backend、compose/backup | 无 | go build/vet/test + 冒烟脚本 | 单 PR revert；迁移 down.sql | completed |
| S2 | 头部改版 + 登录/注册页 + 会话恢复 | frontend | S1 | lint/typecheck/build + GUI 冒烟 | 单 PR revert | completed |
| S3 | /settings 个人设置（头像/昵称/密码） | frontend | S1 S2 | 构建 + 上传端到端冒烟 | 单 PR revert | pending |
| S4 | 评论登录门禁（前后端） | backend、frontend、E2E | S1 S2 | 契约测试 + E2E | 单 PR revert | pending |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 头像存储 | backend 本地磁盘卷（非 MinIO） | 生产无 MinIO 容器，2C4G 不宜加服务；头像量小 | 换对象存储需再迁移；磁盘卷需进备份 |
| 头像上传编码 | JSON body 内 base64（非 multipart） | 便于 protojson 契约测试与 BFF 透传；前端 canvas 压缩后 ≤ 数百 KB | 体积 +33%；服务端校验魔数兜底 |
| 会话恢复 | 公开布局客户端 refresh 恢复 | 复用现有 useAuth 机制，不做 SSR 会话探测 | SSR 首帧短暂显示未登录态（接受） |
| avatar_url 形态 | 存 `/v1/assets/avatars/<rand>.<ext>`，前端渲染时加 `/api` 前缀 | DB 不耦合 BFF 路径；后端静态路由自持 | 前端需要一个 assetUrl 辅助函数 |
| 昵称唯一性 | 不唯一，仅 1–32 字校验 | 博客场景无需唯一；避免注册摩擦 | 同名昵称在评论区可能重名 |
| 注册限流 | 复用登录限流器，独立 bucket，fail-open | 与既有限流模式一致 | 邮箱未验证，垃圾注册靠限流+先审后显兜底 |

## 6. 实施切片

### S1：后端账号基座

- 状态：in_progress
- 修改范围：`backend/api/blog/v1/{auth,user}.proto`、`internal/{biz,service,data,server}`、`migrations/000008_*`、ent User schema、`deploy/compose.prod.yml`、`deploy/backup.sh`
- 步骤：proto 契约（Register / UpdateProfile / UploadAvatar + User.avatar_url）→ buf + ent 生成 → 迁移 SQL → biz/service/data 实现 → 静态资源路由 → 测试 → compose 卷与备份。
- 切片验收：`go build ./... && go vet ./... && go test ./...` 全绿；新增契约测试覆盖注册成功/409/400、未登录改资料 401、头像魔数/大小校验、静态资源与路径穿越；docker 可用时跑真实栈冒烟，否则记录未验证项。
- 回退点：单 PR revert；迁移 000008 带.down.sql。

### S2：前台头部改版 + 登录/注册页

- 状态：completed（2026-10-05/06 交付）
- 修改范围：`layouts/default.vue`、新 UserMenu/NavDropdown 组件、`/login`、`/register`、`utils/assets.ts`、useAuth 扩展（register + avatar_url）
- 步骤：导航收拢（文章▾：全部/分类/标签/归档）→ UserMenu（登录按钮/头像下拉）→ 公开布局 onMounted 恢复会话 → 两个新页面。
- 切片验收：✅ `pnpm lint && pnpm typecheck && pnpm build` 全绿；✅ GUI 冒烟（IAB 浏览器，无后端预览）：登录/注册页渲染（暗色+亮色）、文章下拉展开与 opaque 覆盖、菜单项导航 /posts、头部登录链接导航、注册表单空提交与短密码校验提示、主题切换——证据 /tmp/s2-shots/t1–t6；⛔ 无后端受阻项：真实注册/登录成功流、头像显示、超管进入后台入口、会话恢复——待 docker 栈或生产部署后补测。
- 回退点：单 PR revert。

### S3：个人设置页

- 状态：completed（2026-10-06 交付）
- 修改范围：`/settings` 页面（头像上传预览、昵称、密码）、UserMenu 增「个人设置」入口、useAuth 增 `setUser`
- 步骤：表单 → canvas 压缩（512px，webp 优先/源格式回退）→ base64 上传。
- 切片验收：✅ `pnpm lint && pnpm typecheck && pnpm build` 全绿；✅ GUI 冒烟：未登录访问 /settings 客户端守卫重定向 `/login?redirect=/settings`（证据 /tmp/s2-shots/t7）；⛔ 无后端受阻：真实上传端到端（头像回显）、昵称/密码保存——docker 栈恢复或部署后补；IAB 不支持文件选择器，上传交互待真实环境验证。
- 回退点：单 PR revert。

### S4：评论登录门禁

- 状态：pending
- 修改范围：`comment.proto`（CreateComment 需认证 + user_id）、评论 data/biz/service、`CommentSection.vue`、E2E 用例
- 步骤：契约变更 → 后端门禁与身份落库 → 前端登录引导/身份展示 → E2E 更新。
- 切片验收：未登录 401 契约测试；E2E 发评→待审→审核展示；旧评论展示回归。
- 回退点：单 PR revert。

## 7. 偏移控制

- 当前允许修改的切片范围：S1（backend + deploy 卷/备份）。
- 跨切片共享前置修改：无（S2 起另开分支）。
- 需要重新确认的变化：范围、顺序、接口、迁移或风险发生实质变化（如头像存储改 MinIO、注册加邮箱验证）。
- 不需要重新确认的变化：切片内部实现细节（如静态路由挂载方式、字段校验阈值微调）。

## 8. 契约、迁移与发布

- 兼容策略：新增 RPC 与字段均为加法；`User.avatar_url` 新增不破坏既有消费者；S4 的 CreateComment 收紧认证属破坏性契约变更，随片在契约注释与本文件记录。
- 数据迁移/回填：000008 `users.avatar_url VARCHAR(512) NOT NULL DEFAULT ''`；可空列无回填。
- 发布顺序：S1–S4 各自 PR merge 后随下一次生产部署生效；compose 卷需在部署含头像写的版本前就位。
- 回滚/恢复：迁移 down.sql；头像文件丢失仅表现为头像回退占位，无数据损坏。

## 9. 测试与验证矩阵

| 层级 | 要证明的声明/场景 | Test/Eval/Check | 命令/入口 | 通过条件 |
|---|---|---|---|---|
| 单元 | 注册字段校验/头像魔数与大小/昵称规则 | go test | `go test ./internal/biz/...` | ✅ 2026-10-05 全部通过（含 Register 节流/冲突/签名、avatar 拒收与守卫、sniff 表） |
| 集成/契约 | Register 200/409/400；未登录 profile 401；静态资源 200 与穿越拒绝 | go test（httptest 等价：policy 全路由覆盖 + repo SQLite） | `go test ./internal/server/... ./internal/data/...` | ✅ 2026-10-05 通过（TestPolicyCoversAllRoutes 钉死新路由策略行） |
| E2E/冒烟 | 真实栈注册→登录→传头像（docker 可用时） | smoke 脚本 | `backend/scripts/smoke-account.sh` | ⏳ 脚本已就绪；本机无 docker 守护进程，未跑——记录为未验证项，docker 恢复或部署前补跑 |
| 观测/部署 | compose 卷与备份覆盖 uploads | 人工核对 + compose config | `docker compose -f deploy/compose.prod.yml config` | ⏳ bash -n 通过；compose config 待 docker；备份/恢复脚本改动待服务器下次部署验证 |

## 10. 风险与缓解

| 风险 | 概率/影响 | 早期信号 | 缓解/恢复 |
|---|---|---|---|
| ent 与手写 SQL 漂移（已知技术债） | 中/中 | 测试或 MySQL 实跑列不匹配 | 同一提交内同步改两处并在 PR 描述标注 |
| base64 大 body 被 BFF/框架限制截断 | 中/低 | 上传 400/413 | S1 冒烟用真实大小文件验证；必要时下调前端压缩尺寸 |
| 垃圾注册 | 中/低 | accounts 表异常增长 | IP 限流 fail-open；评论先审后显；后续接 SMTP 加验证 |
| 本机无 docker，集成冒烟受限 | 高/低 | compose 起不来 | SQLite 契约测试为主证据，实栈冒烟延后到部署前 |
| 静态路由路径穿越 | 低/高 | 安全 review | 仅服务固定目录、随机文件名、拒绝 `..`、固定 Content-Type |

## 11. 交付状态与 PR 证据

- 当前状态：merged（S1 + S2；2026-10-05/06 用户逐次授权，guard 全程 consume allow）
- repo / remote：github.com/luohao0308/luohao-blog / origin（https://github.com/luohao0308/luohao-blog.git）
- S1：PR [#52](https://github.com/luohao0308/luohao-blog/pull/52)，head `95c5cf7`，CI 双绿 run 37275181768，squash merge `c59fa80`（首推 `87dcf90` CI 红系 gitignore 吞文件，修复后重授权推送）
- S2：PR [#53](https://github.com/luohao0308/luohao-blog/pull/53)，head `5cb7d89fd7a6a0846e4097267be8cd27f0da304f`，CI 双绿 run 37339094786，squash merge `e72b6faabf9603e78b5bf7368a0116aaba58dc56`
- 独立 reviewer 与批准时间：不适用（manifest 未启用独立 Review）
- S3/S4：交付时按切片续记

## 12. 文档同步

- [x] `TASKS.md`（T-010 立项）
- [ ] `PROJECT-SUMMARY.md`（全部切片完成后补账号体系事实）
- [ ] 架构/ADR（头像存储决策已记录于本文件第 5 节，暂不单开 ADR）
- [ ] 契约/生成物（proto/openapi 随片更新）
- [ ] Runbook/工作日志（部署时补 uploads 卷说明）

## 13. 完成定义

- [ ] 大型计划已获得用户确认并记录切片版本（2026-10-05 approved）。
- [ ] 所有切片验收通过，且过程状态按顺序更新。
- [ ] 适用测试、构建、迁移、重启和冒烟通过。
- [ ] 契约、文档和长期知识已同步。
- [ ] 最终证据、SHA/产物身份和剩余风险已记录。
- [ ] 如已进入远端交付，PR、CI 和独立 Review 证据完整；merge 只发生在 fail-closed 门禁通过后。
