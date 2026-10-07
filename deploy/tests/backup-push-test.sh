#!/usr/bin/env bash
# No real Git repository or remote is touched: every git invocation is faked.
set -euo pipefail
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT
mkdir -p "$TEST_ROOT/app/deploy" "$TEST_ROOT/app/backups" "$TEST_ROOT/bin"
cp "$(dirname "$0")/../backup-push.sh" "$TEST_ROOT/app/deploy/backup-push.sh"
cat > "$TEST_ROOT/bin/git" <<'FAKE'
#!/usr/bin/env bash
if [[ "$1" == push ]]; then exit "${FAKE_PUSH_STATUS:-0}"; fi
exit 0
FAKE
chmod +x "$TEST_ROOT/bin/git"
export PATH="$TEST_ROOT/bin:$PATH"
export BACKUP_METRICS_DIR="$TEST_ROOT/metrics"

bash "$TEST_ROOT/app/deploy/backup-push.sh"
METRICS="$BACKUP_METRICS_DIR/backup-sync.prom"
grep -qx 'blog_backup_sync_success 1' "$METRICS"
PREVIOUS=$(awk '$1 == "blog_backup_sync_last_success_timestamp_seconds" {print $2}' "$METRICS")
[[ "$PREVIOUS" =~ ^[0-9]+$ && "$PREVIOUS" -gt 0 ]]
[[ $(awk '$1 == "blog_backup_sync_last_attempt_timestamp_seconds" {print $2}' "$METRICS") -gt 0 ]]
[[ $(find "$BACKUP_METRICS_DIR" -name '*.prom' | wc -l | tr -d ' ') == 1 ]]
[[ $(find "$BACKUP_METRICS_DIR" -name '*.tmp' | wc -l | tr -d ' ') == 0 ]]
[[ $(stat -f '%Lp' "$METRICS" 2>/dev/null || stat -c '%a' "$METRICS") == 644 ]]

status=0
FAKE_PUSH_STATUS=42 bash "$TEST_ROOT/app/deploy/backup-push.sh" || status=$?
[[ "$status" == 42 ]]
[[ $(find "$BACKUP_METRICS_DIR" -name '*.tmp' | wc -l | tr -d ' ') == 0 ]]
grep -qx 'blog_backup_sync_success 0' "$METRICS"
grep -qx "blog_backup_sync_last_success_timestamp_seconds $PREVIOUS" "$METRICS"

printf 'blog_backup_sync_last_success_timestamp_seconds invalid\n' > "$METRICS"
FAKE_PUSH_STATUS=42 bash "$TEST_ROOT/app/deploy/backup-push.sh" || status=$?
grep -qx 'blog_backup_sync_last_success_timestamp_seconds 0' "$METRICS"

# A regular file in place of the directory makes publication fail even as root.
touch "$TEST_ROOT/not-a-directory"
BACKUP_METRICS_DIR="$TEST_ROOT/not-a-directory" bash "$TEST_ROOT/app/deploy/backup-push.sh" 2> "$TEST_ROOT/warning"
grep -q 'WARNING: cannot publish backup sync metrics' "$TEST_ROOT/warning"
status=0
BACKUP_METRICS_DIR="$TEST_ROOT/not-a-directory" FAKE_PUSH_STATUS=42 bash "$TEST_ROOT/app/deploy/backup-push.sh" 2> "$TEST_ROOT/warning" || status=$?
[[ "$status" == 42 ]]
# Failures before git (for example a missing local backup directory) are also
# attempts, and must preserve the last successful sync timestamp.
mv "$TEST_ROOT/app/backups" "$TEST_ROOT/app/backups-away"
status=0
bash "$TEST_ROOT/app/deploy/backup-push.sh" 2> "$TEST_ROOT/early-error" || status=$?
[[ "$status" -ne 0 ]]
grep -qx 'blog_backup_sync_success 0' "$METRICS"
printf 'backup-push regression tests passed\n'
