"""Actual operator/MySQL acceptance; Docker service lifecycle alone is a test shim."""
import gzip
import difflib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import uuid

from test_mysql_migrations import GO_HELPER

ROOT = Path(__file__).resolve().parents[1]
DUMP_FLAGS = ['--single-transaction', '--tz-utc', '--order-by-primary', '--skip-extended-insert',
              '--complete-insert', '--hex-blob', '--default-character-set=utf8mb4',
              '--skip-comments', '--no-tablespaces', '--set-gtid-purged=OFF']
SHIM = r'''#!/usr/bin/env python3
import hashlib,json,os,pathlib,subprocess,sys
args=sys.argv[1:];cfg=json.loads(pathlib.Path(os.environ['UTC_RESTORE_SHIM_CONFIG']).read_text());root=pathlib.Path(cfg['root'])
with (root/'calls').open('a') as log:log.write(json.dumps(args)+'\n')
if 'run' in args and '--help' in args:
 print('      --pull string policy');raise SystemExit(0)
if args[:2]==['image','inspect']:
 print(cfg['image_id']);raise SystemExit(0)
if 'config' in args and '-q' in args:raise SystemExit(0)
if args[0]=='inspect':
 if 'running' in args[2]:
  running=(root/'running').exists();print(json.dumps({'id':cfg['backend_id'],'running':running,'status':'running' if running else 'exited'}))
 else:
  print(json.dumps({'id':cfg['backend_id'],'image':cfg['image_id'],'files':'/owned/base.yml,/owned/release.yml','project':'owned-utc-test','directory':'/owned/deploy','mounts':[{'Destination':'/app/data/time-contract','Type':'bind','Source':cfg['state_dir']}]}))
 sys.exit(0)
if '/app/time-contract' in args:
 op=args[args.index('/app/time-contract')+1:]
 operation_mount=next((v.split(':')[0] for v in args if ':/time-operation:' in v),None)
 translated=[]
 for arg in op:
  if arg=='/data/conf':arg=cfg['conf_dir']
  elif arg=='/app/data/time-contract':arg=cfg['state_dir']
  elif arg.startswith('/time-operation/'):
   if not operation_mount:raise SystemExit(91)
   arg=operation_mount+arg[len('/time-operation'):]
  translated.append(arg)
 raise SystemExit(subprocess.run([cfg['operator'],*translated],env={k:v for k,v in os.environ.items() if not k.startswith('KRATOS')}).returncode)
if 'stop' in args:
 (root/'running').unlink(missing_ok=True);raise SystemExit(0)
if 'start' in args:
 result=subprocess.run([cfg['operator'],'-mode','status','-conf',cfg['conf_dir'],'-state-dir',cfg['state_dir']],capture_output=True)
 if result.returncode or json.loads(result.stdout).get('ready') is not True:raise SystemExit(92)
 (root/'running').touch();raise SystemExit(0)
if 'ps' in args:print('blog-backend healthy');raise SystemExit(0)
if 'exec' in args and 'mysql' in args:
 command=args[-1]
 if not command.endswith(' blog'):raise SystemExit(93)
 command=command[:-5]+' '+cfg['database']
 real=[cfg['docker'],'--context',cfg['context'],'exec','-i',cfg['mysql'],'sh','-c',command]
 if 'exec mysql ' in command:
  data=sys.stdin.buffer.read()
  if cfg['mode']=='fail-after-marker':
   lines=data.splitlines(keepends=True);cut=next(i for i,line in enumerate(lines) if line.startswith(b'INSERT INTO `time_contract` '));data=b''.join(lines[:cut+1])+b'\nSELECT * FROM deliberately_missing_table;\n'
  raise SystemExit(subprocess.run(real,input=data).returncode)
 result=subprocess.run(real,capture_output=True)
 (root/'observed-redump.sql').write_bytes(result.stdout)
 sys.stdout.buffer.write(result.stdout);sys.stderr.buffer.write(result.stderr)
 raise SystemExit(result.returncode)
raise SystemExit(94)
'''


def run_acceptance(docker, container, base_name, env, output):
    """Caller already proved local context, owned label and loopback port13330."""
    import shutil
    real_docker = shutil.which('docker')
    spec = importlib.util.spec_from_file_location('owned_dump_manifest', ROOT / 'deploy/time_backup_manifest.py')
    manifest_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(manifest_module)
    helper = ROOT / 'backend/cmd/.time-restore-check'
    assert not helper.exists()

    def checked(args, data=None, *, cwd=None, environ=None):
        result = subprocess.run(args, input=data, capture_output=True, cwd=cwd, env=environ or env)
        if result.returncode:
            private = output / 'restore-failure.private.log'
            private.write_bytes(result.stdout + result.stderr)
            private.chmod(0o600)
            raise RuntimeError('owned real restore acceptance failed; private diagnostics withheld')
        return result.stdout

    def sql(database, statement):
        return checked(docker + ['exec', '-i', container, 'mysql', '-uroot', '-N', database], statement.encode()).decode().strip()

    databases = []
    try:
        helper.mkdir()
        (helper / 'main.go').write_text(GO_HELPER)
        with tempfile.TemporaryDirectory(prefix='utc-real-restore-') as temporary:
            tmp = Path(temporary)
            operator, migrate = tmp / 'time-contract', tmp / 'migrate'
            checked(['go', 'build', '-o', str(operator), './cmd/time-contract'], cwd=ROOT / 'backend')
            checked(['go', 'build', '-o', str(migrate), './cmd/.time-restore-check'], cwd=ROOT / 'backend')
            legacy = base_name + '_d'
            sql(base_name, 'CREATE DATABASE `' + legacy + '`;')
            databases.append(legacy)
            raw_dsn = f'root@tcp(127.0.0.1:13330)/{legacy}?parseTime=true&multiStatements=true&loc=UTC'
            fixture_env = env | {'MYSQL_MIGRATION_DSN': raw_dsn}
            checked([str(migrate), '20'], environ=fixture_env)
            fixed = "'2026-11-02 14:50:32'"
            # Fully synthetic protected relationships and time-bearing entities.
            ids = {name: '00000000-0000-7000-8000-' + str(i).zfill(12) for i, name in enumerate(['user','category','tag','article','root','child','identity','checkroot','checkchild','token','collection','revision','subscriber','task'], 801)}
            user, article = ids['user'], ids['article']
            sql(legacy, f"INSERT INTO users(id,created_at,updated_at,email,password_hash,display_name,role,email_verified,credential_version) VALUES('{user}',{fixed},{fixed},'utc-fixture@example.invalid','fixture-hash','Fixture Admin',1,1,7);")
            sql(legacy, f"INSERT INTO categories(id,created_at,updated_at,slug,name) VALUES('{ids['category']}',{fixed},{fixed},'utc-fixture','Fixture'); INSERT INTO tags(id,created_at,updated_at,name) VALUES('{ids['tag']}',{fixed},{fixed},'UTC fixture');")
            sql(legacy, f"INSERT INTO articles(id,created_at,updated_at,slug,title,summary,content_md,content_html,status,published_at,category_id,version) VALUES('{article}',{fixed},{fixed},'utc-fixture-article','Fixture','summary','body','<p>body</p>',2,{fixed},'{ids['category']}',3); INSERT INTO article_tags(article_id,tag_id) VALUES('{article}','{ids['tag']}');")
            sql(legacy, f"INSERT INTO comments(id,created_at,updated_at,article_slug,display_name,content,status,user_id,parent_id,reply_version) VALUES('{ids['root']}',{fixed},{fixed},'utc-fixture-article','Fixture Admin','question',2,'{user}',NULL,0),('{ids['child']}',{fixed},{fixed},'utc-fixture-article','Fixture Admin','answer',2,'{user}','{ids['root']}',1);")
            app = 'wx1111111111111111'
            sql(legacy, f"INSERT INTO wechat_identities(id,user_id,app_id,openid,created_at) VALUES('{ids['identity']}','{user}','{app}','synthetic-openid',{fixed});")
            for item, body in [('root', 'question'), ('child', 'answer')]:
                values = ['mini-comment-v1', ids[item], user, app, ids['identity'], 'utc-fixture-article', 'Fixture Admin', body]
                if item == 'child':
                    values = ['mini-comment-reply-v2', ids['root'], '1', *values[1:]]
                fingerprint = hashlib.sha256(json.dumps(values, separators=(',', ':')).encode()).hexdigest()
                sql(legacy, f"INSERT INTO wechat_comment_checks(id,comment_id,author_id,app_id,identity_id,input_hash,state,generation,name_trace_id,content_trace_id,checked_at) VALUES('{ids['check'+item]}','{ids[item]}','{user}','{app}','{ids['identity']}','{fingerprint}','pass',7,'synthetic-name-trace','synthetic-content-trace','2026-11-02 14:50:32.789123');")
            sql(legacy, f"INSERT INTO account_email_tokens(id,created_at,updated_at,user_id,purpose,token_hash,credential_version,expires_at) VALUES('{ids['token']}',{fixed},{fixed},'{user}','verify','{'a'*64}',7,'2026-11-02 15:50:32'); INSERT INTO collections(id,saved_at,user_id,article_id,article_slug) VALUES('{ids['collection']}',{fixed},'{user}','{article}','utc-fixture-article');")
            snapshot = json.dumps({'published_at':'2026-11-02T14:50:32Z','content_md':'body'}, separators=(',', ':'))
            sql(legacy, f"INSERT INTO article_revisions(id,article_id,version,action,actor_id,created_at,snapshot) VALUES('{ids['revision']}','{article}',1,'edit','{user}',{fixed},'{snapshot}'); INSERT INTO subscribers(id,created_at,updated_at,email) VALUES('{ids['subscriber']}',{fixed},{fixed},'subscriber@example.invalid');")
            sql(legacy, f"INSERT INTO wechat_sync_tasks(id,created_at,updated_at,slug,content_hash,mode,state,article_title,article_summary,article_content_html,article_source_url,article_author,article_thumb_media_id,attempts) VALUES('{ids['task']}',{fixed},{fixed},'utc-fixture-article','{'b'*64}','draft','drafted','Fixture','summary','<p>body</p>','https://example.invalid/post','Fixture','synthetic-thumb',1);")
            conf, state = tmp / 'conf', tmp / 'control'
            conf.mkdir()
            (conf / 'fixture.yaml').write_text('data:\n  database:\n    source: ' + json.dumps(raw_dsn) + '\n')

            def cli(*args, config=conf, control=state):
                return checked([str(operator), '-conf', str(config), '-state-dir', str(control), *args])

            def backup(database, filename):
                raw = checked(docker + ['exec', container, 'mysqldump', '-uroot', *DUMP_FLAGS, database])
                path = tmp / filename
                path.write_bytes(gzip.compress(raw, mtime=0))
                sidecar = Path(str(path) + '.manifest.json')
                sidecar.write_text(json.dumps(manifest_module.inspect_dump(path), sort_keys=True, indent=2) + '\n')
                ddl = {}
                current = None
                for line in raw.splitlines(keepends=True):
                    if line.startswith(b'CREATE TABLE `'):
                        current = line.split(b'`')[1].decode(); ddl[current] = []
                    if current is not None:
                        ddl[current].append(line.decode())
                        if line.startswith(b') ENGINE=') and line.rstrip().endswith(b';'):
                            current = None
                (tmp/(filename+'.ddl.json')).write_text(json.dumps(ddl))
                return path, sidecar

            old_backup, old_sidecar = backup(legacy, 'legacy.sql.gz')
            manifest_path, receipt_path = tmp / 'convert.json', tmp / 'backup-receipt.json'
            plan = cli('-mode', 'plan', '-operation-id', 'real-cli-convert')
            manifest_path.write_bytes(plan)
            plan_hash = hashlib.sha256(plan).hexdigest()
            common = ['-manifest', str(manifest_path), '-manifest-sha256', plan_hash,
                      '-backup-file', str(old_backup), '-backup-manifest', str(old_sidecar), '-writers-paused']
            receipt_path.write_bytes(cli('-mode', 'backup-receipt', *common))
            cli('-mode', 'apply', '-backup-receipt', str(receipt_path), *common)
            assert json.loads(cli('-mode', 'status'))['ready'] is True
            repeated = json.loads(cli('-mode', 'apply', '-backup-receipt', str(receipt_path), *common))
            assert repeated['outcome'] == 'already_complete', 'CLI replay did not remain read-only'
            assert sql(legacy, "SELECT UNIX_TIMESTAMP(created_at) FROM articles WHERE slug='utc-fixture-article';") == '1793631032'
            normalized_backup, _ = backup(legacy, 'normalized.sql.gz')

            def exercise(label, database, configuration, control, source_backup, failure=False):
                case = tmp / label
                case.mkdir()
                shim = case / 'docker'
                shim.write_text(SHIM)
                shim.chmod(0o755)
                (case / 'running').touch()
                config_path = case / 'shim.json'
                config_path.write_text(json.dumps({'root':str(case),'backend_id':'owned-backend-'+label,
                    'image_id':'sha256:'+hashlib.sha256(operator.read_bytes()).hexdigest(),
                    'state_dir':str(control),'conf_dir':str(configuration),'operator':str(operator),
                    'docker':real_docker,'context':docker[2],'mysql':container,'database':database,
                    'mode':'fail-after-marker' if failure else 'success'}))
                shell_env = env | {'PATH':str(case)+os.pathsep+env['PATH'],'UTC_RESTORE_SHIM_CONFIG':str(config_path),
                    'BLOG_RESTORE_WRITERS_PAUSED':'1','BLOG_RESTORE_EVIDENCE_DIR':str(case/'evidence')}
                result = subprocess.run(['bash', str(ROOT/'deploy/restore.sh'), str(source_backup)], env=shell_env, capture_output=True, timeout=90)
                (output/(label+'.log')).write_bytes(result.stdout+result.stderr)
                if (case/'calls').exists():
                    (output/(label+'-calls.jsonl')).write_bytes((case/'calls').read_bytes())
                if (case/'observed-redump.sql').exists():
                    observed = case/'observed.sql.gz'
                    observed.write_bytes(gzip.compress((case/'observed-redump.sql').read_bytes(),mtime=0))
                    try:
                        actual = manifest_module.inspect_dump(observed)
                        expected = json.loads(Path(str(source_backup)+'.manifest.json').read_text())
                        differences = [{'expected':a,'actual':b} for a,b in zip(expected['tables'],actual['tables']) if a!=b]
                        (output/(label+'-comparison.json')).write_text(json.dumps({'tableDifferences':differences,'expectedCount':len(expected['tables']),'actualCount':len(actual['tables'])},indent=2))
                        original_ddl = json.loads((tmp/(source_backup.name+'.ddl.json')).read_text())
                        observed_ddl = {}; name = None
                        for line in (case/'observed-redump.sql').read_text().splitlines(keepends=True):
                            if line.startswith('CREATE TABLE `'):
                                name=line.split('`')[1]; observed_ddl[name]=[]
                            if name is not None:
                                observed_ddl[name].append(line)
                                if line.startswith(') ENGINE=') and line.rstrip().endswith(';'): name=None
                        ddl_diff = ''.join(''.join(difflib.unified_diff(original_ddl[item['expected']['name']],observed_ddl[item['actual']['name']],fromfile='before/'+item['expected']['name'],tofile='after/'+item['actual']['name'])) for item in differences if item['expected']['ddl_sha256']!=item['actual']['ddl_sha256'])
                        (output/(label+'-ddl.diff')).write_text(ddl_diff)
                    except ValueError as error:
                        (output/(label+'-comparison.json')).write_text(json.dumps({'formatError':str(error)}))
                return result, case

            sql(legacy, f"INSERT INTO articles(id,created_at,updated_at,slug,title,status) VALUES('00000000-0000-7000-8000-000000009999',{fixed},{fixed},'restore-extra','extra',1);")
            result, case = exercise('normalized-success', legacy, conf, state, normalized_backup)
            assert result.returncode == 0, 'actual normalized restore failed'
            assert (case/'running').exists() and json.loads(cli('-mode','status'))['ready'] is True
            assert sql(legacy, "SELECT COUNT(*) FROM articles WHERE slug='restore-extra';") == '0'
            sql(legacy, "UPDATE users SET display_name='changed-before-partial-restore';")
            result, case = exercise('marker-then-failure', legacy, conf, state, normalized_backup, True)
            assert result.returncode != 0 and not (case/'running').exists()
            assert sql(legacy, 'SELECT version FROM schema_migrations;') == '21'
            assert sql(legacy, 'SELECT COUNT(*) FROM time_contract;') == '1'
            assert json.loads(cli('-mode','status'))['ready'] is False

            target = base_name + '_e'
            sql(base_name, 'CREATE DATABASE `' + target + '`;')
            databases.append(target)
            target_conf, target_state = tmp/'target-conf', tmp/'target-control'
            target_conf.mkdir()
            target_dsn = raw_dsn.replace('/'+legacy+'?', '/'+target+'?')
            (target_conf/'fixture.yaml').write_text('data:\n  database:\n    source: '+json.dumps(target_dsn)+'\n')
            cli('-mode','fresh-init','-operation-id','fresh-restore-target','-writers-paused',config=target_conf,control=target_state)
            result, case = exercise('legacy-over-normalized', target, target_conf, target_state, old_backup)
            assert result.returncode != 0 and not (case/'running').exists()
            assert sql(target, 'SELECT version FROM schema_migrations;') == '20'
            assert sql(target, 'SELECT COUNT(*) FROM time_contract;') == '1'
            assert json.loads(cli('-mode','status',config=target_conf,control=target_state))['ready'] is False
            summary = {'passed':True,'actualOperatorCLI':True,'actualMySQL':True,'serviceLifecycle':'test shim',
                       'fullSyntheticRelationships':True,'canonicalPlanAndBackupReceiptApply':True,
                       'normalizedRestoreVerifiedBeforeStart':True,'markerThenImportFailureBlocked':True,
                       'legacyRestoreLeftoverMarkerBlocked':True,'productionAccess':False}
            (output/'restore-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    finally:
        for database in databases:
            sql(base_name, 'DROP DATABASE IF EXISTS `' + database + '`;')
        (helper/'main.go').unlink(missing_ok=True)
        if helper.exists():
            helper.rmdir()
