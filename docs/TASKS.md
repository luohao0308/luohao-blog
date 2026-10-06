# Task Board

_last-updated: 2026-10-07_

> **唯一用途**：记录当前进行中、明确待办、阻塞和技术债。稳定事实写入架构/设计文档，详细验证过程写入工作日志（如项目启用）。
>
> 状态：`[~]` 进行中或阻塞 | `[ ]` 已批准但尚未开始 | `[x]` 仅出现在“已完成”章节。
>
> 当前任务的临时目标、决策、交接提示和验证摘要写入 [WORKING-CONTEXT.md](WORKING-CONTEXT.md)，不要复制到本文件。

---

## 进行中 (In Progress)

| ID | 任务 | 范围/仓库 | 上下文 | 阻塞 |
|---|---|---|---|---|
| T-008 | M5 上线基本收官：S1-S4 全部完成（S1 #23；S2 公网 IP 直访；S3 发布流水线 #25+#26+#27——Actions 推 ghcr 实跑绿，服务器更新走本地构建回退（ghcr 国内拉取受限）；S4 备份 cron+看门狗已装）；HTTPS 待备案 | backend + deploy + CI | 计划 `docs/plans/M5-DEPLOY.md` | 生产管理员改密已销项（2026-10-05）；#28 服务器部署已完成并公网 E2E 验收（2026-10-04）；**剩余仅 HTTPS：域名 makerhao.cn 已购（2026-10-07，腾讯云），ICP 备案进行中（约 1–2 周），通过后接解析+Caddy HTTPS+Secure cookie** |
| T-012 | 安全扫描 P1 修复：请求体上限。后端 `RequestDecoder` 包 `MaxBytesReader`（默认 1MiB，头像路由 4MiB）+ Caddy `request_body max_size 8MB` 兜底；分支 `fix/request-body-limit` 本地验证全绿（go build/vet/test、3 项新单测、容器内 `caddy validate`、一次性容器真实栈冒烟：2MB 登录体→400 too large 秒拒、正常解码不受影响、头像路由 2MB→401 证明 4MiB 上限生效）。P2/P3 项记录在案待排期：文章渲染 `WithUnsafe` 复评（开放注册后假设已过期）、refresh cookie `Secure` 属性（随 HTTPS 上线）、Redis requirepass、搜索接口限流+长度上限 | backend + deploy | — | 等 push/PR 人工确认（manual+user） |
| T-011 | 微信小程序阅读端（原生 TypeScript，AppID `wx58089476f518fd3f`）：S1 骨架+文章列表/详情只读浏览+阅读上报+分享（代码完成待 PR）；S2 微信登录绑定 reader 账号（**后端契约变更，动工前单独确认**）；S3 互动 | `miniprogram/`（新增目录）+ backend（S2 起） | 计划 `docs/plans/MINIPROGRAM-2026-10.md`（2026-10-07 用户指示"开始开发小程序"，概括批准） | 无阻塞：开发期 DevTools 关合法域名校验直连生产 BFF；正式发布依赖 makerhao.cn 备案（T-008）；AppSecret 待 S2 前生成且只进后端 secrets |

## 待办 (Todo)

### T-010 遗留补测清单（2026-10-06 生产部署 + 生产 E2E 后基本完成）

- [x] 注册→登录→头像上传→immutable 回读：生产实弹验证（探针账号 `e2e-probe@luohao.blog`，头像 `1f4245a0….png` 卷内落盘+字节一致）
- [x] 生产 E2E（2026-10-06，用户"对生产跑"授权）：公开用例 **12/12 全绿**（首页/列表/标签/详情×2/归档/作品集/关于/搜索/404/AI 问答真实 LLM/错误密码拒绝）；t05 修复为按环境动态选文章（原硬编码 dev 栈 seed slug `m5-prod-smoke`，生产不存在）；**t13–t16（后台列表/发文/评论审核闭环/登出守卫）需管理员凭据，未跑**——生产管理员密码属用户私密，不进对话
- [x] 服务器部署验证：迁移 v9 生效、`backend-uploads` 卷挂载、backup.sh 实跑产出 uploads tar
- [x] 前台 UI 走查（生产，探针账号）：登录→头像下拉（昵称邮箱/个人设置/退出，READER 无后台入口✓）→ /settings 三卡片（头像回显/昵称/改密，未修改时保存禁用✓）→ 文章页「以 部署探针 的身份发表」→ 提交成功→待审核块展示；**探针留言一条在待审区**（`部署验证：登录身份评论链路 OK`），用户可在后台通过或删除
- [x] t13–t16 补跑（2026-10-06，本地生产栈）：镜像从 main 重建（T-010 四切片在镜像内）后 **全套 16/16 全绿**（t01–t16，含 AI 问答真实 LLM）；t13 列表/t14 发文发布/t15 评论审核闭环/t16 登出守卫全部通过。补跑发现并修复 t15 用例两处失真：① 未登录引导断言用 `ctx.new_page()` 模拟访客，但同一 context 共享 cookie，登录后必假——改为登录前在主页面断言；② 提交后断言"未展示"与 S4 语义冲突（作者的待审评论在「待审核」块可见）——改为断言待审块展示 + 已发布列表不出现，审核通过后进入已发布列表；清理断言仅在提交成功后生效。限流自撞现场复现（连跑触顶 t16 登录 429），跑前清 Redis `blog:ratelimit:*`（已落地为套件开关 `BLOG_E2E_CLEAR_RATELIMIT=1`，见技术债销账）。t15 修复已随 [#58](https://github.com/luohao0308/luohao-blog/pull/58) 合并（merge `d13bcb9`）

- [x] T-010 前台账号体系与头部改版（四切片全部交付）：**S1** 后端账号基座（开放注册 READER+自动登录+独立限流、User.avatar_url、UserService UpdateProfile/UploadAvatar/GetAvatar（HttpBody+immutable 缓存）、本地磁盘存储+compose `backend-uploads` 卷+备份/恢复、迁移 000008，[PR #52](https://github.com/luohao0308/luohao-blog/pull/52) merge `c59fa80`）；**S2** 前台头部改版（导航 8→5 项+文章▾ 下拉、UserMenu 登录/头像下拉（超管进后台）、公开布局会话恢复、/login /register，[PR #53](https://github.com/luohao0308/luohao-blog/pull/53) merge `e72b6fa`）；**S3** /settings 个人设置（头像 canvas 压缩上传/昵称/改密、客户端登录守卫，[PR #54](https://github.com/luohao0308/luohao-blog/pull/54) merge `0a86e72`）；**S4** 评论登录门禁（CreateComment 需 JWT、身份取自 token、user_id 落库兼容旧匿名行、头像批量附加、评论区登录引导，迁移 000009，[PR #55](https://github.com/luohao0308/luohao-blog/pull/55) merge `c7ef251`）；各片 go build/vet/test 与 pnpm lint/typecheck/build 全绿，required CI 双绿，guard 全程 consume allow（2026-10-05/06）；交付中一次 gitignore `data/` 吞掉 internal/data/avatar.go 致 CI 红（教训：ignore 锚定 `/data/`）；计划与执行记录 `docs/plans/READER-ACCOUNTS-HEADER-2026-10.md`；真实栈补测清单见上方待办

- [x] T-010/S1 后端账号基座：开放注册（Register API→READER+自动登录，独立 IP 限流 fail-open）、User.avatar_url 契约、UserService（UpdateProfile/UploadAvatar base64 2MiB 魔数校验/GetAvatar HttpBody+immutable 缓存）、本地磁盘存储+compose 命名卷+备份/恢复脚本、迁移 000008、policy 8 行+全路由覆盖测试；go build/vet/test 全绿；交付中 gitignore `data/` 吞掉 internal/data/avatar.go 致 CI typecheck 红（教训：ignore 模式锚定 `/data/`），修复后重授权推送；[PR #52](https://github.com/luohao0308/luohao-blog/pull/52) squash 合并（head `95c5cf7`，merge `c59fa80`，required CI 两项通过，guard push×2/PR×1/merge×1 consume 全 allow，2026-10-05）；计划 `docs/plans/READER-ACCOUNTS-HEADER-2026-10.md`
- [x] 全量代码 review + Python Playwright E2E 套件（16 用例）+ 四切片修复交付：P0 无标签文章打挂全站（前端 3 文件判空 + 后端 protojson codec 根治）、限流键 XFF 最右跳、搜索 400 + page_size 上限、useAuth 刷新单飞、backend/.dockerignore + gitignore/备份脚本泄漏面收口；go/vet/test 与 pnpm lint/typecheck/build 全绿，E2E 16/16，本地生产栈已重建验证；[PR #28](https://github.com/luohao0308/luohao-blog/pull/28) squash 合并（head `c48860f`，merge `6a4d0d3`，required CI 两项通过，guard push/PR/merge 三次 consume 全 allow，2026-10-04）；计划与执行记录 `docs/plans/REVIEW-FIXES-2026-10.md`
- [x] 服务器（193.112.128.245）部署 #28：git pull → `compose up -d --build backend frontend`（用户"你继续就行"授权 AI SSH，与 M5/S2 同模式）；公网核验——API 新 wire 格式（tags 恒在/枚举数字/RFC3339）、全部页面 200、搜索非法 token 400、page_size 上限生效；**公网 E2E 16/16 全绿**（含后台完整 CRUD/评论审核/登出守卫，测试数据自清理，无遗留）；六容器 healthy（2026-10-04）

- [x] M5/S3+S4 发布流水线与运维基线：release.yml（Actions 推 ghcr，实跑绿）+ compose image 指向 ghcr（build 段保留回退；ghcr 国内拉取受限，服务器更新走本地构建回退——两次排障 #26/#27 参数化 GOPROXY/APT_MIRROR）；服务器备份 cron 03:10 + 健康看门狗每 5 分钟（连续 2 失败自动重启 backend）（2026-10-03）
- [x] M5/S2 服务器首次上线：193.112.128.245（腾讯云 Ubuntu 24.04 2C4G）——jobapp-staging 经确认下线清理（释放 35.9GB，80/443 释放）；swap+clone+密钥+compose up --build+seed+reindex；公网验收全通（页面/API/混合搜索/真实 DeepSeek 问答 1.3s）；备份脚本已上服务器；生产管理员 luohao@2days.org（提醒改密）（2026-10-02，用户授权 AI SSH 操作）

- [x] M5/S1 本地生产化：生产 compose（caddy→BFF→backend+mysql/redis/es，六容器 healthy）+ frontend Dockerfile + Caddyfile 启用 + backup/restore 脚本 + prod secrets 双文件模板；Caddy :8080 全功能冒烟 + 真实 DeepSeek 问答 1.3s + 灾难恢复演练；[PR #23](https://github.com/luohao0308/luohao-blog/pull/23) squash 合并（merge `a975bfc`，2026-10-02）

- [x] T-007/S3+secrets M4 收官交付：[PR #21](https://github.com/luohao0308/luohao-blog/pull/21) secrets 文件机制（merge `dc2bc58`）+ [PR #22](https://github.com/luohao0308/luohao-blog/pull/22) 混合检索 + RAG 聊天窗（head `ccc2cfc`，merge `bf9817e`，required CI 两项通过，guard 全程 consume allow，2026-10-02）；交付中堆叠 PR 冲突经本地 merge main 解决（squash 与分支内容同文件不同版本）；DeepSeek /chat/completions 挂起为外部服务状态，reasoning_effort=none 已预置

- [x] T-008 文章列表 status 过滤器 500 修复 + 回归测试：service 声明 `status` 过滤字段 + 解析错误映射 400；data 层枚举名→int 列 resolver + 查询前校验（ent 错误扁平化坑，见技术债）；真实环境冒烟通过；[PR #17](https://github.com/luohao0308/luohao-blog/pull/17) squash 合并（head `8bc1743`，merge `7f0274b`，required CI 两项通过，guard push/PR/merge 三次 consume 全 allow，2026-10-02）
- [x] T-009 演示文章可重复 seed：`cmd/seed -demo-articles [-reset]`，6 篇文章 frontmatter markdown 内嵌（go:embed），走 biz usecase 写入、幂等补缺；空库 E2E（创建/跳过/reset 收敛/MD5 逐对一致）通过；[PR #18](https://github.com/luohao0308/luohao-blog/pull/18) squash 合并（update-branch 后新 head `fa78b29`，merge `c01aa6b`，required CI 两项通过，2026-10-02）
- [x] T-006 M3 互动统计：S1 阅读量（PR #12）、S2 评论后端（PR #13）、S3 评论前端（PR #14）全部 merged（2026-10-01）；评论先审后显、阅读量 24h IP 去重
- [x] T-002 GitHub `main` branch protection：required checks（`Backend lint & build & test`、`Frontend lint & typecheck & build`）、管理员遵守规则、禁止 force push/删除分支、要求 conversation resolution；PR review 要求已按用户指示移除，2026-09-30 通过 GitHub API GET 核验
- [x] T-004/S4 M2 管理后台：Markdown 编辑器与文章管理；实现、全栈冒烟、push、PR、squash merge 全部完成（2026-10-01）
- [x] M2 收尾：ADR-0001/0002 已存在于 `docs/architecture/DECISIONS.md`（核验后勾选，无需新写）（2026-10-01）
- [x] 内容入库第一批：两篇草稿经管理后台 API 入库（slug `ai-delivery-guard`、`fullstack-smoke-pitfalls`，均为 DRAFT 待用户审阅发布）（2026-10-01）
- [x] 内容入库第二批：ent M2M edge、VARCHAR(191) 先以明确标注的“技术案例推演”演示稿发布；后续可用第一手素材替换（2026-10-02）
- [x] 用户审阅第一批两篇草稿（`ai-delivery-guard`、`fullstack-smoke-pitfalls`）：用户确认后已发布（status=PUBLISHED，published_at 盖章），前台 SSR 验证可见（2026-10-01）
- [x] 清理已合并远端分支：11 个远端 feat/* 全部经 merged-PR 核验（#1–#11）后删除，远端仅剩 main；本地同名单同步清理（本地 `feat/m2-s3-rbac` 为 S3 中间态分支、内容已入 main，保守保留）（2026-10-01，用户当次授权）
- [x] M4/S1 ES + BM25 关键词搜索：文章索引同步、公开搜索契约、前端搜索入口；PR #15 squash 合并（head `38fea4b`，merge `645da8e`，required CI 两项通过，2026-10-01）
- [x] M4/S1 analyzer 兼容性修复：stock Elasticsearch 使用内置 `standard` analyzer，PR #16 squash 合并（head `28077dc`，merge `e086e92`，required CI 两项通过，2026-10-02）
- [x] M4/S2 embedding、S3 语义搜索 + RAG：PR #20（embedding 管道）/ #22（混合检索 + RAG 聊天窗）交付；2026-10-05 生产实证——ES 4 文档 1024 维向量在库，两个换说法查询第一名命中正确文章，chat 真实生成 + 引用 + 无编造（此前该行误记为暂缓）


- [x] M2+ 内容发现能力补全：首页文章搜索（复用搜索 API）、按阅读量展示热门文章、文章/项目/标签统计和最近更新时间；前端 lint/typecheck/build 通过（2026-10-04）
- [x] M2+ 内容组织（标签部分）：首页标签云（前 12 + 全部入口）、标签总览页 /tags、导航"标签"入口、tagCounts 聚合函数；标签聚合页 /tags/[tag] 沿用既有；lint/typecheck/build + 预览冒烟通过（2026-10-04）
- [x] M2+ 内容组织（分类部分）：独立分类体系三切片交付——S1 后端 Category 模型/契约/category 过滤（PR #31 squash 合并 merge `96b65f3`）；S2 管理后台分类管理+文章表单分类选择（PR #32 squash 合并 merge `d95875d`）；S3 前台分类导航+/categories 总览+聚合页（PR #33 squash 合并 merge `d3f8398`）；设计与执行记录 `docs/plans/M2PLUS-CATEGORY-2026-10.md`（2026-10-04）
- [x] M2+ 项目展示增强：作品集技术栈筛选（T-005 已有，本轮核验）+ 项目详情关联项目推荐（按技术栈重叠度排序，零重叠兜底展示其余项目）；前端 lint/typecheck/build + 预览冒烟通过（2026-10-04）
- [x] M3 互动与增长能力：RSS 订阅（/rss.xml + head 发现，PR #44 merge `6a95fe7`）、邮件订阅（登记制：后端表+幂等公开接口+限流 PR #43 merge `0853004`，前端表单+管理页 #44）、阅读排行榜（/ranking 前十 + 列表热门位 #38）；生产验收通过（订阅 200/幂等/400、RSS 4 items、页面全 200）（2026-10-05）
- [x] M3 内容推荐体系：相关文章推荐（分类加权+标签重叠，回退最新）+ 文章列表热门阅读位（PR #38 squash 合并 merge `fc7be0d`）（2026-10-05）
- [x] M3 用户互动能力：点赞（后端 like_count+去重 PR #36 merge `f70ff20`；前端按钮 PR #37 merge `524badc`，content-type 修复 #40 merge `301de3f`）、收藏（localStorage 本地方案 + /collections 页）、评论体验（待审块/昵称记忆/字数统计/重试，PR #39 merge `07d582c`）；生产已部署验收（2026-10-05）

## 未授权或未立项 (Do Not Start)

- M5 上线：已上线（服务器 193.112.128.245 生产运行，见上方 T-008）；M4 全部完成（语义检索生产实证 2026-10-05）
- tag/Release、镜像发布、部署、迁移、仓库设置类操作：未授权，按 manifest `privilegedOperationsDefault=deny` 逐次申请

## 已完成 (Done)

- [x] 线上更新 #63+#64（2026-10-06）：服务器 193.112.128.245 `git pull` → **仅重建 frontend**（ghcr 拉取受限走服务器本地构建回退；backend 自上次部署无代码变更，不动以缩小变更面/规避 apt 网络抖动）→ `up -d frontend`；六容器 healthy，公开页全 200，首页/关于页 SSR 已含新邮箱与新 UI，公开 E2E（t01–t11，不含后台登录类）11/11 全绿
- [x] 前台 UI 走查整改：首页改为开屏式介绍落地页（居中 hero + 一行站点数据 + 最新文章/精选项目/订阅，搜索/热门/标签云归位到 /posts、/ranking、/tags 专属页）；顶部导航扁平化（移除「文章」下拉与「收藏」入口，分类/标签/归档改为 /posts 页内浏览入口，收藏/RSS/GitHub 移入页脚导航，导航加当前页高亮，logo 本就回首页）；/settings 昵称行「保存」按钮被挤成竖排的对齐修复；全站按钮统一胶囊形、输入框/卡片 rounded-xl、后台 naive-ui borderRadius 10px + 对话框宽度统一 26rem；E2E t11 气泡选择器改为类名无关（rounded-lg→xl 改版牵连）；frontend lint/typecheck/build 全绿，本地生产栈重建（frontend 镜像从本分支构建）视觉走查亮/暗/移动三态通过，E2E 16/16 全绿（过程数据：本地栈遗留探针账号 ui-visual-probe@luohao.blog 无害留存）；[PR #63](https://github.com/luohao0308/luohao-blog/pull/63) squash 合并（merge `4a9cc73`，required CI 双绿，guard push/PR/merge 三次 consume 全 allow，2026-10-06）
- [x] T-004 M2/S4 管理后台前端：useAuth（内存 token + 401 刷新重试 + refresh cookie 恢复）、admin 守卫与布局、登录页、文章管理表格（状态流转/删除确认）、Milkdown Crepe 编辑器；附带修复后端 update 接口 M1/S2 起恒 400 的存量缺陷（convertArticle 透传 status + 单测）、BFF cookiePathRewrite；`go build/test/vet` 与 `pnpm lint/typecheck/build` 全绿，curl + 浏览器 GUI 冒烟通过；[PR #11](https://github.com/luohao0308/luohao-blog/pull/11) squash 合并（head `71cb60b`，merge `2c4983a`，CI run 36754607113，guard push/PR/merge 三次 consume 全 allow，2026-10-01）
- [x] T-004 M2/S2 认证 API + JWT 中间件：auth 契约（Login/Refresh/Logout/GetMe）、HS256 access JWT（alg 钉死+exp 必填+空密钥拒启）、Redis refresh 会话（GETDEL 原子旋转、httpOnly cookie+header 双通道）、自研 kratos v3 JWT 中间件、登录限流（10 次/5min/IP fail-open）、cmd/seed；`go build/test/lint` 全绿 + 冒烟 30/30（`backend/scripts/smoke-auth.sh`）；经 PR #8 合并（head `a41bb38`，merge `06d97a7`，CI run 36688687955，guard 三步 consume 全 allow）
- [x] T-004 M2/S3 Casbin RBAC：公开文章读取仅返回已发布内容，reader 无法读取草稿，admin 保留管理读取；`go test ./...`, `go vet ./...`, `go build ./...` 与 CI run `36743731504` 全绿；PR #9 squash 合并（head `e0a955b`, merge `b2af83a`, 2026-09-30）；独立 Review 门禁按用户要求移除
- [x] T-005 个人站改版 P2/P3：文章搜索/标签/分页/相邻文章、作品集筛选与详情、移动端和暗色模式；本地 lint/typecheck/build 通过，PR #10 更新 main 后 CI run `36745546102` 两 job 全绿，squash 合并（head `4721d2b`, merge `91e2f5f`, 2026-09-30）
- [x] T-000 仓库初始化与 dev-workflow 接入：`luohao-blog` 建仓并推送 GitHub（Public）；dev-workflow 0.5.0 九包安装；项目画像完成，onboarding ready（证据：`docs/WORKFLOW-ADOPTION.md` 审计记录）
- [x] T-001 M0 脚手架：backend/frontend/deploy/CI 空壳全绿，5+2 提交经 PR #1 合并（merge `13349f6`）
- [x] T-003 M1 内容核心：S1 契约（PR #2，`8d1f8d6`）→ S2 存储与用例（PR #3，`859a8be`）→ S3 Markdown 渲染（PR #4，`a4fe24c`）→ S4 前端 SSR（PR #5，`52a2fb7`），四片全部验收合并；全栈冒烟通过（SSR 页面含渲染 HTML）
- [x] T-001 M0 脚手架：backend（Kratos v3 layout，go build/test/golangci-lint 全过，`GET /v1/todos/list` 冒烟 `{}`）、frontend（Nuxt 4.5，lint/typecheck/build 全过）、deploy（compose config 校验 + mysql/redis 容器 healthy）、CI 工作流；5 个提交在 `feat/m0-scaffold`（ce2c948..09d0143）（证据：docs/development/README.md 命令矩阵）

## 技术债 (Technical Debt)

_第二轮 review（2026-10-04，全量记录见 `docs/plans/REVIEW-ROUND2-2026-10.md`）后合并账：_

| 项目 | 风险 | 说明 |
|---|---|---|
| ~~useAuth 登出与 in-flight 刷新竞态~~ | 已修复 | #29：logout 先 await 在途 refreshOnce 再撤销会话（`useAuth.ts` logout）；2026-10-06 复核代码在位 |
| ~~运维资产在版本库外~~ | 已修复 | #29：docs/、AGENTS.md、deploy/watchdog.sh 入库，runbook §5.1 补定时任务节（backup 03:10 + watchdog */5 真相源）；**残留**：备份仍单机无异地副本、cron 静默失败无告警 |
| ~~LLM 120s vs HTTP server 60s 超时错配~~ | 已修复 | #29：HTTP deadline 提至 150s 并文档化与 llm.timeout 的耦合 |
| ~~page_token 合法大 offset 未钳~~ | 已修复 | #29：offset 限 [0, 9900]（页大小上限内不出 ES max_result_window），越界映射 400，负值同拦 |
| ~~errors.Error 实际走 protojson~~ | 已修复 | #28：codec.go 注释更正（proto 走 protojson、kratos errors 等 non-proto 保持 stdlib 形状）+ TestJSONCodecMarshalKratosError 钉死错误体编码路径；metadata/非法 UTF-8 维持 stdlib 现状（已文档化，不再当缺陷追踪） |
| ~~es.go 零测试 / schema 双源~~ | 部分修复 | #30 补 es_test.go 3 例（索引配置/embedding 输入/维度）；**残留**：查询路径无覆盖、ent 注释 vs 手写 SQL 漂移未收口 |
| ~~release.yml 镜像发布门禁~~ | 已修复 | `ci-gate` 复用 CI 工作流并作为镜像任务前置依赖；保留 concurrency 与 30 分钟超时 |
| ~~backup.sh 空库假成功~~ | 已修复 | 临时文件 + EXIT 清理，gzip 完整性与 dump 头校验通过后原子重命名 |
| ~~限流器 Incr+Expire 异常路径~~ | 已修复 | `TxPipeline` + `ExpireNX` 保持窗口不延长，并补 Redis 故障回归测试 |
| ~~E2E known_issue 失效~~ | 已修复 | #29 摘除过期标注；2026-10-06 复核套件内无 known_issue=True。限流自撞亦已修复（见下行） |
| ~~E2E 限流自撞~~ | 已修复 | 套件开关 `BLOG_E2E_CLEAR_RATELIMIT=1` 启动前清限流键（默认关，docker exec 通道仅限本地栈；2026-10-06 实测：打满 10 次/5min 限流器后 cleared(1) → t12 立即通过；对生产跑不开启） |
| ~~Caddy 无访问日志 / restore 边服务边恢复 / 容器日志无轮转~~ | 已修复 | 三件全部清账（2026-10-06）：Caddy 访问日志启用；容器日志轮转 compose 六服务 json-file 10MB×3；restore.sh 改为「坏包停服前拒收 → uploads 先行 → 停 backend 灌库 → 回启等健康 → 提示 reindex」，本地栈完整 DR 演练（含灌库失败回启路径） |
| Secure cookie 未启用 | 中 | 等域名+TLS（conf.proto 改动需 buf）；TLS 前公网登录明文 |
| v-html + WithUnsafe XSS 面 | 中 | 信任边界=仅 admin 可写；引入第二作者前必须 sanitize |
| ~~frontend healthcheck 用整页 SSR 探针~~ | 已修复 | Nitro 新增 /health（不触数据层）+ compose 探针改打 /health（2026-10-06 本地栈实弹：frontend ~20s 转 healthy，/health 200，首页/E2E t01–t02 无回归）；backend 探针打 /v1/articles/list 属数据层语义，有意保留 |
| demo 文章软删后 seed 冲突 | 低 | `cmd/seed/demo_articles.go:146` 存在性判断应含 DELETED |
| ~~dev compose 端口发布 0.0.0.0~~ | 已修复 | 六端口（mysql/redis/es/minio）全部绑 `127.0.0.1`（compose config 渲染核验，2026-10-06） |
| ~~CI 杂项~~ | 已修复 | ci.yml 6 个 action 全部 pin 官方 SHA + `permissions: contents: read`（workflow_call 兼容）；backup.sh 轮转改 glob 展开免 ls 空白分词（实跑 KEEP=2 轮转核验）；paths 过滤评估后**不做**——workflow 级 paths 会把 required check 留在 Pending 卡死文档型 PR（官方文档），public 仓库 Actions 免费无收益；**残留**：release.yml 4 个 action 未 pin（下次发布窗口一并处理） |
| 搜索 "Chinese analyzed" 名不副实 | 低 | standard analyzer 单字切分（M4/S1 的 #16 决策），改文案或上 ik |
| P3 长尾 | 低 | Accept 协商、int64 类型漂移、IP 规范化、ES 启动 ctx 无超时、tags multi_match 低效、TIMESTAMP 2038、reindex 1000 篇/串行、seed 不回填索引、软删占坑 slug、chat 无缓存、prompt 无定界符、LLM body 无上限、密码无上限——全量见 REVIEW-ROUND2 文档 |
| ent selector 错误扁平化 | 已规避 | data 层返回错误码必须在查询构建前校验（`data.validateStatusFilter` 模式） |
| MinIO 拉取 denied / 宿主 3306 被占 | 低 | 环境类备忘 |

---

_维护方式：任务完成后从“进行中/待办”移动到“已完成”，再更新顶部日期。短期上下文完成或过期后清理；长期经验迁移到 `project-memory/`；历史证据迁移到工作日志。_
