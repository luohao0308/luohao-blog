#!/usr/bin/env python3
"""Run revision/CAS cases only on an owned loopback MySQL fixture, never production."""
import pathlib,subprocess,sys,os,time,uuid,json,re,socket
root=pathlib.Path(__file__).resolve().parents[1];out=root/'output/article-revision-cases';out.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(root/'scripts'));from test_mysql_migrations import GO_HELPER

# A loopback MySQL DSN alone does not constrain Docker's selected daemon.
# Refuse remote overrides before creating any helper, database or container.
def local_docker_command():
 host=os.environ.get('DOCKER_HOST','')
 if host and not host.startswith(('unix://','npipe://')):
  raise RuntimeError('refusing a non-local Docker host for the disposable fixture')
 context=subprocess.check_output(['docker','context','show'],text=True).strip()
 endpoint=json.loads(subprocess.check_output(['docker','context','inspect',context,'--format','{{json .Endpoints.docker.Host}}'],text=True))
 if not isinstance(endpoint,str) or not endpoint.startswith(('unix://','npipe://')):
  raise RuntimeError('refusing a non-local Docker context for the disposable fixture')
 return ['docker','--context',context]

docker=local_docker_command()
name='blog-case-mysql-'+uuid.uuid4().hex[:10];db='case_articles_test_'+uuid.uuid4().hex[:10];port=13327
helper=root/'backend/cmd/.case-migration-check';assert not helper.exists();helper.mkdir();(helper/'main.go').write_text(GO_HELPER)
env={k:v for k,v in os.environ.items() if not k.startswith('KRATOS') and k not in ['GOROOT','BLOG_REVISION_MYSQL_DSN','MYSQL_MIGRATION_DSN']}
dsn=f'root@tcp(127.0.0.1:{port})/{db}?parseTime=true&multiStatements=true';env.update({'MYSQL_MIGRATION_DSN':dsn,'BLOG_REVISION_MYSQL_DSN':dsn})
owned=False

def run(a,**kw):
 p=subprocess.run(a,capture_output=True,**kw)
 if p.returncode:
  (out/'fixture-command-error.log').write_bytes(p.stdout+p.stderr)
  raise RuntimeError('isolated case fixture command failed: '+a[0])
 return p.stdout
try:
 run(docker+['run','-d','--name',name,'--label','codex.task=blog-case-studies','-p',f'127.0.0.1:{port}:3306','-e','MYSQL_ALLOW_EMPTY_PASSWORD=yes','-e','MYSQL_DATABASE='+db,'mysql:8.4']);owned=True
 ready=False
 for _ in range(60):
  p=subprocess.run(docker+['exec',name,'mysqladmin','ping','-uroot'],capture_output=True)
  if p.returncode==0:
   try:
    with socket.create_connection(('127.0.0.1',port),timeout=1) as sock:
     if len(sock.recv(4))==4:ready=True;break
   except OSError:pass
  time.sleep(1)
 assert ready
 info=json.loads(run(docker+['inspect',name]))[0]
 assert info['Config']['Labels'].get('codex.task')=='blog-case-studies'
 assert info['NetworkSettings']['Ports']['3306/tcp']==[{'HostIp':'127.0.0.1','HostPort':str(port)}]
 run(['go','run','./cmd/.case-migration-check','16'],cwd=root/'backend',env=env)
 seed="INSERT INTO articles (id,created_at,updated_at,slug,title,content_md,content_html,status) VALUES ('00000000-0000-0000-0000-000000000123',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'legacy-article','legacy','legacy-before-revisions','<p>legacy-before-revisions</p>',1);"
 run(docker+['exec','-i',name,'mysql','-uroot',db],input=seed.encode())
 pattern='TestArticleCASMySQL|TestArticleHistoryMySQL|TestArticleRestoreMySQL|TestCategoryCreateDeleteMySQL|TestRestoreCategoryDeleteMySQL'
 p=subprocess.run(['go','test','./internal/data','-run',pattern,'-count=1','-timeout=120s','-v'],cwd=root/'backend',env=env,capture_output=True)
 (out/'cas-real-mysql.log').write_bytes(p.stdout+p.stderr)
 assert p.returncode==0 and b'--- SKIP:' not in p.stdout
 for test in pattern.split('|'):assert re.search(rb'^--- PASS: '+test.encode()+rb'(?: \(|$)',p.stdout,re.M),test
 version=run(docker+['exec',name,'mysql','-uroot','-N',db,'-e','SELECT version FROM schema_migrations;']).decode().strip()
 expected=max(int(p.name.split('_')[0]) for p in (root/'backend/migrations').glob('*.up.sql'))
 assert int(version)==expected
 print(json.dumps({'passed':True,'mysql':'8.4','schemaBefore':16,'schemaAfter':int(version),'tests':pattern.split('|'),'silentSkips':0,'productionAccess':False}))
finally:
 # docker run may create a stopped container before a port/start failure.
 probe=subprocess.run(docker+['inspect','--format','{{index .Config.Labels "codex.task"}}',name],capture_output=True)
 if probe.returncode==0:
  assert probe.stdout.decode().strip()=='blog-case-studies'
  run(docker+['rm','-f','-v',name])
 elif owned:
  raise RuntimeError('owned fixture disappeared; no unrelated cleanup attempted')
 (helper/'main.go').unlink();helper.rmdir()
