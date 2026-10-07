#!/usr/bin/env bash
# 备份异地副本：backups/ 产物镜像到 GitHub 私有仓库（单 commit force-push，
# 大小恒定）。服务器本地轮转（backup.sh KEEP=14）不受影响；本仓库只保留
# 最新一份异地镜像。失败静默退出（/var/log/blog-backup-push.log 可查）；
# 告警通道待 SMTP 立项后补。密钥：/root/.ssh/backup_repo_ed25519（deploy key）。
set -euo pipefail
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
