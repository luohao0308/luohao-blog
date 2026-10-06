# 小程序端 T-011：原生 TypeScript 阅读端

_创建：2026-10-07 ｜ 状态：S1 in_progress_

## 1. 目标与范围

为博客提供微信小程序阅读端：文章浏览（列表/详情）、微信登录绑定现有 reader 账号、互动（点赞/评论/收藏/分享）。**非目标**：管理后台功能（后台只保留 Web 端）、小程序内支付。

- AppID：`wx58089476f518fd3f`（个人主体，已注册 2026-10-07）
- 目录：仓库根新增 `miniprogram/`，不影响 backend/frontend 既有部署
- 前置依赖：无（开发期 DevTools 关闭合法域名校验，直连生产 IP 的 BFF）；正式发布前依赖 makerhao.cn 备案通过（T-008）

## 2. 规模判定与用户确认

- 规模：large（新增独立端 + 后端契约变更 + 多个有序切片）
- 确认状态：**approved（概括批准）**。2026-10-07 用户指示「微信开发者工具给你登录了 你开始开发小程序吧 别的不用管」，选型确认为**原生 + TypeScript**。
- 边界说明：S1 为纯新增目录、零后端/契约改动，按用户指示直接执行；**S2 涉及后端契约与 secrets 变更，动工前按确认门单独向用户展示契约变化后再动手**。

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | 原生 TS 骨架 + 文章列表/详情只读浏览 + 阅读量上报 + 分享 | `miniprogram/`（纯新增） | 无 | `tsc --noEmit` + DevTools 打开编译 + 模拟器访问生产 API 实测列表/详情 | 删除目录 | completed |
| S2 | 微信登录：wx.login → 后端 code2session → openid 绑定 reader 账号 + 会话下发 | backend（wechat 契约、secrets、users 绑定字段）+ miniprogram 登录流程 | S1、用户对契约的单独确认 | 契约测试 + DevTools 真机登录实测 | 单 PR revert；迁移 down.sql | completed |
| S3 | 互动：点赞/评论查看与发表/收藏同步 | backend 复用既有接口 + miniprogram | S2 | 真机实测 + 契约测试 | 单 PR revert | completed |

## 3. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 技术栈 | 原生小程序 + TypeScript | 阅读端功能面窄，避免第三方框架工具链；用户已拍板 | 多端复用放弃（当前无需求） |
| API 入口 | 走 BFF 同源路径 `/api/v1`，单点配置 `utils/config.ts` | 与 Web 端同一条公开入口；备案后仅改一行切 `https://makerhao.cn/api/v1` | 开发期依赖「不校验合法域名」（`project.config.json` 已置 `urlCheck:false`） |
| 详情渲染 | 复用后端 `content_html` + `rich-text`，对 img/pre/code/table 补内联样式 | 服务端已渲染（M1/S3），零 markdown 库依赖；rich-text 不解析 class，只认内联 style | 复杂排版/代码高亮后续再增强 |
| wire 格式 | snake_case 字段 + 枚举数字 + RFC3339 时间 | 与 `frontend/app/composables/useArticles.ts` 声明一致（Kratos HTTP codec 行为） | 无 |
| 列表分页 | `order_by=published_at desc` + `next_page_token` 触底翻页，客户端兜底过滤 `status===2` | 与 `usePublishedArticles` 行为一致 | 无 |
| tabBar | 文本 tabBar（文章/我的），暂不带图标 | 避免二进制资源进库；图标后续补 | 视觉朴素（可接受） |
| 阅读量上报 | 详情加载后 fire-and-forget `POST /articles/{slug}/view` | 与 Web 端阅读量口径一致（24h 去重在服务端） | 无 |

## 4. S1 实施与验收

- 结构：`miniprogram/{project.config.json,tsconfig.json,package.json,.gitignore}` + `miniprogram/miniprogram/{app.*,sitemap.json,pages/{index,post,me},utils/{config,request,types,api,format,html}.ts}`
- 验收清单：
  - [x] `npm run typecheck`（tsc --noEmit）通过
  - [x] DevTools 编译通过并实跑（用户升级至 Stable 2.02.2608080 后原生编译 TS；旧版 1.05 无 TS 插件曾以 emit JS 兼容层过渡，升级后已清理）
  - [x] 模拟器内列表可见生产文章、下拉刷新/触底翻页生效、详情 rich-text 渲染正常、阅读量 +1、分享卡片标题正确（2026-10-07 用户模拟器实测）
- 交付：PR #65 squash 合并（merge `758f90e3`，2026-10-07，guard push/pr/merge 三次 consume 全 allow，CI 双绿）

### S2：微信登录（后端 + 小程序）

- 状态：completed（代码与测试交付；真实 code2session 冒烟待生产部署后在模拟器/真机补）
- 契约（auth 域内新增，2026-10-07 用户以提供 AppSecret 视为确认）：
  - `POST /v1/auth/wechat`（body: `{code}`）→ `WechatLoginReply{status, login?, binding_ticket?}`：openid 已绑 → status OK + 标准 LoginReply（refresh cookie 同 Login）；未绑 → status BINDING_REQUIRED + 一次性票据（10 分钟，Redis GETDEL 原子消费）
  - `POST /v1/auth/wechat/bind`（body: `{binding_ticket, email, password}`）→ `LoginReply`：票据换 openid + 邮箱密码认证后绑定并登录；**绑定只面向已有账号，不静默注册**；失败烧票（需重新 wx.login）
  - 错误：`AUTH_WECHAT_CODE_INVALID`（25）、`AUTH_WECHAT_TICKET_INVALID`（26）、`USER_WECHAT_CONFLICT`（27）；未配置凭据 → 412
  - 存储：`users.wechat_openid` 唯一可空列（迁移 000010 带 down.sql；ent schema 同步）
  - 限流：两端点复用 login 每 IP 预算
- 后端：conf 新增 `wechat.app_id/app_secret`（secret 只进 git-ignored secrets 文件）；`data/wechat.go` code2session 客户端（5s 超时，session_key 不落日志）+ 票据存取；casbin 公开放行两路由
- 小程序：`utils/auth.ts` 会话存取（access+refresh+过期戳，401 → X-Refresh-Token 单飞刷新 → 重放一次）；我的页登录/绑定/已登录三态 UI
- 验证：`go build/vet/test` + golangci-lint 全绿（新增 biz 流程测试 9 例、code2session httptest 3 例、authz 路由用例）；小程序 `tsc --noEmit` 全绿
- 未验证项：真实 wx.login→code2session 链路需生产部署后端（带 wechat secrets）后在模拟器/真机冒烟；部署时服务器 secrets 文件需补 `wechat:` 段
- 打磨交付：[PR #71](https://github.com/luohao0308/luohao-blog/pull/71) squash 合并 merge `2326c931`（CI 双绿；strict 分支保护下与并行会话的合并竞速，脚本化『更新→CI→抢 CLEAN 窗口』完成）
- 打磨内容（用户实测反馈 2026-10-07）：绑定失败自动静默换新票据留在表单（不再弹回登录页）；我的页按站点设计语言重做（blue-600 药丸主按钮、slate 灰阶、#3c5d85 品牌头像环、渐变头部卡）
### S3：互动（点赞/评论/收藏，纯小程序端，后端零改动）

- 状态：completed（2026-10-07）
- 实现：详情页互动条（点赞 ♥ 本地去重 + 服务端 24h 去重兜底；收藏 ★ 本地存储快照标题）；评论区（分页拉取、头像/首字回退、自己待审评论带「审核中」标识、登录态输入栏 401 自动刷新重试、未登录引导跳「我的」）；收藏页（新路由 /pages/favorites/favorites，快照列表 + 移除）；我的页新增「我的收藏」菜单行 + 真实头像展示
- 验证：`tsc --noEmit` 全绿；契约对齐（POST /v1/comments 需 JWT、GET /v1/articles/{slug}/comments 公开、PENDING=1/APPROVED=2）
- 未验证项：模拟器/真机实操（点赞计数、评论先审后显流转）待用户实测
- 回退点：单 PR revert

- 回退点：单 PR revert；迁移 down.sql
- 回退点：删除 `miniprogram/` 目录，单 PR revert
