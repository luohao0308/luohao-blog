/** A self-contained, progressive-enhancement ink field for the About introduction. */
export function mountAboutInk(canvas: HTMLCanvasElement): { setProgress(value: number): void; destroy(): void } | null {
  const gl = canvas.getContext('webgl', { alpha: false, antialias: false, depth: false, stencil: false, powerPreference: 'low-power' })
  if (!gl) { canvas.dataset.inkState = 'unsupported'; return null }
  const vertex = `attribute vec2 position; varying vec2 uv; void main(){uv=position*.5+.5;gl_Position=vec4(position,0.,1.);}`
  const fragment = `
#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
#else
precision mediump float;
#endif
varying vec2 uv;
uniform vec2 resolution;
uniform float time, progress, dark;
uniform vec3 wakes[8];
uniform vec4 pulse;
float hash(vec2 p){p=fract(p*vec2(.1031,.11369));float n=dot(p,p.yx+vec2(.33,.19));return fract(n*7.13);}
float noise(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1.,0.)),f.x),mix(hash(i+vec2(0.,1.)),hash(i+vec2(1.)),f.x),f.y);}
float field(vec2 p){return noise(p)*.58+noise(p*2.03+7.)*.28+noise(p*4.01)*.14;}
void main(){
 vec2 p=uv;float aspect=resolution.x/resolution.y;
 for(int i=0;i<8;i++){
   vec2 d=p-wakes[i].xy;d.x*=aspect;float envelope=exp(-dot(d,d)*15.)*wakes[i].z;
   p+=vec2(-d.y,d.x/aspect)*envelope*1.25;
   p+=vec2(sin(d.y*16.+time),cos(d.x*12.-time))*.024*envelope;
 }
 vec2 pd=p-pulse.xy;pd.x*=aspect;float distance=length(pd);float ring=exp(-pow((distance-pulse.z*.25)*12.,2.))*pulse.w;
 p+=vec2(pd.x/aspect,pd.y)/(distance+.03)*ring*.045;
 vec2 q=vec2(p.x*aspect,p.y);
 float t=time*.10;
 vec2 warp=vec2(field(q*1.7+vec2(t,-t*.6)),field(q*1.6+vec2(4.-t*.7,t*.5)));
 float ribbon=field(q*2.+warp*2.8+vec2(-t*.2,t*.45));
 float fold=sin(q.y*5.2+q.x*2.4+warp.x*6.+t)*.5+.5;
 float width=mix(.34,.055,progress);
 float center=mix(.63,.50,progress)+sin(p.y*4.+t)*.055*(1.-progress);
 float spread=exp(-pow((p.x-center)/width,2.));
 float body=smoothstep(.47,.65,ribbon)*spread;
 float edge=exp(-pow((ribbon-.545)*62.,2.))*spread;
 float silverRim=exp(-pow((ribbon-.57)*85.,2.))*spread;
 float silver=smoothstep(.59,.70,ribbon)*spread;
 float ink=clamp(body*.65+edge*.35+fold*body*.16,0.,1.);
 vec3 paper=mix(vec3(.995,.996,.999),vec3(.055,.073,.105),dark);
 vec3 blue=mix(vec3(.235,.365,.522),vec3(.47,.65,.78),dark);
 vec3 metal=mix(vec3(.75,.80,.85),vec3(.29,.40,.50),dark);
 vec3 color=mix(paper,blue,ink*.78);
 color=mix(color,metal,silver*.66+silverRim*.28);
 gl_FragColor=vec4(color,1.);
}`
  const shaders: WebGLShader[] = []
  const compile = (kind: number, source: string) => {
    const shader = gl.createShader(kind)
    if (!shader) return null
    shaders.push(shader)
    gl.shaderSource(shader, source); gl.compileShader(shader)
    if (gl.getShaderParameter(shader, gl.COMPILE_STATUS)) return shader
    console.error('[About ink] shader compilation failed', gl.getShaderInfoLog(shader))
    return null
  }
  let program: WebGLProgram | null = null
  let buffer: WebGLBuffer | null = null
  const release = () => { if (buffer) gl.deleteBuffer(buffer); if (program) gl.deleteProgram(program); shaders.forEach(shader => gl.deleteShader(shader)) }
  let uniforms: Record<string, WebGLUniformLocation | null>
  try {
  const vs = compile(gl.VERTEX_SHADER, vertex), fs = compile(gl.FRAGMENT_SHADER, fragment)
  program = gl.createProgram(); buffer = gl.createBuffer()
  if (!vs || !fs || !program || !buffer) { console.error('[About ink] GPU resource allocation or shader compilation failed');canvas.dataset.inkState='error'; release(); return null }
  gl.attachShader(program, vs); gl.attachShader(program, fs); gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) { console.error('[About ink] program linking failed', gl.getProgramInfoLog(program)); canvas.dataset.inkState='error'; release(); return null }
  gl.useProgram(program); gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1,1,-1,-1,1,-1,1,1,-1,1,1]), gl.STATIC_DRAW)
  const position = gl.getAttribLocation(program, 'position')
  gl.enableVertexAttribArray(position); gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
  uniforms = Object.fromEntries(['resolution','time','progress','dark','wakes[0]','pulse'].map(name => [name, gl.getUniformLocation(program!, name)]))
  } catch(error) { release();canvas.dataset.inkState='error';console.error('[About ink] GPU setup failed',error);return null }
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
  const wakes = new Float32Array(24)
  for (let i=0;i<8;i++) { wakes[i*3]=-10; wakes[i*3+1]=-10 }
  let wakeIndex=0, pointerX=-10, pointerY=-10, pulseX=-10, pulseY=-10, pulseAge=10
  let frame=0, last=0, elapsed=0, progress=0, visible=true, disposed=false, lost=false
  let isDark=document.documentElement.classList.contains('dark')
  const active = () => !disposed && !lost && visible && !document.hidden && !reduced.matches
  const draw = (delta: number) => {
    elapsed+=delta; pulseAge+=delta
    for(let i=0;i<8;i++) wakes[i*3+2]=Math.max(0,(wakes[i*3+2] ?? 0)-delta*.6)
    gl.uniform2f(uniforms.resolution!,canvas.width,canvas.height)
    gl.uniform1f(uniforms.time!,elapsed); gl.uniform1f(uniforms.progress!,progress); gl.uniform1f(uniforms.dark!,isDark?1:0)
    gl.uniform3fv(uniforms['wakes[0]']!,wakes)
    gl.uniform4f(uniforms.pulse!,pulseX,pulseY,pulseAge,Math.max(0,1-pulseAge*.55))
    gl.drawArrays(gl.TRIANGLES,0,6)
  }
  const tick = (now: number) => {
    frame=0
    if (!active()) return
    const delta=last?Math.min(.05,(now-last)/1000):0
    last=now; draw(delta); frame=requestAnimationFrame(tick)
  }
  const sync = () => {
    if (frame) cancelAnimationFrame(frame)
    frame=0;last=0
    if(disposed || lost) return
    canvas.dataset.inkState=active()?'active':'still'
    if(active()) frame=requestAnimationFrame(tick)
    else if(visible && !document.hidden) draw(0)
  }
  const resize = () => {
    if(disposed || lost) return
    const bounds=canvas.getBoundingClientRect()
    const width=Math.max(1,bounds.width),height=Math.max(1,bounds.height)
    const ratio=Math.min(window.devicePixelRatio||1,1.5,Math.sqrt(1200000/(width*height)))
    canvas.width=Math.max(1,Math.floor(width*ratio));canvas.height=Math.max(1,Math.floor(height*ratio))
    gl.viewport(0,0,canvas.width,canvas.height);sync()
  }
  const point = (event: PointerEvent) => {
    const box=canvas.getBoundingClientRect()
    pointerX=(event.clientX-box.left)/Math.max(1,box.width);pointerY=1-(event.clientY-box.top)/Math.max(1,box.height)
    if(pointerX<0 || pointerX>1 || pointerY<0 || pointerY>1) return false
    return true
  }
  const move = (event: PointerEvent) => {
    if(disposed || lost || !visible || document.hidden || !point(event)) return
    wakes[wakeIndex*3]=pointerX;wakes[wakeIndex*3+1]=pointerY;wakes[wakeIndex*3+2]=1
    wakeIndex=(wakeIndex+1)%8
    if(reduced.matches) draw(0)
  }
  const down = (event: PointerEvent) => {
    if(disposed || lost || !visible || document.hidden || !point(event)) return
    pulseX=pointerX;pulseY=pointerY;pulseAge=0
    if(reduced.matches) draw(0)
  }
  // Listen on the containing hero so typography does not interrupt pointer feedback.
  const surface=canvas.parentElement || canvas
  let observer: ResizeObserver | null = null
  let intersection: IntersectionObserver | null = null
  let palette: MutationObserver | null = null
  const cleanup = () => {
    disposed=true;if(frame)cancelAnimationFrame(frame)
    observer?.disconnect();intersection?.disconnect();palette?.disconnect()
    surface.removeEventListener('pointermove',move as EventListener);surface.removeEventListener('pointerdown',down as EventListener)
    canvas.removeEventListener('webglcontextlost',onLost);document.removeEventListener('visibilitychange',sync)
    reduced.removeEventListener('change',sync);window.removeEventListener('resize',resize)
    release()
  }
  const onLost = (event: Event) => { event.preventDefault();lost=true;if(frame)cancelAnimationFrame(frame);frame=0;canvas.dataset.inkState='lost' }
  try {
  surface.addEventListener('pointermove',move as EventListener,{passive:true})
  surface.addEventListener('pointerdown',down as EventListener,{passive:true})
  canvas.addEventListener('webglcontextlost',onLost)
  document.addEventListener('visibilitychange',sync)
  reduced.addEventListener('change',sync)
  window.addEventListener('resize',resize,{passive:true})
  observer=typeof ResizeObserver==='undefined'?null:new ResizeObserver(resize)
  observer?.observe(canvas)
  intersection=typeof IntersectionObserver==='undefined'?null:new IntersectionObserver(entries=>{visible=entries.some(entry=>entry.isIntersecting);sync()})
  intersection?.observe(canvas)
  palette=new MutationObserver(()=>{isDark=document.documentElement.classList.contains('dark');if(!active())sync()})
  palette.observe(document.documentElement,{attributes:true,attributeFilter:['class']})
  resize()
  } catch (error) {
    cleanup();canvas.dataset.inkState='error'
    console.error('[About ink] observer or event setup failed',error)
    return null
  }
  return {
    setProgress(value) { if(disposed || lost) return; progress=Math.max(0,Math.min(1,Number.isFinite(value)?value:0));if(!active())sync() },
    destroy() {
      if(disposed)return
      cleanup();delete canvas.dataset.inkState
    },
  }
}
