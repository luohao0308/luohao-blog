#!/usr/bin/env bash
# Docker's in-container healthcheck is authoritative; backend has no host port.
set -euo pipefail
umask 077
LOG=${BLOG_WATCHDOG_LOG:-/var/log/blog-watchdog.log}
STATE_DIR=${BLOG_WATCHDOG_STATE_DIR:-/var/lib/blog-watchdog}

log() { printf '%s %s\n' "$(date '+%F %T')" "$1" >> "$LOG"; }
if ! command -v flock >/dev/null 2>&1; then
  log "flock unavailable; watchdog skipped"
  exit 0
fi
if [ -L "$STATE_DIR" ]; then
  log "unsafe state directory; watchdog skipped"
  exit 0
fi
mkdir -p -m 700 "$STATE_DIR"
exec 9>"$STATE_DIR/lock"
flock -n 9 || exit 0

clear_failures() { printf '0\n' > "$STATE_DIR/health-fails"; }
if ! probe=$(docker inspect --format '{{.State.Status}} {{.State.Running}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' blog-backend 2>/dev/null); then
  clear_failures
  log "backend inspection unavailable; no restart"
  exit 0
fi
case "$probe" in
  'running true healthy') clear_failures; exit 0 ;;
  'running true unhealthy') ;;
  *)
    clear_failures
    log "backend not in running/unhealthy state; no restart"
    exit 0
    ;;
esac

fails=$(cat "$STATE_DIR/health-fails" 2>/dev/null || printf '0')
case "$fails" in 0|1|2) ;; *) fails=0 ;; esac
if [ "$fails" -lt 2 ]; then fails=$((fails + 1)); fi
printf '%s\n' "$fails" > "$STATE_DIR/health-fails"
log "backend unhealthy check #$fails"
if [ "$fails" -lt 2 ]; then exit 0; fi
if [ "${BLOG_WATCHDOG_RESTART_ENABLED:-1}" != 1 ]; then
  log "automatic restart disabled; alert only"
  exit 0
fi

now=$(date +%s)
last=$(cat "$STATE_DIR/last-restart" 2>/dev/null || printf '0')
if ! [[ "$now" =~ ^(0|[1-9][0-9]{0,11})$ ]] || ! [[ "$last" =~ ^(0|[1-9][0-9]{0,11})$ ]]; then
  log "invalid restart clock; no restart"
  exit 0
fi
# Clock rollback also stays in cooldown rather than causing a restart storm.
if [ "$last" -gt 0 ] && [ "$((now - last))" -lt 1800 ]; then
  log "restart cooldown active; no restart"
  exit 0
fi
# Record the attempt before Docker is called, including failed restarts.
printf '%s\n' "$now" > "$STATE_DIR/last-restart"
clear_failures
if docker restart blog-backend >/dev/null 2>&1; then
  log "restarted blog-backend after consecutive unhealthy checks"
else
  log "backend restart failed; cooldown retained"
fi
