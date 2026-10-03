# M3 互动统计实施计划

_状态：completed | 更新：2026-10-01 | 关联任务：T-006 | 关联设计：M2 计划（completed）_

## 1. 目标、成功标准与停止条件

- 目标结果：访客可以在文章页看阅读量、发表评论（先审后显）；作者在管理后台审核评论。
- 可验收成功标准：同一 IP 24h 内重复访问同一文章只计 1 次阅读；匿名评论提交后进入 pending，admin 通过后前台可见，未通过的评论前台不可见；公开评论接口有限流。
- 完成后停止条件：不做点赞/收藏（未立项）、不做评论邮件通知、不做 ES/语义搜索（M4）、不做部署（M5）。

## 2. 范围与非范围

### 范围

- backend：articles 加 view_count 列迁移；comments 表迁移；Article 契约加 view_count；新 RPC MarkArticleViewed（公开）与 CommentService（公开 Create + admin List/Approve/Delete）；评论限流（复用 M2 固定窗口模式）；casbin 策略扩展
- frontend：文章页展示阅读量 + 客户端上报浏览；文章页评论区（列表 + 发表表单 + 审核中提示）；管理后台评论管理页（通过/删除）

### 非范围

- 登录评论/READER 注册（注册保持关闭）；点赞、分享、通知；ES（M4）；反垃圾之外的审计

## 3. 用户确认决策（2026-10-01 对话）

1. 切片：3 片，S1 阅读量闭环 → S2 评论后端 → S3 评论前端（每片独立验收回退）
2. 评论身份：匿名（昵称 + 内容），先审后显（pending → approved 才可见；admin 可删除）
3. 阅读量口径：Redis `SET NX EX 24h` 按「IP+slug」去重，命中才 +1；由客户端 onMounted 上报（POST 语义干净，SSR/爬虫预取不计）

## 4. 规模判定与确认

- 规模：large（跨 backend/frontend；阅读量与评论两个可独立验收的结果；2 个有序实施阶段）
- 确认状态：approved（2026-10-01 AskUserQuestion 三项决策）
- Issue：以 TASKS.md T-006 代替

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | 阅读量闭环 | `backend/migrations/000003`、ent Article、article.proto（view_count + MarkArticleViewed）、service/biz/data、authz 策略、`posts/[slug].vue` 展示+上报 | M2 | 冒烟：访问→去重计数→前台展示；负向（重复访问不涨、草稿不可达） | main `2c4983a` | completed + **merged**（[PR #12](https://github.com/luohao0308/luohao-blog/pull/12)，squash merge `30a6f59`，2026-10-01） |
| S2 | 评论后端 | `backend/migrations/000004`、comment.proto（Create 公开 / List+Approve+Delete admin）、ent Comment、biz/data/service、限流 | S1 | 冒烟：发评 pending → 前台不可见 → admin 通过 → 可见；负向（超长/限流 429/未审核不可见） | S1 合并点 | completed + **merged**（[PR #13](https://github.com/luohao0308/luohao-blog/pull/13)，squash merge `5e21499`，2026-10-01） |
| S3 | 评论前端 + 后台审核 | `pages/posts/[slug].vue` 评论区、`pages/admin/comments.vue`、`layouts/admin.vue` 入口 | S2 | 全栈浏览器冒烟：发评 → 后台通过 → 前台可见 → 删除 | S2 合并点 | completed + **merged**（[PR #14](https://github.com/luohao0308/luohao-blog/pull/14)，squash merge `31ef3fd`，2026-10-01） |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 阅读量存储 | MySQL `view_count` 列直接 UPDATE + Redis 24h 去重 | 个人博客流量量级无需 Redis 聚合回写；单行 UPDATE 足够 | 高流量下写放大（当前可接受） |
| 浏览上报 | 独立 POST 端点，客户端 onMounted 调用 | GET 无副作用；SSR/爬虫不计数 | 多一次请求 |
| 评论审核流 | pending → approved；admin 删除 | 无登录体系下防垃圾的唯一强边界 | 评论可见性延迟 |
| 评论限流 | 复用 M2 固定窗口限流器（按 IP） | 模式已验证 | 无 |
| 评论字段 | display_name（≤32 字符）+ content（≤1000 字符），无邮箱 | 最小可用；不留联系方式降低滥用面 | 无法回复到人 |

## 6. 契约、迁移与发布

- 兼容策略：article.proto 加 view_count 为新增字段（向后兼容）；CommentService 全新；MarkArticleViewed 公开（限流+去重兜底）
- 数据迁移：`000003_add_view_count`（articles 加列，前向兼容）；`000004_comments`（新表）；均可重放
- 发布顺序：S1→S2→S3 各自独立 PR
- 回滚/恢复：revert PR；迁移 down 文件

## 7. 偏移控制

- 跨切片共享前置修改：记入本节
- 需要重新确认的变化：范围、顺序、接口、迁移或风险实质变化
- 不需要重新确认：切片内普通实现调整

## 8. 测试与验证矩阵

| 层级 | 场景 | 方式 | 通过条件 |
|---|---|---|---|
| 单元 | view 去重键构造、评论字段校验、状态机流转 | `go test ./internal/...` | 全绿 |
| 集成/契约 | view 上报去重、评论 pending→approved 流转、限流 429 | curl 冒烟 | 断言通过 |
| 迁移 | 000003/000004 可重放 | up/down/up 演练 | 幂等 |
| E2E/冒烟 | 发评→审核→前台可见；浏览→阅读量展示 | curl + 浏览器 GUI（S3） | 断言通过 |
| 安全 | 匿名评论超长/空字段 400、限流 429、未审核评论公开不可见 | 负向用例 | 全部拒绝/不可见 |

## 9. 风险与缓解

| 风险 | 概率/影响 | 缓解 |
|---|---|---|
| 公开评论被刷 | 中/中 | 限流（复用 M2 模式）+ 先审后显 + 字段长度上限 |
| view_count 与去重键不一致（Redis 清空） | 低/低 | 去重失效只是多计一次，可接受 |
| 迁移与 ent 双源漂移 | 中/中 | M2 已有流程：迁移演练 + 基线比对 |

## 10. 交付状态与 PR 证据

## 10. 交付状态与 PR 证据

- 当前状态：S1 **merged**（[PR #12](https://github.com/luohao0308/luohao-blog/pull/12)，head `5f256d11e97397210d6f5042e99e6d253d56492b`，squash merge `30a6f591f99b80a9c74750fefe142465531afe45`，2026-10-01，CI run 36759914938 两 job pass；push/pr/merge guard consume 全 allow，grantId m3s1-*，用户一次授权覆盖三步）
- 备注：首轮 push 后 golangci-lint 抓出 ineffassign（Redis fail-open 分支的无效赋值），修复提交 `5f256d1` 后重过 CI；本机已安装 golangci-lint v2.14.0（`~/go/bin`，此前 WORKING-CONTEXT 记录的缺口已补）
- S2 **merged**（[PR #13](https://github.com/luohao0308/luohao-blog/pull/13)，head `5ba6a41b4ea657422614130ec96f520d4d9bdad6`，squash merge `5e21499031b6fa573654c2d7f8929f40ce534f93`，2026-10-01，CI run 36762574762 两 job pass；guard consume 全 allow，grantId m3s2-*）
- 实现备注：status 过滤用 int 字面量（`status = 1`）——ents.ApplyFilter 不做枚举名转换；repo.Create 缺省由 schema 生成 UUIDv7（显式 SetID(uuid.Nil) 曾导致重复提交 500）
- S3 **merged**（[PR #14](https://github.com/luohao0308/luohao-blog/pull/14)，head `f3ad0b2e07fb9022388caccf4e91f3974b0d1ecc`，squash merge `31ef3fd22cc5d2b971fe740758a720a59ffc249c`，2026-10-01，CI run 36765085833 两 job pass；guard consume 全 allow，grantId m3s3-*）
- **M3 全部切片完成**（S1 阅读 / S2 评论后端 / S3 评论前端）
