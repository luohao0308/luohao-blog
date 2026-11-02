// Real project screenshots on an isolated read-only API; no CMS or account writes.
const { chromium } = require('playwright')
const fs = require('node:fs'), path = require('node:path'), http = require('node:http')
const { spawn } = require('node:child_process'), assert = require('node:assert/strict')
const root = path.resolve(__dirname, '../..'), out = path.join(root, 'output/portfolio-real/browser')
async function main() {
  fs.mkdirSync(out, { recursive: true })
  const bundle = JSON.parse(fs.readFileSync(path.join(root, 'docs/content/portfolio-real-demos-2026-11.json'), 'utf8'))
  const files = { 'dev-workflow':['dev-workflow-install-preview.png','dev-workflow-install-audit.png'], 'forge-harness':['forge-workspace.png','forge-run-detail.png','forge-plan-evidence.png'] }
  const projects = bundle.operations.map(o => ({ id:o.id, slug:o.slug, name:o.slug==='forge-harness'?'Forge Harness':'dev-workflow', summary:'开源作品真实运行展示', role:'个人开源项目', stack:[], year:'2026', published:true, repo_url:`https://github.com/luohao0308/${o.slug}`, ...o.after }))
  const images = projects.flatMap(p => p.screenshots.map((url,i)=>({ url, file:files[p.slug][i] })))
  const api = http.createServer((req, res) => {
    const u = new URL(req.url, 'http://fixture')
    const json = (d, code=200) => { res.writeHead(code, { 'Content-Type':'application/json' }); res.end(JSON.stringify(d)) }
    if (u.pathname.includes('/auth/')) return json({}, 401)
    if (req.method !== 'GET') return json({}, 405)
    const image = images.find(x => x.url === u.pathname)
    if (image) { res.writeHead(200, { 'Content-Type':'image/png' }); return res.end(fs.readFileSync(path.join(root, 'frontend/public/images/projects/demos', image.file))) }
    const slug = u.pathname.match(/^\/v1\/projects\/([^/]+)$/)?.[1]
    if (slug) return json(projects.find(x => x.slug===slug) || {}, projects.some(x=>x.slug===slug)?200:404)
    if (u.pathname.endsWith('/projects/list')) return json({ projects })
    return json({})
  })
  await new Promise((resolve,reject)=>{api.once('error',reject);api.listen(14361,'127.0.0.1',resolve)})
  const log=fs.openSync(path.join(out,'server.log'),'w')
  const server=spawn(process.execPath,['.output/server/index.mjs'],{cwd:path.join(root,'frontend'),detached:true,stdio:['ignore',log,log],env:{...process.env,HOST:'127.0.0.1',PORT:'14360',NUXT_BACKEND_BASE:'http://127.0.0.1:14361',NUXT_PUBLIC_SITE_URL:'http://127.0.0.1:14360'}})
  fs.closeSync(log);let exited=false;server.on('exit',()=>{exited=true});let browser;const results=[]
  try {
    let ready=false
    for(let i=0;i<120;i++){assert(!exited);try{ready=(await fetch('http://127.0.0.1:14360/health')).ok}catch{}if(ready)break;await new Promise(r=>setTimeout(r,500))}
    assert(ready)
    browser=await chromium.launch({headless:true,executablePath:process.env.BLOG_E2E_CHROMIUM||undefined})
    for(const width of [1440,390])for(const project of projects){
      const c=await browser.newContext({viewport:{width,height:1000},reducedMotion:'reduce'});const p=await c.newPage();const errors=[];p.on('pageerror',e=>errors.push(e.message))
      await p.goto('http://127.0.0.1:14360/projects/'+project.slug,{waitUntil:'networkidle'})
      const links=p.locator('a[aria-label$="原图"]');assert.equal(await links.count(),project.screenshots.length)
      for(const link of await links.all()){const img=link.locator('img');await img.scrollIntoViewIfNeeded();await p.waitForFunction(el=>el.complete&&el.naturalWidth===1440,await img.elementHandle());assert((await img.getAttribute('alt')).includes(project.name));assert((await link.getAttribute('href')).startsWith('/api/v1/assets/media/'))}
      await links.first().scrollIntoViewIfNeeded();await p.screenshot({path:path.join(out,`${project.slug}-${width}.png`)})
      const [popup]=await Promise.all([c.waitForEvent('page'),links.first().click()]);await popup.waitForLoadState();assert(popup.url().includes('/api/v1/assets/media/'));await popup.close()
      assert(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));assert.deepEqual(errors,[])
      assert(await p.getByText(project.slug==='forge-harness'?'图内已标注本地演示环境':'展示选择 Core',{exact:false}).count()>0)
      results.push({width,slug:project.slug,images:project.screenshots.length,originalImageLink:true});await c.close()
    }
    fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({passed:true,results},null,2));console.log('PASS: two real project galleries, desktop/mobile, native images and original links, explicit provenance')
  }finally{await browser?.close();if(!exited){const closed=new Promise(r=>server.once('exit',r));process.kill(-server.pid,'SIGTERM');await closed}await new Promise(r=>api.close(r))}
}
main().catch(e=>{console.error(e);process.exitCode=1})
