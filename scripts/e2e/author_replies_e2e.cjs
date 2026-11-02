// Synthetic API + production-built frontend. No real accounts, database or upstream checks.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http'),{spawn}=require('node:child_process'),{chromium}=require('playwright')
const {makeArticle}=require('./ui_fixture.cjs'),root=path.resolve(__dirname,'../..'),out=path.join(root,'output/series/replies-browser'),base='http://127.0.0.1:14390'
async function main(){
 fs.mkdirSync(out,{recursive:true})
 const admin={id:'fixture-admin',email:'author@example.invalid',display_name:'博客作者',role:1},question={id:'00000000-0000-7000-8000-000000000001',article_slug:'fixture-article',display_name:'读者',user_id:'fixture-reader',content:'如何避免两个窗口覆盖正文？',status:2,created_at:'2026-11-02T00:00:00Z'}
 let reply=null,conflict=false,puts=0,deletes=0
 const api=http.createServer(async(req,res)=>{const u=new URL(req.url,'http://fixture'),send=(v,s=200)=>{res.writeHead(s,{'Content-Type':'application/json'});res.end(JSON.stringify(v))}
 if(u.pathname.startsWith('/v1/auth/'))return send({access_token:'synthetic-admin-token',token_type:'Bearer',expires_in:900,user:admin})
 if(u.pathname===`/v1/comments/${question.id}/author-reply`){if(req.method==='GET')return reply?send(reply):send({},404);if(req.method==='PUT'){puts++;let body='';for await(const part of req)body+=part;const input=JSON.parse(body);assert.equal(typeof input.expected_reply_version,'string');if(conflict){conflict=false;reply={...reply,content:'另一个窗口保存的回复',reply_version:'2'};return send({code:409,reason:'AUTHOR_REPLY_CONFLICT'},409)}if(input.expected_reply_version!==(reply?.reply_version??'0'))return send({},409);reply={id:'00000000-0000-7000-8000-000000000002',article_slug:question.article_slug,parent_id:question.id,reply_version:String(Number(reply?.reply_version??0)+1),user_id:admin.id,display_name:admin.display_name,status:2,content:input.content,created_at:question.created_at};return send(reply)}}
 if(u.pathname==='/v1/comments/00000000-0000-7000-8000-000000000002'&&req.method==='DELETE'){deletes++;reply=null;return send({})}
 if(u.pathname==='/v1/comments'&&req.method==='GET')return send({comments:[{...question,author_reply:reply}],next_page_token:''})
 if(u.pathname==='/v1/articles/fixture-article/comments')return send({comments:[{...question,author_reply:reply?.status===2?reply:undefined}],next_page_token:''})
 if(u.pathname==='/v1/articles/list')return send({articles:[makeArticle()],next_page_token:''})
 if(u.pathname==='/v1/articles/fixture-article')return send(makeArticle())
 if(u.pathname.endsWith('/view')||u.pathname.endsWith('/like'))return send({view_count:0,like_count:0})
 if(u.pathname.includes('/projects'))return send({projects:[],next_page_token:''})
 return send({})})
 await new Promise((resolve,reject)=>{api.once('error',reject);api.listen(14391,'127.0.0.1',resolve)})
 const log=fs.openSync(path.join(out,'server.log'),'w'),server=spawn(process.execPath,[path.join(root,'frontend/.output/server/index.mjs')],{cwd:path.join(root,'frontend'),detached:true,stdio:['ignore',log,log],env:{...process.env,HOST:'127.0.0.1',PORT:'14390',NUXT_BACKEND_BASE:'http://127.0.0.1:14391',NUXT_PUBLIC_SITE_URL:base}});fs.closeSync(log)
 let exited=false,browser;server.once('exit',()=>exited=true)
 try{let ready=false;for(let i=0;i<100;i++){assert(!exited);try{ready=(await fetch(base+'/health')).ok}catch{}if(ready)break;await new Promise(r=>setTimeout(r,250))}assert(ready)
 browser=await chromium.launch({headless:true,executablePath:process.env.BLOG_E2E_CHROMIUM||undefined})
 for(const width of [1440,390]){reply=null;const c=await browser.newContext({viewport:{width,height:1000},reducedMotion:'reduce'}),page=await c.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto(base+'/admin/comments',{waitUntil:'networkidle'});await page.getByRole('button',{name:'作者回复',exact:true}).click();const editor=page.getByRole('region',{name:'编辑作者回复'}),input=editor.getByRole('textbox',{name:'回复内容'}),save=editor.getByRole('button',{name:'保存回复',exact:true});await input.fill('按版本条件保存，冲突时保留草稿。');await save.click();await editor.getByRole('status').filter({hasText:'作者回复已公开'}).waitFor();assert.equal(reply.reply_version,'1')
 conflict=true;const before=puts;await input.fill('当前窗口的新回复草稿');await save.click();await editor.getByRole('status').filter({hasText:'草稿已保留'}).waitFor();assert.equal(await input.inputValue(),'当前窗口的新回复草稿');assert(await save.isDisabled());assert.equal(puts,before+1)
 await editor.getByRole('button',{name:'加载最新版本',exact:true}).click();await editor.getByText('当前回复：另一个窗口保存的回复',{exact:true}).waitFor();assert.equal(await input.inputValue(),'当前窗口的新回复草稿');await save.click();await editor.getByRole('status').filter({hasText:'作者回复已公开'}).waitFor();assert.equal(reply.reply_version,'3')
 await editor.screenshot({path:path.join(out,`editor-${width}.png`)})
 const publicPage=await c.newPage();await publicPage.goto(base+'/posts/fixture-article',{waitUntil:'networkidle'});const comments=publicPage.getByRole('region',{name:'评论区'});await comments.getByText('当前窗口的新回复草稿',{exact:true}).waitFor();assert.equal(await comments.locator('[aria-label="作者回复"]').count(),1);assert(await publicPage.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await comments.screenshot({path:path.join(out,`public-${width}.png`)});await publicPage.close()
 await editor.getByRole('button',{name:'删除作者回复',exact:true}).click();await page.getByRole('button',{name:/^(确定|确认)$/,exact:true}).click();await editor.getByRole('status').filter({hasText:'原评论保留'}).waitFor();assert.equal(reply,null);assert.equal(await page.getByText(question.content,{exact:true}).count(),2);assert.deepEqual(errors,[]);await c.close()}
 assert.equal(deletes,2);fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({passed:true,viewports:[1440,390],create:true,casConflictDraftRetention:true,explicitReload:true,deleteChildKeepsRoot:true,publicNestedReply:true},null,2));console.log('PASS: desktop/mobile real admin editor create, 409 draft/reload/CAS, public nested reply and precise child deletion')
 }finally{await browser?.close();if(!exited){const closed=new Promise(r=>server.once('exit',r));process.kill(-server.pid,'SIGTERM');await closed}await new Promise(r=>api.close(r))}
}
main().catch(e=>{console.error(e);process.exitCode=1})
