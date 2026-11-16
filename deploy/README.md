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

`caddy/Caddyfile` 已启用：所有请求先到 Nuxt（:3000），由 Nuxt BFF 把 `/api/v1/*` 代理至 Kratos（:8000），同时处理 refresh cookie 路径。当前公开入口为 `https://makerhao.cn`；生产通过 `SITE_ADDRESS` 和域名配置启用 Caddy HTTPS，`AUTH_SECURE_COOKIE=true` 明确开启安全会话。

域名模式的生产字段、DNS、验收及回退见 [域名切换 Runbook](../docs/operations/runbooks/domain-cutover.md)。生产模板默认 makerhao.cn HTTPS；本地 HTTP 冒烟需按该指南覆盖五个入口字段。

## 生产前端同镜像替换

`retained_frontend_release.py` 面向当前登记的博客生产前端。它固定 Dev Workflow 的已发布 `main` 提交并使用该版本的审批客户端和 worker。博客预检探针来自博客已发布的 `main`，通过独立归档在 SSH 上读取登记、容器、Caddy 和事务状态。容器替换使用 Dev Workflow 原始归档。

在两边的迁移改动都进入 `main` 后，从博客的 `main` checkout 运行。需要 Python 3.12+、已登录的 GitHub CLI、现有生产 SSH 配置，以及 origin 为 `https://github.com/luohao0308/dev-workflow.git` 的本地 Dev Workflow checkout。

```bash
runtime_checkout=/absolute/path/to/dev-workflow
runtime_sha=$(gh api --hostname github.com repos/luohao0308/dev-workflow/branches/main --jq '.commit.sha')
workspace=$(cd "$(mktemp -d)" && pwd -P)
chmod 700 "$workspace"
python3 -I deploy/retained_frontend_release.py preflight \
  --runtime-checkout "$runtime_checkout" --runtime-sha "$runtime_sha" --workspace "$workspace"
```

`preflight` 只做生产读取，将基线保存为本地 `baseline.json`。如要执行经授权的生产替换，使用同一组参数、同一目录和 `release` 模式，并增加 `--recipient-private-key /absolute/path/to/private-key`。密钥文件须为操作人所有且权限为 `0600`。源提交、运行时或生产基线有变化时，用新建的私有目录重新运行预检。

`release` 会输出 GitHub 审批链接与精确审批评论。一次审批绑定同一发布计划，由 Dev Workflow 取得各阶段的短期授权。`completed` 的 `disposition` 为 `retained-candidate` 时替换已结案，原容器停机保留。若切换后检测到已验证的就绪故障并成功恢复原容器，返回 `restored-original` 和退出码 2。遇到 `blocked` 时查看事务状态再决定后续操作。第 14 代生产验收覆盖同镜像替换的成功路径。[验收运行 #43830947832](https://github.com/luohao0308/dev-workflow/actions/runs/43830947832)
