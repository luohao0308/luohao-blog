#!/usr/bin/env python3
"""Versioned migration and backup/restore checks on this task's isolated MySQL only."""
import argparse
import gzip
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import uuid

GO_HELPER = r"""package main
import (
 "database/sql"
 "fmt"
 "os"
 "strconv"
 "github.com/luohao0308/luohao-blog/backend/migrations"
 "github.com/golang-migrate/migrate/v4"
 "github.com/golang-migrate/migrate/v4/source/iofs"
 mm "github.com/golang-migrate/migrate/v4/database/mysql"
 _ "github.com/go-sql-driver/mysql"
)
func main() {
 target,e:=strconv.Atoi(os.Args[1]);if e!=nil || target<1 {os.Exit(1)}
 db,e:=sql.Open("mysql",os.Getenv("MYSQL_MIGRATION_DSN"));if e!=nil {os.Exit(2)}
 src,e:=iofs.New(migrations.FS,".");if e!=nil {os.Exit(3)}
 driver,e:=mm.WithInstance(db,&mm.Config{});if e!=nil {os.Exit(4)}
 m,e:=migrate.NewWithInstance("iofs",src,"mysql",driver);if e!=nil {os.Exit(5)}
 defer m.Close()
 if e=m.Migrate(uint(target));e!=nil && e!=migrate.ErrNoChange {fmt.Fprintln(os.Stderr,"fixture migration failed");os.Exit(6)}
 version,dirty,e:=m.Version();if e!=nil || dirty {os.Exit(7)}
 fmt.Printf("%d %t\n",version,dirty)
}
"""

ROOT=Path(__file__).resolve().parents[1]
CONTAINER="website-completion-mysql"

def run(args, *, data=None, env=None, cwd=None):
    r=subprocess.run(args,input=data,capture_output=True,env=env,cwd=cwd)
    if r.returncode: raise RuntimeError("fixture command failed: "+args[0]+" (output withheld)")
    return r.stdout

def mysql(database, sql):
    assert re.fullmatch(r"website_completion_[a-f0-9_]+",database)
    return run(["docker","exec","-i",CONTAINER,"sh","-c",'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot -N --default-character-set=utf8mb4 '+database],data=sql.encode()).decode()

def administrative(sql):
    return run(["docker","exec","-i",CONTAINER,"sh","-c",'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot -N'],data=sql.encode())

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--baseline",type=int,default=11)
    parser.add_argument("--target",type=int,required=True)
    opts=parser.parse_args()
    assert 1<=opts.baseline<opts.target
    assert len(list((ROOT/"backend/migrations").glob(f"{opts.target:06d}_*.up.sql")))==1, "target migration is not ready"
    info=json.loads(run(["docker","inspect",CONTAINER]))[0]
    assert info["Config"]["Labels"].get("codex.task")=="website-completion"
    assert {m.get("Name") for m in info["Mounts"]}=={"website-completion-mysql"}
    assert info["NetworkSettings"]["Ports"]["3306/tcp"]==[{"HostIp":"127.0.0.1","HostPort":"13316"}]
    assert info["State"]["Health"]["Status"]=="healthy"
    envfile=ROOT/".dev-workflow/website-completion-mysql.env"
    password=next(line.split("=",1)[1] for line in envfile.read_text().splitlines() if line.startswith("MYSQL_ROOT_PASSWORD="))
    assert re.fullmatch(r"[a-f0-9]{48}",password)
    identity=uuid.uuid4().hex[:12]
    db="website_completion_"+identity
    restore="website_completion_"+identity+"_b"
    created=[]
    source=ROOT/"backend/cmd/.website-migration-check"
    assert not source.exists(),"another fixture build owns this directory"
    source.mkdir()
    try:
        # Generated diagnostic source imports the actual embedded migration FS.
        (source/"main.go").write_text(GO_HELPER)
        with tempfile.TemporaryDirectory(prefix="site-migration-check-") as tmp:
            binary=str(Path(tmp)/"migrate")
            run(["go","build","-o",binary,str(source)],cwd=ROOT/"backend")
            for name in (db,restore):
                assert re.fullmatch(r"website_completion_[a-f0-9_]+",name)
                administrative(f"CREATE DATABASE `{name}` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;")
                created.append(name)
            env=os.environ|{"MYSQL_MIGRATION_DSN":f"root:{password}@tcp(127.0.0.1:13316)/{db}?multiStatements=true&parseTime=true"}
            assert run([binary,str(opts.baseline)],env=env).decode().strip()==f"{opts.baseline} false"
            mysql(db,"INSERT INTO articles(id,created_at,updated_at,slug,title,summary,content_md,content_html,status,view_count,like_count) VALUES('00000000-0000-0000-0000-000000000001',NOW(),NOW(),'migration-fixture','旧文章','保留摘要','# 旧文章','<h1>旧文章</h1>',2,7,3);")
            original=mysql(db,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")
            dump=run(["docker","exec",CONTAINER,"sh","-c",'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --skip-comments --no-tablespaces --set-gtid-purged=OFF '+db])
            compressed=gzip.compress(dump)
            assert gzip.decompress(compressed)==dump
            assert run([binary,str(opts.target)],env=env).decode().strip()==f"{opts.target} false"
            assert mysql(db,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")==original
            assert mysql(db,"SELECT COALESCE(cover_url,''),featured FROM articles WHERE slug='migration-fixture';").rstrip("\r\n")=="\t0", "new article metadata must default to empty/false"
            mysql(db,"UPDATE articles SET cover_url='/v1/assets/media/00000000000000000000000000000001.jpg',featured=1 WHERE slug='migration-fixture';")
            assert mysql(db,"SELECT featured FROM articles WHERE slug='migration-fixture';").strip()=="1"
            # A genuine pre-upgrade SQL dump restores to an independent schema.
            mysql(restore,gzip.decompress(compressed).decode())
            assert mysql(restore,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")==original
            assert mysql(restore,"SELECT version,dirty FROM schema_migrations;").strip()==f"{opts.baseline}\t0"
            restore_env=env|{"MYSQL_MIGRATION_DSN":f"root:{password}@tcp(127.0.0.1:13316)/{restore}?multiStatements=true&parseTime=true"}
            assert run([binary,str(opts.target)],env=restore_env).decode().strip()==f"{opts.target} false"
            print(json.dumps({"baseline":opts.baseline,"target":opts.target,"dirty":False,"old_rows_preserved":True,"metadata_defaults":True,"backup_restore_and_reupgrade":True,"isolated_fixture":True}))
    finally:
        for name in created:
            assert re.fullmatch(r"website_completion_[a-f0-9_]+",name)
            administrative(f"DROP DATABASE `{name}`;")
        shutil.rmtree(source)

if __name__=="__main__":
    try: main()
    except Exception as e:
        # Never include MySQL DSNs, provider credentials, or command output.
        print("MySQL fixture verification failed: "+type(e).__name__)
        raise SystemExit(1)
