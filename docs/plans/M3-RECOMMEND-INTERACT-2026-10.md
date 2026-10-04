# M3 内容推荐体系 + 用户互动能力实施计划

_status: approved（2026-10-04，用户批准四切片）_
_task: M3 待办两项（内容推荐体系、用户互动能力）_
_scope: backend / frontend-public_

## 目标

补齐 M3 两项待办：内容推荐体系（相关文章推荐、热门内容推荐位）与用户互动能力
（点赞、收藏、评论体验增强）。站点无访客账号体系：点赞为服务端计数（客户端去重），
收藏为浏览器本地存储方案，两者均在界面明示语义。

## 非目标

- 不引入访客账号/登录体系。
- 不做推荐算法（无协同过滤/向量召回）；相关性 = 标签/分类重叠度。
- 不改评论数据模型。

## 切片

### S1 点赞后端（`feat/m3-likes-backend`）

- 目标结果：`articles.like_count` 计数 + 公开 `POST /v1/articles/{slug}/like`；
  Redis 24h 按客户端去重（复用 view 计数模式），仅已发布文章可点赞。
- 修改范围：迁移 000006、ent schema、article.proto（like_count 字段 14 + MarkArticleLiked
  RPC）、biz/data/service、policy.csv 公开路由、openapi 重生成。
- 验收方式：go build/vet/test 全绿（去重、计数、未发布拒绝用例）；CI 两项过；服务器实弹。
- 回退点：独立 PR revert；列为增量列，无破坏。

### S2 点赞 + 收藏前端（`feat/m3-interact-frontend`）

- 目标结果：文章详情点赞按钮（乐观更新，localStorage 记忆已点赞）；收藏开关 +
  `/collections` 收藏页（localStorage，无账号，界面标注"仅保存在本浏览器"）。
- 依赖：S1 合并。
- 验收方式：lint/typecheck/build + 预览冒烟（点赞态、收藏页读写）。
- 回退点：独立 PR，纯前端。

### S3 内容推荐体系（`feat/m3-recommend-frontend`）

- 目标结果：文章详情相关文章（分类/标签重叠度排序，前 3，无重叠回退最新文章）；
  文章列表页热门文章推荐位（view_count 前 3）。
- 依赖：无（可与 S1/S2 并行；顺序实施）。
- 验收方式：lint/typecheck/build + 预览冒烟。
- 回退点：独立 PR，纯前端。

### S4 评论体验增强（`feat/m3-comment-ux`）

- 目标结果：详情页评论数展示、发表后待审状态清晰化、失败重试、空态文案——以现有
  CommentSection 实现取最小增量。
- 验收方式：lint/typecheck/build + 预览冒烟。
- 回退点：独立 PR，纯前端。

## 风险与备注

- 迁移 000006 为增量列（bigint unsigned DEFAULT 0），与 view_count 同型；上线即生效。
- 点赞端点公开但按客户端去重（24h 窗口）+ 限流语义与 view 一致；NAT 后多用户共享出口 IP
  时 24h 内计一次——与阅读量口径一致，可接受。
- proto 声明顺序遵守静态路由先于参数路由的约定（PR #35 教训）。
