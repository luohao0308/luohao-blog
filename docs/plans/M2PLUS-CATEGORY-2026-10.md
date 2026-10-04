# M2+ 分类体系（Category）实施计划

_status: delivered（2026-10-04，三切片全部合并/交付）_
_task: M2+ 内容组织（分类部分）_
_scope: backend / frontend-admin / frontend-public_

## 交付记录

- S1 后端模型与契约：PR #31 squash 合并（merge `96b65f3`，required CI 两项通过，guard push/PR/merge 全 consume allow）
- S2 管理后台：PR #32 squash 合并（merge `d95875d`，required CI 两项通过，guard push/PR/merge 全 consume allow）
- S3 前台导航与聚合页：本 PR（含本计划文档与任务板收尾）

## 目标

为文章引入独立于标签的分类（category）体系，支撑分类导航与分类聚合页。
分类与文章为**单选、可选**关系：一篇文章最多属于一个分类，也可以不属于任何分类（未分类）；
存量文章无需迁移即可继续工作。

## 非目标

- 不做多级分类（仅一层扁平分类）。
- 不改变标签（tags）体系；两者并存。
- 不做分类的软删除（分类行硬删除，删除前先解除文章引用）。

## 切片

### S1 后端：Category 模型与公开契约（`feat/category-s1-backend`）

- 目标结果：分类 CRUD（admin）+ 公开 ListCategories（含已发布文章计数）+ 文章可挂分类
  （创建/更新按 update_mask 路径 `category`）+ ListArticles 支持 `category:"<slug>"` 过滤；
  Article 响应携带分类摘要（slug+name）。
- 修改范围：
  - `backend/internal/data/ent/schema/category.go`（新）：IDMixin+TimeMixin；slug 唯一（≤64）、name（≤64）、sort（int32，默认 0）。
  - `backend/internal/data/ent/schema/article.go`：新增 `edge.To("category", Category.Type).Unique().Optional()`（FK 于 articles 表）。
  - `backend/api/blog/v1/category.proto`（新）：CategoryService——Create/Update/Delete/Get（admin）、List（public，带 article_count）；Article 消息新增 `category` 摘要字段。
  - biz/data/service/server 注册与实现，沿用 Article 现有分层与校验模式（slug 校验、page_size/page_token 钳制、错误码语义）。
  - `backend/openapi.yaml` 经 `make api` 重生成（snake_case 流水线）。
- 验收方式：`go build/vet/test ./...` 全绿（service/data 层新增用例：分类 CRUD 校验、未知分类 slug 挂载报错、category 过滤、计数）；CI required 两项过。
- 回退点：独立 PR squash，revert 即回退；新增表/列对既有读方无破坏（未分类兼容）。

### S2 管理后台：分类管理与文章关联（`feat/category-s2-admin`）

- 目标结果：admin 后台新增分类管理页（列表/新建/改名/排序/删除确认）；文章表单支持选择分类（含"未分类"）。
- 修改范围：`frontend/app/pages/admin/*`、`frontend/app/components/admin/ArticleForm.vue`、必要的 composable（`useCategories`）。
- 依赖：S1 合并。
- 验收方式：pnpm lint/typecheck/build + 浏览器 GUI 冒烟（分类 CRUD、文章设置分类、删除分类后文章回落未分类）。
- 回退点：独立 PR，纯前端可单独 revert。

### S3 前台：分类导航与聚合页（`feat/category-s3-public`）

- 目标结果：主导航新增"分类"入口；`/categories` 总览（含计数）；`/categories/[slug]` 聚合页（服务端 category 过滤，复用文章卡片）。
- 修改范围：`frontend/app/layouts/default.vue`、`frontend/app/pages/categories/*`、`useArticles.ts`（category 过滤参数）。
- 依赖：S1 合并（S2 可并行，不阻塞 S3）。
- 验收方式：pnpm lint/typecheck/build + 预览冒烟（总览/聚合页 200、未分类文章不受影响）。
- 回退点：独立 PR，纯前端可单独 revert。

- 风险与备注（S1 实施中发现）：
  - 数据库：新增 categories 表 + articles 表外键列（ent StorageKey 钉名 `category_id`）；项目使用版本化 SQL 迁移（golang-migrate），S1 新增 `migrations/000005_categories.{up,down}.sql`；上线顺序 = 先部署后端再发前端，旧前端对新字段不感知（proto3 兼容）。
- update_mask 白名单在 S1 增加 `category` 路径（与 #30 引入的校验协同）。
- 分类计数只统计已发布文章，与前台可见性一致。
