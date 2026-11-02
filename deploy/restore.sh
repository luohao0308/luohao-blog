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
operation_dir=$(cd "$operation_dir" && pwd -P)
# Keep attempt/verification receipts; remove only the private temporary redump.
trap 'rm -f -- "$operation_dir/redump.sql.gz"' EXIT
# Capture only non-secret runtime metadata before stopping the container.
docker inspect --format '{"id":{{json .Id}},"image":{{json .Image}},"files":{{json (index .Config.Labels "com.docker.compose.project.config_files")}},"project":{{json (index .Config.Labels "com.docker.compose.project")}},"directory":{{json (index .Config.Labels "com.docker.compose.project.working_dir")}},"mounts":{{json .Mounts}}}' blog-backend > "$operation_dir/backend.json"
python3 - "$operation_dir" <<'PYMETA'
import json,pathlib,sys
root=pathlib.Path(sys.argv[1]);meta=json.loads((root/'backend.json').read_text())
assert root.is_absolute() and meta['project'] and meta['directory'] and meta['files'] and meta['id']
assert meta['image'].startswith('sha256:') and len(meta['image'])==71
mounts={m['Destination']:m for m in meta['mounts']}
assert len(mounts)==len(meta['mounts']) and '/time-operation' not in mounts
assert '/app/data/time-contract' in mounts and '/app/data/uploads' in mounts
assert mounts['/app/data/time-contract'].get('RW') is False
volumes={}
base=[]
for index,(target,mount) in enumerate(mounts.items()):
 assert target.startswith('/') and not any(c.isspace() or c=='\\' for c in target)
 assert mount['Type'] in ('bind','volume')
 source=mount['Name'] if mount['Type']=='volume' else mount['Source']
 assert source and ':' not in source and not any(c in source for c in '\r\n')
 item={'type':mount['Type'],'target':target,'read_only':True}
 if mount['Type']=='volume':
  key='utc-restore-volume-'+str(index);volumes[key]={'external':True,'name':source};item['source']=key
 else:
  assert pathlib.Path(source).is_absolute();item.update(source=source,bind={'create_host_path':False})
 base.append(item)
base.append({'type':'bind','source':str(root),'target':'/time-operation','read_only':True,'bind':{'create_host_path':False}})
expected={}
for mode,writable in [('operator',{'/app/data/time-contract','/time-operation'}),('uploads',{'/app/data/uploads'})]:
 items=[dict(item,read_only=item['target'] not in writable) for item in base]
 overlay={'services':{'backend':{'image':meta['image'],'read_only':True,'volumes':items}},'volumes':volumes}
 (root/('compose.'+mode+'.yml')).write_text(json.dumps(overlay,indent=2).replace('$','$$')+'\n')
 expected[mode]={'/':'ro',**{item['target']:'ro' if item['read_only'] else 'rw' for item in items}}
(root/'mount-expectations.json').write_text(json.dumps(expected))
(root/'image-id').write_text(meta['image'])
args=['docker','compose','--project-name',meta['project'],'--project-directory',meta['directory'],'--env-file',str(pathlib.Path(meta['directory'])/'.env.prod')]
for path in meta['files'].split(','):args+=['-f',path]
(root/'compose.args').write_bytes(b''.join(arg.encode()+b'\0' for arg in args))
PYMETA
COMPOSE=()
while IFS= read -r -d '' arg; do COMPOSE+=("$arg"); done < "$operation_dir/compose.args"
image_id=$(cat "$operation_dir/image-id")
OPERATOR=("${COMPOSE[@]}" -f "$operation_dir/compose.operator.yml")
UPLOADS=("${COMPOSE[@]}" -f "$operation_dir/compose.uploads.yml")
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
if ! "${OPERATOR[@]}" config -q >/dev/null 2>&1 || ! "${UPLOADS[@]}" config -q >/dev/null 2>&1; then
 echo 'Compose configuration validation failed; no recovery state changed' >&2; exit 1
fi
# Check actual container mounts before pending or stopping; CLI -v flags do not
# reliably override read_only declared by the existing service volume.
probe_mounts() {
 local mode=$1
 shift
 "$@" run --rm --no-deps --pull never -T --entrypoint /bin/sh backend -ec "awk '{print \$5,\$6}' /proc/self/mountinfo" > "$operation_dir/$mode-mounts.raw"
 python3 - "$operation_dir" "$mode" <<'PYMOUNTS'
import json,pathlib,sys
root=pathlib.Path(sys.argv[1]);mode=sys.argv[2]
expected=json.loads((root/'mount-expectations.json').read_text())[mode]
actual={}
for line in (root/(mode+'-mounts.raw')).read_text().splitlines():
 parts=line.split()
 if len(parts)!=2:raise SystemExit('invalid mount probe; no recovery state changed')
 path,flags=parts
 if path in expected:
  modes=set(flags.split(','))&{'ro','rw'}
  if len(modes)!=1 or path in actual:raise SystemExit('ambiguous mount probe; no recovery state changed')
  actual[path]=modes.pop()
if actual!=expected:raise SystemExit('actual recovery mount permissions differ; no recovery state changed')
(root/(mode+'-mounts.json')).write_text(json.dumps(actual,sort_keys=True)+'\n')
PYMOUNTS
}
probe_mounts operator "${OPERATOR[@]}"
if [ -n "$uploads" ]; then probe_mounts uploads "${UPLOADS[@]}"; fi
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
 "${OPERATOR[@]}" run --rm --no-deps --pull never -T backend /app/time-contract -conf /data/conf -state-dir /app/data/time-contract "$@"
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
 "${UPLOADS[@]}" run --rm --no-deps --pull never -T backend tar xzf - -C /app/data < "$uploads"
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
