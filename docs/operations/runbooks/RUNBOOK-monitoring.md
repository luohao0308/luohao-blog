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
| Data layer down | exporter up、mysql_up、redis_up、elasticsearch_clusterinfo_up 任一不可用或数据层指标缺失 | 3m |
| Offsite backup failed or stale | 异地同步失败、指标缺失或最近成功距今 > 26h | 3m |

异地同步指标由 `deploy/backup-push.sh` 的 EXIT trap 写入
`/var/lib/blog-node-metrics/backup-sync.prom`（可由 `BACKUP_METRICS_DIR` 覆盖），node-exporter textfile collector 读取。
文件原子替换（临时文件扩展名 `.tmp`），目录 755、文件 644，只含状态与时间戳，不含凭据：

- `blog_backup_sync_success`：最近尝试成功为 1，失败为 0。
- `blog_backup_sync_last_success_timestamp_seconds`：最后成功时间，失败保留原值。
- `blog_backup_sync_last_attempt_timestamp_seconds`：最近尝试时间。

指标发布失败只警告，保留备份脚本原退出码；缺失/超过 26h 未成功仍触发 Grafana 告警。
本地备份的 Kuma Push 心跳与异地同步告警分别覆盖两个任务。首次上线应验证指标抓取与告警 provisioning，不能仅凭脚本存在判断已生效。

本地回归（不访问备份私仓，不执行真实 force-push）：

```bash
bash deploy/tests/backup-push-test.sh
docker run --rm --entrypoint promtool -v "$PWD/deploy/tests:/tests:ro" prom/prometheus:v3.5.0 test rules /tests/closeout-rules-test.yml
```

PromQL fixture 覆盖同步失败、超期、指标缺失及数据层 target 消失；修改告警表达式时同时保持 fixture 与 provisioning 一致。

### 日志（Loki + Promtail）

- Promtail 经 docker SD 采集全部容器 stdout 日志（`container` 标签区分），并采集宿主机 `/var/log/blog-*.log`（watchdog/备份）。
- 查询入口：Grafana → Explore → 数据源选 Loki，如 `{container="blog-backend"}`、`{job="host-scripts"}`。
- 保留 7 天（Loki compactor 清理），内存/磁盘均有上限。

### 外部拨测兜底（GitHub Actions）

- `.github/workflows/external-probe.yml`：每 5 分钟从 GitHub 探测公网首页 + 核心 API，覆盖"宿主机整机失联时 Kuma 一同沉默"的盲区。
- 失败通知 = GitHub Actions 失败邮件（仓库所有者默认开启）+ 运行历史留痕。不进钉钉：机器人是 IP 白名单模式，GitHub 出口 IP 不固定；如需钉钉，可另建一个"自定义关键词"安全设置的机器人再接入。

## 3. 钉钉通知（用户配置一次）

机器人创建：钉钉群 → 群设置 → 智能群助手 → 添加机器人 → 自定义（安全设置选"加签"，复制 webhook 与密钥）。

**Kuma**：Settings → Notifications → Setup Notification → 类型选 `DingDing`（或 DingDing webhook），粘贴 webhook；`Apply to all existing monitors` 勾选。
**Grafana**：Alerting → Contact points → Add contact point → 类型 `DingDing`，粘贴 webhook 与密钥 → Test；再在 Notification policies 把默认策略的接收方设为该 contact point。

## 4. 日常运维

| 操作 | 命令 |
|---|---|
| 启动监控栈 | `cd /opt/luohao-blog/app && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring up -d` |
| 更新（runbook 同生产） | `git pull && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring pull && docker compose -f deploy/compose.monitoring.yml --env-file deploy/.env.monitoring up -d` |
| 状态/日志 | `docker compose -f deploy/compose.monitoring.yml ps`；`docker logs blog-kuma/blog-prometheus/blog-grafana/blog-loki/blog-promtail` |
| 全栈回退 | `docker compose -f deploy/compose.monitoring.yml down`（`-v` 会删 prom/grafana/kuma/loki 数据，慎用） |
| 改告警阈值/看板 | 改 `deploy/grafana/provisioning/alerting/blog-alerts.yml` 或 `dashboards/*.json` 后 `up -d --force-recreate grafana`（provisioning 只读，UI 不可改） |
| 改抓取目标 | 改 `deploy/prometheus/prometheus.yml` 后 `docker restart blog-prometheus` |
| 重建 MySQL exporter 口令 | 服务器 `deploy/.env.monitoring` 改 `MYSQLD_EXPORTER_PASSWORD` 后在 mysql 容器里 `ALTER USER exporter@'%' IDENTIFIED BY '<新密码>'`，再 `up -d --force-recreate mysqld-exporter` |
| 内存排查 | Grafana 主机内存告警时 `docker stats --no-stream` 查大头；Prometheus 可临时降 `retention.size`，Loki 可临时降 `retention_period` |

> 单文件 bind mount 的坑（T-012 教训）：服务器上 `git pull` 或 scp 替换 prometheus.yml /
> blog-alerts.yml / loki/promtail 配置后，必须对对应容器 `up -d --force-recreate`
> 才能吃到新内容——scp 的 sftp 写入会换 inode。

## 5. 已知边界与后续可选项

- 同机拨测有网络盲区：宿主机宕机时 Kuma 也一起失联。已由 GitHub Actions 外部拨测兜底（§2，失败发邮件）。
- cAdvisor 未部署（服务器 gcr.io 不可达、Docker Hub 无 tag）：容器级 CPU/内存暂用 `docker stats` 手查；后续可达时可把 `cadvisor` 服务加回 `compose.monitoring.yml` 并恢复 prometheus 抓取 job（历史版本在 Git 记录中）。
- 公开状态页：Kuma 内已建 `/status/blog`（含全部监控项），公网暴露等域名/HTTPS（T-008）落地后经 Caddy 子域接入，不要裸 IP 暴露 Kuma 登录入口。
- Loki 3.x 对旧式 push 有 "negative structured metadata bytes received" 的 error 刷屏：已确认不影响摄入与查询（实测日志可查），属已知计量噪音；后续若升级 Alloy 可消除。
- MySQL exporter 专用账号 `exporter@%` 需 PROCESS/REPLICATION CLIENT/SELECT + MAX_USER_CONNECTIONS 3，密码仅在服务器 `deploy/.env.monitoring`。账号不存在时先 CREATE USER，再 GRANT；口令由服务器现有配置在进程内部读取，禁止输出到日志/命令行。必须查询 `mysql_up == 1`，不能仅凭 Prometheus target up 判定数据库连接成功。
- 异地备份指标首次接入仅可用已核实的远端备份提交时间初始化，不能将部署时间冒充备份成功。确认 backups 本地 HEAD 与备份私仓 latest 一致后使用该提交的时间；后续由 cron 脚本维护。
