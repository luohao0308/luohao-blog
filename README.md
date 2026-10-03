# luohao-blog

个人技术博客 —— 全栈开发 & AI Agent 方向。

## 技术栈

- **后端**：Go + Kratos（protobuf 定义 API，HTTP + gRPC 同源）
- **前端**：Vue 3 + Nuxt 4（SSR，Tailwind CSS + Naive UI + Milkdown 编辑器）
- **数据**：MySQL 8 · Redis · Elasticsearch 8（BM25 全文检索 + kNN 向量）
- **Agent**：RAG 文章问答（检索增强 + OpenAI 兼容 LLM 接口）
- **交付**：Docker Compose · GitHub Actions（CI + 镜像发布到 ghcr）

## 里程碑

| 阶段 | 内容 |
|---|---|
| M0 | 脚手架（monorepo、Kratos + Nuxt 空壳、本地依赖、CI lint） |
| M1 | 内容核心（文章 CRUD、Markdown 渲染、SSR 页面） |
| M2 | 账号后台（JWT + RBAC、管理后台） |
| M3 | 互动统计（评论、阅读量） |
| M4 | Agent 管道（embedding 入 ES、语义搜索、RAG 聊天窗） |
| M5 | 上线（生产部署、发布流水线、备份与看门狗；域名 HTTPS 待办） |

## 状态

🚀 **已上线运行**：http://193.112.128.245

- ✅ **M0–M4 全部完成**（2026-10-02）：内容核心、JWT + RBAC 账号后台、Milkdown 编辑器、评论先审后显、阅读量 24h 去重、ES + BM25/语义混合搜索、RAG 文章问答（真实 LLM，附出处）。
- ✅ **M5 基本收官**（2026-10-03）：生产 compose 六容器（caddy → BFF → backend + mysql/redis/es）、发布流水线推 ghcr、每日备份 + 健康看门狗、灾难恢复演练。HTTPS 待域名。
- ✅ **两轮全量代码 review + Playwright E2E 套件**（`scripts/e2e/`，16 用例），修复项见 git log（PR #28 起）。
- 开发流程遵循 [dev-workflow](https://github.com/luohao0308/dev-workflow)（delivery guard、PR + required CI 门禁）。

