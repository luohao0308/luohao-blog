const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')

class Events {
  listeners = new Map()
  addEventListener(type, fn) { const list=this.listeners.get(type)||[]; list.push(fn);this.listeners.set(type,list) }
  removeEventListener(type, fn) { this.listeners.set(type,(this.listeners.get(type)||[]).filter(item=>item!==fn)) }
  fire(type, event={}) { for(const fn of [...(this.listeners.get(type)||[])])fn(event) }
  count() { return [...this.listeners.values()].reduce((sum,list)=>sum+list.length,0) }
}
function fixture({ reduced=false, compile=true, context=true, link=true, observerFails=false }={}) {
  const canvas=new Events(),surface=new Events(),document=new Events(),window=new Events(),media=new Events()
  const frames=new Map(),deleted=[],uniforms=new Map(),observers=[],errors=[]
  let next=0,draws=0
  const gl={
    createShader:()=>({kind:'shader'}),shaderSource(){},compileShader(){},getShaderParameter:()=>compile,
    createProgram:()=>({kind:'program'}),createBuffer:()=>({kind:'buffer'}),attachShader(){},linkProgram(){},getProgramParameter:()=>link,getShaderInfoLog:()=>'shader diagnostic',getProgramInfoLog:()=>'link diagnostic',
    deleteShader:item=>deleted.push(item),deleteProgram:item=>deleted.push(item),deleteBuffer:item=>deleted.push(item),
    useProgram(){},bindBuffer(){},bufferData(){},getAttribLocation:()=>0,enableVertexAttribArray(){},vertexAttribPointer(){},
    getUniformLocation:(_,name)=>name,uniform1f:(name,value)=>uniforms.set(name,value),uniform2f(){},uniform3fv(){},uniform4f(){},
    drawArrays:()=>draws++,viewport(){},
  }
  canvas.getContext=()=>context?gl:null;canvas.parentElement=surface;canvas.dataset={}
  canvas.getBoundingClientRect=()=>({left:0,top:0,width:2000,height:1000})
  document.hidden=false;document.documentElement={classList:{contains:()=>false}}
  media.matches=reduced;window.matchMedia=()=>media;window.devicePixelRatio=3
  const requestAnimationFrame=fn=>{ frames.set(++next,fn);return next }
  const cancelAnimationFrame=id=>frames.delete(id)
  class Observer {
    constructor(callback){this.callback=callback;this.disconnected=false;observers.push(this)}
    observe(){if(observerFails)throw new Error('observer rejected')} disconnect(){this.disconnected=true}
  }
  const exports={}
  vm.runInNewContext(ts.transpileModule(readFileSync(path.resolve(__dirname,'../app/utils/aboutInk.ts'),'utf8'),{
    compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022},
  }).outputText,{exports,console:{error:(...args)=>errors.push(args)},document,window,requestAnimationFrame,cancelAnimationFrame,ResizeObserver:Observer,IntersectionObserver:Observer,MutationObserver:Observer,Float32Array})
  const controller=exports.mountAboutInk(canvas)
  return {canvas,surface,document,window,media,frames,deleted,uniforms,observers,errors,controller,
    get draws(){return draws},flush(now=100){const pending=[...frames.values()];frames.clear();pending.forEach(fn=>fn(now))},
  }
}

test('unavailable WebGL returns fallback; failed compilation releases partially created resources',()=>{
  const absent=fixture({context:false})
  assert.equal(absent.controller,null);assert.equal(absent.frames.size,0);assert.equal(absent.canvas.dataset.inkState,'unsupported');assert.equal(absent.errors.length,0)
  const failed=fixture({compile:false})
  assert.equal(failed.controller,null);assert.equal(failed.canvas.dataset.inkState,'error');assert.ok(failed.errors.some(error=>error.includes('shader diagnostic')))
  assert.deepEqual(failed.deleted.map(item=>item.kind).sort(),['buffer','program','shader','shader'])
  assert.equal(failed.surface.count(),0);assert.equal(failed.observers.length,0)
})

test('reduced motion has no animation loop and explicit input renders a still field with clamped progress',()=>{
  const f=fixture({reduced:true})
  assert.equal(f.canvas.dataset.inkState,'still');assert.equal(f.frames.size,0);assert.equal(f.draws,1)
  assert.ok(f.canvas.width*f.canvas.height<=1200000)
  f.controller.setProgress(5);assert.equal(f.uniforms.get('progress'),1)
  f.controller.setProgress(-5);assert.equal(f.uniforms.get('progress'),0)
  f.controller.setProgress(NaN);assert.equal(f.uniforms.get('progress'),0)
  const before=f.draws
  f.surface.fire('pointermove',{clientX:100,clientY:100})
  f.surface.fire('pointerdown',{clientX:100,clientY:100})
  assert.equal(f.draws,before+2);assert.equal(f.frames.size,0)
  f.controller.destroy()
})

test('hidden and offscreen canvases stop pending frames and resume only when visible',()=>{
  const f=fixture()
  assert.equal(f.canvas.dataset.inkState,'active');assert.equal(f.frames.size,1)
  f.flush();assert.equal(f.draws,1);assert.equal(f.frames.size,1)
  f.document.hidden=true;f.document.fire('visibilitychange')
  assert.equal(f.frames.size,0);assert.equal(f.canvas.dataset.inkState,'still')
  const before=f.draws
  f.controller.setProgress(.4);assert.equal(f.draws,before)
  f.document.hidden=false;f.document.fire('visibilitychange');assert.equal(f.frames.size,1)
  f.observers[1].callback([{isIntersecting:false}]);assert.equal(f.frames.size,0)
  f.controller.setProgress(.8);assert.equal(f.draws,before)
  f.observers[1].callback([{isIntersecting:true}]);assert.equal(f.frames.size,1)
  f.media.matches=true;f.media.fire('change');assert.equal(f.frames.size,0)
  f.controller.destroy()
})

test('context loss stops rendering; destroy is idempotent and removes listeners, observers, resources and frames',()=>{
  const f=fixture()
  let prevented=false
  f.canvas.fire('webglcontextlost',{preventDefault(){prevented=true}})
  assert.equal(prevented,true);assert.equal(f.canvas.dataset.inkState,'lost');assert.equal(f.frames.size,0)
  f.controller.destroy();f.controller.destroy()
  assert.equal(f.deleted.length,4);assert.ok(f.observers.every(observer=>observer.disconnected))
  for(const target of [f.canvas,f.surface,f.document,f.window,f.media])assert.equal(target.count(),0)
  assert.equal(f.canvas.dataset.inkState,undefined)
  f.controller.setProgress(.5);assert.equal(f.frames.size,0)
  const pending=fixture();assert.equal(pending.frames.size,1);pending.controller.destroy();assert.equal(pending.frames.size,0)
})


test('link and observer setup failures are diagnosed and release every partial resource',()=>{
  const link=fixture({link:false})
  assert.equal(link.controller,null);assert.equal(link.canvas.dataset.inkState,'error')
  assert.equal(link.deleted.length,4);assert.ok(link.errors.some(error=>error.includes('link diagnostic')))
  const setup=fixture({observerFails:true})
  assert.equal(setup.controller,null);assert.equal(setup.canvas.dataset.inkState,'error')
  assert.equal(setup.deleted.length,4);assert.equal(setup.frames.size,0)
  assert.ok(setup.errors.some(error=>error[0].includes('setup failed')))
  assert.ok(setup.observers.every(observer=>observer.disconnected))
  for(const target of [setup.canvas,setup.surface,setup.document,setup.window,setup.media])assert.equal(target.count(),0)
})
