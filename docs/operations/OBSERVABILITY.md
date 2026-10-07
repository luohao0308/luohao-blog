# 可观测性与健康信号

_状态：active（监控栈 S1/S2 落地） | 更新：2026-10-07 | Runbook：[RUNBOOK-monitoring.md](runbooks/RUNBOOK-monitoring.md)_

## 信号地图

| 能力/服务 | 健康检查 | 关键日志 | 指标/SLO | 追踪 | 告警 |
|---|---|---|---|---|---|
| 入口 Caddy | Kuma 拨测「首页/API 文章列表」 | `docker logs blog-caddy`（访问日志） | 拨测可用性 | trace_id 透传 | Kuma→钉钉 |
| Frontend（Nuxt BFF） | compose healthcheck `/health`（15s）+ Kuma | `docker logs blog-frontend` | 拨测可用性 | — | Kuma |
| Backend（Kratos） | compose healthcheck 文章列表 + Prometheus 抓 `/metrics`（compose 内网） | `docker logs blog-backend`（slog 结构化，trace_id/span_id 提取） | `blog_api_requests_total`、`blog_api_request_duration_seconds`（p95、5xx 按 operation）、`blog_api_requests_inflight`、go runtime | trace_id 关联日志（otel TraceAttrs） | Grafana：5xx>5% 5m、p95>2s 10m；Kuma 直探 |
| MySQL | compose healthcheck `mysqladmin ping` | `docker logs blog-mysql-prod` | 容器存活；间接由 API 全链路拨测覆盖 | — | 链路拨测失败暴露 |
| Redis | compose healthcheck `redis-cli ping`（带认证） | `docker logs blog-redis-prod` | 同上（限流/会话依赖） | — | 同上 |
| Elasticsearch | compose healthcheck `/_cluster/health` + Kuma JSON 探活 | `docker logs blog-es-prod` | 集群状态 green/yellow | — | Kuma |
| 主机资源 | node-exporter 抓取 | journald | CPU/内存/swap/磁盘/负载 | — | Grafana：磁盘>85% 15m、可用内存<400MB 5m |
| 每晚备份 | `backup.sh` 成功 ping Kuma push 监控（死信开关） | `/var/log/blog-backup.log`、`/var/log/blog-backup-push.log` | push 心跳 | — | Kuma：24h+ 未 ping→钉钉 |
| 宿主看门狗 | cron 5min 探 API，连续 2 失败重启 backend | `/var/log/blog-watchdog.log` | 自愈动作 | — | 事后日志审查 |

## 语义约定

- Liveness 只证明进程是否需要重启；Readiness 证明是否可接收流量；业务冒烟证明关键用户路径。
- 健康端点不返回凭据、连接串、内部主机或原始异常。
- 日志必须具备请求/任务关联、结构化字段、敏感信息脱敏和明确错误码。
- 指标和告警应对应可执行的 Runbook，避免只有阈值没有处理入口。
- 监控端点与 UI 一律不暴露公网：`/metrics` 仅 compose 内网；Kuma/Grafana/Prometheus 端口绑 127.0.0.1，经 SSH 隧道访问。

## 发布观测窗口

- 发布前基线：Grafana Blog Overview 截图当前 QPS/p95/5xx 与主机水位。
- 发布后重点指标：`blog_api_requests_total` 5xx 率、p95 延迟、容器重启次数、主机内存。
- 观察时长：15 分钟（一次拨测周期 ×15 + 告警 for 窗口内生效）。
- 回滚触发条件：5xx 告警触发且错误指向新版本行为；按 [RELEASE-CHECKLIST.md](RELEASE-CHECKLIST.md) 回滚镜像。

## 常用排障入口

| 问题 | 第一证据 | 下一步 | Runbook |
|---|---|---|---|
| 站点打不开 | Kuma「首页/API」拨测红 | `docker compose -f deploy/compose.prod.yml ps` 看哪个容器 unhealthy | [RUNBOOK-monitoring.md](runbooks/RUNBOOK-monitoring.md) §4 |
| API 5xx 突增 | Grafana「API 5xx 错误率」面板定位 operation | `docker logs blog-backend --since 15m` 查错误码堆栈 | RUNBOOK-monitoring §4 |
| API 变慢 | Grafana p95 面板 + 慢在哪个 operation | RAG 聊天看 LLM 上游；搜索/列表看 ES/MySQL | RUNBOOK-monitoring §4 |
| 磁盘告急 | Grafana 根分区面板 | `docker system df` 清构建缓存；journal vacuum | — |
| 内存告急 | Grafana 主机内存面板 | `docker stats --no-stream` 定位大头 | RUNBOOK-monitoring §4 |
| 备份没跑 | Kuma 备份心跳变红 | `/var/log/blog-backup.log` 尾部；`crontab -l` | [RUNBOOK-monitoring.md](runbooks/RUNBOOK-monitoring.md) §4 |
