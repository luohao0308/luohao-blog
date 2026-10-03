# 开发与交付入口

_状态：M0 已初始化 | 更新：2026-09-29_

本页把项目真实可执行的开发命令、Git 策略、服务边界和变更影响集中到一个入口。命令必须来自仓库脚本、配置或 CI，不凭经验猜测。

## 仓库与工作区

| 仓库/路径 | 默认集成分支 | 包管理/运行环境 | 所有权 | 备注 |
|---|---|---|---|---|
| `luohao-blog`（monorepo） | `main` | Go 1.25+（backend）、Node ≥20.19 + pnpm 11（frontend）、Docker（依赖服务） | luohao0308 | 单仓库三目录：backend / frontend / deploy |

## 命令矩阵

| 目的 | 命令 | 工作目录 | 适用条件 |
|---|---|---|---|
| 安装依赖 | `pnpm install` | `frontend/` | 需要 Node ≥20.19 与 pnpm 11；postinstall 自动 `nuxt prepare` |
| 本地启动（前端） | `pnpm dev` | `frontend/` | 开发预览，http://localhost:3000 |
| 本地启动（依赖服务） | `docker compose -f deploy/docker-compose.yml up -d` | 仓库根 | 需要 Docker；MySQL/Redis/ES/MinIO |
| 定向测试（后端） | `go test ./internal/...` | `backend/` | 按改动模块 |
| 全量验证（后端） | `go build ./... && go test ./... && golangci-lint run` | `backend/` | 提交前/发布前 |
| lint/format（前端） | `pnpm lint` | `frontend/` | ESLint 9 + @nuxt/eslint |
| 类型检查（前端） | `pnpm typecheck` | `frontend/` | `nuxt prepare` + `nuxt typecheck`（vue-tsc） |
| 构建（前端） | `pnpm build` | `frontend/` | Nuxt SSR 产物 `.output/` |
| 构建（后端） | `go build ./...` | `backend/` | Kratos 单二进制 `cmd/server` |
| proto 生成 | `make api`（api/）/ `make config`（conf） | `backend/` | buf 插件走 `go run` 固定版本；禁止手改生成物 |
| 创建作者账号 | `go run ./cmd/seed -conf ./configs -email <email> -password <pw> -name <显示名>` | `backend/` | 注册关闭，seed 是唯一建号入口；会先幂等执行迁移 |
| 演示文章 seed | `go run ./cmd/seed -conf ./configs -demo-articles`（`-reset` 覆写已有） | `backend/` | 内容内嵌 `cmd/seed/demoarticles/*.md`；默认仅补缺，不覆盖后台改动 |
| 重建搜索索引 | `go run ./cmd/reindex -conf ./configs` | `backend/` | 删建 ES 索引（含 dense_vector）+ 重灌已发布文章；配 `KRATOS_EMBEDDING_*` 后运行可生成向量 |
| embedding 配置 | 密钥写 `backend/configs/secrets.local.yaml`（Git 忽略；模板见同目录 example），或环境变量 `KRATOS_EMBEDDING_BASE_URL / _API_KEY / _MODEL / _DIMENSIONS` | `backend/` | OpenAI 兼容接口；不配置则索引 BM25-only（自动降级），配置后跑一次 reindex。优先级：secrets 文件 > 环境变量 > 默认值 |
| LLM 配置（S3 预留） | 同上文件 `llm:` 节点（`base_url` / `api_key` / `model`），DeepSeek 如 `https://api.deepseek.com` + `deepseek-chat` | `backend/` | OpenAI 兼容 `/chat/completions`；M4/S3 Chat RPC 生成端使用 |
| JWT 密钥 | 也可写入 secrets 文件 `auth.jwt.secret`（如 `openssl rand -hex 32` 生成），设置后启动无需 `KRATOS_JWT_SECRET` 环境变量 | `backend/` | 空密钥仍 fail-fast；env 注入方式继续有效（secrets 文件优先） |
| 认证冒烟 | `./scripts/smoke-auth.sh` | `backend/` | 需依赖服务已起 + jq；先 `export KRATOS_JWT_SECRET=$(openssl rand -hex 32)` |
| 数据迁移 | 启动时自动幂等 up（golang-migrate，嵌入 `backend/migrations/`） | `backend/` | 无需手动命令；迁移文件进 Git 评审 |
| CI | `.github/workflows/ci.yml` | 仓库根 | backend：lint+build+test；frontend：lint+typecheck+build |
| 后端启动 | `KRATOS_JWT_SECRET=<密钥> go run ./cmd/server -conf ./configs` | `backend/` | 空密钥启动即 panic（fail-fast）；2026-09-30 认证冒烟 30/30 通过 |

## Git 与隔离策略

- Worktree 模式：`disabled`（单人项目，目录即工作区；需要并行实验时临时建 worktree 再改回）
- 分支决策：仅为有意进入 Git 的共享交付创建；本机记忆、临时上下文和只读任务不建分支。
- 分支命名：`feat/*`、`fix/*`、`docs/*`、`chore/*`；本机临时分支 `codex/*`（禁止 push）
- 提交格式：Conventional Commits（`feat|fix|docs|chore|ci|refactor|test: 描述`，英文）
- 集成策略：PR → required CI → squash/merge 到 `main`；独立 Review 仅在 manifest 明确要求时作为门禁
- Git 交付策略：以 `.dev-workflow/manifest.json` 的 `gitPolicy` 为当前机器的执行权限权威；当前为 `manual + user`，push/PR/远端 merge 均需用户逐次确认，AI 执行前跑 `python3 scripts/delivery_guard.py check`。
- 本地流程文件：以 `info/exclude` 的实际 `git check-ignore` 结果为准；已跟踪或被项目规则重新放行的路径必须在交付前处理。
- 流程要求：feature/bug/security/跨模块工作关联 Issue（当前阶段以 TASKS.md 编号代替 GitHub Issue，branch protection 生效后切换为真 Issue）；交付经过 PR 和 required CI，独立 Review 按 manifest 策略执行。
- 自动允许：本地可逆操作（构建、测试、lint、本地分支与提交）。
- 需要确认：push、PR 创建/更新、远端 PR merge（manual + user）。
- 一次性授权：不适用（未启用 auto 模式）；如启用，文件放 `.dev-workflow/authorizations/`。
- 远端强制门：branch protection 未配置（TASKS.md T-002，用户操作）；配置前质量门靠本页约定与人工 Review。
- 高权限操作：`privilegedOperationsDefault=deny`；发布、部署、迁移、回滚、流量、仓库设置、凭据和删除按明确目标另行授权。

AI 执行远端操作前先运行 Core 自带、零第三方依赖的 `python3 scripts/delivery_guard.py check ...`。`actor=user` 时 guard 固定拒绝为 AI 放行；`actor=ai` 时必须提供 `.dev-workflow/authorizations/` 下、权限不宽于 `0600` 的一次性授权 JSON。真正执行前使用 `--consume` 原子记录消耗次数。每类操作都必须提供五分钟内从托管平台读取、并绑定当前仓库/remote/ref/SHA 的 provider 证据；push 证据还要证明非删除、非 force、fast-forward 且目标分支未受保护，merge 证据还要覆盖 PR、CI、独立人工 Review 和分支保护。guard 不读取 Token，也拒绝带凭据、query 或 fragment 的 remote URL。

## 本地服务登记

| 服务 | 启动入口 | 健康/冒烟入口 | 端口策略 | 安全停止方式 |
|---|---|---|---|---|
| MySQL 8.4 | `docker compose -f deploy/docker-compose.yml up -d mysql` | `docker compose ... ps`（healthy） | 宿主 3306 被已有本机 MySQL 占用，容器映射 **3307** | `docker compose stop mysql` |
| Redis 7.4 | 同上（redis） | `redis-cli ping` | 固定 6379 | `docker compose stop redis` |
| Elasticsearch 8.17.4 | 同上（elasticsearch） | `curl http://127.0.0.1:9200/_cluster/health` | 固定 9200 | `docker compose stop elasticsearch` |
| MinIO | 同上（minio） | `curl http://127.0.0.1:9000/minio/health/live` | 固定 9000/9001 | `docker compose stop minio` |
| Nuxt dev | `pnpm dev`（frontend） | `curl http://localhost:3000` | 固定 3000 | 终端 Ctrl-C，删除 `.nuxt/dev` 可清缓存 |
| Kratos server | `KRATOS_JWT_SECRET=<密钥> go run ./cmd/server`（backend） | `curl http://127.0.0.1:8000/v1/articles/list` | 固定 8000（HTTP）/9000（gRPC） | 终端 Ctrl-C；JWT 密钥经 env 注入，不入库 |

## 变更影响矩阵

| 变更类型 | 最低验证 | 需要同步的文档/产物 |
|---|---|---|
| 新增或改变模块边界 | 定向测试 + 静态检查 | `PROJECT-SUMMARY.md`、`architecture/` |
| API/事件/Schema 变化 | 契约测试 + 消费方回归 | `contracts/`、生成物、迁移说明 |
| 数据模型/迁移 | 迁移演练 + 数据断言 | 迁移模板、备份/恢复入口、架构数据说明 |
| 运行时代码/配置/依赖 | 定向测试 + 重启 + 冒烟 | 本页命令、Runbook、配置说明 |
| 部署/基础设施 | 配置校验（`docker compose config -q`）+ Preflight + 回滚演练 | `operations/`、Runbook、观测入口 |
| 团队可复用的重复性故障经验 | 修复回归测试 | `operations/runbooks/` |
| 纯文档 | 链接、格式、事实来源检查 | 对应索引 |

## 完成定义

- 变更范围清晰且没有夹带无关修改。
- 适用检查通过，或未运行项有原因与替代证据。
- 运行时变更完成任务自有服务重启和冒烟。
- 契约、迁移、架构、任务和长期知识已按影响同步。
- 交付摘要包含文件、命令、结果、风险和后续动作。
- 交付状态和证据包含 `committed → pushed → pr_open → ci_passed → merged` 中实际到达的阶段、准确 SHA、PR head/base 与 CI；仅在策略要求时记录独立 Review。
