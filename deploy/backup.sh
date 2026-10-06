#!/usr/bin/env bash
# MySQL 逻辑备份：compose 内 mysqldump | gzip → backups/blog-<时间戳>.sql.gz，
# 保留最近 KEEP 份。恢复见同目录 restore.sh。
set -euo pipefail
cd "$(dirname "$0")/.."

# 备份含全库数据：文件权限一律 600，防止同机其他用户读取
umask 077

KEEP=${KEEP:-14}
COMPOSE=(docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod)

mkdir -p backups
ts="$(date +%Y%m%d-%H%M%S)"
file="backups/blog-$ts.sql.gz"
uploads="backups/uploads-$ts.tar.gz"
tmp="${file}.tmp.$$"
utmp="${uploads}.tmp.$$"

cleanup() {
	local status=$?
	if (( status != 0 )); then
		rm -f -- "$tmp" "$utmp"
	fi
	return "$status"
}
trap cleanup EXIT

"${COMPOSE[@]}" exec -T mysql sh -c 'exec mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --default-character-set=utf8mb4 --single-transaction blog' | gzip > "$tmp"

# gzip -t catches truncation; awk consumes the full stream so pipefail cannot
# turn a valid dump into a false failure when a downstream matcher exits early.
gzip -t -- "$tmp"
if ! gunzip -c -- "$tmp" | awk '/^-- (MySQL|MariaDB) dump|^CREATE TABLE|^INSERT INTO/ { found = 1 } END { exit !found }'; then
	echo "invalid or empty MySQL dump: $tmp" >&2
	exit 1
fi
mv -- "$tmp" "$file"

# 用户上传（头像）随库一起备份：从 backend 容器打包 uploads 目录（compose
# 命名卷 backend-uploads）。空目录产出合法的空包，不会失败。
"${COMPOSE[@]}" exec -T backend tar czf - -C /app/data uploads > "$utmp"
gzip -t -- "$utmp"
mv -- "$utmp" "$uploads"

# 轮转删除超出 KEEP 的最旧备份。文件名由本脚本生成（时间戳命名，字典序即
# 时间序）；glob 展开不经过空白分词，替代此前的 ls | xargs 管道。
rotate() {
	local pattern=$1 keep=$2
	shopt -s nullglob
	local files=($pattern)
	shopt -u nullglob
	local excess=$((${#files[@]} - keep))
	local i
	for ((i = 0; i < excess; i++)); do
		rm -- "${files[i]}"
	done
}
rotate 'backups/blog-*.sql.gz' "$KEEP"
rotate 'backups/uploads-*.tar.gz' "$KEEP"
echo "backup written: $file, $uploads"
