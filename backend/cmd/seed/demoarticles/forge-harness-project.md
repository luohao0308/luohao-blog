---
slug: forge-harness-project
title: Forge Harness：围绕一次任务组织 Agent 运行时
summary: 从版本化任务计划到知识、工具、上下文与评测，介绍我实现的 Agent 控制面与本地工作区。
tags: Python, FastAPI, React, AI Agent, 个人项目
status: draft
---

Forge Harness 是我设计与开发的个人 Agent 工程项目，面向私有化部署场景构建 Agent 控制面。它将模型、上下文、知识、记忆、工具、策略、执行、评测与审计组织进同一个运行时，采用 Model + Harness = Agent 的架构思路。

项目包含 React / TypeScript 控制台、Python / FastAPI 服务和 Electron 本地工作区。服务侧使用 PostgreSQL、Redis、SQLAlchemy、Alembic、Dramatiq 与 asyncio；本地端使用 SQLite 保存运行记录，Docker Compose 提供服务部署入口。

## 从目标进入版本化计划

主链路从 Agent Studio 配置开始，由 Workspace 接收目标，Planner 生成版本化 Plan DAG，再交给 Executor、Subagent 或 Multi-Agent 执行。计划图表达步骤之间的依赖，让目标、计划与后续动作建立明确联系。

我基于 Event Store 持久化 Run、Step、ModelCall、ToolCall、Approval 与成本信息，并通过 SSE 向界面提供运行更新。暂停、恢复、取消、重试、回放和故障诊断都围绕这些运行记录展开。排查一个任务时，可以沿着具体步骤查看模型调用、工具动作与审批记录。

## 知识检索要能追到原始依据

Knowledge / RAG 模块覆盖知识源生命周期、文档版本、分块、检索与引用快照，并处理作用域隔离和迁移恢复。我将检索证据、Prompt Manifest、Context Manifest 与 ModelCall 关联，让回答使用的资料和模型实际接收的上下文能够对应起来。

中文检索增加了 CJK lexical fallback，证据不足时停止依赖不足的回答路径。文档发生更新后，版本与引用快照保留了定位依据，使后续回看能够理解当时使用的内容。

项目知识另有面向工作目录的接入路径：从可信根目录扫描，应用 ignore 与扫描预算，维护文档版本和 tombstone，并使用 project:// 引用连接到项目文件。

## Web Research 与外部知识连接器

我接入了 Tavily Web Research，以及 Dify / Coze 知识连接器。外部资料进入任务前，调用链检查 provider、key 与 query，并对 URL、DNS、IP 执行访问策略，应用 allow/deny 规则与私网阻断。

调用次数和 attempt ledger 记录具体尝试，引用与实际来源绑定。这一层使外部研究能够保留请求和来源轨迹，并与站内 RAG 共同成为任务的证据来源。

## 在 Token 预算内组织上下文

Memory 支持组织、Agent、用户与 Run 等作用域，记录来源、策略标记和过期删除。Context Router V2 在 Token 预算内组织历史消息、固定上下文、压缩摘要、Memory、RAG、Web 与工具证据。

我采用确定性的上下文编排，并持久化上下文清单。这样一次模型调用使用了哪些材料、为何进入当前上下文，以及相应成本和策略依据，都可以沿清单检查。长期记忆与单次运行上下文因此有了各自的维护位置。

## 工具能力、版本与执行权限

能力链从 CapabilityRegistry 进入 AgentCapabilityAttachment，再关联 CapabilityVersion，最终由 ToolRunner 执行。它支持能力发现、安装配置和版本快照，接入 MCP HTTP / SSE / stdio，以及 GitHub、Slack、Sandbox 等适配器。

工具调用统一经过 Policy、Approval、Sandbox、Idempotency 与 Audit。未绑定的能力按拒绝路径处理。能力版本与调用记录一起保留，使一次实际动作能够追到当时使用的配置与权限条件。

## 本地工作区与后台自动化

Electron 桌面端包含 sidecar runtime，按 Profile 隔离环境，使用 SQLite offline-sync 持久化 Run、Event、ModelCall、ToolCall 和 ToolApproval。本地文件写入强制审批，运行过程支持离线执行、取消、崩溃恢复、Profile 切换和服务端幂等证据导入。

后台自动化将 Webhook、schedule、file 与 git Trigger 统一进入持久化 Invocation / Run。幂等、租约、fencing、重试、kill switch 和审批恢复共同约束自动任务推进，并在工作区边界条件不足时停止执行。

## 评测、观测与私有化交付

Eval Harness 使用 Dataset、Case、Eval Run、Trace Grader 和 Regression Gate，评估 groundedness、引用命中、无依据陈述、禁泄漏片段、安全与拒答、角色契约、工具使用、延迟和成本。它将输出质量与执行轨迹放到同一组回归任务里比较。

观测侧通过 OpenTelemetry、Prometheus、Grafana、Loki 与 Tempo 关联 Run、Event、Span、ModelCall、ToolCall 和策略证据。交付侧维护 Docker Compose、Alembic、Knowledge / RAG seed、迁移恢复冒烟、OpenAPI、Feature Catalog、Playwright 发布冒烟与文档门禁。

这套工程的重心，是让一个 Agent 任务拥有可追踪的计划、依据、动作和结果，并将同样的运行语义延伸到浏览器控制台与本地工作区。

[查看源码 ↗](https://github.com/luohao0308/forge-harness)
