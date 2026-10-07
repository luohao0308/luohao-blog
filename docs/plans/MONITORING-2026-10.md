# 站点监控（拨测 + 指标 + 告警）实施计划

_状态：in_progress | 更新：2026-10-07 | 关联任务：T-013 | 关联设计：`docs/operations/OBSERVABILITY.md`_

## 1. 目标、成功标准与停止条件

- 目标结果：全站（caddy→frontend→backend→mysql/redis/es）可用性拨测、每接口 QPS/延迟/错误率指标、资源水位与"备份静默失败"死信开关；异常经钉钉主动通知。
- 可验收成功标准：① 手动停 backend 容器能收到钉钉告警；② Grafana 能看到每个 API 的 QPS/p95/5xx；③ 备份脚本成功后 Kuma push 监控变绿，超过 24h 不 ping 触发告警；④ 监控端口全部不暴露公网。
- 完成后停止条件：S1+S2 验收通过即停止；S3（日志聚合/DB exporter/公开状态页）为可选项，用户提出再做。

## 2. 范围与非范围

### 范围

- `deploy/compose.monitoring.yml`（新增监控栈，独立于生产 compose）
- `deploy/prometheus/`、`deploy/grafana/`（抓取配置、数据源、看板、告警规则 provisioning）
- `deploy/backup.sh`（成功后 ping Kuma push 监控；URL 读 Git 外的 `.env.prod`）
- `backend/internal/server/`（metrics 中间件 + `/metrics` 挂载，HTTP+gRPC 共用）
- `docs/operations/OBSERVABILITY.md`、`docs/operations/runbooks/RUNBOOK-monitoring.md`、`deploy/README.md`

### 非范围

- Loki 日志聚合、mysqld/redis/es 专属 exporter、公开状态页（S3 可选项）
- 追踪后端（Tempo/Jaeger）；暂不引入 Alertmanager（Grafana 统一告警承担）
- 域名/HTTPS（T-008 备案流程独立推进）

## 3. 当前证据基线

- 代码/配置：backend 已全量 build/test/lint 通过（feat/api-monitoring 分支）；本地实跑验证 `/metrics` 输出 `blog_api_requests_total`、`blog_api_request_duration_seconds_bucket`（operation=路由模板，无高基数）、`blog_api_requests_inflight`、go runtime 指标
- 测试/CI：新增 `metrics_test.go`（成功/错误码/空 transport 三用例）；全量 `go test ./...` 绿
- 契约/数据：无 API 契约变化、无迁移；`/metrics` 仅 compose 内网可达（backend 不发布端口）
- 运行事实（2026-10-07 服务器摸底）：2C4G，可用内存 1.9G（ES ~750M），磁盘清理后 29%（释放 25G 构建缓存+journal）；Docker Hub 经腾讯 mirror 可拉，gcr.io 不可达
- Unknown：钉钉机器人 webhook（需用户在钉钉群创建后提供）；生产 backend `/metrics` 待 PR 合并+发布流水线出镜像后在服务器 pull 生效

## 4. 规模判定与用户确认

- 规模：large
- 触发信号：跨 backend/deploy/docs 三个目录边界；两个有序切片；产出可独立验收
- 确认状态：approved
- 用户确认时间或消息指针：2026-10-07 对话确认"钉钉 按这个切片就行"（方案：Kuma+Prometheus/Grafana 分层，通知选钉钉）
- 用户调整：通知渠道定为钉钉（原方案留了邮件/企业微信选项）
- Issue：未创建（按仓库既有惯例以 PR 追踪；如需 Issue 由用户创建）

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| S1 | Uptime Kuma 拨测（全链路 4 项+ES）+ 备份 push 死信开关 | `compose.monitoring.yml`、`backup.sh`、服务器部署 | 无 | 停 backend 容器收到通知；push 监控变绿 | `compose down`（监控独立栈，不影响生产） | in_progress |
| S2 | 后端 `/metrics` + Prometheus/Grafana 栈 + 看板与告警规则 | `backend/internal/server/`、`deploy/prometheus|grafana/` | S1（通知渠道就绪） | Grafana 看到 API QPS/p95/5xx；演练触发一条告警 | 分支回退 / `compose down`；backend 镜像按 runbook 回滚 | in_progress |
| S3（可选） | Loki 日志聚合、DB/ES exporter、公开状态页 | deploy 配置扩展 | S2 | 用户另行确认后定义 | — | pending |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 指标栈 | Prometheus+Grafana（精简，无 Alertmanager） | 用户核心诉求是"按接口的 API 指标"；Grafana 统一告警免加组件 | 比 beszel 重 ~500M，用 mem_limit 封顶 |
| 拨测 | Uptime Kuma（容器内网探活+钉钉通知+状态页） | 半天可落地，覆盖"挂了主动通知"；与指标互补 | 同机探测有网络盲区（宿主机宕机无法自报），后续可加免费外部拨测兜底 |
| cAdvisor | 暂不部署 | 服务器 gcr.io 不可达且 Docker Hub 无对应 tag（2026-10-07 实测）；主机+拨测指标已覆盖核心场景 | 容器级 CPU/内存面板暂缺，`docker stats` 手查 |
| metrics 中间件 | 自研 ~80 行（prometheus/client_golang） | Kratos v3 无 contrib/metrics 适配器（go list 实测）；路由模板做 label 防高基数 | 自维护代码，已配单测 |
| `/metrics` 暴露 | 只在 compose 内网，不经 Caddy | backend 容器不发布端口；ES 9200 无安全层同理不暴露 | 无 |
| 告警通知 | 钉钉（Kuma 通知 + Grafana contact point） | 用户指定；webhook 是密钥，只在 UI 配置不进 Git | 需用户手动创建机器人 |

## 6. 实施切片

### S1：Uptime Kuma 拨测 + 备份死信开关

- 状态：in_progress（2026-10-07；配置就绪，服务器部署与 Kuma 配置进行中）
- 实现：`deploy/compose.monitoring.yml`（kuma 绑 127.0.0.1:3001，挂生产网络）；监控项 6 个（首页/API 全链路/frontend /health/backend 直探/ES 集群健康/备份 push）；`backup.sh` 末尾按 `BACKUP_PUSH_URL` ping push 监控
- 验证：见 §7 证据

### S2：API 指标 + Prometheus/Grafana

- 状态：in_progress（代码与配置就绪；生产生效依赖 PR 合并→ghcr 镜像→服务器 pull）
- 实现：见 §3/§5

### S3（可选）：暂缓

- 状态：pending（用户未确认，不启动）

## 7. 验证与证据

- 2026-10-07 本地：`go build ./... && go test ./... && golangci-lint run` 全绿；本地栈实跑 `curl :8000/metrics` 确认三个 `blog_api_*` 指标族与 go runtime 指标（counter `code="200"` 正确计数）
- 2026-10-07 服务器：监控栈 `docker compose config` 校验通过；prom/grafana/node-exporter/kuma 镜像实拉成功（腾讯 mirror）；`bash -n backup.sh` 语法通过
- 待验证：服务器监控栈 up、Kuma 拨测项配置全绿、停 backend 容器收通知、钉钉链路（等待用户提供机器人 webhook）
