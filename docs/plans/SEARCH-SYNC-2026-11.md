# 搜索同步状态与单篇重试

- 日期：2026-11-01；用户已明确要求开发；L2 兼容管理员 API 扩展，单个端到端切片；状态 completed（本地完成，未发布）。
- 目标：后台编辑页分清正文保存与索引同步，显示当前文章索引是否缺失、过期、已同步、无需索引、部分同步或不可核实，并允许修复当前文章。
- 决策：直接读取 Elasticsearch 实时文档与 MySQL 当前内容比对；无需新增 MySQL 表、迁移或持久化任务队列。状态重新读取即可核实，不依赖进程内保存的成功标记。以当前索引状态为准，不提供历史失败记录。
- 最小方案：管理员 GET/POST 独立端点；公开 Article 接口不加管理字段；普通保存维持 best-effort，不把 ES 失败当正文保存失败。后台状态单独加载，重试结束以实际读取结果判断，不无条件显示成功。
- 向量：刻意仅配置关键词检索时可显示关键词已同步；预期向量而 embedding 失败或 mapping 不支持时显示部分同步。mapping 缺失需既有运维重建，不在本轮自动删除/重建索引。
- 并发：同一 ArticleUsecase 的保存/删除同步与单篇重试共用条带锁，锁内重新读取当前正文，再操作派生索引。适用仓库当前单后端部署；不代表跨进程锁，全量 CLI reindex 与其他实例写入需运维互斥，本轮不运行。
- 可见性：普通保存仍沿用原 refresh=false；手动重试 wait_for。状态文案确认索引文档，不承诺所有搜索词立即命中或向量模型质量。
- 权限/失败：HTTP/gRPC 均仅 ADMIN；参数非法400、文章不存在/已删除404；ES 未配置与启动时不可用现有 nil indexer 无法区分，统一诚实提示。异常只返回固定提示，不透出地址、凭据或上游原始错误。
- 验收：关键 fixture 测试先行；保存成功+ES失败、缺失/过期/已同步、重试失败与恢复、未发布清理、向量降级、未配置、并发旧写与删除、匿名/读者拒绝；数据层模拟ES HTTP、真实服务HTTP契约、后台隔离浏览器、Go build/test/vet/lint与前端lint/typecheck/build。
- 范围：biz/service/data ES层、proto生成、Casbin规则、后台编辑状态组件及验证文档；无新依赖、数据库变更、生产环境操作。
- 兼容与回退：旧公开/保存响应不变；先交付后端再交付新后台。旧后端不支持新端点时，状态组件提示无法查询但正文编辑仍可用。撤销本轮增量即回退，原有用户改动保留。
- 治理：当前 manifest 未启用技术 capability；按 Core、backend分层和既有契约规则实施，不声称 API 专项 profile 已启用。后续可显式采用 api:rest-openapi/api:grpc，本轮不擅改权限策略或启用配置。

## 相关边界补强

- 修正原 ES _mapping 解包为真实 `index.mappings.properties` 结构，检测 dense_vector 维度与 index 开关；实时核验通过 RWMutex 安全更新本进程的语义检索开关，避免状态就绪但实际检索仍停留旧缓存。
- 索引级404使用领域 sentinel：后台不提供单篇修复按钮；重试动作在模型/文档写入/删除前 HEAD 核验，不自动新建默认 mapping。CLI重建等外部维护仍需互斥。
- 新后台组件位于文章编辑页，正文保存成功后单独刷新状态，说明核验的是服务器已保存版本；旧后端/状态读取失败不阻断编辑。

## 最终验收

- 关键 biz、data、service/server fixture 与真实 HTTP/gRPC 权限测试通过：保存成功/索引失败、实时缺失/过期/已同步、未发布残留清理、未知不可冒充清理完成、向量部分就绪、无客户端、索引级缺失禁止重试。
- 并发：biz 同进程旧retry与新save/delete的最终结果、ES实时映射刷新缓存与并发Search均通过 `go test -race`。日志 `output/search-sync/backend-race.log`、`output/search-sync-data-race.log`。
- 最终源码 `go build ./...`、`go test ./...`、`go vet ./...`、`golangci-lint run` 全通过（0 issues）；日志 `output/search-sync/backend-{build,full,vet,lint}.log`。proto/HTTP/gRPC/OpenAPI从源生成，snake_case映射同步，不手改生成物。
- 前端 lint/typecheck/build 全通过，日志 `output/search-sync/frontend-{lint,typecheck,build}.log`。旧构建的浏览器用例确实在缺少面板时失败，记录 `frontend-before-build.log`；最终 1440/390px 专项通过，见 `frontend-results.txt`、`search_sync_e2e-final.log`。
- 浏览器覆盖重试HTTP失败与恢复、刷新回读、正文保存成功索引待更新、部分同步/无需索引/不可用、坏响应不得显示绿色成功、权限拒绝不发起修复、查询失败不阻断保存。已查看手机部分同步截图；截图在 `output/search-sync/partial-*.png`、`synced-*.png`。
- 既有防丢稿回归通过，见 `output/search-sync/draft_protection_e2e-final.log`。成功文案改为“正文已保存”；恢复步骤等待 Vue 输入值更新后再断言，并在重载前检查草稿标题确已持久化，消除异步渲染时序误判。
- 独立复核发现的“索引丢失会自动创建默认映射”问题已通过 sentinel、重试前 HEAD 及无 PUT/DELETE/embedding fixture 修复。
- 交付边界：无新依赖、无数据库迁移、无真实 ES/付费模型调用；未提交/push/创建PR/部署，既有工作树改动保留。状态表示当前已保存正文与索引，不记录历史失败尝试；单进程锁不替代维护互斥。
