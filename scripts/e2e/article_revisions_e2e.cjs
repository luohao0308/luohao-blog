// Isolated revision history/restore contract through the built Nuxt app.
const assert=require('node:assert/strict'),http=require('node:http'),path=require('node:path'),fs=require('node:fs/promises')
const {spawn}=require('node:child_process'),{chromium}=require('playwright')
const root=path.resolve(__dirname,'../..'),out=path.join(root,'output/article-revisions')
let article={id:'rev',slug:'rev',version:'7',title:'当前标题',summary:'当前摘要',content_md:'当前正文',content_html:'<p>当前正文</p>',tags:[],status:2,cover_url:'',featured:true,view_count:11,like_count:3}
const make=(version,action,text)=>({article_slug:'rev',version,action,created_at:'2026-11-01T00:00:00Z',snapshot:{...article,version,title:version==='4'?'<script>window.revisionInjected=1</script>':'旧标题'+version,content_md:text,status:1}})
const history=[make('7','update','当前正文'),make('4','update','旧正文<script>window.revisionInjected=1</script>'),make('3','update','超长内容'.repeat(50000)),make('1','baseline','接入前正文')]
let mode='normal',restoreCount=0
const listen=s=>new Promise(r=>s.listen(0,'127.0.0.1',r))
async function main(){
 const api=http.createServer((req,res)=>{
  const u=new URL(req.url,'http://fixture'),send=(code,value)=>{res.writeHead(code,{'Content-Type':'application/json'});res.end(JSON.stringify(value))}
  if(u.pathname.includes('/auth/'))return send(200,{access_token:'fixture',user:{id:'revision-author',role:1,display_name:'作者'}})
  if(u.pathname.endsWith('/admin/media/images'))return setTimeout(()=>send(503,{}),600)
  if(u.pathname.endsWith('/revisions'))return send(200,{revisions:history.map(({snapshot,...meta})=>meta),next_page_token:''})
  const restore=u.pathname.match(/\/revisions\/(\d+)\/restore$/)
  if(restore){let raw='';req.on('data',c=>raw+=c);return req.on('end',()=>{
   restoreCount++;const input=JSON.parse(raw)
   if(mode==='category')return send(409,{reason:'ARTICLE_REVISION_CATEGORY_UNAVAILABLE'})
   if(mode==='media')return send(409,{reason:'ARTICLE_REVISION_MEDIA_UNAVAILABLE'})
   if(mode==='conflict'||input.expected_version!==article.version)return send(409,{reason:'ARTICLE_VERSION_CONFLICT'})
   const old=history.find(r=>r.version===restore[1]);article={...old.snapshot,slug:'rev',version:String(BigInt(article.version)+1n),status:article.status,view_count:11,like_count:3}
   history.unshift(make(article.version,'restore',article.content_md));setTimeout(()=>send(200,article),400)
  })}
  const selected=u.pathname.match(/\/revisions\/(\d+)$/)
  if(selected){const revision=history.find(r=>r.version===selected[1]);return setTimeout(()=>send(200,revision),mode==='late'?1000:0)}
  if(u.pathname.endsWith('/articles/rev')||u.pathname.endsWith('/articles/other'))return send(200,{...article,slug:u.pathname.split('/').at(-1)})
  if(u.pathname.endsWith('/articles/list'))return send(200,{articles:[article]})
  send(200,{categories:[],state:1,article_slug:'rev',can_retry:false,message:'索引已同步'})
 })
 await listen(api);const reservation=http.createServer();await listen(reservation);const port=reservation.address().port;await new Promise(r=>reservation.close(r));const base=`http://127.0.0.1:${port}`
 const child=spawn(process.execPath,['.output/server/index.mjs'],{cwd:path.join(root,'frontend'),env:{...process.env,NITRO_PORT:String(port),NITRO_HOST:'127.0.0.1',NUXT_BACKEND_BASE:`http://127.0.0.1:${api.address().port}`},stdio:'ignore'})
 let browser
 try{
  for(let i=0;i<100;i++){try{if((await fetch(base+'/health')).ok)break}catch{}await new Promise(r=>setTimeout(r,100))}
  browser=await chromium.launch({headless:true,executablePath:process.env.BLOG_E2E_CHROMIUM||undefined})
  const page=await browser.newPage({viewport:{width:390,height:900}});page.setDefaultTimeout(12000);page.on('dialog',d=>d.accept());const errors=[];page.on('pageerror',e=>errors.push(e.message))
  await page.goto(base+'/admin/posts/rev/edit');const panel=page.getByRole('region',{name:'修订历史'}),title=page.getByPlaceholder('文章标题')
  async function select(version){await panel.getByRole('button',{name:new RegExp('^查看版本 '+version+' ')}).click();await panel.getByRole('heading',{name:new RegExp('^版本 '+version+' 与')}).waitFor()}
  const restore=panel.getByRole('button',{name:'确认恢复为新版本',exact:true})
  await select('7');assert.equal(await restore.isDisabled(),true);assert.equal(await panel.getByRole('checkbox').isDisabled(),true)
  await select('4');assert.equal(await page.evaluate(()=>window.revisionInjected),undefined)
  await title.fill('未保存输入');assert.equal(await restore.isDisabled(),true);assert.equal(await panel.getByRole('checkbox').isDisabled(),true);assert.equal(restoreCount,0)
  await title.fill('当前标题');await panel.getByRole('checkbox').check()
  const upload=page.waitForRequest(r=>r.url().includes('/admin/media/images'))
  await page.locator('[data-testid=article-image-input]').setInputFiles({name:'pixel.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2nH0AAAAASUVORK5CYII=','base64')})
  await upload;assert.equal(await panel.getByRole('checkbox').isDisabled(),true)
  await page.getByText('图片上传失败，请稍后重试',{exact:true}).waitFor()
  await page.waitForFunction(()=>!document.querySelector('[aria-label="修订历史"] input[type=checkbox]').disabled)
  for(const [bad,message]of[['conflict','恢复发生版本冲突'],['category','历史分类已失效'],['media','历史封面已不可用']]){
   mode=bad;await restore.click();await page.getByRole('alert').filter({hasText:message}).waitFor();assert.equal(await title.inputValue(),'当前标题');assert.equal(article.version,'7')
  }
  mode='normal';const completed=page.waitForResponse(r=>r.url().endsWith('/revisions/4/restore'))
  await restore.click();assert.equal(await title.isDisabled(),true);await completed
  await page.waitForFunction(()=>document.querySelector('input[placeholder="文章标题"]').value==='<script>window.revisionInjected=1</script>')
  assert.equal(article.version,'8');assert.equal(article.status,2);assert.equal(history.some(r=>r.version==='4'),true)
  await page.locator('.milkdown .ProseMirror').filter({hasText:'旧正文'}).waitFor()
  assert.equal(await page.evaluate(()=>window.revisionInjected),undefined)
  await select('3');await panel.getByText(/内容超出差异计算上限/).waitFor();assert.equal(await panel.getByRole('button',{name:'下载历史全文'}).count(),1)
  await fs.mkdir(out,{recursive:true});await page.screenshot({path:path.join(out,'s3-history-mobile.png'),fullPage:true})
  mode='late';await panel.getByRole('button',{name:/^查看版本 1 /}).click();await page.getByRole('link',{name:'返回列表',exact:true}).click();await page.waitForURL('**/admin/posts');await page.waitForTimeout(1100)
  assert.equal(await page.getByRole('heading',{name:/^版本 1 与/}).count(),0)
  mode='normal';await page.goto(base+'/admin/posts/rev/edit');await title.waitFor();if(await page.getByRole('button',{name:'丢弃草稿',exact:true}).count())await page.getByRole('button',{name:'丢弃草稿',exact:true}).click();await select('4');await panel.getByRole('checkbox').check()
  await restore.click();await page.getByRole('link',{name:'返回列表',exact:true}).click();await page.waitForURL('**/admin/posts');await page.waitForTimeout(600)
  assert.equal(await page.getByPlaceholder('文章标题').count(),0);assert.deepEqual(errors,[])
  await fs.writeFile(path.join(out,'s3-front-results.txt'),'PASS: history metadata/detail, current disabled, dirty protection, CAS/category/media errors, successful new version/editor sync, XSS escaping, bounded diff fallback, navigation isolation\n')
  console.log('PASS: history, safe diff, restore CAS, dirty/frozen/editor state, preserved history, typed failures, late response')
 }catch(e){console.error(e);throw e}finally{await browser?.close();child.kill('SIGTERM');api.closeAllConnections();await new Promise(r=>api.close(r))}
}
main().catch(e=>{console.error(e);process.exitCode=1})
