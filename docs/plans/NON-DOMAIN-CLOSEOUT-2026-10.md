# 非域名收尾与上线

状态：approved（用户 2026-10-08 确认四切片）；仅 S4 in_progress。

| 切片 | 结果与范围 | 依赖 | 验收 | 回退 |
|---|---|---|---|---|
| S1 completed | backend：ES 查询覆盖、seed 软删冲突、reindex 分页、LLM 响应上限 | 无 | 回归测试、build/test/vet/lint | 回退提交/旧镜像 |
| S2 completed | deploy/backup-push.sh 告警、release actions 固定 SHA | S1 | 成功/失败路径测试、配置检查 | 保留旧脚本/配置 |
| S3 completed | 小程序可执行验收、相关任务/计划文档同步 | S2 | typecheck、API 检查；人工 UI 未验如实保留 | 回退文档/测试改动 |
| S4 in_progress | PR/CI 后部署准确版本 | S3 | 备份、公开页面/API/搜索与监控检查 | 旧镜像及监控配置备份 |

不含域名/HTTPS/Secure cookie、SMTP、小程序正式发布；不扩展未核实的 P3 长尾。
线上预检：六个 Prometheus target up；内存可用 1.2 GiB、磁盘可用35 GiB。服务器监控配置存在未提交改动，部署前必须比对并备份，不能覆盖未知改动。

S1 验证：全量 go test/build/vet 与 golangci-lint 通过（env -u GOROOT），2005 篇分页、软删保持、LLM 超限和 ES 查询边界回归覆盖。无迁移。reindex 执行期间应暂停文章发布/删除，offset 分页需要稳定数据集。

S2 验证：backup-push 隔离 fake-git 回归通过（未真实 force-push）；监控 compose 校验通过；七场景 promtool 通过且表达式与 Grafana provisioning 一致。release 四 actions 经官方远端标签解析固定 SHA。额外修正数据层告警从仅抓取 up 改为同时检查 mysql_up/redis_up/elasticsearch_clusterinfo_up；生产 mysql_up=0 根因为 exporter 账号不存在，S4 按原最小权限配置补齐。

S3 验证：小程序 typecheck + mocked wx 逻辑回归通过；生产只读 API 契约通过。修复 401 刷新重复 POST 与点赞失败去重标记残留并加入 CI。DevTools/真机 UI 仍待人工验收。任务板与 Runbook 已纠正历史完成状态和监控误判。
