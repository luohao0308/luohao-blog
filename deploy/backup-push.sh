#!/usr/bin/env bash
# 备份异地副本：backups/ 产物镜像到 GitHub 私有仓库（单 commit force-push，
# 大小恒定）。服务器本地轮转（backup.sh KEEP=14）不受影响；本仓库只保留
# 最新一份异地镜像。结果写入 node-exporter textfile，供异地备份告警使用。
# 日志：/var/log/blog-backup-push.log；密钥由服务器 SSH 配置管理。
set -euo pipefail

METRICS_DIR=${BACKUP_METRICS_DIR:-/var/lib/blog-node-metrics}

publish_metrics() (
  # Metrics carry no private backup paths or credentials, and need to be
  # readable by node-exporter even though the backup itself uses umask 077.
  umask 022
  metric_file="$METRICS_DIR/backup-sync.prom"
  previous=0
  if [[ -r "$metric_file" ]]; then
    previous=$(awk '$1 == "blog_backup_sync_last_success_timestamp_seconds" && $2 ~ /^[0-9]+$/ && NF == 2 {print $2; exit}' "$metric_file") || return 1
    previous=${previous:-0}
  fi
  now=$(date +%s) || return 1
  success=0
  if [[ "$1" == 0 ]]; then success=1; previous=$now; fi
  mkdir -p "$METRICS_DIR" || return 1
  chmod 755 "$METRICS_DIR" || return 1
  temporary=$(mktemp "$METRICS_DIR/backup-sync.XXXXXX.tmp") || return 1
  trap 'rm -f "$temporary"' EXIT
  printf 'blog_backup_sync_success %s\nblog_backup_sync_last_success_timestamp_seconds %s\nblog_backup_sync_last_attempt_timestamp_seconds %s\n' \
    "$success" "$previous" "$now" > "$temporary" || return 1
  chmod 644 "$temporary" || return 1
  mv -f "$temporary" "$metric_file" || return 1
)

finish() {
  status=$?
  trap - EXIT
  if ! publish_metrics "$status"; then
    printf 'WARNING: cannot publish backup sync metrics\n' >&2
  fi
  exit "$status"
}
trap finish EXIT

cd "$(dirname "$0")/.."

REMOTE=backup-github
BRANCH=latest

umask 077
cd backups
if [ ! -d .git ]; then
  git init -q -b "$BRANCH"
  git config user.email "backup@luohao.blog"
  git config user.name "luohao-blog backup"
  git remote add "$REMOTE" git@github.com-backup:luohao0308/luohao-blog-backups.git
fi
git add -A
git commit -q -m "backup $(date +%Y-%m-%d)" --allow-empty
git push -q --force "$REMOTE" "$BRANCH"
