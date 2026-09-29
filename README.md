# luohao-blog

个人技术博客 —— 全栈开发 & AI Agent 方向。

## 技术栈（规划）

- **后端**：Go + Kratos（protobuf 定义 API，HTTP + gRPC 同源）
- **前端**：Vue 3 + Nuxt 4（SSR/SSG，Tailwind CSS + Naive UI）
- **数据**：MySQL 8 · Redis · Elasticsearch 8（全文检索 + kNN 向量）· MinIO
- **Agent**：Eino + RAG（文章问答、语义搜索，LLM 走 OpenAI 兼容接口）
- **交付**：Docker Compose · GitHub Actions · Prometheus/Grafana/Loki

## 里程碑

| 阶段 | 内容 |
|---|---|
| M0 | 脚手架（monorepo、Kratos + Nuxt 空壳、本地依赖、CI lint） |
| M1 | 内容核心（文章 CRUD、Markdown 渲染、SSR 页面） |
| M2 | 账号后台（JWT + RBAC、管理后台） |
| M3 | 互动统计（评论、阅读量） |
| M4 | Agent 管道（embedding 入 ES、语义搜索、RAG 聊天窗） |
| M5 | 上线（可观测三件套、域名 HTTPS、生产部署） |

## 状态

🚧 **M0 脚手架已完成**（2026-09-29）：`backend/` Go + Kratos、`frontend/` Nuxt 4、`deploy/` 本地依赖编排、CI lint 就绪，见 [.github/workflows/ci.yml](.github/workflows/ci.yml)。下一步 M1 内容核心。开发流程遵循 [dev-workflow](https://github.com/luohao0308/dev-workflow)。
