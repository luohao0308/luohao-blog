#!/usr/bin/env bash
# MySQL 逻辑备份：compose 内 mysqldump | gzip → backups/blog-<时间戳>.sql.gz，
# 保留最近 KEEP 份。恢复见同目录 restore.sh。
set -euo pipefail
cd "$(dirname "$0")/.."

# 备份含全库数据：文件权限一律 600，防止同机其他用户读取
umask 077

KEEP=${KEEP:-14}
COMPOSE="docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod"

mkdir -p backups
file="backups/blog-$(date +%Y%m%d-%H%M%S).sql.gz"
$COMPOSE exec -T mysql sh -c 'exec mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --default-character-set=utf8mb4 --single-transaction blog' | gzip > "$file"
ls -1t backups/blog-*.sql.gz | tail -n +$((KEEP + 1)) | xargs -r rm --
echo "backup written: $file"
