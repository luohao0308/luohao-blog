# Runbook：首次部署上线（M5/S2）

> 适用：全新 Ubuntu 22.04/24.04 VPS（2C4G 起步）+ 一个解析到服务器 IP 的域名。
> 前置验证：S1 已在本机跑通「生产形态」全栈与灾难恢复（`deploy/compose.prod.yml`）。
> 执行约定：本文按「AI 准备、用户在服务器执行」编写；每一步的预期输出都写明，
> 卡住时把输出交给 AI 排查。生产相关 AI 远程操作需逐次授权。

## 0. 前置检查（购买/开通后）

```bash
# 本机（不是服务器）：DNS 是否生效，返回服务器 IP 即可
dig +short <你的域名>
# 服务器上：确认架构与资源
uname -m          # x86_64 或 aarch64
nproc && free -h  # 建议 ≥2 核 / ≥4G
```

- 云厂商安全组/防火墙放行 **22（SSH）、80、443**。
- 国内服务器需完成 ICP 备案后域名才能解析使用；海外无此要求。

## 1. 服务器初始化（root 或 sudo 用户）

```bash
apt update && apt -y upgrade
apt install -y ca-certificates curl git

# Docker 官方源安装（含 compose 插件）
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  > /etc/apt/sources.list.d/docker.list
apt update && apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 防火墙：只放行 SSH/HTTP/HTTPS
ufw allow OpenSSH && ufw allow 80,443/tcp && ufw --force enable
```

预期：`docker compose version` 输出 v2.x。

## 2. 代码与密钥上服务器

```bash
mkdir -p /opt/luohao-blog && cd /opt/luohao-blog
git clone https://github.com/luohao0308/luohao-blog.git app && cd app

# 两个密钥文件不进 Git，从本机 scp（在本机执行）：
#   scp deploy/.env.prod                 user@服务器:/opt/luohao-blog/deploy/.env.prod
#   scp deploy/prod-conf/secrets.local.yaml user@服务器:/opt/luohao-blog/deploy/prod-conf/secrets.local.yaml
chmod 600 deploy/.env.prod deploy/prod-conf/secrets.local.yaml
```

`.env.prod`（模板 `deploy/.env.prod.example`）：`SITE_ADDRESS=<你的域名>`、
`MYSQL_ROOT_PASSWORD=<强密码>`、`DATABASE_SOURCE=root:<同密码>@tcp(mysql:3306)/blog?...`。
`prod-conf/secrets.local.yaml`：`auth.jwt.secret=<openssl rand -hex 32>`、
embedding/llm 段填 SiliconFlow/DeepSeek 的 key（同本机 secrets 文件）。

## 3. 首次部署

```bash
docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod up -d --build
docker compose -f deploy/compose.prod.yml ps        # 六服务全部 Up，backend/frontend/mysql healthy
```

首次构建约 5-10 分钟（Nuxt + Go 在服务器上编译；2C4G 可行，勿同时跑其他重活）。

初始化数据（账号 + 演示文章 → 搜索向量）：

```bash
DS=$(grep '^DATABASE_SOURCE=' deploy/.env.prod | cut -d= -f2-)
docker run --rm --network luohao-blog-prod_default -e KRATOS_DATABASE_SOURCE="$DS" \
  -v "$PWD/deploy/prod-conf:/data/conf:ro" luohao-blog-backend:latest \
  ./seed -conf /data/conf -email <你的邮箱> -password '<≥8位密码>' -name '<显示名>' -demo-articles
docker run --rm --network luohao-blog-prod_default -e KRATOS_DATABASE_SOURCE="$DS" \
  -v "$PWD/deploy/prod-conf:/data/conf:ro" luohao-blog-backend:latest \
  ./reindex -conf /data/conf
```

预期：`seed: created admin ...`；`reindex: rebuilt index with 4 published articles`
（或更多）。**改掉演示账号密码或删除演示文章**（管理后台操作）后再正式使用。

## 4. 上线验证清单（浏览器 + 命令）

```bash
curl -sI https://<你的域名> | head -3        # HTTP/2 200，证书为 Caddy 签发的有效证书
curl -s "https://<你的域名>/api/v1/articles/list?page_size=3" | head -c 200
```

浏览器走完：首页 → 文章阅读 → 搜索 → 登录（admin）→ 发布 → 文章页「问一问」提问。
HTTPS 证书由 Caddy 自动向 Let's Encrypt 申请；若 443 无响应，检查 80 端口是否可从公网到达（ACME 校验需要）。

## 5. 日常运维

| 操作 | 命令 |
|---|---|
| 更新部署（流水线） | main 合并后 Actions 自动构建推 ghcr；服务器执行 `cd /opt/luohao-blog/app && git pull && docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod pull backend frontend && docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod up -d backend frontend` |
| 更新部署（无网络构建回退） | `docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod up -d --build`（compose 内 build 段保留） |
| 应用回滚 | `docker pull ghcr.io/luohao0308/luohao-blog-backend:<旧sha>` 后把 compose 的 image tag 固定为该 sha 再 `up -d`；数据回滚用 restore.sh |
| 备份 | `deploy/backup.sh`（cron 安装见 §5.1） |
| 恢复 | `deploy/restore.sh backups/blog-<时间戳>.sql.gz [uploads-<时间戳>.tar.gz]`——脚本自动停/启 backend（写方离线防竞态），坏包在停服前拒收；完成后按末尾提示跑 reindex 对齐 ES |
| 日志 | `docker logs blog-backend --tail 100` / `blog-frontend` / `blog-caddy`；json-file 轮转 10MB×3 已在 compose 六服务的 `logging` 声明（改配置经 `up -d` 滚动重建生效，无需动 daemon.json） |
| 服务状态 | `docker compose -f deploy/compose.prod.yml ps` |

### 5.1 定时任务（备份 + 看门狗，真相源）

两台定时任务都在 **root 的 crontab** 里，脚本本体在仓库 `deploy/` 下：

```cron
# 每日 03:10 数据库备份（KEEP=14 份轮转，日志 /var/log/blog-backup.log）
10 3 * * * /opt/luohao-blog/app/deploy/backup.sh >> /var/log/blog-backup.log 2>&1
# 每 5 分钟健康看门狗：连续 2 次失败自动 docker restart blog-backend
# （日志 /var/log/blog-watchdog.log）
*/5 * * * * /opt/luohao-blog/app/deploy/watchdog.sh
```

安装（root）：

```bash
crontab -e   # 追加上面两行
```

验证：

```bash
crontab -l | grep luohao-blog            # 两行都在
ls -la /opt/luohao-blog/app/deploy/{backup,watchdog}.sh   # 脚本存在且可执行
tail -5 /var/log/blog-watchdog.log       # watchdog 在运行（应有周期性记录或为空即健康）
ls backups/ | tail -3                    # 次日 03:10 后应出现新 dump
```

注意：watchdog/backup 由 root crontab 触发；服务器重建或换机后这两行必须重装，
脚本本体已随仓库走（`deploy/watchdog.sh`、`deploy/backup.sh`）。

## 6. 回滚

- 应用回滚：`git log --oneline` 找上一个可用提交 → `git checkout <sha>` → 重新 `up -d --build`。
- 数据回滚：`deploy/restore.sh` 最近一次备份（注意会覆盖当前数据）。
- ES 索引是可重建的派生数据：数据库恢复后跑一次 reindex 即对齐。

## 7. 排错速查（本项目已踩过的坑）

| 症状 | 原因/解法 |
|---|---|
| 容器起不来：port 80 already in use | 服务器上有别的进程占 80：`ss -ltnp | grep :80`；或临时用 `HTTP_PORT=8080` |
| backend unhealthy | `docker logs blog-backend`；常见为 DSN 密码与 MYSQL_ROOT_PASSWORD 不一致 |
| frontend 构建失败 ERR_PNPM_IGNORED_BUILDS | pnpm-workspace.yaml 必须在 `pnpm install` 前存在（Dockerfile 已处理；勿删 COPY 行） |
| 搜索没结果/无向量 | 跑一次 reindex；确认 secrets 文件 embedding 段已填且 `dimensions` 与模型一致 |
| chat 回答变成「引用列表」 | LLM 上游慢/不可用，看 `docker logs blog-backend | grep "llm generation failed"`；DeepSeek 建议配 `reasoning_effort: none` |
| 登录后刷新掉线 | 检查域名与 SITE_ADDRESS 一致（cookie 作用域） |


## 8. 实际部署记录（2026-10-02，193.112.128.245 腾讯云 Ubuntu 24.04 2C4G）

- 服务器上原有 jobapp-staging（FastAPI+nginx+PostgreSQL，含 80/443）经确认后下线清理，释放 35.9GB；宿主 nginx（反代该 staging）一并 stop+disable
- 部署路径：`/opt/luohao-blog/app`；新增 2G swap（构建缓冲）；密钥文件 600 权限
- 踩坑：国内直连 deb.debian.org 卡死 → backend Dockerfile runtime 层换腾讯内网镜像源（已进 main）
- 公网验收：页面/API/登录/发布/混合搜索/真实 DeepSeek 问答全通；安全组 80 需在腾讯云控制台放行（本例已放行）
- 生产管理员：seed 创建（首次登录后请立即改密码）；演示文章可在后台删除
