<template>
  <div class="home-visual" aria-hidden="true">
    <div class="visual-orbit" />
    <div class="visual-orbit visual-orbit-wide" />
    <div class="visual-cube">
      <div class="cube-face cube-front" /><div class="cube-face cube-back" />
      <div class="cube-face cube-left" /><div class="cube-face cube-right" />
      <div class="cube-face cube-top" /><div class="cube-face cube-bottom" />
    </div>
    <span class="visual-label visual-stack">Go / Nuxt</span>
    <span class="visual-label visual-agent">AI Agent ↗</span>
    <div class="visual-code">
      <div class="visual-code-dots"><i /><i /><i /></div>
      <span class="code-keyword">const</span> next = {<br>
      &nbsp; ideas: <span class="code-value">"worth building"</span>,<br>
      &nbsp; curiosity: <span class="code-value">true</span><br>
      }
    </div>
  </div>
</template>

<style scoped>
.home-visual { position: relative; min-width: 0; height: 310px; perspective: 1000px; overflow: clip; }
.visual-orbit { position: absolute; width: 66%; aspect-ratio: 1; border: 1px solid color-mix(in srgb, var(--accent) 18%, transparent); border-radius: 50%; left: 17%; top: 4%; transform: rotateX(65deg) rotateZ(-20deg); }
.visual-orbit-wide { width: 84%; left: 8%; top: -4%; border-style: dashed; transform: rotateX(68deg) rotateY(23deg) rotateZ(36deg); }
.visual-cube { position: absolute; width: 112px; height: 112px; top: 42px; left: calc(50% - 56px); transform-style: preserve-3d; transform: rotateX(-22deg) rotateY(-32deg); }
.cube-face { position: absolute; inset: 0; border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent); background: color-mix(in srgb, var(--accent) 6%, transparent); }
.cube-front { transform: translateZ(56px); }
.cube-front::before, .cube-front::after { content: ''; position: absolute; background: color-mix(in srgb, var(--accent) 28%, transparent); }
.cube-front::before { width: 1px; height: 100%; left: 50%; }
.cube-front::after { width: 100%; height: 1px; top: 50%; }
.cube-back { transform: rotateY(180deg) translateZ(56px); }
.cube-left { transform: rotateY(-90deg) translateZ(56px); }
.cube-right { transform: rotateY(90deg) translateZ(56px); }
.cube-top { transform: rotateX(90deg) translateZ(56px); }
.cube-bottom { transform: rotateX(-90deg) translateZ(56px); }
.visual-label { position: absolute; padding: .4rem .65rem; border: 1px solid var(--border-default); border-radius: .4rem; background: var(--surface-page); font: .6875rem ui-monospace, monospace; color: var(--text-muted); }
.visual-stack { left: 5%; top: 46px; }
.visual-agent { right: 1%; top: 100px; }
.visual-code { position: absolute; left: 8%; bottom: 23px; width: 86%; max-width: 360px; padding: 1rem 1.2rem; border: 1px solid var(--border-default); border-radius: .7rem; background: var(--surface-page); box-shadow: 0 16px 35px color-mix(in srgb, var(--accent) 12%, transparent); transform: rotateY(-9deg) rotateX(7deg) rotateZ(-3deg); font: .75rem/1.9 ui-monospace, monospace; color: var(--text-muted); }
.visual-code-dots { display: flex; gap: 4px; margin-bottom: .5rem; }
.visual-code-dots i { width: 4px; height: 4px; background: var(--border-strong); border-radius: 50%; }
.code-keyword { color: var(--accent); }
.code-value { color: var(--mint-text); }
@media (max-width: 640px) { .home-visual { height: 265px; max-width: 440px; width: 100%; margin: 0 auto; } .visual-cube { top: 23px; } .visual-code { bottom: 12px; font-size: .6875rem; } .visual-stack { top: 24px; } .visual-agent { top: 68px; } }
</style>
