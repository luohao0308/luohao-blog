---
slug: article-search-sync-case-study
title: 正文已保存，搜索为什么还没更新？
summary: 从一次可复现的索引失败出发，拆开正文保存、实时核验与单篇恢复，说明 MySQL 与 Elasticsearch 之间的一致性边界。
tags: Go, Elasticsearch, 一致性, 个人博客, 案例推演
status: draft
---
后台显示“正文已保存”，搜索却仍返回旧标题。此时再点一次保存，究竟是在修复搜索，还是把正文又写了一遍？

在这个博客里，文章保存在 MySQL，Elasticsearch 承担全文与语义检索。它们有各自的失败路径。我给后台增加了独立的搜索同步状态与单篇重试，把“内容写入成功”和“派生索引就绪”分别反馈给编辑者。

本文用仓库中的隔离测试复现这一情形，记录当前实现与限制，不将它描述为一次真实线上事故。整体项目见 [luohao-blog 作品页](/projects/luohao-blog)，前后台联调的背景见 [从登录到发布的验收路径](/posts/fullstack-smoke-pitfalls)。

## 先固定一个能判断对错的场景

设文章已经发布，索引里还是旧内容。编辑者修改标题和正文，MySQL 写入成功，随后 ES 写入超时。这里有两个独立事实：数据库已经保存新版本，索引同步这次没有完成。

预期行为是：编辑页确认正文保存成功，搜索面板提示索引待更新；ES 恢复后，管理员可以只重试这一篇。检查点也要具体：数据库内容没有因索引错误回滚，重试读取的是当前正文，最后的状态来自实际回读。

仓库的 TestArticleUsecaseSearchIndexIsBestEffort 用失败的索引替身验证前一条；TestSearchSyncStatesAndRetry 验证缺失、失败重试与恢复。这样可以稳定制造错误，不需要关闭生产搜索服务。

## MySQL 是正文的真相源

文章更新完成后，业务层调用 syncIndex。它取得同进程内该 slug 对应的锁，重新读取数据库中的文章，再交给索引实现。写索引失败会记录警告，文章保存仍然成功。

这是一种明确的 best-effort 选择：正文写入不能因为派生索引暂时不可用而被判定失败。它没有建立 MySQL 与 ES 的分布式事务，也没有在这里引入持久化的自动重试队列。失败后的恢复需要状态核验和显式重试承接。

可以把调用关系压缩成下面的流程示意，它不是可直接执行的程序：

```text
文章事务提交
  → 取得当前进程的 slug 索引锁
  → 重新读取 MySQL 当前内容
  → 已发布：写入 ES；未发布或已删除：清理文档
  → 索引异常：记录警告，保留正文保存成功的结果
```

实现可对照 [ArticleUsecase 的 syncIndex 与 dropIndex](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/biz/article.go) 和 [重新读取当前正文的 writeLatestIndex](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/biz/article_search_sync.go)。

## 状态面板检查的是什么

核验接口不会因为上次保存没有报错就显示绿色。它实时读取 ES 文档快照，再与 MySQL 当前文章比较。标题、摘要、Markdown、标签集合和发布时间都参与判断；标签的排列顺序与重复项会先归一，避免无意义的差异。

| 状态 | 实际含义 | 恢复方向 |
|---|---|---|
| missing | 索引存在，但当前已发布文章的文档缺失 | 可单篇同步 |
| stale | 文档存在，参与比较的内容尚未对齐 | 用当前正文重新同步 |
| partial | 关键词内容已对齐，预期的语义索引未就绪 | 根据向量映射和维度判断是否可重试 |
| not required | 文章未发布，且没有索引残留 | 无需写入搜索 |
| unavailable | 未配置、无法核实，或整个索引不存在 | 查看原因与 can_retry，不统一当成可修复 |
| synced | 当前内容与已核验索引一致 | 仍不承诺每个搜索词都立即命中 |

未发布文章如果还有索引残留，会被判为待清理，而不是“无需索引”。核验失败时也不能声称清理已完成。面板里的 checked_at 表示这次观察的时间，并不是某次历史同步成功的记录。

## 文档缺失与整个索引缺失要分开

这两个情况都可能让读取返回 404，却不能使用同一条修复路径。

单篇文档缺失时，可以依据当前文章重新写入。整个索引不存在时，重试会停止，要求管理员显式初始化或安排重建。原因是索引还承载字段类型、向量维度等配置；让一次单篇写入悄悄创建默认映射，会把内容恢复与基础设施变更混在一起。

数据层在重试前先检查索引是否存在。核验与重试之间索引也可能消失，因此业务层还会处理重试返回的“索引不存在”错误。TestRetryIndexMissingNeverCreatesImplicitMapping 验证这条路径不会继续写文档、删除文档或生成 embedding。

相关处理在 [ES 快照读取和 RetryArticleIndex](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/data/es.go)。

## 为什么“部分同步”值得单独显示

配置了 embedding 时，索引写入可能因上游故障降级为仅含关键词内容的文档。此时全文检索仍可能工作，语义检索却没有就绪。

状态检查会读取当前映射，核对 dense_vector 的维度与可用性，并检查文档里的向量长度。映射可用、当前文档缺向量时，重试可能有帮助；映射缺失或维度不匹配时，反复点击按钮不能修复结构，应转入显式维护。

因此 partial 既不等同于整个搜索不可用，也不能包装成“关键词与语义全部同步”。这个状态描述的是索引准备情况，不是回答质量或语义命中率的评测结果。

## 重试结束后，还要再次读取

重试先取得同一组 slug 锁，再读取当前文章。ES 手动重试使用 refresh=wait_for，普通保存仍使用 refresh=false，避免每次正文更新都强制刷新整个索引。

重试调用结束后，业务层再次读取文章并核验索引；前端还会校验返回的 slug 和状态字段，再展示结果。HTTP 200 本身不代表已同步：例如整个索引缺失时，接口可以正常返回“不可用、不可重试”的状态。

这条路径也要考虑旧重试与新保存相遇。TestSearchSyncRetryCannotOverwriteLaterSaveOrDelete 会让旧重试阻塞，期间更新或删除数据库文章，再触发对应同步，最后检查索引没有被旧内容占住或把已删除文章复活。

锁只协调一个 ArticleUsecase 所在进程的保存、删除和重试。它不是分布式锁，不能据此声称多实例部署或另一个 CLI 重建任务已经获得同样的互斥；这些维护入口仍需单独协调。

## 在隔离环境复现

使用满足仓库 go.mod 的 Go 工具链，在仓库根目录运行：

```sh
go -C backend test ./internal/biz \
  -run 'TestSearchSync|TestArticleUsecaseSearchIndexIsBestEffort' -count=1 -v

go -C backend test ./internal/data ./internal/server \
  -run 'TestArticleIndexSnapshot|TestRetryArticleIndex|TestRetryIndexMissingNeverCreatesImplicitMapping|TestSearchSyncHTTPAndGRPCPermissions' \
  -count=1 -v
```

本次写作核对时，两组命令均通过。业务测试使用索引替身，数据层用本地 HTTP 服务模拟 ES 响应，权限测试覆盖匿名拒绝、普通读者拒绝和管理员访问；这些结果不等同于真实 ES 集群、付费 embedding 或模型效果验收。

测试入口见 [业务状态与并发恢复](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/biz/article_search_sync_test.go)、[ES HTTP 边界](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/data/es_search_sync_test.go) 和 [HTTP／gRPC 权限](https://github.com/luohao0308/luohao-blog/blob/400d24244ae607a5a22bf7565166f811c1b558fa/backend/internal/server/search_sync_test.go)。

这项改动给编辑者留下了两个可分别核对的结果：正文是否已经保存，以及搜索准备到了哪一步。出现异常时，恢复入口也有明确边界，不需要靠重复提交正文来猜测系统状态。

如果还想看正文自身的并发保护，可以继续读[两个窗口同时改文章的版本与修订设计](/posts/article-cas-revisions-case-study)。
