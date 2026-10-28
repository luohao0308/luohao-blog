<script setup lang="ts">
defineProps<{ effect: 'float' | 'focus' | 'wipe' }>()
</script>

<template>
  <AboutScrollReveal class="scroll-demo" :effect="effect">
    <section data-section class="demo-intro">
      <p data-reveal class="intro-label">关于我</p>
      <h1 data-reveal>你好，我是罗浩。</h1>
      <p data-reveal class="intro-copy">做产品，写代码，也折腾 AI。</p>
      <p data-reveal>我喜欢把一个想法做出来，也喜欢记录中间的判断、踩坑和发现。这里分享具体的实践，希望给同样爱折腾的人一点启发。</p>
      <p data-reveal class="scroll-hint">向下滚动，继续了解 ↓</p>
    </section>

    <section data-section>
      <h2 data-reveal>这里会写什么</h2>
      <p data-reveal><strong>产品实践。</strong>记录从想法到实现的过程、需求取舍与使用反馈。</p>
      <p data-reveal><strong>AI 工具与工作流。</strong>分享日常尝试，聊聊工具如何融入创作与开发。</p>
      <p data-reveal><strong>技术复盘。</strong>把踩过的坑、做过的判断和新发现整理下来。</p>
    </section>

    <section data-section>
      <h2 data-reveal>正在做的事</h2>
      <p data-reveal><strong>dev-workflow。</strong>探索人与 AI 协作开发的流程，边使用边记录。</p>
      <p data-reveal><strong>Forge Harness。</strong>把 AI 开发中的想法做成工具，分享过程中的取舍与发现。</p>
      <p data-reveal><strong>这个网站。</strong>把实践整理成文章与项目，也欢迎读者交流自己的尝试。</p>
    </section>

    <section data-section>
      <h2 data-reveal>从这里开始</h2>
      <p data-reveal>如果第一次来，可以先看看这些内容。</p>
      <NuxtLink data-reveal to="/projects/dev-workflow" class="demo-link">dev-workflow：把项目规则留在仓库里 <span aria-hidden="true">↗</span></NuxtLink>
      <NuxtLink data-reveal to="/projects/forge-harness" class="demo-link">Forge Harness：围绕任务组织运行过程 <span aria-hidden="true">↗</span></NuxtLink>
      <NuxtLink data-reveal to="/posts" class="demo-link">阅读最新文章 <span aria-hidden="true">↗</span></NuxtLink>
    </section>

    <section data-section class="demo-contact">
      <h2 data-reveal>一起交流</h2>
      <p data-reveal>如果你也在做产品、写代码或尝试 AI，欢迎分享问题、想法和实践。</p>
      <div class="contact-actions">
        <a data-reveal href="https://github.com/luohao0308" target="_blank" rel="noopener" class="contact-button">
          <span>访问 GitHub <span aria-hidden="true">↗</span></span>
          <span class="contact-address">github.com/luohao0308</span>
        </a>
        <a data-reveal href="mailto:2429260713@qq.com" class="contact-button">
          <span>给我发邮件 <span aria-hidden="true">↗</span></span>
          <span class="contact-address">2429260713@qq.com</span>
        </a>
      </div>
    </section>
  </AboutScrollReveal>
</template>

<style scoped>
.scroll-demo { position: relative; z-index: 1; max-width: 760px; margin: 0 auto; padding: 0 24px; }
section { padding: 48px 0; }
section + section { border-top: 1px solid var(--border-default, #e2e8f0); }
.demo-intro { padding-top: 40px; }
h1 { margin: 22px 0 20px; font-size: clamp(30px, 4vw, 44px); line-height: 1.4; font-weight: 600; letter-spacing: -.03em; }
h2 { margin-bottom: 24px; font-size: clamp(23px, 3vw, 28px); font-weight: 550; line-height: 1.5; }
p { margin: 16px 0; font-size: 16px; line-height: 2; color: var(--text-secondary, #475569); }
strong { font-weight: 550; color: var(--text-primary, #0f172a); }
.intro-label, .scroll-hint { font-size: 13px; color: var(--text-muted, #64748b); }
.intro-copy { font-size: 20px; color: var(--text-primary, #0f172a); }
.scroll-hint { margin-top: 24px; }
.demo-link { display: flex; justify-content: space-between; gap: 24px; margin-top: 18px; padding: 18px 0; border-bottom: 1px solid var(--border-default, #e2e8f0); font-size: 16px; line-height: 1.8; }
.demo-link:hover { color: var(--accent, #3c5d85); }
.demo-link:focus-visible { outline: 2px solid var(--accent, #3c5d85); outline-offset: 5px; }
[data-reveal] { transform-origin: left center; }
.demo-contact { padding-bottom: 64px; }
.contact-actions { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 24px; }
.contact-button { display: flex; flex-direction: column; gap: 12px; min-height: 100px; min-width: 240px; max-width: 100%; padding: 22px 28px; border: 1px solid var(--border-strong); border-radius: 16px; background: var(--surface-page); transition: transform 250ms, border-color 250ms; }
.contact-button:hover { transform: translateY(-4px); border-color: var(--accent); }
.contact-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 5px; }
.contact-button > span:first-child { display: flex; justify-content: space-between; gap: 24px; font-weight: 500; }
.contact-address { font-size: 12px; color: var(--text-muted); overflow-wrap: anywhere; }
@media (max-width: 640px) { section { padding: 32px 0; } .scroll-demo { padding-inline: 20px; } .intro-copy { font-size: 18px; } .contact-actions { flex-direction: column; } .contact-button { min-width: 0; width: 100%; } }
@media (prefers-reduced-motion: reduce) { .contact-button { transition: none; } .contact-button:hover { transform: none; } }
</style>
