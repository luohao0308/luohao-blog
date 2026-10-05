# Task Board

_last-updated: 2026-10-04_

> **唯一用途**：记录当前进行中、明确待办、阻塞和技术债。稳定事实写入架构/设计文档，详细验证过程写入工作日志（如项目启用）。
>
> 状态：`[~]` 进行中或阻塞 | `[ ]` 已批准但尚未开始 | `[x]` 仅出现在“已完成”章节。
>
> 当前任务的临时目标、决策、交接提示和验证摘要写入 [WORKING-CONTEXT.md](WORKING-CONTEXT.md)，不要复制到本文件。

---

## 进行中 (In Progress)

| ID | 任务 | 范围/仓库 | 上下文 | 阻塞 |
|---|---|---|---|---|
| T-008 | M5 上线基本收官：S1-S4 全部完成（S1 #23；S2 公网 IP 直访；S3 发布流水线 #25+#26+#27——Actions 推 ghcr 实跑绿，服务器更新走本地构建回退（ghcr 国内拉取受限）；S4 备份 cron+看门狗已装）；HTTPS 待域名 | backend + deploy + CI | 计划 `docs/plans/M5-DEPLOY.md` | 生产管理员改密：API+后台入口已上线（PR #47 merge `7ef5439`），**等用户在后台完成改密后销项**；HTTPS 待域名；**服务器待部署 #28 修复（见技术债）** |

## 待办 (Todo)

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
| **运维资产在版本库外** | **高** | watchdog.sh（服务器 untracked、cron */5）、backup cron（仅服务器 crontab）、**docs/ 与 AGENTS.md 整体未入 git**——服务器重建即失传；备份单机无异地副本、cron 静默失败无告警。修法：watchdog.sh 入库 + docs/AGENTS.md 提交推送 + runbook 补定时任务节 |
| **useAuth 登出与 in-flight 刷新竞态** | **高** | logout 后 in-flight refresh 会"复活"会话（假登录，#28 单飞修复的镜像 bug）：`useAuth.ts:137` + `biz/auth.go:156`；修法 logout 先 await refreshOnce 或引入 session epoch |
| LLM 120s vs HTTP server 60s 超时错配 | 中 | reasoning 模型下聊天 60s 必失败：`data/llm.go:41` vs `configs/config.yaml:5`，统一并文档化耦合 |
| errors.Error 实际走 protojson | 中 | codec.go"stdlib 形状"注释错误（内嵌 Status 满足 proto.Message）；错误体多 `metadata:{}`；非法 UTF-8 → 裸 500 空体；需特判 + 钉死编码路径测试 |
| page_token 合法大 offset 未钳 | 中 | 解析失败已修，但合法编码的巨大 offset 直通 ES from（>10000 → 500）与 MySQL OFFSET：`service/search.go:45`，补 offset 上限 |
| release.yml 镜像发布门禁 | 已修复 | `ci-gate` 复用 CI 工作流并作为镜像任务前置依赖；保留 concurrency 与 30 分钟超时 |
| 搜索 "Chinese analyzed" 名不副实 | 低 | standard analyzer 单字切分（M4/S1 的 #16 决策），改文案或上 ik |
| Caddy 无访问日志 / restore 边服务边恢复 / 容器日志无轮转 | 中 | 可观测性与恢复安全三件 |
| backup.sh 空库假成功 | 已修复 | 临时文件 + EXIT 清理，gzip 完整性与 dump 头校验通过后原子重命名 |
| E2E known_issue 失效 + 限流自撞 | 中 | P0 已修但标注未摘（回归会静默放行）；一轮耗 7 次登录限流连跑必挂 |
| demo 文章软删后 seed 冲突 | 低 | `cmd/seed/demo_articles.go:146` 存在性判断应含 DELETED |
| 限流器 Incr+Expire 异常路径 | 已修复 | `TxPipeline` + `ExpireNX` 保持窗口不延长，并补 Redis 故障回归测试 |
| dev compose 端口发布 0.0.0.0 | 低 | MySQL/Redis/ES/MinIO 对局域网暴露，建议 `127.0.0.1:` 前缀 |
| CI 杂项 | 低 | ci.yml 第三方 action 未 pin SHA、无 permissions 块、无 paths 过滤；backup.sh 清理按空白分词 |
| Secure cookie 未启用 | 中 | 等域名+TLS（conf.proto 改动需 buf）；TLS 前公网登录明文 |
| es.go 零测试 / schema 双源 | 中 | httptest 可覆盖；ent 注释 vs 手写 SQL 漂移是"SQLite 测试过但 MySQL 不同"的根因 |
| v-html + WithUnsafe XSS 面 | 中 | 信任边界=仅 admin 可写；引入第二作者前必须 sanitize |
| frontend healthcheck 用整页 SSR 探针 | 中 | 数据层抖动即 unhealthy 并绑架 caddy 启动；建议 Nitro /health |
| P3 长尾 | 低 | Accept 协商、int64 类型漂移、IP 规范化、gui-test-screenshots 未 ignore、ES 启动 ctx 无超时、tags multi_match 低效、TIMESTAMP 2038、reindex 1000 篇/串行、seed 不回填索引、软删占坑 slug、chat 无缓存、prompt 无定界符、LLM body 无上限、密码无上限——全量见 REVIEW-ROUND2 文档 |
| ent selector 错误扁平化 | 已规避 | data 层返回错误码必须在查询构建前校验（`data.validateStatusFilter` 模式） |
| MinIO 拉取 denied / 宿主 3306 被占 | 低 | 环境类备忘 |

---

_维护方式：任务完成后从“进行中/待办”移动到“已完成”，再更新顶部日期。短期上下文完成或过期后清理；长期经验迁移到 `project-memory/`；历史证据迁移到工作日志。_
