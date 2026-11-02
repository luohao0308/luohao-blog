// Preview the proposed case-study articles with real backend-rendered HTML on local fixtures.
const { chromium }=require('playwright'),fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict'),{spawn}=require('node:child_process')
const root=path.resolve(__dirname,'../..'),out=path.join(root,'output/blog-case-studies/browser')
async function main(){
 fs.mkdirSync(out,{recursive:true})
 const bundle=JSON.parse(fs.readFileSync(path.join(root,'content/releases/blog-case-studies-20261102.json'),'utf8'))
 const html=JSON.parse(fs.readFileSync(path.join(root,'output/blog-case-studies/rendered.json'),'utf8'))
 const articles=bundle.operations.map((o,i)=>({id:`case-preview-${i}`,version:2,...o.fields,content_html:html[o.slug],created_at:'2026-11-02T07:00:00Z',updated_at:'2026-11-02T07:00:00Z',published_at:'2026-11-02T07:00:00Z',view_count:0,like_count:0}))
 const project={id:'preview-project',slug:'luohao-blog',name:'luohao-blog',summary:'个人博客与AI检索实践',description:'## 实现与文章\n\n本地草稿预览。',stack:['Go','Nuxt'],role:'个人项目',screenshots:[],published:true,repo_url:'https://github.com/luohao0308/luohao-blog'}
 let hideSecond=false,failList=false
 const api=http.createServer((req,res)=>{const u=new URL(req.url,'http://fixture'),reply=(d,s=200)=>{res.writeHead(s,{'Content-Type':'application/json'});res.end(JSON.stringify(d))}
 if(u.pathname.includes('/auth/'))return reply({},401)
 if(u.pathname.endsWith('/view')||u.pathname.endsWith('/like'))return reply({view_count:0,like_count:0})
 if(req.method!=='GET')return reply({},405)
 if(u.pathname.endsWith('/articles/list')){if(failList)return reply({},503);return reply({articles:[articles[0],{...articles[1],status:hideSecond?1:2},{...articles[0],slug:'fullstack-smoke-pitfalls',title:'PRIVATE_DRAFT_SENTINEL',status:1}],next_page_token:''})}
 if(u.pathname.endsWith('/projects/list'))return reply({projects:[project],next_page_token:''})
 if(u.pathname==='/v1/projects/luohao-blog')return reply(project)
 const slug=u.pathname.match(/^\/v1\/articles\/([^/]+)$/)?.[1];if(slug){const a=articles.find(x=>x.slug===slug);return a?reply({...a,status:hideSecond&&slug===articles[1].slug?1:2}):reply({},404)}
 return reply({comments:[],collections:[],next_page_token:''})})
 await new Promise((resolve,reject)=>{api.once('error',reject);api.listen(14371,'127.0.0.1',resolve)})
 const log=fs.openSync(path.join(out,'server.log'),'w'),server=spawn(process.execPath,['.output/server/index.mjs'],{cwd:path.join(root,'frontend'),detached:true,stdio:['ignore',log,log],env:{...process.env,HOST:'127.0.0.1',PORT:'14370',NUXT_BACKEND_BASE:'http://127.0.0.1:14371',NUXT_PUBLIC_SITE_URL:'http://127.0.0.1:14370'}})
 fs.closeSync(log);let exited=false;server.on('exit',()=>{exited=true});let browser;const results=[]
 try{
 let ready=false;for(let i=0;i<120;i++){assert(!exited);try{ready=(await fetch('http://127.0.0.1:14370/health')).ok}catch{}if(ready)break;await new Promise(r=>setTimeout(r,500))}assert(ready)
 browser=await chromium.launch({headless:true,executablePath:process.env.BLOG_E2E_CHROMIUM||undefined})
 for(const width of [1440,390])for(const a of articles){const c=await browser.newContext({viewport:{width,height:1000},reducedMotion:'reduce'}),p=await c.newPage(),errors=[];p.on('pageerror',e=>errors.push(e.message));await p.goto('http://127.0.0.1:14370/posts/'+a.slug,{waitUntil:'networkidle'});assert.equal(await p.locator('h1').textContent(),a.title);assert(await p.locator('[data-article-detail] pre').count()>0);assert(await p.locator('[data-article-detail] h2').count()>=5);if(a.slug.includes('-cas-'))assert(await p.locator('[data-article-detail] table').count()>0);assert(await p.locator('[data-article-detail] a[href="/projects/luohao-blog"]').count()>0);assert(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));assert.deepEqual(errors,[]);await p.screenshot({path:path.join(out,`${a.slug}-${width}.png`)});results.push({width,slug:a.slug,headings:true,code:true,overflow:false});await c.close()}
 const c=await browser.newContext(),p=await c.newPage();await p.goto('http://127.0.0.1:14370/projects/luohao-blog',{waitUntil:'networkidle'});const related=p.getByRole('region',{name:'项目相关文章'});assert.equal(await related.getByRole('link').count(),2);assert.equal(await p.getByText('PRIVATE_DRAFT_SENTINEL').count(),0)
 hideSecond=true;await p.reload({waitUntil:'networkidle'});assert.equal(await related.getByRole('link').count(),1);assert.equal(await related.getByRole('link',{name:articles[1].title,exact:false}).count(),0)
 failList=true;await p.reload({waitUntil:'networkidle'});await p.getByRole('alert').filter({hasText:'相关文章暂时无法加载'}).waitFor();failList=false;hideSecond=false;await p.getByRole('button',{name:'重新加载',exact:true}).click();await related.getByRole('link',{name:articles[1].title,exact:false}).waitFor();await c.close()
 fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({passed:true,results,relatedPublicOnly:true,listFailureRetry:true},null,2));console.log('PASS: two case articles desktop/mobile, real rendering, public-only reciprocal project links and failure retry')
 }finally{await browser?.close();if(!exited){const closed=new Promise(r=>server.once('exit',r));process.kill(-server.pid,'SIGTERM');await closed}await new Promise(r=>api.close(r))}
}
main().catch(e=>{console.error(e);process.exitCode=1})
