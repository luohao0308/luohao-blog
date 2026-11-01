# AI 问本文

- 日期：2026-11-01；用户选择继续开发下一项，范围为文章问答与章节参考；状态 completed（本地完成，未发布）。
- 目标：文章页默认只针对当前已发布文章回答，提供可定位的参考章节/摘录，覆盖正文末尾而非只截开头；保留全站问答兼容。
- 决策：L2 兼容 API 扩展，一个可独立验收的端到端切片；ChatRequest 新增可选 article_slug，旧 query/top_k 与 citation 原字段不变；CitedArticle 附加章节/锚点/摘录，不引入持久化或权限模型变化。
- 方案：复用 GetPublishedArticle；无指定文章沿用检索，有指定文章不走 ES；复用 Markdown 解析器抽取章节并限定上下文大小。引用是后端选出的参考段落，不声称逐句引用已核验，不接受模型构造 URL。
- 边界：不存在/草稿/删除同样不可访问；无匹配内容明确告知、生成故障保留参考；保持限流。前端切文后清空消息，旧请求不能串文，失败可以重试。
- 非目标：连续对话、流式输出、跨文章综合回答、账号/订阅、索引同步改造、付费真实模型验证或部署。
- 验收：关键行为先写固定 fixture 单测/Eval；兼容旧请求；权限负向、文末命中、中文/重复标题锚点、无匹配、故障降级、注入与限流；Web 隔离浏览器验证 scope 请求、段落链接、错误重试及切文；Go build/test/vet/lint、make api、前端 lint/typecheck/build。
- 回退：本轮增量独立撤回，旧客户端可忽略新响应字段；交付顺序后端先于前端，前端随旧后端回退；不做生产改动。
- 治理：功能清单流程包未启用；workflow_check 的 capability 列表为空，API 专项 profile 未启用，本轮按 Core 与既有分层/契约规则验证，不声称专项治理已启用。建议后续显式采用 api:rest-openapi/api:grpc，不在本轮擅改权限或配置。

- 兼容补强：ChatReply 新增 article_slug 字段4回显实际范围，避开缓存主分支已有 outcome 字段3；新版 UI 缺少正确回显则停止显示答案，避免新客户端配旧后端时意外回到全站。

## 最终实现与验证

- 字段：ChatRequest.article_slug=3；ChatReply.article_slug=4；CitedArticle 的 section_title/anchor/excerpt 为 4/5/6。旧请求与小程序不改，原字段含义保留。
- 本文直接走公开文章读取，不依赖 ES。关键词匹配取至多 3 个章节、每章 1800 字窗口；章节标题限 120 字、摘录限 300 字、过长锚点回退整篇链接。中文双字/英文词匹配并支持明确概览问题，不是语义召回，换一种说法可能无匹配。
- 固定 fixture 验证：权限/限流/输入边界、正文 2000 字后的命中、单节末尾窗口、超长标题/上下文上界、h1 消耗ID与重复中文h2锚点、无匹配不调用模型、失败带参考段落和范围回显。
- 后端 `go build ./... && go test ./... && go vet ./...` exit 0；定向 fixture 与真实 HTTP 契约通过，`golangci-lint run` 0 issues。日志 `output/article-chat/backend-targeted.log`、`backend-lint.log`，全量检查结果见本轮工具输出。
- `make api` 等价生成命令（仅命令级 GOROOT/GOSUMDB 纠正本机工具路径）重新生成 chat.pb.go/OpenAPI；没有手改生成物或新增依赖。openapi_snake_case.py 补 articleSlug/sectionTitle/topK，既有 Comment.articleSlug 文档同时改为真实 snake_case，无运行行为变化。
- 前端 lint/typecheck/build 通过；1440/390px 浏览器问本文专项通过，包括旧后端缺回显拒绝、旧引用兼容、未知锚点退化、摘录转义、失败重试与导航隔离。已查看手机暗色回答截图。
- 既有阅读工具明暗四组和 SEO 专项均通过。日志、截图与专项结果在 `output/article-chat/`。
- 独立代码复核未发现阻塞。固定假模型仅证明检索、范围、封装与退化；没有调用真实付费模型，实际回答质量尚未实测。
- 交付边界：仅本地开发；未提交/push/创建PR/部署，原有未提交改动保留。契约指南见 `docs/contracts/CHAT.md`，运行方法见 `scripts/e2e/README.md`。
