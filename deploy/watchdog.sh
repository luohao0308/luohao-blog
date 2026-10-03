#!/usr/bin/env bash
# 博客健康看门狗：连续失败则重启 backend，并记录日志
set -u
LOG=/var/log/blog-watchdog.log
H=http://127.0.0.1:8000/v1/articles/list?page_size=1
if curl -fsS --max-time 10 "$H" >/dev/null 2>&1; then
  echo 0 > /tmp/blog-health-fails
  exit 0
fi
fails=$(( $(cat /tmp/blog-health-fails 2>/dev/null || echo 0) + 1 ))
echo $fails > /tmp/blog-health-fails
echo "$(date "+%F %T") health fail #$fails" >> $LOG
if [ "$fails" -ge 2 ]; then
  echo "$(date "+%F %T") restarting blog-backend" >> $LOG
  docker restart blog-backend >/dev/null 2>&1
  echo 0 > /tmp/blog-health-fails
fi
