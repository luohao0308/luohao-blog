<script setup lang="ts">
const popoverId = useId()
const panel = ref<HTMLElement | null>(null)
function close() { panel.value?.hidePopover() }
</script>

<template>
  <div>
    <aside class="about-wechat-docked wechat-card" aria-label="微信联系与公众号">
      <WechatFollow />
    </aside>
    <button type="button" :popovertarget="popoverId" class="about-wechat-toggle rounded-full border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3 text-sm shadow-md" aria-label="打开微信联系与公众号">微信</button>
    <div :id="popoverId" ref="panel" popover="auto" role="dialog" aria-label="微信联系与公众号" class="about-wechat-popover wechat-card rounded-xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-3 shadow-lg" @keydown.esc.prevent="close">
      <div class="mb-2 flex justify-end">
        <button type="button" :popovertarget="popoverId" popovertargetaction="hide" class="rounded-full px-3 py-1 text-sm text-slate-500 dark:text-slate-400" aria-label="关闭微信联系">关闭</button>
      </div>
      <WechatFollow />
    </div>
  </div>
</template>

<style scoped>
.about-wechat-docked { display: none; }
.about-wechat-toggle { position: fixed; right: 1.5rem; bottom: max(1.5rem, env(safe-area-inset-bottom)); z-index: 30; }
.about-wechat-popover {
  position: fixed;
  inset: auto 1rem max(5rem, calc(env(safe-area-inset-bottom) + 4rem)) auto;
  margin: 0;
  width: min(22rem, calc(100vw - 2rem));
  max-height: calc(100dvh - 8rem);
  overflow-y: auto;
}
.wechat-card :deep(section) { flex-direction: column; align-items: stretch; padding: 1rem; }
.wechat-card :deep(button) { white-space: normal; }
@media (min-width: 1360px) {
  .about-wechat-toggle, .about-wechat-popover:popover-open { display: none; }
  .about-wechat-docked {
    display: block;
    position: fixed;
    top: 9rem;
    left: calc(50% + 454px + 1rem);
    width: 12rem;
    max-height: calc(100dvh - 11rem);
    overflow-y: auto;
    z-index: 20;
  }
}
</style>
