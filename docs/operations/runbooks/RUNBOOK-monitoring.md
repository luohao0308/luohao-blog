# Runbook：站点监控（拨测 + 指标 + 告警）

> 监控栈 = Uptime Kuma（可用性拨测/通知）+ Prometheus + Grafana（指标/告警）+ node-exporter。
> 与生产栈解耦：`compose.monitoring.yml` 独立启停，`down` 不影响博客服务。
> 首次搭建与决策背景见 `docs/plans/MONITORING-2026-10.md`。

## 1. 访问（全部走 SSH 隧道，端口不暴露公网）

本机执行：

```bash
ssh -N root@193.112.128.245 \
  -L 3001:127.0.0.1:3001 \
  -L 3002:127.0.0.1:3002 \
  -L 9090:127.0.0.1:9090
```

| 服务 | 本机地址 | 用途 |
|---|---|---|
| Uptime Kuma | http://localhost:3001 | 拨测状态、通知设置、公开状态页 |
| Grafana | http://localhost:3002 | 指标看板、告警规则 |
| Prometheus | http://localhost:9090 | 原始指标查询、抓取目标状态 |

凭据：Grafana 管理员在服务器 `deploy/.env.monitoring`（Git 外）；Kuma 管理员账号密码同记于该文件 `KUMA_ADMIN_*` 两行。

## 2. 监控项清单

### Kuma 拨测（60s 间隔，失败重试 3 次）

| 名称 | 目标 | 覆盖链路 |
|---|---|---|
| 首页 | `http://caddy/` | caddy→frontend SSR |
| API 文章列表 | `http://caddy/api/v1/articles/list?page_size=1` | caddy→frontend→backend→mysql 全链路 |
| Frontend 健康 | `http://frontend:3000/health` | Nuxt/Nitro 进程 |
| Backend 直探 | `http://backend:8000/v1/articles/list?page_size=1` | Kratos→mysql（与宿主 watchdog 同口径） |
| ES 集群健康 | `http://es:9200/_cluster/health`，JSON query `$.status` ∈ {green, yellow} | ES 存活与分片状态 |
| 备份心跳（Push） | cron 备份成功后 ping | 备份静默失败死信开关 |

> 不要添加 RAG 聊天（`/api/v1/chat/*`）拨测：会消耗 LLM token 且该接口有独立限流。

### Grafana 告警（Blog Alerts 文件夹，provisioning 管理）

| 规则 | 条件 | 持续 |
|---|---|---|
| Host disk usage high | 根分区使用率 > 85% | 15m |
| Host memory low | 可用内存 < 400MB | 5m |
| API 5xx rate high | 5xx 占比 > 5% | 5m |
| API p95 latency high | 全站 p95 > 2s | 10m |

> 后端指标在 `/metrics` 镜像发布前无数据，`noDataState=OK` 使其静默，指标出现后自动生效。

## 3. 钉钉通知（用户配置一次）

机器人创建：钉钉群 → 群设置 → 智能群助手 → 添加机器人 → 自定义（安全设置选"加签"，复制 webhook 与密钥）。

**Kuma**：Settings → Notifications → Setup Notification → 类型选 `DingDing`（或 DingDing webhook），粘贴 webhook；`Apply to all existing monitors` 勾选。
**Grafana**：Alerting → Contact points → Add contact point → 类型 `DingDing`，粘贴 webhook 与密钥 → Test；再在 Notification policies 把默认策略的接收方设为该 contact point。

## 4. 日常运维

| 操作 | 命令 |
|---|---|
| 启动监控栈 | `cd /opt/luohao-blog/app && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring up -d` |
| 更新（runbook 同生产） | `git pull && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring pull && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring up -d` |
| 状态/日志 | `docker compose -f deploy/compose.monitoring.yml ps`；`docker logs blog-kuma/blog-prometheus/blog-grafana` |
| 全栈回退 | `docker compose -f deploy/compose.monitoring.yml down`（`-v` 会删 prom/grafana/kuma 数据，慎用） |
| 改告警阈值/看板 | 改 `deploy/grafana/provisioning/alerting/blog-alerts.yml` 或 `dashboards/*.json` 后 `up -d grafana`（provisioning 只读，UI 不可改） |
| 改抓取目标 | 改 `deploy/prometheus/prometheus.yml` 后 `docker restart blog-prometheus` |
| 内存排查 | Grafana 主机内存告警时 `docker stats --no-stream` 查大头；Prometheus 可临时降 `retention.size` |

## 5. 已知边界与后续可选项

- 同机拨测有网络盲区：宿主机宕机时 Kuma 也一起失联，收不到通知。可选兜底：UptimeRobot/healthchecks.io 免费外部拨测首页一个点。
- cAdvisor 未部署（服务器 gcr.io 不可达、Docker Hub 无 tag）：容器级 CPU/内存暂用 `docker stats` 手查；后续可达时可把 `cadvisor` 服务加回 `compose.monitoring.yml` 并恢复 prometheus 抓取 job（历史版本在 Git 记录中）。
- Loki 日志聚合、mysqld/redis/es exporter、公开状态页：S3 可选项，用户确认后再排期。
- Kuma 拨测走容器内网，公网入口（安全组/防火墙）故障探测不到；HTTPS 上线后把首页/API 两项改成 `https://域名`。
