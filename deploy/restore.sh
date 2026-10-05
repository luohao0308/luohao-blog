#!/usr/bin/env bash
# 从 backups/blog-<时间戳>.sql.gz 恢复到 compose 内的 MySQL。
# 可选第二参数：同时恢复用户上传（头像）包 uploads-<时间戳>.tar.gz。
# 用法：deploy/restore.sh backups/blog-20261002-120000.sql.gz [backups/uploads-....tar.gz]
set -euo pipefail
cd "$(dirname "$0")/.."

file=${1:?usage: deploy/restore.sh backups/blog-<timestamp>.sql.gz [backups/uploads-<timestamp>.tar.gz]}
[ -f "$file" ] || { echo "not found: $file"; exit 1; }
# 恢复前先验完整性：截断的 .gz 会在 EOF 处静默结束，直接灌库会造成半库假成功
gzip -t "$file" || { echo "corrupt gzip archive: $file"; exit 1; }
docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod exec -T mysql sh -c \
  'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --default-character-set=utf8mb4 blog' \
  < <(gunzip -c "$file")
echo "restored: $file"

if [ -n "${2:-}" ]; then
	uploads=$2
	[ -f "$uploads" ] || { echo "not found: $uploads"; exit 1; }
	gzip -t "$uploads" || { echo "corrupt gzip archive: $uploads"; exit 1; }
	docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod exec -T backend \
		tar xzf - -C /app/data < "$uploads"
	echo "restored: $uploads"
fi
