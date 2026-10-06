#!/usr/bin/env bash
# 从 backups/blog-<时间戳>.sql.gz 恢复到 compose 内的 MySQL。
# 可选第二参数：同时恢复用户上传（头像）包 uploads-<时间戳>.tar.gz。
# 用法：deploy/restore.sh backups/blog-20261002-120000.sql.gz [backups/uploads-....tar.gz]
#
# 恢复是破坏性的（整库覆盖）。顺序：先离线校验两个包（不碰服务，坏包在
# 停服前就被拒）；uploads 直写卷不涉库，趁 backend 在线先行恢复；随后停
# backend——它是唯一的库写方，边服务边恢复会与应用连接竞态写坏数据——
# 灌库后回启并等健康。ES 索引是派生数据，恢复完成后需重建（末尾提示）。
# frontend/caddy 全程在线，数据层缺失期间的短暂 5xx 在 backend 回启后自愈。
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod)

file=${1:?usage: deploy/restore.sh backups/blog-<timestamp>.sql.gz [backups/uploads-<timestamp>.tar.gz]}
[ -f "$file" ] || { echo "not found: $file"; exit 1; }
# 恢复前先验完整性：截断的 .gz 会在 EOF 处静默结束，直接灌库会造成半库假成功
gzip -t "$file" || { echo "corrupt gzip archive: $file"; exit 1; }
# 头部校验：确认是 mysqldump 产物，别停了服务才发现灌的不是库
header=$(gunzip -c -- "$file" | head -c 4096 || true)
grep -aq "MySQL dump" <<< "$header" || { echo "not a mysqldump file: $file"; exit 1; }

uploads=${2:-}
if [ -n "$uploads" ]; then
	[ -f "$uploads" ] || { echo "not found: $uploads"; exit 1; }
	gzip -t "$uploads" || { echo "corrupt gzip archive: $uploads"; exit 1; }
fi

if [ -n "$uploads" ]; then
	# uploads 是独立卷的文件覆盖，不涉数据库（backend 在线即可执行）
	"${COMPOSE[@]}" exec -T backend tar xzf - -C /app/data < "$uploads"
	echo "restored: $uploads"
fi

echo "==> stopping backend（恢复期间唯一写方必须离线）"
"${COMPOSE[@]}" stop backend

rc=0
gunzip -c -- "$file" | "${COMPOSE[@]}" exec -T mysql sh -c \
	'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --default-character-set=utf8mb4 blog' || rc=$?
if (( rc != 0 )); then
	echo "!! 灌库失败 rc=$rc：backend 已回启；库可能处于半恢复态，排查后再试" >&2
	"${COMPOSE[@]}" start backend
	exit "$rc"
fi
echo "restored: $file"

echo "==> starting backend"
"${COMPOSE[@]}" start backend
for _ in $(seq 1 24); do
	health=$("${COMPOSE[@]}" ps --format '{{.Name}} {{.Health}}' | awk '$1 == "blog-backend" {print $2}')
	[ "$health" = "healthy" ] && { echo "backend healthy"; break; }
	sleep 5
done

echo "==> 下一步：重建派生的 ES 索引"
echo "    ${COMPOSE[*]} exec backend /app/reindex -conf /data/conf"
