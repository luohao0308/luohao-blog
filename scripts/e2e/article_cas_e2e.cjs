// Local fake-CAS contract through real Nuxt, no CMS/database operations.
const assert = require('node:assert/strict')
const http = require('node:http')
const path = require('node:path')
const { spawn } = require('node:child_process')
const { chromium } = require('playwright')
const root = path.resolve(__dirname, '../..')
const article = { id: 'cas', slug: 'cas', version: '9007199254740993', title: '服务器标题', summary: '', content_md: '正文', content_html: '<p>正文</p>', tags: [], status: 2, cover_url: '', featured: false }
const writes = []
let delay = 0
const listen = s => new Promise(resolve => s.listen(0, '127.0.0.1', resolve))
async function main() {
 const api = http.createServer((req,res) => {
  const u = new URL(req.url,'http://fixture')
  const send=(code,data)=>{res.writeHead(code,{'Content-Type':'application/json'});res.end(JSON.stringify(data))}
  if(u.pathname.includes('/auth/')) return send(200,{access_token:'fixture',user:{id:'cas-author',role:1,display_name:'作者'}})
  if(u.pathname.endsWith('/articles/update')) {
   let raw='';req.on('data', c=>{raw+=c});return req.on('end',()=>{
    const expected=u.searchParams.get('expected_version');writes.push(expected)
    if(expected!==article.version)return send(409,{reason:'ARTICLE_VERSION_CONFLICT'})
    Object.assign(article,JSON.parse(raw));article.version=String(BigInt(article.version)+1n)
    const saved={...article};setTimeout(()=>send(200,saved),delay)
   })
  }
  if(u.pathname.endsWith('/articles/list'))return send(200,{articles:[article]})
  if(u.pathname.endsWith('/articles/cas') && req.method==='DELETE'){writes.push(u.searchParams.get('expected_version'));return send(200,{})}
  if(u.pathname.endsWith('/articles/cas'))return send(200,article)
  send(200,{categories:[],state:1,article_slug:'cas',can_retry:false,message:'索引已同步'})
 })
 await listen(api)
 const reservation=http.createServer();await listen(reservation);const port=reservation.address().port;await new Promise(r=>reservation.close(r))
 const base=`http://127.0.0.1:${port}`
 const child=spawn(process.execPath,['.output/server/index.mjs'],{cwd:path.join(root,'frontend'),env:{...process.env,NITRO_PORT:String(port),NITRO_HOST:'127.0.0.1',NUXT_BACKEND_BASE:`http://127.0.0.1:${api.address().port}`},stdio:'ignore'})
 let browser
 try{
  for(let i=0;i<100;i++){try{if((await fetch(base+'/health')).ok)break}catch{}await new Promise(r=>setTimeout(r,100))}
  browser=await chromium.launch({headless:true,executablePath:process.env.BLOG_E2E_CHROMIUM||undefined})
  const context=await browser.newContext();const a=await context.newPage(),b=await context.newPage()
  for(const p of [a,b]){p.on('dialog',d=>d.accept());await p.goto(base+'/admin/posts/cas/edit');await p.getByPlaceholder('文章标题').waitFor()}
  await a.getByPlaceholder('文章标题').fill('窗口A修改');await b.getByPlaceholder('文章标题').fill('窗口B保留输入');await a.getByRole('button',{name:'保存',exact:true}).click();await a.getByText('正文已保存',{exact:true}).waitFor()
  assert.equal(writes[0],'9007199254740993')
  assert.ok(await b.evaluate(()=>Object.keys(localStorage).some(k=>k.startsWith('blog:article-draft:cas-author:cas:')&&JSON.parse(localStorage.getItem(k)).title==='窗口B保留输入')), 'A saving cannot erase B draft')
  await b.getByPlaceholder('文章标题').fill('窗口B保留输入');await b.getByRole('button',{name:'保存',exact:true}).click()
  await b.getByRole('alert').filter({hasText:'文章已有新版本'}).waitFor()
  assert.equal(await b.getByPlaceholder('文章标题').inputValue(),'窗口B保留输入');assert.equal(article.title,'窗口A修改')
  assert.equal(writes.length,2);assert.equal(writes[1],writes[0]);assert.equal(await b.getByRole('link',{name:'打开最新版本对照'}).getAttribute('target'),'_blank')
  await b.reload();await b.getByRole('button',{name:'恢复草稿',exact:true}).click();await b.getByRole('button',{name:'保存',exact:true}).click()
  await b.getByRole('alert').filter({hasText:'文章已有新版本'}).waitFor();assert.equal(writes.at(-1),'9007199254740993')
  delay=600
  await a.getByPlaceholder('文章标题').fill('提交中标题');const pending=a.waitForResponse(r=>r.url().includes('/articles/update'))
  await a.getByRole('button',{name:'保存',exact:true}).click();await a.getByPlaceholder('文章标题').fill('提交后继续输入');await pending
  await a.waitForFunction(()=>JSON.parse(localStorage.getItem('blog:article-draft:cas-author:cas')).base_version==='9007199254740995')
  assert.equal(await a.getByPlaceholder('文章标题').inputValue(),'提交后继续输入')
  // Simulate a duplicated tab owner writing a different draft during a save.
  await a.getByPlaceholder('文章标题').fill('相同owner提交')
  const sameOwnerSaved=a.waitForResponse(r=>r.url().includes('/articles/update'))
  await a.getByRole('button',{name:'保存',exact:true}).click()
  await a.evaluate(()=>{const base='blog:article-draft:cas-author:cas';const own=base+':'+sessionStorage.getItem('blog:article-draft-tab');const d=JSON.parse(localStorage.getItem(own));d.title='相同owner另一窗口草稿';for(const k of [base,own])localStorage.setItem(k,JSON.stringify(d))})
  await sameOwnerSaved
  assert.equal(await a.evaluate(()=>JSON.parse(localStorage.getItem('blog:article-draft:cas-author:cas')).title),'相同owner另一窗口草稿')
  delay=0
  const count=writes.length
  await a.evaluate(()=>{const k='blog:article-draft:cas-author:cas';const d=JSON.parse(localStorage.getItem(k));delete d.base_version;localStorage.setItem(k,JSON.stringify(d));localStorage.removeItem(k+':'+sessionStorage.getItem('blog:article-draft-tab'))})
  await a.reload();await a.getByRole('button',{name:'恢复草稿',exact:true}).click();await a.getByRole('button',{name:'保存',exact:true}).click()
  await a.getByText('缺少草稿基准版本，不能覆盖服务器内容。请打开最新版本对照后手动整理。',{exact:true}).waitFor();assert.equal(writes.length,count)
  const list=await context.newPage();await list.goto(base+'/admin/posts');await list.getByRole('button',{name:'转为草稿',exact:true}).waitFor()
  const rowVersion=article.version;const rowSave=list.waitForResponse(r=>r.url().includes('/articles/update'))
  await list.getByRole('button',{name:'转为草稿',exact:true}).click();await rowSave;assert.equal(writes.at(-1),rowVersion)
  await list.getByRole('button',{name:'发布',exact:true}).waitFor();const deleteVersion=article.version
  await list.getByRole('button',{name:'删除',exact:true}).click();const deleted=list.waitForResponse(r=>r.request().method()==='DELETE')
  await list.locator('.n-popconfirm__action button').last().click();await deleted;assert.equal(writes.at(-1),deleteVersion)
  console.log('PASS: two windows, uint64 exactness, conflict retains input, reload draft keeps original base, in-flight edit advances baseline, legacy draft blocks overwrite')
 }finally{await browser?.close();child.kill('SIGTERM');api.closeAllConnections();await new Promise(r=>api.close(r))}
}
main().catch(e=>{console.error(e);process.exitCode=1})
