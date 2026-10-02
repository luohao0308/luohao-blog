#!/usr/bin/env bash
# 从 backups/blog-<时间戳>.sql.gz 恢复到 compose 内的 MySQL。
# 用法：deploy/restore.sh backups/blog-20261002-120000.sql.gz
set -euo pipefail
cd "$(dirname "$0")/.."

file=${1:?usage: deploy/restore.sh backups/blog-<timestamp>.sql.gz}
[ -f "$file" ] || { echo "not found: $file"; exit 1; }
docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod exec -T mysql sh -c \
  'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --default-character-set=utf8mb4 blog' \
  < <(gunzip -c "$file")
echo "restored: $file"
