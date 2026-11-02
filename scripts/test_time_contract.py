#!/usr/bin/env python3
"""Owned loopback MySQL baseline only; never loads application or production config."""
import json,os,pathlib,socket,subprocess,time,uuid
ROOT=pathlib.Path(__file__).resolve().parents[1]
LABEL='utc-time-contract-baseline'
def local_docker_command():
 host=os.environ.get('DOCKER_HOST','')
 if host and not host.startswith(('unix://','npipe://')): raise RuntimeError('refusing a non-local Docker host')
 context=subprocess.check_output(['docker','context','show'],text=True).strip()
 endpoint=json.loads(subprocess.check_output(['docker','context','inspect',context,'--format','{{json .Endpoints.docker.Host}}'],text=True))
 if not isinstance(endpoint,str) or not endpoint.startswith(('unix://','npipe://')):raise RuntimeError('refusing a non-local Docker context')
 return ['docker','--context',context]
def main():
 docker=local_docker_command() # Gate before even creating output or fixtures.
 out=ROOT/'output/time-contract/baseline';out.mkdir(parents=True,exist_ok=True)
 name='time-baseline-'+uuid.uuid4().hex[:10];db='time_contract_test_'+uuid.uuid4().hex[:10];restore=db+'_restore';normalized=db+'_normalized'
 env={k:v for k,v in os.environ.items() if not k.startswith('KRATOS') and 'DSN' not in k and k!='GOROOT'}
 env['BLOG_TIME_MYSQL_DSN']=f'root@tcp(127.0.0.1:13329)/{db}?parseTime=true'
 def run(a,data=None):
  p=subprocess.run(a,input=data,capture_output=True,env=env)
  if p.returncode:
   f=out/'failure.private.log';f.write_bytes(p.stdout+p.stderr);f.chmod(0o600)
   raise RuntimeError('owned baseline command failed; diagnostics withheld')
  return p.stdout
 def sql(q,target=db):return run(docker+['exec','-i',name,'mysql','-uroot','-N',target],q.encode()).decode().strip()
 try:
  run(docker+['run','-d','--name',name,'--label','codex.task='+LABEL,'-p','127.0.0.1:13329:3306','-e','MYSQL_ALLOW_EMPTY_PASSWORD=yes','-e','MYSQL_DATABASE='+db,'mysql:8.4'])
  ready=False
  for _ in range(90):
   p=subprocess.run(docker+['exec',name,'mysqladmin','ping','-uroot'],capture_output=True)
   if p.returncode==0:
    try:
     with socket.create_connection(('127.0.0.1',13329),timeout=1) as s:ready=len(s.recv(4))==4
    except OSError:pass
   if ready:break
   time.sleep(1)
  assert ready
  info=json.loads(run(docker+['inspect',name]))[0]
  assert info['Config']['Labels'].get('codex.task')==LABEL
  assert info['NetworkSettings']['Ports']['3306/tcp']==[{'HostIp':'127.0.0.1','HostPort':'13329'}]
  p=subprocess.run(['go','test','./internal/data','-run','^TestTimeBaselineRoundtrip$','-count=1','-timeout=90s','-v'],cwd=ROOT/'backend',env=env,capture_output=True)
  (out/'go-tests.log').write_bytes(p.stdout+p.stderr)
  assert p.returncode==0 and b'--- SKIP:' not in p.stdout and b'--- PASS: TestTimeBaselineRoundtrip' in p.stdout
  # Independent dump baseline: UTC dump preserves internal epoch, not legacy API labeling.
  sql("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL); INSERT INTO schema_migrations VALUES(20,0); CREATE TABLE legacy_probe(id INT PRIMARY KEY,stamp TIMESTAMP NULL); SET SESSION time_zone='+08:00'; INSERT INTO legacy_probe VALUES(1,'2026-11-02 14:50:32');")
  old=sql('SELECT UNIX_TIMESTAMP(stamp) FROM legacy_probe;');assert old=='1793602232'
  dump=run(docker+['exec',name,'mysqldump','-uroot','--single-transaction','--no-tablespaces','--set-gtid-purged=OFF',db]);assert b"TIME_ZONE='+00:00'" in dump
  sql('CREATE DATABASE `'+restore+'`; CREATE DATABASE `'+normalized+'`;')
  sql(dump.decode(),restore);assert sql('SELECT UNIX_TIMESTAMP(stamp) FROM legacy_probe;',restore)==old
  # A legacy dump has no new marker table: importing it cannot remove a preexisting marker.
  sql('CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL); INSERT INTO schema_migrations VALUES(21,0); CREATE TABLE fixture_marker(version INT); INSERT INTO fixture_marker VALUES(21);',normalized)
  sql(dump.decode(),normalized);assert sql('SELECT version FROM fixture_marker;',normalized)=='21'
  assert sql('SELECT UNIX_TIMESTAMP(stamp) FROM legacy_probe;',normalized)==old
  assert sql('SELECT version FROM schema_migrations;',normalized)=='20'
  # A normalized dump can import a marker and then fail: marker alone proves nothing.
  failed=subprocess.run(docker+['exec','-i',name,'mysql','-uroot',restore],input=b'UPDATE schema_migrations SET version=21; CREATE TABLE fixture_marker(version INT); INSERT INTO fixture_marker VALUES(21); SELECT * FROM deliberately_missing_table;',capture_output=True)
  assert failed.returncode!=0 and sql('SELECT version FROM fixture_marker;',restore)=='21'
  assert sql('SELECT version FROM schema_migrations;',restore)=='21'
  result={'passed':True,'tests':['TestTimeBaselineRoundtrip'],'silentSkips':0,'legacyEpoch':int(old),'expectedApiEpoch':1793631032,'offsetSeconds':28800,'utcRoundtrip':True,'datetimeMicrosecondsJsonNullPreserved':True,'expiryDueLeaseBoundaries':True,'dumpPreservesLegacyEpoch':True,'legacyRestoreLeavesMarker':True,'failedRestoreCanLeaveMarker':True,'actualS2RuntimeGateVerified':False,'productionAccess':False}
  (out/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
 finally:
  p=subprocess.run(docker+['inspect','--format','{{index .Config.Labels "codex.task"}}',name],capture_output=True)
  if p.returncode==0:
   assert p.stdout.decode().strip()==LABEL
   run(docker+['rm','-f','-v',name])
if __name__=='__main__':main()
