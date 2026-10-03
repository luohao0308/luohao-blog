# M1 内容核心实施计划

_状态：completed | 更新：2026-09-30 | 关联任务：T-003 | 关联设计：README 里程碑 / 根 AGENTS.md 项目扩展区_

## 1. 目标、成功标准与停止条件

- 目标结果：博客能发布和阅读文章——article 契约 + MySQL 存储 + Markdown 渲染 + Nuxt SSR 文章页/列表页，替换全部 todo demo。
- 可验收成功标准：通过 HTTP 创建一篇文章后，`GET /v1/articles/{slug}` 返回渲染 HTML；前端 `posts/[slug]` SSR 页面输出文章标题与正文；`pnpm build` 与全量后端验证通过；无 todo 残留引用。
- 完成后停止条件：S1–S4 全部验收并合并；不做评论、后台编辑器、ES 索引（M2/M4 范围）。

## 2. 范围与非范围

### 范围

- `backend/api/blog/v1/`（article 契约与生成物）、`internal/{biz,service,data,server}` 对应实现、`internal/data/ent` schema、`configs/`
- 渲染：goldmark + chroma（GFM、代码高亮），写入时渲染缓存 HTML 列
- 前端：`app/pages/`（index/posts/[slug]/archives 接真数据）、`composables/`、Nuxt server route 代理
- 删除：todo demo（api/todo、service/data/server 注册、ent todo schema、测试）

### 非范围

- 管理后台与编辑器（M2）、JWT/RBAC（M2）、评论与阅读量（M3）、ES/embedding（M4）、容器化部署（M5）

## 3. 当前证据基线

- 代码/配置：M0 骨架已合并基线（PR #1，SHA 09d0143 + CI 修复 1478f6a）；layout 含 ent + MySQL 连通（3307）
- 测试/CI：backend build/test/lint 与 frontend 全绿（本地）；CI 修复等待推送
- 契约/数据：api/todo 为模板 demo；ent auto_migrate 开启（configs/config.yaml）
- 运行事实：`GET /v1/todos/list` 冒烟通过；mysql/redis 容器 healthy
- Unknown：goldmark 中文标题→slug 规则细节（S1 内定，限定 `[a-z0-9-]`，中文标题手动或拼音转换策略 S1 内验证）

## 4. 规模判定与用户确认

- 规模：large
- 触发信号：跨 backend/frontend 两模块 + 四个有序切片 + 多个可独立验证结果（满足两项以上）
- 确认状态：approved
- 用户确认时间或消息指针：2026-09-29 对话确认「按 S1–S4 批准（推荐）」（AskUserQuestion 答复）
- 用户调整：待记录
- Issue：以 TASKS.md T-003 代替（branch protection 生效后切 GitHub Issue）

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | article 契约替换 demo | backend/api/blog/v1、service/data/server 的 todo 移除、契约文档 | M0 合并 | buf generate + build/test 全绿 + 契约登记 | PR #1 合并点 | completed（本地验收过，交付状态见 §11） |
| S2 | 文章存储与用例 | ent schema（Article/Tag）、biz usecase + repo、data 实现 + 单测 | S1 | 单测（SQLite 内存）+ build/test | S1 合并点 | completed（本地验收过；service CRUD 一并落地，S3 收窄为渲染，见切片内调整） |
| S3 | HTTP API + Markdown 渲染 | service 转换层、goldmark 渲染、server 注册 | S2 | 冒烟：create→get 返回 HTML | S2 合并点 | completed + **merged**（PR #4，merge `a4fe24c`） |
| S4 | 前端 SSR 文章页 | Nuxt server 代理、index/posts/[slug]/archives | S3 | typecheck/build + preview HTML 断言 | S3 合并点 | completed + **merged**（PR #5，merge `52a2fb7`） |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| ORM | ent（layout 原生） | schema 即事实源、自带 SQLite 测试缝、与 auto_migrate 已联动 | 放弃 GORM；版本化迁移治理延后 |
| 迁移策略 | M1 沿用 ent auto_migrate（仅本地开发），golang-migrate 推迟到 M2 后台引入时决策 | schema 早期演进频繁，双轨维护不值；风险已记录 | 生产级迁移门禁未建立（M2 解决） |
| Markdown 渲染 | 写入时渲染存 `content_html` 列 | 读多写少、查询直出、前端零渲染逻辑 | 更新必须重渲染（repo 层封装） |
| 前后端联通 | Nuxt server route 代理 `/api/*` → backend:8000 | 免 CORS；生产 Caddy 同路径规则一致 | 多一跳转发（可忽略） |
| slug 规则 | 手写 slug（唯一索引、`[a-z0-9-]`），不自动拼音转换 | 可控、SEO 友好；中文标题手动定 slug | 创作时多一步 |

默认开发方法：先定义可验证行为，再按风险选择测试或 Eval；不要求每个任务启用全部验证方法。

## 6. 实施切片

### S1：article 契约替换 demo

- 状态：completed（2026-09-29 本地验收；交付状态见 §11）
- 修改范围：`backend/api/blog/v1/*.proto`、api 生成物、`internal/service|data|server` todo 引用、`docs/contracts/`
- 切片内调整（不触发重新确认）：① ent schema（Article/Tag）从 S2 提前到 S1——ent 代码生成要求至少一个 schema，契约与数据模型同期评审也更内聚；② S1 交付传输壳层（service 方法返回 Unimplemented），biz usecase 与 data repo 的接线（ProviderSet/wire）随 S2 恢复，因此 wireApp 签名暂不含 *conf.Data；③ 标签在契约上为 `repeated string tags`，ent 侧为 Article↔Tag M2N。
- 步骤：定义 Article CRUD proto（AIP 风格，slug 为公共标识符）→ `make api` → 删 todo（api/ent schema/service/data/server）→ ent 重生成（Article/Tag）→ wire 重生成 → build/test/lint → 冒烟
- 切片验收：`go build ./... && go test ./...` 全绿 ✓；`golangci-lint run` 0 issues ✓；openapi.yaml 更新 ✓；`docs/contracts/README.md` 契约索引登记 ✓；冒烟 `GET /v1/articles/list` → 501 Unimplemented（路由与服务已接）✓
- 回退点：PR #1 合并点（revert 单 PR）

### S2：文章存储与用例

- 状态：completed（2026-09-29 本地验收；交付状态见 §11）
- 修改范围：`internal/data/ent/schema/`（Article/Tag）、`internal/biz/article/`、`internal/data/article.go`
- 切片内调整（不触发重新确认）：① service 层真实 CRUD 在本切片落地（原计划归 S3 的转换层提前），S3 相应收窄为 goldmark 渲染 + content_html 填充 + 渲染冒烟；② wire/data/biz 依赖图在 S2 恢复完整（S1 暂离的部分）；③ 契约 `body: "article"` 的语义 = HTTP body 即 Article 字段本身（非包裹），冒烟中验证并以此为准。
- 步骤：biz usecase（slug 校验 `^[a-z0-9-]+$`、强制 DRAFT 创建、发布时间戳转换、DELETED 只能走 DeleteArticle）→ data repo（tag upsert、软删除、约束映射 409）→ service 转换层 → wire 重生成 → 单测（SQLite 内存）→ MySQL 冒烟
- 切片验收：`go build ./... && go test ./...` 全绿 ✓；`golangci-lint run` 0 issues ✓；单测覆盖 repo CRUD/分页/排序稳定性/软删除/slug 冲突 + usecase 校验/状态机 ✓；MySQL 冒烟 create→get→409 冲突 ✓
- 回退点：S1 合并点（PR #2 merge commit `8d1f8d6`）

### S3：HTTP API + Markdown 渲染

- 状态：completed（2026-09-30 本地验收；交付状态见 §11）
- 修改范围：`internal/biz/article/render/`（新包）、`internal/biz/article.go`、`internal/data/ent/schema/`（字段类型）、ent 生成物
- 切片内调整（不触发重新确认）：① API 与转换层已在 S1/S2 落地，本切片聚焦渲染；② 渲染选项定版：GFM + 自动标题 ID + 硬换行（中文写作习惯）+ 保留作者原始 HTML（信任作者，M2 加权限门后复核）+ chroma CSS class 模式（主题归前端）；③ 暴露并修复一个 S2 遗留问题——Tag 侧缺少反向 edge 使 M2M 退化成 tags 表上的 O2M 外键，标签无法被多篇文章共享；④ Article 长文本字段从 VARCHAR(191) 改为 Text（渲染 HTML 超长导致 Data too long）。
- 步骤：render 包 + 单测 → usecase 创建/更新接入渲染 → ent 字段/edge 修正 → 全量验证 → MySQL 冒烟（两篇文章共享标签 + 渲染断言）
- 切片验收：build/test/lint 全绿 ✓；冒烟 `content_html` 409 字符（chroma/table/h1 断言通过）✓；M2M 共享标签 ✓
- 回退点：S2 合并点（PR #3 merge commit `859a8be`）

### S4：前端 SSR 文章页

- 状态：pending
- 修改范围：`frontend/server/api/`（代理）、`app/pages/{index,posts/[slug],archives}.vue`、`app/components/`（ArticleCard 等）、`app/composables/useArticles.ts`
- 步骤：server route 代理 → composable → 列表页/详情页/归档页 → `pnpm typecheck && pnpm build` → preview 冒烟
- 切片验收：preview 页面 SSR HTML 含文章标题与正文片段；typecheck/build 绿
- 回退点：S3 合并点

## 7. 偏移控制

- 当前允许修改的切片范围：各切片列出的路径；跨切片共享前置（如统一错误码）记入本节后再动
- 跨切片共享前置修改：无
- 需要重新确认的变化：范围、顺序、接口、迁移或风险发生实质变化
- 不需要重新确认的变化：已确认切片内部的一般实现细节调整

## 8. 契约、迁移与发布

- 兼容策略：M1 无外部消费者，proto 变更自由；自 S1 登记后按 contracts 变更清单执行
- 数据迁移/回填：ent auto_migrate（本地）；无回填
- 发布顺序：S1→S2→S3→S4 各自独立 PR，逐片合并
- 回滚/恢复：revert 对应 PR；auto_migrate 产生的表可 drop 重建（本地无数据价值）

## 9. 测试与验证矩阵

| 层级 | 要证明的声明/场景 | Test/Eval/Check | 命令/入口 | 通过条件 |
|---|---|---|---|---|
| 单元 | repo CRUD/分页/标签过滤；usecase 状态机；转换正确性 | Test（SQLite 内存） | `go test ./internal/...` | 全绿 |
| 契约 | proto→生成物一致；openapi 更新 | make api + diff 检查 | `make api`、`git diff --exit-code` 二次生成无漂移 | 无漂移 |
| 集成/渲染 | Markdown→HTML（高亮/表格/中文） | Test（渲染用例） | `go test ./internal/biz/article/...` | 全绿 |
| E2E/冒烟 | create→get HTML→SSR 页面含正文 | curl + preview 断言 | 冒烟脚本（S3/S4 内联） | 断言通过 |
| 观测/部署 | 请求日志含 trace_id | 冒烟时人工查看 | server 日志 | 有 trace_id |

## 10. 风险与缓解

| 风险 | 概率/影响 | 早期信号 | 缓解/恢复 |
|---|---|---|---|
| 删 demo 引用残留导致编译失败 | 中/低 | go build 报错 | grep 全仓 `todo` 清点后删除；S1 单独 PR 便于定位 |
| ent 多对多与 MySQL 8.4 兼容 | 低/中 | auto_migrate 报错 | layout 已验证 ent+MySQL；必要时降级为 JSON 列存 tags（记录决策） |
| goldmark 高亮样式与前端 Tailwind 冲突 | 中/低 | 页面样式乱 | chroma 输出 class 模式 + 前端提供样式表；S4 内处理 |
| 代理路径与 Caddy 生产规则不一致 | 低/中 | preview 404 | 统一 `/api/*` 约定并写入 contracts 文档 |

## 11. 交付状态与 PR 证据

- 当前状态：S1–S4 全部 **merged**
- repo / remote：luohao-blog / origin
- PR 编号或链接：[PR #2](https://github.com/luohao0308/luohao-blog/pull/2)（S1）/ [PR #3](https://github.com/luohao0308/luohao-blog/pull/3)（S2）/ [PR #4](https://github.com/luohao0308/luohao-blog/pull/4)（S3）/ [PR #5](https://github.com/luohao0308/luohao-blog/pull/5)（S4）；M0 基线 = PR #1（merge `13349f6`）
- source ref / target ref：feat/m1-s{1,2,3,4}-* → main
- exact head SHA：S1 `5a46feb` · S2 `0e044dd` · S3 `a714680` · S4 `13030e9`
- required CI 结果与时间：四个 PR 两 job 全部 pass（2026-09-29/30）
- merge commit：S1 `8d1f8d6` · S2 `859a8be` · S3 `a4fe24c` · S4 `52a2fb7`

按实际到达的阶段顺序更新；没有证据的后续阶段不得提前标记。实现者不得作为唯一审批者，AI review 不计作独立批准。

## 12. 文档同步

- [ ] `TASKS.md` / 上下文
- [ ] `PROJECT-SUMMARY.md`
- [ ] 架构/ADR
- [ ] 设计/契约/生成物
- [ ] Runbook/工作日志

## 13. 完成定义

- [x] 大型计划已获得用户确认并记录切片版本，或本计划已标记为 `small / not_required`。（2026-09-29 对话批准 S1–S4）
- [x] 所有切片验收通过，且过程状态按顺序更新。（S1–S4 全部 completed + merged）
- [x] 适用测试、构建、迁移、重启和冒烟通过。（各切片证据见实施段）
- [x] 契约、文档和长期知识已同步。（契约索引登记；PROJECT-SUMMARY 更新）
- [x] 最终证据、SHA/产物身份和剩余风险已记录。
- [x] 如已进入远端交付，PR、CI 和当时适用的交付门禁证据完整；merge 经 fail-closed guard 校验。（PR #2/#3/#4/#5 均经 guard --consume）
