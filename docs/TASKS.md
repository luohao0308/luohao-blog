# Task Board

_last-updated: 2026-10-20_

> **唯一用途**：记录当前进行中、明确待办、阻塞和技术债。稳定事实写入架构/设计文档，详细验证过程写入工作日志（如项目启用）。
>
> 状态：`[~]` 进行中或阻塞 | `[ ]` 已批准但尚未开始 | `[x]` 仅出现在“已完成”章节。
>
> 当前任务的临时目标、决策、交接提示和验证摘要写入 [WORKING-CONTEXT.md](WORKING-CONTEXT.md)，不要复制到本文件。

---

## 进行中 (In Progress)

| ID | 任务 | 范围/仓库 | 上下文 | 阻塞 |
|---|---|---|---|---|
| UI-CARDS | 用户选定封面式卡片；从最新main隔离集成，保留封面字段/聚光/原地展开；78测试、lint/typecheck/build、组合浏览器回归与独立复核通过，准备准确SHA授权交付 | frontend | [计划](plans/COVER-CARDS-2026-10.md) | 远端与生产需准确版本一次性授权；未上线 |
| T-023 | 首页模型城与关于页固定水面涟漪 | Nuxt Web前端 | [已批准计划](plans/HOME-CITY-ABOUT-BACKGROUNDS-2026-10.md) | 已固定涟漪、修复滚动中途归位；城市透明居中且飞机全首页。太阳已抬高、演示/暂停重置控件已移除；78测试、lint/typecheck/隔离build、桌面手机浏览器及独立复核通过，用户已验收并要求提交推送；按准确版本推送守卫交付 |
| T-022 | 个人设置B＋C融合布局 | Nuxt Web前端 | [实施计划](plans/SETTINGS-LAYOUT-2026-10.md) | 本地完成；lint/typecheck/build、62单测、设置/头像/邮箱三套mock回归与独立审查通过，待用户查看；未push/PR/部署 |
| T-021 | 关于页：流动的数字墨迹预览 | Nuxt Web前端 | [视觉规范](plans/ABOUT-INK-2026-10.md) | 本地完成；59单测、lint/typecheck/build、墨流与真实资料浏览器验证通过，独立复核APPROVE；等待视觉反馈，未发布 |
| T-020 | 公开网页卡片统一，文章与项目原地展开 | Nuxt Web前端 | [卡片规范](plans/BENTO-CARDS-2026-10.md) | 本地完成；本轮61单测、lint/typecheck/build、项目12项与文章12项浏览器回归通过；前轮Bento13组与独立复核通过；真实详情路由保留，未发布 |
| T-019 | 首页原位组装与暗色首屏闪现修复，本地验证通过 | Nuxt Web前端 | [实施计划](plans/HOME-MOTION-2026-10.md) | 改为真实SSR区块原位组装，保留大幅度与原速；默认浅色且尊重显式偏好。线上仍PR #105 / 3d72f8b；40单测、lint/typecheck/build、组合E2E与6宽度归位0像素差通过，独立审查APPROVE；本轮新版本未发布 |
| T-017 | 后台已验收，推进正式小程序接入与发布 | backend + miniprogram + docs/platform | [收尾](plans/ADMIN-MINIPROGRAM-CLOSEOUT-2026-10.md) / [正式接入](plans/FORMAL-MINIPROGRAM-TRANSITION-2026-10.md) | 正式后端744/v16与身份绑定、域名和核心真机已验收；PR #100退出会话/正文图地址修复已合并8feb334且CI/release通过，新29e预览手机确认图片显示。UGC已声明，隐私最新为已更新，备案仍管局审核中。手机新增发现首次文章登录提示失步、正文图无点击预览；已本地修复并通过测试/独立复核和模拟器点击链路，待新准确版本交付与手机复验。专属测试文章已回草稿、公开404，正文/精选未变；服务器退出撤销真机验收尚未进行 |
| T-008 | 域名收尾：S1 主站 HTTPS/备案/Secure Cookie 已验收；S2 小程序 HTTPS 与公众号 HTTPS origin 已交付；S3 状态页与 HTTPS 拨测代码已随 PR #93 合并，公开状态页已上线 | backend + frontend + miniprogram + deploy | `docs/plans/DOMAIN-CLOSEOUT-2026-10.md` | 2026-10-16 本轮只读复核 `https://status.makerhao.cn` 返回 200、最终为 `/status/blog`；不替代完整路由正负向复验。最近 external-probe [run #38776227141](https://github.com/luohao0308/luohao-blog/actions/runs/38776227141) 成功（2026-10-16北京时间01:18启动，main30e）；本轮未额外触发。正式号核心真机流程已用户确认；正文图片显示已手机确认；点击预览、会话复验和平台发布准备仍待完成 |
| T-011 | 微信小程序阅读端：S1–S4已合并；正式目标wxf27116cc66cc49a4，旧代码/预览wx58089476f518fd3f | miniprogram + backend | [原计划](plans/MINIPROGRAM-2026-10.md) / [正式接入](plans/FORMAL-MINIPROGRAM-TRANSITION-2026-10.md) | 正式AppID身份隔离、域名/凭据及核心真机已完成；正文图片显示已确认，点击预览/会话补丁待交付；隐私已更新、UGC已声明，备案结果及上传提审发布尚待完成；没有web-view |

## 待办 (Todo)

- [x] 非域名收尾：六项技术债代码、小程序可执行验收及服务器线上更新已完成；PR #82 merge `7ffd5ed`，required CI/release 全绿，线上六 target、三项数据层信号和公开入口已核验。DevTools/真机 UI 点验仍保留为人工事项，域名/HTTPS 不在本项范围。

## 未授权或未立项 (Do Not Start)

- M5 上线：已上线（服务器 193.112.128.245 生产运行，见上方 T-008）；M4 全部完成（语义检索生产实证 2026-10-05）
- 此前 T-016 的 PR/CI 与准确版本生产更新授权已用于既定交付；不延伸为 T-017 的生产测试写入、远端 Git 或小程序上传/提审/发布授权。后续具体操作按目标与准确版本办理。
- T-016 已上线并启用 DirectMail，指定邮箱已确认收到唯一配置测试邮件；真实账号验证/找回及旧密码、刷新会话失效已完成生产验收；额外邮件按单独明确授权发送。已移除的 RSS/首页邮件订阅不在本轮恢复范围。
- 小程序提审发布已获用户概括批准继续准备；主站 ICP/HTTPS 已验收，不能继续写作“等网站备案”。小程序自身备案、隐私指引、合法域名与可用真机/开发者工具状态需平台复核；正式提交及发布仍按准确版本与平台门禁办理。网站公安备案状态另待核实。

## 已完成 (Done)

- [x] 背景实验室21版保留供回看；用户最终选定商务模型城，已扩图并腾出三处楼位（现9栋），加入摩天轮/横向展开且14秒分段变速的过山车、16秒整车沿路闭环公交、独立东侧广场喷泉和带更强点击喷气加速/航空表演式三股红白蓝烟羽的跨视窗飞机、立体环城飞艇，接入本地关于页并保留连续镜头/真实内容。验证与当前截图见[选定背景记录](plans/BACKGROUND-LAB-2026-10.md)。用户已验收并授权正式交付；准确版本的PR/CI、前端固定镜像发布与线上验证按交付守卫推进。

- [x] T-021 关于页连续镜头本地交付完成：姓名→技能→经历→代表内容→联系，真实滚轮正反可逆、普通阅读切换、流动光场辅助；保留真实资料与GitHub/mailto，精准删除指定句。恢复后修复矮预览窗口静默回退，800×600/900×650可播放，较小窗口/减少动态明确提示；最终lint/typecheck与隔离生产构建通过，用户已批准随商务模型城正式交付，准确版本与发布证据以PR/CI及部署记录为准。见[实施与验证](plans/ABOUT-NARRATIVE-2026-10.md)。

- [x] 公共页面统一宽度：保留导航1088px，所有主内容统一1024px外框，桌面相对导航两侧各内收32px，手机单层24px留白，含关于墨流、文章/作品列表与详情、个人设置及公共辅助页，保留860px阅读列。前轮lint/typecheck、62单测通过；本轮lint、最终build、35组内收布局检查与设置页mock回归通过；未发布。见[范围与验证](plans/PUBLIC-PAGE-WIDTH-2026-10.md)。

- [x] 两类详情860px、列表页内展开搜索/按需标签、目录平滑滚动与点击外圈修复、微信模块移至关于页悬浮侧栏及慢请求展开动画回退：本地完成，用户已授权正式上线；远端交付/生产更新待guard准确版本授权，验证见 `docs/plans/DETAIL-WIDTH-2026-10.md`。

- [x] T-018 博客视觉与交互优化 S1–S4 本地完成：统一主题/键盘基线、搜索与单轮问答结果状态、阅读复制/图片预览、编辑器恢复/同步保存与离开保护；前端/Go/小程序检查、最终六套生产模式浏览器及独立复核通过。分支 `codex/blog-ui-refinement`，未推送/PR/部署，真实LLM与小程序真机未验；详见[计划](plans/UI-REFINEMENT-2026-10.md)。

- [x] T-015 头像入口/编码修复、RSS 与首页邮件订阅入口移除、公众号自动草稿同步已生产交付；保留既有订阅数据。永久封面与 HTTPS 原文地址已配置，真实草稿联调通过。个人主体上次公开发表接口返回 48001，现采用自动同步草稿后人工发表，未声称已具备公开发表/群发权限；当前权限如需改变须重新核实。文章/作品/关于页的后续实现归 T-016。计划见 `docs/plans/ACCOUNT-AVATAR-WECHAT-2026-10.md`。

- [x] T-016 网站能力补齐六片已上线：[PR #95](https://github.com/luohao0308/luohao-blog/pull/95) 与并发测试通知修复 [PR #96](https://github.com/luohao0308/luohao-blog/pull/96)，生产版本 `30e291dc2167229e6018bd3cd3e1a540a7607ea0`；CI 与 [release](https://github.com/luohao0308/luohao-blog/actions/runs/38772498032) 通过。SQL/uploads/配置备份校验、v11→v15（dirty=false）、原账号/文章/头像保留、42 项公开/API/SEO/负向检查及 8 个桌面/手机页面通过；看门狗已改用容器健康状态。DirectMail 唯一配置测试邮件已由用户确认进入收件箱；随后以自有 READER 账号完成生产验证邮件、密码找回与新密码登录，旧密码和原刷新会话均返回401；账号按授权保留，其他账号不变。管理员写入未在本轮重复执行。计划见 `docs/plans/WEBSITE-COMPLETION-2026-10.md`。

- [x] T-008/S1 主站域名：makerhao.cn/www 解析、可信 HTTPS、308 跳转、备案号渝ICP备2026025040号-1、canonical 与 Secure Cookie 已通过真实生产验收；账号注册/登录/刷新/退出及精确清理通过。Git ed8be10 与 main d206e0e 树一致，运行镜像775455a3；PR #90/#91 CI全绿，继续 S2/S3。

- [x] T-014 评论审核与交互优化（PR #86 已合并，并随生产版本 `30e291dc2167229e6018bd3cd3e1a540a7607ea0` 运行；本轮管理员生产复验待做）：默认本地 smart 自动通过普通评论，可疑内容和限流异常待审；manual 回退；下架评论隐藏；Web/后台分页、小程序状态回显、跨账号/分页竞态和 Unicode 长度修复。Go 全量 test/build/vet/lint、frontend lint/typecheck/build、小程序 typecheck/test 与真实 HTTP 本地浏览器闭环通过。计划与复现命令见 `docs/plans/COMMENT-MODERATION-2026-10.md`、`scripts/e2e/README.md`。

- [x] T-013 监控 S1–S3：PR #79/#81 已合并，Loki、三个 exporter、外部拨测和内部状态页已交付；状态页公网接入随 T-008。本轮核实 MySQL exporter 账号缺失，target up 不代表 mysql_up；原数据层告警表达式漏报，修正与账号补齐纳入非域名收尾 S4。

### T-010 遗留补测清单（2026-10-06 生产部署 + 生产 E2E 后基本完成）

- [x] 注册→登录→头像上传→immutable 回读：生产实弹验证（探针账号 `e2e-probe@luohao.blog`，头像 `1f4245a0….png` 卷内落盘+字节一致）
- [x] 生产 E2E（2026-10-06，用户"对生产跑"授权）：公开用例 **12/12 全绿**（首页/列表/标签/详情×2/归档/作品集/关于/搜索/404/AI 问答真实 LLM/错误密码拒绝）；t05 修复为按环境动态选文章（原硬编码 dev 栈 seed slug `m5-prod-smoke`，生产不存在）；**t13–t16（后台列表/发文/评论审核闭环/登出守卫）需管理员凭据，未跑**——生产管理员密码属用户私密，不进对话
- [x] 服务器部署验证：迁移 v9 生效、`backend-uploads` 卷挂载、backup.sh 实跑产出 uploads tar
- [x] 前台 UI 走查（生产，探针账号）：登录→头像下拉（昵称邮箱/个人设置/退出，READER 无后台入口✓）→ /settings 三卡片（头像回显/昵称/改密，未修改时保存禁用✓）→ 文章页「以 部署探针 的身份发表」→ 提交成功→待审核块展示；**探针留言一条在待审区**（`部署验证：登录身份评论链路 OK`），用户可在后台通过或删除
- [x] t13–t16 补跑（2026-10-06，本地生产栈）：镜像从 main 重建（T-010 四切片在镜像内）后 **全套 16/16 全绿**（t01–t16，含 AI 问答真实 LLM）；t13 列表/t14 发文发布/t15 评论审核闭环/t16 登出守卫全部通过。补跑发现并修复 t15 用例两处失真：① 未登录引导断言用 `ctx.new_page()` 模拟访客，但同一 context 共享 cookie，登录后必假——改为登录前在主页面断言；② 提交后断言"未展示"与 S4 语义冲突（作者的待审评论在「待审核」块可见）——改为断言待审块展示 + 已发布列表不出现，审核通过后进入已发布列表；清理断言仅在提交成功后生效。限流自撞现场复现（连跑触顶 t16 登录 429），跑前清 Redis `blog:ratelimit:*`（已落地为套件开关 `BLOG_E2E_CLEAR_RATELIMIT=1`，见技术债销账）。t15 修复已随 [#58](https://github.com/luohao0308/luohao-blog/pull/58) 合并（merge `d13bcb9`）

- [x] T-010 前台账号体系与头部改版（四切片全部交付）：**S1** 后端账号基座（开放注册 READER+自动登录+独立限流、User.avatar_url、UserService UpdateProfile/UploadAvatar/GetAvatar（HttpBody+immutable 缓存）、本地磁盘存储+compose `backend-uploads` 卷+备份/恢复、迁移 000008，[PR #52](https://github.com/luohao0308/luohao-blog/pull/52) merge `c59fa80`）；**S2** 前台头部改版（导航 8→5 项+文章▾ 下拉、UserMenu 登录/头像下拉（超管进后台）、公开布局会话恢复、/login /register，[PR #53](https://github.com/luohao0308/luohao-blog/pull/53) merge `e72b6fa`）；**S3** /settings 个人设置（头像 canvas 压缩上传/昵称/改密、客户端登录守卫，[PR #54](https://github.com/luohao0308/luohao-blog/pull/54) merge `0a86e72`）；**S4** 评论登录门禁（CreateComment 需 JWT、身份取自 token、user_id 落库兼容旧匿名行、头像批量附加、评论区登录引导，迁移 000009，[PR #55](https://github.com/luohao0308/luohao-blog/pull/55) merge `c7ef251`）；各片 go build/vet/test 与 pnpm lint/typecheck/build 全绿，required CI 双绿，guard 全程 consume allow（2026-10-05/06）；交付中一次 gitignore `data/` 吞掉 internal/data/avatar.go 致 CI 红（教训：ignore 锚定 `/data/`）；计划与执行记录 `docs/plans/READER-ACCOUNTS-HEADER-2026-10.md`；真实栈补测清单见本章节

### T-012 安全扫描修复（已交付）+ 服务器落地（已完成）

- [x] **P1 请求体上限**（[PR #66](https://github.com/luohao0308/luohao-blog/pull/66)，merge `72d0738`）：后端 `RequestDecoder` 包 `MaxBytesReader`（默认 1MiB、头像路由 4MiB）+ Caddy `request_body max_size 8MB` 兜底；真实栈冒烟（2MB 登录体→400 too large、正常解码不受影响、头像路由 2MB→401 证明路由上限生效）
- [x] **P2 文章渲染 XSS 收敛**（[PR #68](https://github.com/luohao0308/luohao-blog/pull/68)，merge `af7d7ee`）：移除 goldmark `WithUnsafe`（原始 HTML 整体省略）；AST transformer 将链接/图片 destination 限定为 http/https/mailto + 相对地址；影响面审计确认生产与 seed 内容零原始 HTML，视觉无变化
- [x] **P3 搜索限流**（[PR #69](https://github.com/luohao0308/luohao-blog/pull/69)，merge `b5c2246`）：每 IP 60 次/分钟（fail-open）+ 查询 200 字上限 + `SEARCH_TOO_MANY_ATTEMPTS` 429 枚举 + conf `search_attempts/search_window`；上限在搜索 service 层以不误伤 chat 内部检索
- [x] **P3 Redis requirepass**（[PR #70](https://github.com/luohao0308/luohao-blog/pull/70)，merge `2d235a2`）：生产 compose 强制 `--requirepass`（缺 `REDIS_PASSWORD` 拒绝启动）、后端经 `KRATOS_REDIS_PASSWORD` 注入、healthcheck 带认证；开发 compose 保持无密码（仅绑 127.0.0.1）
- [x] **服务器落地（2026-10-07 用户授权 AI SSH）**：`/opt/luohao-blog/app` pull 至 `2d235a2` → `.env.prod` 生成并写入 `REDIS_PASSWORD` → `up -d --build redis backend`；验证：无密码 ping→NOAUTH / 带密码→PONG、后端 healthy、API/搜索 200、注册→刷新→登出→401 全链路；Caddy 重载时发现单文件 bind mount 的 inode 陷阱（旧配置仍在容器内），`--force-recreate` 后 12MB 探针 413、站点 200；三个已合并分支经 MERGED 核验后删除

四个 PR 均 squash 合并、required CI 双绿、guard 全程 consume allow（2026-10-07）；教训两则：① golangci-lint 必须进本地验证链（CI errcheck 抓到 ast.Walk 未检查返回值，本地已装 v2.14.0 与 CI 一致）；② 服务器单文件 bind mount 在 git pull 后必须 force-recreate 才能吃到新内容

- [x] 全量代码 review + Python Playwright E2E 套件（16 用例）+ 四切片修复交付：P0 无标签文章打挂全站（前端 3 文件判空 + 后端 protojson codec 根治）、限流键 XFF 最右跳、搜索 400 + page_size 上限、useAuth 刷新单飞、backend/.dockerignore + gitignore/备份脚本泄漏面收口；go/vet/test 与 pnpm lint/typecheck/build 全绿，E2E 16/16，本地生产栈已重建验证；[PR #28](https://github.com/luohao0308/luohao-blog/pull/28) squash 合并（head `c48860f`，merge `6a4d0d3`，required CI 两项通过，guard push/PR/merge 三次 consume 全 allow，2026-10-04）；计划与执行记录 `docs/plans/REVIEW-FIXES-2026-10.md`
- [x] 服务器（193.112.128.245）部署 #28：git pull → `compose up -d --build backend frontend`（用户"你继续就行"授权 AI SSH，与 M5/S2 同模式）；公网核验——API 新 wire 格式（tags 恒在/枚举数字/RFC3339）、全部页面 200、搜索非法 token 400、page_size 上限生效；**公网 E2E 16/16 全绿**（含后台完整 CRUD/评论审核/登出守卫，测试数据自清理，无遗留）；六容器 healthy（2026-10-04）

- [x] M5/S3+S4 发布流水线与运维基线：release.yml（Actions 推 ghcr，实跑绿）+ compose image 指向 ghcr（build 段保留回退；ghcr 国内拉取受限，服务器更新走本地构建回退——两次排障 #26/#27 参数化 GOPROXY/APT_MIRROR）；服务器备份 cron 03:10 + 健康看门狗每 5 分钟（2026-10-03 初版）；T-016 已替换为 Docker 容器健康状态，连续 2 次 running/unhealthy 才重启，flock 防重入、30 分钟冷却；停止/启动中/探测失败不自动重启
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

- [x] 备份异地副本（2026-10-07，用户"开始做"授权）：GitHub 私仓 `luohao-blog-backups` + 服务器 ed25519 deploy key（仅该仓写权限）+ `deploy/backup-push.sh`（单 commit force-push 最新镜像，恒定大小）；cron 03:40 每日推送，首推 13 个备份文件验证落库。本地备份已有 Kuma 心跳；异地同步 textfile 指标与 Grafana 告警在本轮完成代码验证，服务器生效待 S4
- [x] 远端分支清理（2026-10-07，用户"继续推进"授权）：删除 21 个已核验远端分支（本任务 4 个特性分支 + ci-probe 探针 + 16 个历史交付分支，逐个经 merged-PR 或 main-祖先核验）；保留 `docs/page-layout-record`、`docs/t012-close` 两个无合并记录分支，远端仅剩 main + 待合并记录分支


- [x] 线上更新 #63+#64（2026-10-06）：服务器 193.112.128.245 `git pull` → **仅重建 frontend**（ghcr 拉取受限走服务器本地构建回退；backend 自上次部署无代码变更，不动以缩小变更面/规避 apt 网络抖动）→ `up -d frontend`；六容器 healthy，公开页全 200，首页/关于页 SSR 已含新邮箱与新 UI，公开 E2E（t01–t11，不含后台登录类）11/11 全绿
- [x] 前台 UI 走查整改：首页改为开屏式介绍落地页（居中 hero + 一行站点数据 + 最新文章/精选项目/订阅，搜索/热门/标签云归位到 /posts、/ranking、/tags 专属页）；顶部导航扁平化（移除「文章」下拉与「收藏」入口，分类/标签/归档改为 /posts 页内浏览入口，收藏/RSS/GitHub 移入页脚导航，导航加当前页高亮，logo 本就回首页）；/settings 昵称行「保存」按钮被挤成竖排的对齐修复；全站按钮统一胶囊形、输入框/卡片 rounded-xl、后台 naive-ui borderRadius 10px + 对话框宽度统一 26rem；E2E t11 气泡选择器改为类名无关（rounded-lg→xl 改版牵连）；frontend lint/typecheck/build 全绿，本地生产栈重建（frontend 镜像从本分支构建）视觉走查亮/暗/移动三态通过，E2E 16/16 全绿（过程数据：本地栈遗留探针账号 ui-visual-probe@luohao.blog 无害留存）；[PR #63](https://github.com/luohao0308/luohao-blog/pull/63) squash 合并（merge `4a9cc73`，required CI 双绿，guard push/PR/merge 三次 consume 全 allow，2026-10-06）
- [x] T-004 M2/S4 管理后台前端：useAuth（内存 token + 401 刷新重试 + refresh cookie 恢复）、admin 守卫与布局、登录页、文章管理表格（状态流转/删除确认）、Milkdown Crepe 编辑器；附带修复后端 update 接口 M1/S2 起恒 400 的存量缺陷（convertArticle 透传 status + 单测）、BFF cookiePathRewrite；`go build/test/vet` 与 `pnpm lint/typecheck/build` 全绿，curl + 浏览器 GUI 冒烟通过；[PR #11](https://github.com/luohao0308/luohao-blog/pull/11) squash 合并（head `71cb60b`，merge `2c4983a`，CI run 36754607113，guard push/PR/merge 三次 consume 全 allow，2026-10-01）
- [x] T-004 M2/S2 认证 API + JWT 中间件：auth 契约（Login/Refresh/Logout/GetMe）、HS256 access JWT（alg 钉死+exp 必填+空密钥拒启）、Redis refresh 会话（GETDEL 原子旋转、httpOnly cookie+header 双通道）、自研 kratos v3 JWT 中间件、登录限流（10 次/5min/IP fail-open）、cmd/seed；`go build/test/lint` 全绿 + 冒烟 30/30（`backend/scripts/smoke-auth.sh`）；经 PR #8 合并（head `a41bb38`，merge `06d97a7`，CI run 36688687955，guard 三步 consume 全 allow）
- [x] T-004 M2/S3 Casbin RBAC：公开文章读取仅返回已发布内容，reader 无法读取草稿，admin 保留管理读取；`go test ./...`, `go vet ./...`, `go build ./...` 与 CI run `36743731504` 全绿；PR #9 squash 合并（head `e0a955b`, merge `b2af83a`, 2026-09-30）；独立 Review 门禁按用户要求移除
- [x] T-005 个人站改版 P2/P3：文章搜索/标签/分页/相邻文章、作品集筛选与详情、移动端和暗色模式；本地 lint/typecheck/build 通过，PR #10 更新 main 后 CI run `36745546102` 两 job 全绿，squash 合并（head `4721d2b`, merge `91e2f5f`, 2026-09-30）
- [x] T-000 仓库初始化与 dev-workflow 接入：`luohao-blog` 建仓并推送 GitHub（Public）；dev-workflow 0.5.0 九包安装；项目画像完成，onboarding ready（证据：`docs/WORKFLOW-ADOPTION.md` 审计记录）
- [x] T-001 M0 脚手架：backend/frontend/deploy/CI 空壳全绿，5+2 提交经 PR #1 合并（merge `13349f6`）
- [x] T-003 M1 内容核心：S1 契约（PR #2，`8d1f8d6`）→ S2 存储与用例（PR #3，`859a8be`）→ S3 Markdown 渲染（PR #4，`a4fe24c`）→ S4 前端 SSR（PR #5，`52a2fb7`），四片全部验收合并；全栈冒烟通过（SSR 页面含渲染 HTML）

## 技术债 (Technical Debt)

_第二轮 review（2026-10-04，全量记录见 `docs/plans/REVIEW-ROUND2-2026-10.md`）后合并账：_

| 项目 | 风险 | 说明 |
|---|---|---|
| 小程序退出未撤销服务器刷新会话 | 中 | 本地已修：退出携带当前刷新令牌，按会话版本等待对应旋转，阻止旧刷新恢复登录或撤销错误会话；固定失败诊断不附令牌。先失败后修复的回归、typecheck/test及独立复审通过，PR #100已交付并生成正式号预览；手机退出撤销尚未实测，本轮服务器版本未变。 |
| ~~useAuth 登出与 in-flight 刷新竞态~~ | 已修复 | #29：logout 先 await 在途 refreshOnce 再撤销会话（`useAuth.ts` logout）；2026-10-06 复核代码在位 |
| ~~运维资产在版本库外~~ | 已修复 | #29：docs/、AGENTS.md、deploy/watchdog.sh 入库，runbook §5.1 补定时任务节（backup 03:10 + watchdog */5 真相源）；本地备份心跳已有 Kuma；异地同步失败/超过 26h 未成功的告警配置本轮验证通过，服务器生效待 S4 |
| ~~LLM 120s vs HTTP server 60s 超时错配~~ | 已修复 | #29：HTTP deadline 提至 150s 并文档化与 llm.timeout 的耦合 |
| ~~page_token 合法大 offset 未钳~~ | 已修复 | #29：offset 限 [0, 9900]（页大小上限内不出 ES max_result_window），越界映射 400，负值同拦 |
| ~~errors.Error 实际走 protojson~~ | 已修复 | #28：codec.go 注释更正（proto 走 protojson、kratos errors 等 non-proto 保持 stdlib 形状）+ TestJSONCodecMarshalKratosError 钉死错误体编码路径；metadata/非法 UTF-8 维持 stdlib 现状（已文档化，不再当缺陷追踪） |
| ~~es.go 零测试 / schema 双源~~ | 部分修复 | #30 补 es_test.go 3 例（索引配置/embedding 输入/维度）；本轮补查询 HTTP 边界（BM25/混合查询/降级/错误响应）回归通过；残留 ent 注释 vs 手写 SQL 漂移 |
| ~~release.yml 镜像发布门禁~~ | 已修复 | `ci-gate` 复用 CI 工作流并作为镜像任务前置依赖；保留 concurrency 与 30 分钟超时 |
| ~~backup.sh 空库假成功~~ | 已修复 | 临时文件 + EXIT 清理，gzip 完整性与 dump 头校验通过后原子重命名 |
| ~~限流器 Incr+Expire 异常路径~~ | 已修复 | `TxPipeline` + `ExpireNX` 保持窗口不延长，并补 Redis 故障回归测试 |
| ~~E2E known_issue 失效~~ | 已修复 | #29 摘除过期标注；2026-10-06 复核套件内无 known_issue=True。限流自撞亦已修复（见下行） |
| ~~E2E 限流自撞~~ | 已修复 | 套件开关 `BLOG_E2E_CLEAR_RATELIMIT=1` 启动前清限流键（默认关，docker exec 通道仅限本地栈；2026-10-06 实测：打满 10 次/5min 限流器后 cleared(1) → t12 立即通过；对生产跑不开启） |
| ~~Caddy 无访问日志 / restore 边服务边恢复 / 容器日志无轮转~~ | 已修复 | 三件全部清账（2026-10-06）：Caddy 访问日志启用；容器日志轮转 compose 六服务 json-file 10MB×3；restore.sh 改为「坏包停服前拒收 → uploads 先行 → 停 backend 灌库 → 回启等健康 → 提示 reindex」，本地栈完整 DR 演练（含灌库失败回启路径） |
| ~~Secure cookie 未启用~~ | 已修复 | T-008/S1 已加入显式 `auth.secure_cookie`；生产 HTTPS 签发/清除 Secure/HttpOnly/Lax Cookie 已实测，本地 HTTP 可显式关闭 |
| ~~v-html + WithUnsafe XSS 面~~ | 基本修复 | #68 移除 goldmark WithUnsafe（原始 HTML 整体省略）+ 链接/图片协议白名单——服务端 content_html 即消毒后的安全源；前端 v-html 消费该输出，残留风险低（引入第二作者时复核一次即可销账） |
| ~~frontend healthcheck 用整页 SSR 探针~~ | 已修复 | Nitro 新增 /health（不触数据层）+ compose 探针改打 /health（2026-10-06 本地栈实弹：frontend ~20s 转 healthy，/health 200，首页/E2E t01–t02 无回归）；backend 探针打 /v1/articles/list 属数据层语义，有意保留 |
| ~~demo 文章软删后 seed 冲突~~ | 代码已修复 | seed 对占用 slug 跳过，`-reset` 也不恢复软删；SQLite+seed 回归通过，服务器待 S4 |
| ~~dev compose 端口发布 0.0.0.0~~ | 已修复 | 六端口（mysql/redis/es/minio）全部绑 `127.0.0.1`（compose config 渲染核验，2026-10-06） |
| ~~CI 杂项~~ | 已修复 | ci.yml 6 个 action 全部 pin 官方 SHA + `permissions: contents: read`（workflow_call 兼容）；backup.sh 轮转改 glob 展开免 ls 空白分词（实跑 KEEP=2 轮转核验）；paths 过滤评估后**不做**——workflow 级 paths 会把 required check 留在 Pending 卡死文档型 PR（官方文档），public 仓库 Actions 免费无收益；release.yml 四个 action 本轮固定官方 SHA（不改变发布行为），服务器版本待 S4 |
| 搜索 "Chinese analyzed" 名不副实 | 低 | standard analyzer 单字切分（M4/S1 的 #16 决策），改文案或上 ik |
| P3 长尾 | 低 | Accept 协商、int64 类型漂移、IP 规范化、ES 启动 ctx 无超时、tags multi_match 低效、TIMESTAMP 2038、reindex 串行（本轮已补分页，不再截断 1000 篇）、seed 不回填索引、chat 无缓存、prompt 无定界符（本轮已补 LLM 完整响应 1MiB 上限）；新设密码已有 1024 UTF-8 字节上限，历史长密码登录兼容保留（T-016），不再作为“密码无上限”追踪——全量见 REVIEW-ROUND2 文档 |
| ent selector 错误扁平化 | 已规避 | data 层返回错误码必须在查询构建前校验（`data.validateStatusFilter` 模式） |
| MinIO 拉取 denied / 宿主 3306 被占 | 低 | 环境类备忘 |

---

_维护方式：任务完成后从“进行中/待办”移动到“已完成”，再更新顶部日期。短期上下文完成或过期后清理；长期经验迁移到 `project-memory/`；历史证据迁移到工作日志。_
