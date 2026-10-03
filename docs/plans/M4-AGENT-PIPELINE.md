# M4 Agent 管道实施计划

_状态：completed | 更新：2026-10-02 | 关联任务：T-007 | 关联设计：M3 计划（completed）_

## 1. 目标、成功标准与停止条件

- 目标结果：文章进入 ES 索引，访客可全文搜索；发布内容生成 embedding 支持语义搜索；文章页提供 RAG 问答聊天窗。
- 可验收成功标准：发布/更新/删除文章后 ES 索引同步；关键词搜索命中标题与正文；语义搜索返回相关文章；聊天窗基于检索到的文章内容回答并附引用。
- 完成后停止条件：不做部署变更（M5）、不做评论/阅读量增强（M3 已收口）、不做多租户/权限扩展。

## 2. 范围与非范围

### 范围

- backend：ES 客户端接入（conf + data 层，compose 已有 8.17.4 单节点）；文章索引管道（发布/更新/删除同步写 ES，失败仅告警不阻断写路径）；SearchArticles 契约（BM25，smartcn 中文分析）；embedding 生成与 dense_vector 字段（OpenAI 兼容接口，base URL/key/model 全部环境变量注入）；语义搜索（kNN + BM25 混合）；Chat RPC（检索增强，返回回答 + 引用文章列表）
- frontend：文章列表/独立搜索页搜索入口；文章页 RAG 聊天窗

### 非范围

- 独立向量库（决策记录：ES kNN 一件两用）；消息队列（同步索引，个人博客量级）；流式响应之外的聊天记忆/多轮上下文管理；ES 集群化/安全特性（单节点 security off）

## 3. 用户确认决策（2026-10-01 对话）

1. 切片：3 片，**S1（ES + BM25 关键词搜索）先行**——不依赖任何 LLM 凭据；S2 embedding 管道、S3 语义搜索 + 聊天窗在凭据就绪后开工
2. LLM/embedding：OpenAI 兼容接口，base URL + API key + model 环境变量注入，代码不绑定厂商（用户提供，S2 开工前就绪）

## 4. 规模判定与确认

- 规模：large（跨 backend/frontend；ES 首次接入代码 + 新外部依赖；三个可独立验收的结果）
- 确认状态：approved（2026-10-01 AskUserQuestion 两项决策）
- Issue：以 TASKS.md T-007 代替

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | ES 索引管道 + BM25 搜索 | conf.proto（es 节）、data/es.go（客户端/索引/搜索）、biz EsIndexer 接口与挂接、search.proto、authz 策略、compose 起 ES、前端搜索入口 | M3 | 单测、构建、ES 服务健康检查；发布→索引可查；中文关键词命中；删除→索引移除；负向（ES 停机不阻断写作） | main `31ef3fd` | merged via PR #15 (`645da8e`); local ES integration smoke remains pending |
| S2 | embedding 管道 | conf（openai 兼容节）、embedding 客户端、发布事件生成向量、ES dense_vector mapping 演进、重建工具 | S1 + 用户凭据 | 冒烟：发布→向量入库；缺 key 降级为仅 BM25 | S1 合并点 | merged via PR #20 (`9ad85ef`)；向量入库与降级路径已验证（假 embedding 服务）；真实凭据接入 = 配 env + 跑一次 reindex |
| S3 | 语义搜索 + RAG 聊天窗 | SearchArticles 混合排序、Chat 契约与实现（检索 + 生成 + 引用）、前端聊天窗 | S2 | 全栈浏览器冒烟：提问→带引用回答；无检索结果时明确说明 | S2 合并点 | 代码完成（分支 `feat/m4-s3-chat`）；全链路已验证（真实向量检索 + 假 LLM 浏览器冒烟）；真实 DeepSeek 生成待其 /chat/completions 服务恢复（见 §10 观测） |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| ES 中文分析 | standard（内置） | 2026-10-02 真实空索引初始化证明 stock ES 8.17.4 未安装 smartcn；standard 初始化与中文查询通过 | 中文按字切分，检索精度弱于专用分词插件 |
| 索引同步 | 写路径同步调用 + 失败告警不阻断 | 无 MQ；个人博客量级；最终一致可全量重建 | 索引失败需人工触发重建 |
| LLM 接入 | OpenAI 兼容 HTTP，环境变量配置 | 厂商可替换；key 不入库 | 无 |
| 向量存储 | ES dense_vector + kNN | 一件两用（既有决策），不引独立向量库 | kNN 规模上限（个人博客无虞） |
| 搜索契约 | 独立 SearchArticles RPC | 与 ListArticles 的结构化 filter 职责分离 | 端点略多 |

## 6. 契约、迁移与发布

- 兼容策略：search.proto 全新；article 契约不变；ES mapping 演进按索引重建处理（个人博客数据量小）
- 数据迁移：无 SQL 迁移；ES 索引由代码 ensure 创建
- 发布顺序：S1→S2→S3 各自独立 PR
- 回滚/恢复：revert PR；ES 索引可随时删除重建

## 7. 偏移控制

- 跨切片共享前置修改：conf.proto 的 `llm` 配置节点（base_url/api_key/model/timeout）已随 secrets 文件机制先行加入（2026-10-02），S3 直接消费；本机密钥统一收敛到 git-ignored 的 `configs/secrets.local.yaml`（auth.jwt / embedding / llm），该文件按字段深合并覆盖 config.yaml
- 需要重新确认的变化：范围、顺序、接口、迁移或风险实质变化
- 不需要重新确认：切片内普通实现调整

## 8. 测试与验证矩阵

| 层级 | 场景 | 方式 | 通过条件 |
|---|---|---|---|
| 单元 | 索引文档构造、搜索请求组装、降级路径（ES 不可达） | `go test ./internal/...` | 全绿 |
| 集成 | 发布→索引、更新→重索引、删除→移除；中文关键词命中 | curl + ES 冒烟 | 断言通过 |
| E2E | 前端搜索入口→结果页 | curl/浏览器 | 断言通过 |
| 安全 | 搜索接口公开但只返回已发布内容 | 负向用例 | 草稿不可见 |

## 9. 风险与缓解

| 风险 | 概率/影响 | 缓解 |
|---|---|---|
| ES 不可达拖慢写路径 | 中/中 | 索引调用带超时 + 失败仅告警；写路径不等待重试 |
| smartcn 分词效果不足 | 中/低 | S1 先交付可用搜索；效果不满意后续评估 ik 自建镜像 |
| embedding API 不稳定/费用 | 低/中 | 失败降级 BM25；key/模型可配置 |

## 10. 交付状态与 PR 证据

- 当前状态：计划 approved，S1 merged（PR #15，head `38fea4b`，merge `645da8e`），analyzer 修复 PR #16 已合并；S2/S3 deferred by user；当前本地实现分支 `main`
- repo / remote：luohao-blog / origin

### 2026-10-02 本地集成验证与修复

- Docker Desktop 已启动；MySQL/Redis healthy，ES 恢复后 green。MinIO 镜像拉取 denied，未启动。
- 原有 `articles` 索引无 mapping，最初生命周期冒烟无法证明初始化正确；在确认文档数为 0 后重建，发现 smartcn 未安装导致 mapping 创建 400。
- 修复分支 `fix/m4-s1-standard-analyzer` 改用内置 standard；空索引初始化、中文搜索、删除后文档数归零通过。
- 发布、BM25/标题查询、更新与重索引、删除移除通过；暂停 ES 时创建/发布/公开读取/删除仍成功。
- 后端 build/test/vet/golangci-lint v2.14.0 全部通过。临时后端已停止，依赖容器保留运行；测试文章已软删除。
- analyzer 修复已经 PR #16 squash 合并（head `28077dc`，merge `e086e92`）；S1 交付闭环完成。

### 2026-10-02 S2 embedding 管道实现（分支 `feat/m4-s2-embedding`）

- conf.proto 新增 `embedding` 节点（base_url/api_key/model/dimensions/timeout，全部可经 `KRATOS_EMBEDDING_*` 环境变量注入）；config.yaml 默认空 = 禁用。
- `data/embedding.go`：OpenAI 兼容 `/embeddings` 客户端；配置不完整返回 nil embedder；向量按输入序重排并校验维度。
- `data/es.go`：mapping 按 embedder 有无加 dense_vector（cosine, index=true）；IndexArticle 发布文档带向量；**embedding 失败降级 BM25-only（告警不阻断）**；启动时检测存量索引缺向量字段则告警提示 reindex。
- `biz.ArticleSearchIndex` 新增 `RecreateIndex`（mapping 演进路径）；重建工具 `cmd/reindex`：删建索引 + 重灌全部已发布文章 + 强制 refresh。
- 验证（本地 ES 8.17.4 + 假 embedding 服务 8 维）：A 正常→4 篇向量入库 + BM25 不受影响；B API 500→4 条降级告警、文档照常入库；C 未配置→BM25-only mapping（与 S1 一致）；服务端真实写路径 create→publish→向量 8 维入库→删除→文档移除。
- 单测：embedding 客户端 httptest 用例（请求 schema/bearer/乱序重排/维度不匹配/HTTP 错误/空输入）。
- 运维注意：`go run` 的子进程 argv 是 `/tmp/go-build*/exe/server`，`pkill -f "cmd/server"` 杀不掉，须 `pkill -f "exe/server"`；登录限流 10 次/5min 会挡连续冒烟（清 `blog:ratelimit:*`）。
- PR #20 squash 合并（head `952a93`，merge `9ad85ef`，required CI 两项通过）。交付事故记录：合并前误执行 `git push origin --delete` 删掉了 PR head 分支导致 PR 被关闭——恢复方式：本地同 SHA 重推 + `gh pr reopen`（CI 重跑后 CLEAN），merge 授权绑定同一 SHA 的第二次 consume 完成合并。教训：merge 命令不携带任何 delete 动作；删除类命令必须独立执行、单独核对目标。

### 2026-10-02 S3 语义搜索 + RAG 聊天窗（分支 `feat/m4-s3-chat`，基于 secrets 分支）

- 真实凭据激活：SiliconFlow key（用户手填时丢末字符致 401，比对控制台掩码补齐后 200）；4 篇已发布文章的 bge-m3 1024 维真实向量入库；语义检索实证：3 个换说法查询全部第一名命中正确文章。
- **DeepSeek 观测（2026-10-02）**：key 有效（/models 秒回），但 /chat/completions 全变体（默认/low/none effort）挂起分钟级无响应——外部服务状态问题。chatLLM 客户端 120s 超时触发后按设计降级为「引用列表回答」。`conf.Llm.reasoning_effort` 配置已就位（none 时秒回，DeepSeek 恢复后真实生成即用）。
- 实现：chat.proto（Chat RPC + CitedArticle）；error_reason 扩 CHAT_*；es.go 混合检索（kNN boost 2.0 + BM25 分数求和，embedder 缺失/索引未重建/查询 embedding 失败三种回落 BM25）；biz.ChatUsecase（校验→限流→检索→生成→引用，无结果不调 LLM，LLM 失败降级引用回答，grounding prompt 强制「不编造、无覆盖就说没有」）；data/llm.go + NewChatRateLimiter（10 次/5min/IP 默认）；policy.csv 公开行；wire 重新生成。
- server.http/grpc timeout 1s→60s（chat 生成本来就超过 1s，这是 config.yaml 的本意修正）。
- 验证：单测 11 新用例全绿（chat 用例 7 + llm 客户端 4）；真实 SiliconFlow 向量 + 假 LLM 的 API 级 E2E（正常/无结果/参数校验/混合搜索）；**浏览器全栈冒烟通过**（文章页提问→回答气泡→4 条出处链接，暗色模式截图验收）。
- 运维补充：孤儿 server 的可靠清理是**按端口杀**（`lsof -ti :8000 -ti :9000 | xargs kill`）——`pkill -f "exe/server"` 匹配不到 go-build 缓存二进制（`~/Library/Caches/go-build/.../server`）。
