#!/usr/bin/env bash
# Fixed-format backup restore. Failure leaves the independent guard pending and backend stopped.
set -euo pipefail
umask 077
cd "$(dirname "$0")/.."
COMPOSE=(docker compose -f deploy/compose.prod.yml --env-file deploy/.env.prod)
file=${1:?usage: deploy/restore.sh backup.sql.gz [uploads.tar.gz]}
uploads=${2:-}
manifest="${file}.manifest.json"
[ -f "$file" ] && [ -f "$manifest" ] || { echo 'backup and integrity sidecar required; legacy arbitrary dumps need a separate recovery procedure' >&2; exit 1; }
python3 deploy/time_backup_manifest.py check "$file" "$manifest" >/dev/null
if [ -n "$uploads" ]; then
 python3 - "$uploads" <<'PY'
import pathlib,sys,tarfile
with tarfile.open(sys.argv[1], 'r:gz') as archive:
 for item in archive:
  parts=pathlib.PurePosixPath(item.name).parts
  if not parts or parts[0]!='uploads' or '..' in parts or item.name.startswith('/') or not (item.isfile() or item.isdir()):
   raise SystemExit('unsafe uploads archive')
PY
fi
[ "${BLOG_RESTORE_WRITERS_PAUSED:-0}" = 1 ] || { echo 'explicit confirmation that all other writers are paused is required' >&2; exit 1; }
evidence_root=${BLOG_RESTORE_EVIDENCE_DIR:-"$PWD/backups"}
mkdir -p -m 700 "$evidence_root"
operation_dir=$(mktemp -d "$evidence_root/restore.XXXXXXXX")
# Keep attempt/verification receipts; remove only the private temporary redump.
trap 'rm -f -- "$operation_dir/redump.sql.gz"' EXIT
# Capture only non-secret runtime metadata before stopping the container.
docker inspect --format '{"id":{{json .Id}},"image":{{json .Image}},"files":{{json (index .Config.Labels "com.docker.compose.project.config_files")}},"project":{{json (index .Config.Labels "com.docker.compose.project")}},"directory":{{json (index .Config.Labels "com.docker.compose.project.working_dir")}},"mounts":{{json .Mounts}}}' blog-backend > "$operation_dir/backend.json"
python3 - "$operation_dir" <<'PYMETA'
import json,pathlib,sys
root=pathlib.Path(sys.argv[1]);meta=json.loads((root/'backend.json').read_text())
assert meta['project'] and meta['directory'] and meta['files'] and meta['id']
assert meta['image'].startswith('sha256:') and len(meta['image'])==71
state=next(m for m in meta['mounts'] if m['Destination']=='/app/data/time-contract')
assert state['Type'] in ('bind','volume')
source=state['Name'] if state['Type']=='volume' else state['Source']
assert source and ':' not in source
(root/'state-source').write_text(source)
(root/'image-id').write_text(meta['image'])
(root/'compose.operator.yml').write_text('services:\n  backend:\n    image: '+json.dumps(meta['image'])+'\n')
args=['docker','compose','--project-name',meta['project'],'--project-directory',meta['directory'],'--env-file',str(pathlib.Path(meta['directory'])/'.env.prod')]
for path in meta['files'].split(',')+[str(root/'compose.operator.yml')]:args+=['-f',path]
(root/'compose.args').write_bytes(b''.join(arg.encode()+b'\0' for arg in args))
PYMETA
COMPOSE=()
while IFS= read -r -d '' arg; do COMPOSE+=("$arg"); done < "$operation_dir/compose.args"
state_source=$(cat "$operation_dir/state-source")
image_id=$(cat "$operation_dir/image-id")
# Unsupported Compose versions and absent images must fail before pending/stop.
if ! "${COMPOSE[@]}" run --help > "$operation_dir/run-help.txt" 2>&1; then
 echo 'Compose run capability check failed; no recovery state changed' >&2; exit 1
fi
python3 - "$operation_dir/run-help.txt" <<'PYCAP'
import pathlib,re,sys
help_text=pathlib.Path(sys.argv[1]).read_text()
if not re.search(r'^\s+--pull(?:[ =]|$)', help_text, re.M):
 raise SystemExit('Compose run must support --pull; no recovery state changed')
PYCAP
if ! cached_image=$(docker image inspect --format '{{.Id}}' "$image_id" 2>/dev/null) || [ "$cached_image" != "$image_id" ]; then
 echo 'immutable operator image is not cached; no recovery state changed' >&2; exit 1
fi
if ! "${COMPOSE[@]}" config -q >/dev/null 2>&1; then
 echo 'Compose configuration validation failed; no recovery state changed' >&2; exit 1
fi
operation_id=$(python3 -c 'import uuid;print(uuid.uuid4())')
backup_sha=$(python3 - "$manifest" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))['archive_sha256'])
PY
)
manifest_sha=$(python3 - "$manifest" <<'PY'
import hashlib,sys
print(hashlib.sha256(open(sys.argv[1],'rb').read()).hexdigest())
PY
)
operator() {
 "${COMPOSE[@]}" run --rm --no-deps --pull never -T -v "$state_source:/app/data/time-contract:rw" -v "$operation_dir:/time-operation:rw" backend /app/time-contract -conf /data/conf -state-dir /app/data/time-contract "$@"
}
# This operation state survives DB import and container recreation.
operator -mode restore-begin -operation-id "$operation_id" -backup-sha256 "$backup_sha" -manifest-sha256 "$manifest_sha" > "$operation_dir/begin.json"
"${COMPOSE[@]}" stop backend
# Verify the original target actually stopped; do not infer it from command exit alone.
docker inspect --format '{"id":{{json .Id}},"running":{{json .State.Running}},"status":{{json .State.Status}}}' blog-backend > "$operation_dir/stopped.json"
python3 - "$operation_dir/backend.json" "$operation_dir/stopped.json" <<'PYSTOP'
import json,sys
before=json.load(open(sys.argv[1]));after=json.load(open(sys.argv[2]))
if after['id']!=before['id'] or after['running'] is not False or after['status'] not in ('exited','created'):
 raise SystemExit('original backend is not stopped; recovery remains pending')
PYSTOP
# No failure branch starts a backend. All other writers must also be paused.
if [ -n "$uploads" ]; then
 "${COMPOSE[@]}" run --rm --no-deps --pull never -T backend tar xzf - -C /app/data < "$uploads"
fi
gunzip -c -- "$file" | "${COMPOSE[@]}" exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --default-character-set=utf8mb4 blog'
"${COMPOSE[@]}" exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --tz-utc --order-by-primary --skip-extended-insert --complete-insert --hex-blob --default-character-set=utf8mb4 --skip-comments --no-tablespaces --set-gtid-purged=OFF blog' | gzip > "$operation_dir/redump.sql.gz"
python3 deploy/time_backup_manifest.py compare "$operation_dir/redump.sql.gz" "$manifest" >/dev/null
operator -mode snapshot > "$operation_dir/snapshot.json"
python3 - "$operation_dir/snapshot.json" "$operation_dir/verification.json" "$operation_id" "$backup_sha" "$manifest_sha" "$operation_dir/begin.json" <<'PY'
import json,sys
snapshot=json.load(open(sys.argv[1]))
begin=json.load(open(sys.argv[6]))
nonce=begin.get("attempt_nonce", "")
if len(nonce)!=64 or any(c not in "0123456789abcdef" for c in nonce):raise SystemExit("restore begin nonce missing or invalid")
receipt={'format_version':1,'attempt_nonce':nonce,'operation_id':sys.argv[3],'target':snapshot['target'],'backup_sha256':sys.argv[4],'manifest_sha256':sys.argv[5],'import_succeeded':True,'redump_verified':True,'snapshot_sha256':snapshot['snapshot_sha256']}
with open(sys.argv[2],'x') as target:json.dump(receipt,target)
PY
operator -mode restore-complete -operation-id "$operation_id" -backup-sha256 "$backup_sha" -manifest-sha256 "$manifest_sha" -verification-receipt /time-operation/verification.json -writers-paused > "$operation_dir/complete.json"
python3 - "$operation_dir/complete.json" <<'PY'
import json,sys
result=json.load(open(sys.argv[1]))
if result.get('ready') is not True:raise SystemExit('restore verified but UTC runtime is not ready; keep stopped for explicit contract recovery')
PY
"${COMPOSE[@]}" start backend
healthy=0
for _ in $(seq 1 24); do
 health=$("${COMPOSE[@]}" ps --format '{{.Name}} {{.Health}}' | awk '$1 == "blog-backend" {print $2}')
 if [ "$health" = healthy ]; then healthy=1; break; fi
 sleep 5
done
[ "$healthy" = 1 ] || { echo 'backend health check failed' >&2; exit 1; }
echo 'restore complete and UTC guard ready; derived search reindex remains a separate operation'
