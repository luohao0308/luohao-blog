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
| 更新部署（流水线） | main 合并后 Actions 自动构建推 ghcr；先核验准确源码、固定镜像和当前 Compose 覆盖文件链，按 §5.2 仅更新获授权的服务。存在分服务覆盖文件时，不能仅凭基础文件与全局 `IMAGE_TAG` 重建应用 |
| 更新部署（无网络构建回退） | `docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod up -d --build`（compose 内 build 段保留） |
| 应用回滚 | 使用发布前记录的准确旧镜像和完整覆盖文件链，仅回退获授权的服务；应用回退不自动恢复或覆盖数据库 |
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

### 5.2 分服务固定镜像发布

更新前只读检查运行容器的 `com.docker.compose.project` 与 `com.docker.compose.project.config_files` 标签，确认完整且可信的 Compose 文件链。前后端可能来自不同次发布；基础文件之外的发布覆盖文件可分别固定它们的镜像摘要，不能遗漏已有覆盖文件，也不能从 Git HEAD 或全局 IMAGE_TAG 推定运行镜像。

先核验源码树、原始发布摘要/平台及现有回退镜像，完成获授权的备份/完整性检查。使用相同 project、project-directory、env-file 和完整 `-f` 链，仅对目标服务执行 `up -d --no-deps --no-build --pull never <service>`；执行前必须保证批准的固定镜像已在服务器缓存。验证目标镜像、健康和关键接口，并核对其他容器ID与配置未变。传输格式转换后若摘要不同，不直接替代批准镜像；保留原始 OCI 图及摘要可避免此类差异。

发布和回退均保存受控的原镜像/覆盖链/备份校验证据。一次发布的源码、镜像与文件链记录属于该次交付材料；本Runbook不硬编码一次性版本或签名URL。

## 6. 回滚

- 应用回滚：核验已授权的准确旧镜像及配置兼容性，按 §5.2 保留完整覆盖链，仅重建目标应用服务。
- 数据恢复会覆盖当前数据，必须另获明确授权并校验目标备份；不作为应用回退的自动步骤。
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


### 看门狗健康判定（网站补齐 S1）

`deploy/watchdog.sh` 复用 `blog-backend` 的 Docker healthcheck；backend不开放宿主8000，不能用宿主地址检测。只有 running + unhealthy 连续两轮才重启；healthy清零。starting/restarting/stopped/missing/没有healthcheck/inspect失败只记录告警并清零，不重启未知状态。每次重启尝试后30分钟冷却，失败也保留冷却。`flock`非阻塞锁避免cron重叠；缺少flock退出告警，不自动安装系统包。

默认私有状态目录 `/var/lib/blog-watchdog`、日志 `/var/log/blog-watchdog.log`，无需新增容器或数据库迁移。既有每5分钟cron保持；上线使用已批准版本的脚本，备份旧脚本与cron并验证容器健康、日志不误报。最小回退为在该cron命令前设置 `BLOG_WATCHDOG_RESTART_ENABLED=0` 只告警，不恢复错误宿主8000探针。

本地无真实Docker副作用的回归：`python3 scripts/test_watchdog.py`、`bash -n deploy/watchdog.sh`。仅本地通过不代表新脚本已上线；生产修改仍按准确版本授权执行。
