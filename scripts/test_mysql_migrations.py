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
            previous_metadata=None
            if opts.baseline>=12:
                mysql(db,"UPDATE articles SET cover_url='/v1/assets/media/00000000000000000000000000000001.jpg',featured=1 WHERE slug='migration-fixture';")
                previous_metadata=mysql(db,"SELECT cover_url,featured FROM articles WHERE slug='migration-fixture';")
            original=mysql(db,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")
            original_user=None
            previous_account_metadata=None
            if opts.baseline>=10:
                # Public fixture sentinels only; never read real account data.
                mysql(db,"INSERT INTO users(id,created_at,updated_at,email,password_hash,display_name,role,avatar_url,wechat_openid) VALUES('00000000-0000-0000-0000-000000000002',NOW(),NOW(),'migration-fixture@example.com','fixture-placeholder-hash','旧读者',2,'/v1/assets/avatars/00000000000000000000000000000002.png','fixture-legacy-openid');")
                original_user=mysql(db,"SELECT email,password_hash,display_name,role,avatar_url,wechat_openid FROM users ORDER BY id;")
                if opts.baseline>=14:
                    mysql(db,"UPDATE users SET email_verified=1,credential_version=7 WHERE email='migration-fixture@example.com';")
                    previous_account_metadata=mysql(db,"SELECT email_verified,credential_version FROM users ORDER BY id;")
            dump=run(["docker","exec",CONTAINER,"sh","-c",'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --skip-comments --no-tablespaces --set-gtid-purged=OFF '+db])
            compressed=gzip.compress(dump)
            assert gzip.decompress(compressed)==dump
            assert run([binary,str(opts.target)],env=env).decode().strip()==f"{opts.target} false"
            assert mysql(db,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")==original
            if original_user is not None:
                assert mysql(db,"SELECT email,password_hash,display_name,role,avatar_url,wechat_openid FROM users ORDER BY id;")==original_user
            if opts.target>=14 and original_user is not None:
                account_metadata=mysql(db,"SELECT email_verified,credential_version FROM users ORDER BY id;")
                assert account_metadata==(previous_account_metadata or "0\t0\n"), "old accounts must start unverified/version zero; existing metadata must survive"
                assert mysql(db,"SELECT COUNT(*) FROM account_email_tokens;").strip()=="0"
                # Execute the real repository against MySQL, including
                # changed-rows/no-op semantics that SQLite cannot prove.
                probe_env=env|{"BLOG_ACCOUNT_EMAIL_MYSQL_DSN":env["MYSQL_MIGRATION_DSN"]}
                probe=run(["go","test","./internal/data","-run","^TestAccountEmailTokenRepoMySQL$","-count=1","-v"],env=probe_env,cwd=ROOT/"backend").decode()
                assert "=== RUN   TestAccountEmailTokenRepoMySQL" in probe and "--- PASS: TestAccountEmailTokenRepoMySQL" in probe and "--- SKIP:" not in probe, "real MySQL repo probe must run, not silently skip"
                assert mysql(db,"SELECT COUNT(*) FROM account_email_tokens;").strip()=="0"
            if previous_metadata is not None:
                assert mysql(db,"SELECT cover_url,featured FROM articles WHERE slug='migration-fixture';")==previous_metadata
            else:
                assert mysql(db,"SELECT COALESCE(cover_url,''),featured FROM articles WHERE slug='migration-fixture';").rstrip("\r\n")=="\t0", "new article metadata must default to empty/false"
            if opts.target>=13:
                assert mysql(db,"SELECT COUNT(*) FROM projects;").strip()=="3"
                assert mysql(db,"SELECT COUNT(*) FROM abouts WHERE singleton_key='main';").strip()=="1"
                assert mysql(db,"SELECT slug FROM projects ORDER BY sort,slug;").splitlines()==["luohao-blog","dev-workflow","job-application-workbench"]
                mysql(db,"UPDATE projects SET summary='fixture-edited-project' WHERE slug='luohao-blog'; UPDATE abouts SET intro='fixture-edited-about' WHERE singleton_key='main';")
                # Seed SQL must never overwrite a user's later edits.
                seed=(ROOT/"backend/migrations/000013_projects_about.up.sql").read_text()
                mysql(db,seed)
                assert mysql(db,"SELECT summary FROM projects WHERE slug='luohao-blog';").strip()=="fixture-edited-project"
                assert mysql(db,"SELECT intro FROM abouts WHERE singleton_key='main';").strip()=="fixture-edited-about"
                assert mysql(db,"SELECT COUNT(*) FROM projects;").strip()=="3"
                assert mysql(db,"SELECT COUNT(*) FROM abouts;").strip()=="1"
            mysql(db,"UPDATE articles SET cover_url='/v1/assets/media/00000000000000000000000000000001.jpg',featured=1 WHERE slug='migration-fixture';")
            assert mysql(db,"SELECT featured FROM articles WHERE slug='migration-fixture';").strip()=="1"
            # A genuine pre-upgrade SQL dump restores to an independent schema.
            mysql(restore,gzip.decompress(compressed).decode())
            assert mysql(restore,"SELECT slug,title,summary,status,view_count,like_count FROM articles ORDER BY slug;")==original
            assert mysql(restore,"SELECT version,dirty FROM schema_migrations;").strip()==f"{opts.baseline}\t0"
            restore_env=env|{"MYSQL_MIGRATION_DSN":f"root:{password}@tcp(127.0.0.1:13316)/{restore}?multiStatements=true&parseTime=true"}
            assert run([binary,str(opts.target)],env=restore_env).decode().strip()==f"{opts.target} false"
            if previous_metadata is not None:
                assert mysql(restore,"SELECT cover_url,featured FROM articles WHERE slug='migration-fixture';")==previous_metadata
            if original_user is not None:
                assert mysql(restore,"SELECT email,password_hash,display_name,role,avatar_url,wechat_openid FROM users ORDER BY id;")==original_user
            if opts.target>=14 and original_user is not None:
                assert mysql(restore,"SELECT email_verified,credential_version FROM users ORDER BY id;")==(previous_account_metadata or "0\t0\n")
                assert mysql(restore,"SELECT COUNT(*) FROM account_email_tokens;").strip()=="0"
                mysql(db,"UPDATE users SET email_verified=1,credential_version=9 WHERE email='migration-fixture@example.com';")
                token_values="'00000000-0000-0000-0000-000000000003',NOW(),NOW(),'00000000-0000-0000-0000-000000000002','verify','"+"a"*64+"',9,DATE_ADD(NOW(),INTERVAL 24 HOUR)"
                mysql(db,"INSERT INTO account_email_tokens(id,created_at,updated_at,user_id,purpose,token_hash,credential_version,expires_at) VALUES("+token_values+");")
                # Each purpose has one current token; hashes are globally unique.
                duplicate_purpose=token_values.replace("000000000003","000000000004").replace("a"*64,"b"*64)
                duplicate_hash=token_values.replace("000000000003","000000000005").replace("'verify'","'reset'")
                mysql(db,"INSERT IGNORE INTO account_email_tokens(id,created_at,updated_at,user_id,purpose,token_hash,credential_version,expires_at) VALUES("+duplicate_purpose+"),("+duplicate_hash+");")
                assert mysql(db,"SELECT COUNT(*) FROM account_email_tokens;").strip()=="1"
                assert mysql(db,"SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='account_email_tokens';").strip()=="1"
                orphan=token_values.replace("000000000003","000000000006").replace("000000000002","000000000099").replace("a"*64,"c"*64)
                try:
                    mysql(db,"INSERT INTO account_email_tokens(id,created_at,updated_at,user_id,purpose,token_hash,credential_version,expires_at) VALUES("+orphan+");")
                except RuntimeError:
                    pass
                else:
                    raise AssertionError("account token FK must reject missing users")
                assert mysql(db,"SELECT COUNT(*) FROM account_email_tokens;").strip()=="1"
                # Also restore a post-upgrade backup, retaining verified state
                # and credential revocation generation as well as old columns.
                upgraded_dump=run(["docker","exec",CONTAINER,"sh","-c",'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --skip-comments --no-tablespaces --set-gtid-purged=OFF '+db])
                mysql(restore,gzip.decompress(gzip.compress(upgraded_dump)).decode())
                assert mysql(restore,"SELECT email_verified,credential_version FROM users ORDER BY id;").strip()=="1\t9"
                assert mysql(restore,"SELECT email,password_hash,display_name,role,avatar_url,wechat_openid FROM users ORDER BY id;")==original_user
                assert mysql(restore,"SELECT version,dirty FROM schema_migrations;").strip()==f"{opts.target}\t0"
                assert mysql(restore,"SELECT purpose,credential_version,LENGTH(token_hash) FROM account_email_tokens;").strip()=="verify\t9\t64"
                assert mysql(restore,"SELECT token_hash FROM account_email_tokens;").strip()=="a"*64
            print(json.dumps({"baseline":opts.baseline,"target":opts.target,"dirty":False,"old_rows_preserved":True,"metadata_compatibility":True,"portfolio_seeds_idempotent":opts.target>=13,"account_metadata_compatibility":opts.target>=14,"token_constraints_and_restore":opts.target>=14,"backup_restore_and_reupgrade":True,"isolated_fixture":True}))
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
