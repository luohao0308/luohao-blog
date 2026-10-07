# luohao-blog 本地基础设施

> 生产栈与监控栈见本目录 `compose.prod.yml` / `compose.monitoring.yml`：
> 监控（Kuma 拨测 + Prometheus/Grafana）的启停、访问隧道和告警配置见
> `docs/operations/runbooks/RUNBOOK-monitoring.md`；本文件其余部分讲本地开发依赖。

## 启动

```bash
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps   # 等待全部 healthy
```

| 服务 | 地址 | 说明 |
|---|---|---|
| MySQL 8.4 | `127.0.0.1:3307` | 库 `blog`，root/root123（本地占位）；宿主机 3306 被本机已有 MySQL 占用，故映射 3307 |
| Redis 7.4 | `127.0.0.1:6379` | 无密码（本地占位） |
| Elasticsearch 8.17.4 | `http://127.0.0.1:9200` | 单节点，安全特性关闭（本地占位） |
| MinIO | API `:9000` / 控制台 `:9001` | minioadmin/minioadmin（本地占位） |

数据持久化在命名卷 `luohao-blog-dev_*` 中；重置数据用 `docker compose down -v`（会删除全部本地数据）。

## Caddy（生产反代，M5 启用）

`caddy/Caddyfile` 已启用：所有请求先到 Nuxt（:3000），由 Nuxt BFF 把 `/api/v1/*` 代理至 Kratos（:8000），同时处理 refresh cookie 路径。当前公网入口为 HTTP IP；域名备案完成后配置 `SITE_ADDRESS` 启用 Caddy HTTPS，并同步 Secure cookie。
