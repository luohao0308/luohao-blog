<script setup lang="ts">
import { backgroundStudies } from '~/data/backgroundLab'

definePageMeta({ layout: false })
useSeoMeta({ title: '背景动效实验室 · 罗浩', robots: 'noindex, nofollow' })
const route = useRoute()
const router = useRouter()
const initialIndex = backgroundStudies.findIndex(item => item.id === route.query.effect)
const selectedIndex = ref(initialIndex >= 0 ? initialIndex : backgroundStudies.findIndex(item => item.id === 'business-city'))
const selected = computed(() => backgroundStudies[selectedIndex.value]!)
const selectedGroup = computed(() => selected.value.group ?? 'abstract')
const original = computed(() => backgroundStudies.find(item => item.id === selected.value.comparison))
const visibleStudies = computed(() => backgroundStudies.filter(item => (item.group ?? 'abstract') === selectedGroup.value))
function selectStudy(id: string) {
  const index = backgroundStudies.findIndex(item => item.id === id)
  if (index < 0) return
  selectedIndex.value = index
  void router.replace({ query: { ...route.query, effect: id } })
}
watch(() => route.query.effect, id => {
  const index = backgroundStudies.findIndex(item => item.id === id)
  if (index >= 0) selectedIndex.value = index
})
const playing = ref(true)
const resetToken = ref(0)
const hitLabel = ref('')
const showCopy = ref(true)
const reduced = ref(false)
const expanded = ref(false)
const stage = ref<HTMLElement>()
const expandButton = ref<HTMLButtonElement>()
let previousOverflow = ''
async function toggleExpanded() {
  expanded.value = !expanded.value
  if (expanded.value) {
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
    stage.value?.querySelector<HTMLButtonElement>('button:not([disabled])')?.focus()
  }
  else {
    document.body.style.overflow = previousOverflow
    await nextTick()
    expandButton.value?.focus()
  }
}
function handleStageKey(event: KeyboardEvent) {
  if (!expanded.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    void toggleExpanded()
  }
  if (event.key !== 'Tab') return
  const items = [...(stage.value?.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], input:not([disabled]), [tabindex="0"]') ?? [])]
  const first = items[0]
  const last = items.at(-1)
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  }
  else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
onBeforeUnmount(() => {
  if (expanded.value) document.body.style.overflow = previousOverflow
})
</script>

<template>
  <main class="background-lab">
    <header class="lab-header" :inert="expanded">
      <NuxtLink to="/about" class="back-link">← 返回关于我</NuxtLink><span class="edition">VISUAL STUDIES / 2026</span>
    </header>
    <section class="lab-intro" :inert="expanded">
      <p class="eyebrow">四组探索 · 二十一种可能</p><h1>背景动效实验室<span>。</span></h1>
      <p class="intro-note">先看形状，再玩一玩。四种机械变化、两种城市模型可切换对照，原有候选全部保留。</p>
      <p class="preview-note">当前选择：商务模型城，已接入本地关于页；其余候选保留对照。</p>
    </section>
    <nav class="lab-groups" aria-label="候选分组" :inert="expanded">
      <button type="button" :aria-pressed="selectedGroup === 'variant'" @click="selectStudy('business-city')">造型／玩法变化 <span>06</span></button>
      <button type="button" :aria-pressed="selectedGroup === 'focus'" @click="selectStudy('kinetic-sculpture')">精选深化 <span>03</span></button>
      <button type="button" :aria-pressed="selectedGroup === 'scene'" @click="selectStudy('kinetic')">场景创意 <span>06</span></button>
      <button type="button" :aria-pressed="selectedGroup === 'abstract'" @click="selectStudy('blocks')">基础抽象 <span>06</span></button>
    </nav>
    <nav class="study-picker" :class="{ focused: selectedGroup === 'focus', variants: selectedGroup === 'variant' }" aria-label="背景候选" :inert="expanded">
      <button v-for="study in visibleStudies" :key="study.id" type="button" :aria-pressed="selected.id === study.id" :class="['study-choice', { active: selected.id === study.id }]" :style="{ '--thumb-paper': study.paper, '--thumb-ink': study.accent }" @click="selectStudy(study.id)">
        <span :class="['study-thumb', `thumb-${study.id}`]" aria-hidden="true"><i /><i /><i /><i /><i /></span>
        <span class="choice-label"><small>{{ String(backgroundStudies.findIndex(item => item.id === study.id) + 1).padStart(2, '0') }}</small>{{ study.name }}</span>
      </button>
    </nav>
    <section ref="stage" :class="['lab-stage', { expanded, 'focus-scene': selectedGroup === 'focus' || selectedGroup === 'variant' }]" :style="{ '--stage-paper': selected.paper, '--stage-ink': selected.ink, '--stage-accent': selected.accent }" :role="expanded ? 'dialog' : undefined" :aria-modal="expanded ? true : undefined" aria-label="背景动态预览" @keydown="handleStageKey">
      <BackgroundLabCanvas :effect="selected.id" :playing="playing" :reset-token="resetToken" @interaction-label="hitLabel = $event" @motion-preference="reduced = $event" />
      <div v-if="selected.id !== 'business-city'" class="stage-caption"><span>{{ selected.english }}</span><span>{{ reduced ? '减少动态 · 静态预览' : playing ? '动态预览中' : '已暂停' }}</span></div>
      <div v-if="showCopy" class="sample-copy"><p class="sample-kicker">全栈开发 / AI Agent</p><h2>你好，<br>我是罗浩<span>。</span></h2><p class="sample-role">Go · Vue · AI</p></div>
      <div class="stage-bottom" :class="{ 'city-bottom': selected.id === 'business-city' }">
        <p v-if="selected.id !== 'business-city'" class="pointer-hint">{{ reduced ? selected.interactive ? '减少动态：点击部件查看静态反馈' : '尊重系统减少动态设置' : hitLabel || selected.interaction }}<span class="mobile-hint">触摸屏也可直接观看自动动态</span></p>
        <div class="stage-controls">
          <button type="button" :disabled="reduced" :aria-pressed="playing" @click="playing = !playing">{{ playing ? 'Ⅱ 暂停' : '▷ 播放' }}</button>
          <button v-if="selected.interactive" type="button" @click="resetToken++">重置互动</button>
          <button type="button" :aria-pressed="showCopy" @click="showCopy = !showCopy">{{ showCopy ? '隐藏文字' : '显示文字' }}</button>
          <button ref="expandButton" type="button" :aria-label="expanded ? '退出放大预览' : '放大预览'" @click="toggleExpanded">{{ expanded ? '↙ 收起' : '↗ 放大' }}</button>
        </div>
      </div>
    </section>
    <section class="study-detail" :inert="expanded" aria-live="polite" aria-atomic="true">
      <div><p class="eyebrow">当前候选 / {{ String(selectedIndex + 1).padStart(2, '0') }}</p><h2>{{ selected.name }}</h2></div>
      <div class="detail-description"><p>{{ selected.description }}</p><NuxtLink v-if="original" class="comparison-link" :to="{ path: '/background-lab', query: { effect: original.id } }">对照原版 · {{ original.name }} ↗</NuxtLink><p class="reference-note">运动机制参考，画面由本站原创绘制；未引入参考站点的代码或素材。</p><a :href="selected.reference.href" target="_blank" rel="noopener noreferrer">查看参考 · {{ selected.reference.name }} ↗</a></div>
    </section>
    <footer class="lab-footer" :inert="expanded"><span>从抽象机制，到完整场景。</span><span>仅运行当前候选 · 切到后台自动暂停</span></footer>
  </main>
</template>

<style scoped>
.background-lab{min-height:100vh;background:#f7f6f2;color:#282b2c;padding:0 clamp(20px,5vw,88px) 28px;font-family:Arial,"PingFang SC","Microsoft YaHei",sans-serif;color-scheme:light}
.lab-header,.lab-footer{display:flex;justify-content:space-between;gap:20px;align-items:center;max-width:1440px;margin:auto}.lab-header{min-height:88px;border-bottom:1px solid #deded7}.back-link{min-height:44px;display:inline-flex;align-items:center;color:inherit;text-decoration:none;font-size:14px}.edition,.eyebrow{font-size:11px;letter-spacing:.14em;font-weight:600;color:#777970}.lab-intro{max-width:1440px;margin:46px auto 30px}.lab-intro h1{font-size:clamp(32px,4.6vw,64px);line-height:1.2;letter-spacing:-.055em;margin:14px 0 20px;font-weight:600}.lab-intro h1 span{color:#a56540}.intro-note{font-size:15px;line-height:1.8;color:#60635e;margin:0}.preview-note{font-size:12px;color:#8b7b68;margin:9px 0 0}.lab-groups{display:flex;flex-wrap:wrap;gap:8px;max-width:1440px;margin:0 auto 18px}.lab-groups button{min-height:44px;padding:0 18px;border:1px solid #deded7;border-radius:999px;font-size:13px;cursor:pointer}.lab-groups button[aria-pressed="true"]{background:#303c32;color:#f7f4e9;border-color:#303c32}.lab-groups span{margin-left:10px;font:10px monospace;opacity:.6}.study-picker{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:12px;max-width:1440px;margin:0 auto 20px}.study-picker.focused{grid-template-columns:repeat(3,minmax(0,1fr))}.study-picker.focused .study-choice{display:grid;grid-template-columns:80px minmax(0,1fr);align-items:center;gap:14px;min-height:92px}.study-picker.focused .choice-label{font-size:16px}.comparison-link{margin-right:22px}.study-choice{border:1px solid #deded7;background:transparent;border-radius:12px;padding:7px;text-align:left;cursor:pointer;transition:border-color .2s,background .2s;color:inherit;min-height:96px}.study-choice:hover{background:#eeede6}.study-choice.active{border-color:#966348;background:#fffdf7;box-shadow:0 0 0 1px #966348}.study-thumb{height:52px;display:flex;align-items:center;justify-content:center;overflow:hidden;background:var(--thumb-paper);color:var(--thumb-ink);border-radius:7px;gap:4px;perspective:100px}.study-thumb i{display:block;background:currentColor;width:13px;height:20px;opacity:.65}.thumb-blocks i{transform:skewY(-22deg);box-shadow:5px -4px 0 color-mix(in srgb,currentColor,white 50%)}.thumb-blocks i:nth-child(even){height:32px}.thumb-grid{background-image:linear-gradient(#a4c4d633 1px,transparent 1px),linear-gradient(90deg,#a4c4d633 1px,transparent 1px);background-size:16px 12px}.thumb-grid i{background:none;border:1px solid currentColor;transform:rotate(-15deg);width:20px;height:30px}.thumb-orbits i{background:none;border:1px solid currentColor;border-radius:50%;width:46px;height:24px;margin:0 -20px;transform:rotate(-35deg)}.thumb-orbits i:nth-child(even){transform:rotate(35deg)}.thumb-type::before{content:'{ GO / VUE }';font:bold 22px monospace;white-space:nowrap;transform:rotate(-9deg)}.thumb-type i{display:none}.thumb-folds i{width:23px;height:45px;transform:skewY(-24deg);margin-left:-4px}.thumb-folds i:nth-child(even){transform:skewY(24deg);opacity:.25}.thumb-kinetic::before,.thumb-kinetic-sculpture::before,.thumb-kinetic-playground::before{content:'⚙';font:44px serif}.thumb-kinetic i,.thumb-kinetic-sculpture i,.thumb-kinetic-playground i{display:none}.thumb-city-future i{width:13px;height:32px;background:transparent;border:2px solid currentColor;border-radius:50% 50% 0 0}.thumb-city-future i:first-child{width:32px;height:20px;border-radius:50%;transform:rotate(-12deg)}.thumb-city-future i:nth-child(n+4){display:none}.thumb-papercut i{width:18px;height:40px;border-radius:40px 40px 0 0;transform:translateY(8px);opacity:.3}.thumb-papercut i:nth-child(even){height:30px;opacity:.7}.thumb-city i{width:14px;height:28px;transform:skewY(-15deg);border-top:5px solid #d2b38c}.thumb-city i:nth-child(even){height:40px}.thumb-poster::before{content:'IDEA';font:bold 29px sans-serif;transform:rotate(-10deg)}.thumb-poster i{display:none}.thumb-cosmos::before{content:'';width:35px;height:35px;border-radius:50%;background:linear-gradient(110deg,#dbbd8d,#383d65);box-shadow:0 0 0 8px #ffffff15}.thumb-cosmos i{display:none}.thumb-garden i{width:12px;height:28px;border-radius:90% 0 90% 0;transform:rotate(-35deg)}.thumb-garden i:nth-child(even){transform:rotate(35deg);height:38px}.thumb-pixels{gap:5px}.thumb-pixels i{width:9px;height:9px;box-shadow:0 13px currentColor,13px 0 currentColor}.thumb-pixels i:nth-child(even){transform:translateY(-10px);opacity:.3}.choice-label{display:flex;gap:7px;align-items:center;padding:10px 3px 3px;font-size:13px;white-space:nowrap}.choice-label small{font-size:10px;color:#91918b;font-variant-numeric:tabular-nums}.lab-stage{position:relative;isolation:isolate;overflow:hidden;max-width:1440px;margin:auto;border-radius:18px;min-height:480px;height:clamp(480px,52vw,670px);background:var(--stage-paper);color:var(--stage-ink);box-shadow:0 18px 50px #282b2c0b}.lab-stage :deep(canvas){position:absolute;inset:0;width:100%;height:100%;z-index:-1}.stage-caption{position:absolute;top:26px;left:30px;right:30px;display:flex;justify-content:space-between;gap:15px;font-size:10px;letter-spacing:.13em;opacity:.7;pointer-events:none}.sample-copy{position:absolute;left:8%;top:50%;transform:translateY(-50%);pointer-events:none;max-width:80%}.sample-kicker{font-size:12px;letter-spacing:.18em;margin-bottom:24px}.sample-copy h2{font-size:clamp(48px,6.8vw,96px);font-weight:600;line-height:1.13;letter-spacing:-.055em;margin:0}.sample-copy h2 span{color:var(--stage-accent)}.sample-role{font-size:15px;letter-spacing:.15em;margin-top:25px}.stage-bottom{position:absolute;bottom:23px;left:30px;right:30px;display:flex;justify-content:space-between;align-items:end;gap:20px}.pointer-hint{font-size:11px;line-height:1.7;opacity:.75;margin:0}.mobile-hint{display:none}.stage-controls{display:flex;gap:6px;flex-shrink:0}.stage-controls button{min-height:44px;padding:0 15px;border-radius:8px;border:1px solid color-mix(in srgb,var(--stage-ink) 18%,transparent);background:color-mix(in srgb,var(--stage-paper) 86%,transparent);color:var(--stage-ink);font:inherit;font-size:12px;cursor:pointer;backdrop-filter:blur(8px)}.stage-controls button:hover{background:var(--stage-paper)}.stage-controls button:disabled{opacity:.45;cursor:default}.background-lab :is(button,a):focus-visible{outline:3px solid #b98a55;outline-offset:4px}.lab-stage.expanded{position:fixed;inset:14px;z-index:1000;max-width:none;width:auto;height:auto;min-height:0;border-radius:14px;box-shadow:0 0 0 30px #222b}.expanded .sample-copy h2{font-size:clamp(56px,9vw,136px)}.study-detail{max-width:1440px;margin:32px auto 36px;display:grid;grid-template-columns:1fr 2fr;gap:32px}.study-detail h2{font-size:28px;letter-spacing:-.03em;margin:10px 0 0;font-weight:500}.detail-description>p:first-child{font-size:16px;line-height:1.8;margin:0 0 12px}.reference-note{font-size:11px;line-height:1.8;color:#7d7f76}.detail-description a{display:inline-flex;align-items:center;min-height:44px;font-size:12px;color:#785239;text-underline-offset:4px}.lab-footer{border-top:1px solid #deded7;padding-top:21px;font-size:11px;color:#85877f;line-height:1.7}
@media(max-width:760px){.background-lab{padding-inline:18px}.lab-header{min-height:70px}.edition{font-size:9px;letter-spacing:.05em}.lab-intro{margin-top:30px}.intro-note{font-size:13px}.study-picker{grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}.study-choice{min-height:90px}.study-thumb{height:42px}.choice-label{font-size:12px;gap:5px}.lab-stage{height:560px;min-height:0;border-radius:12px}.stage-caption{left:18px;right:18px;top:20px;font-size:8px;letter-spacing:.05em}.sample-copy{left:8%;top:45%}.sample-copy h2{font-size:clamp(48px,10vw,70px)}.sample-kicker{font-size:10px;margin-bottom:18px}.sample-role{font-size:12px}.stage-bottom{left:16px;right:16px;bottom:18px;flex-direction:column;align-items:start;gap:14px}.stage-controls{width:100%;gap:5px}.stage-controls button{flex:1;padding:0 7px;font-size:11px}.pointer-hint{font-size:10px}.mobile-hint{display:block}.study-detail{grid-template-columns:1fr;gap:18px;margin:26px auto}.study-detail h2{font-size:25px}.detail-description>p:first-child{font-size:14px}.lab-footer{flex-direction:column;align-items:start;gap:4px}.lab-stage.expanded{inset:8px}.expanded .sample-copy h2{font-size:clamp(52px,12vw,96px)}}
@media(max-width:760px){.study-picker.focused .study-choice{grid-template-columns:1fr;gap:0;min-height:100px}.study-picker.focused .choice-label{font-size:12px;display:block;white-space:normal;line-height:1.5;padding:8px 0 0}.study-picker.focused .choice-label small{display:block;margin-bottom:2px}.study-picker.focused .study-thumb{width:100%}}
@media(max-width:760px){.lab-stage.focus-scene .sample-copy{top:24%;max-width:84%}.lab-stage.focus-scene .sample-copy h2{font-size:38px;line-height:1.12}.lab-stage.focus-scene .sample-kicker{margin-bottom:10px}.lab-stage.focus-scene .sample-role{margin-top:12px;font-size:11px}}
@media(max-width:380px){.choice-label{font-size:11px;gap:4px;padding-inline:0}.choice-label small{font-size:9px}}
@media(max-height:480px){.lab-stage.expanded .sample-copy{top:45%}.expanded .sample-copy h2{font-size:38px;line-height:1.08}.expanded .sample-kicker{margin-bottom:10px}.expanded .sample-role{margin-top:12px;font-size:11px}.expanded .pointer-hint{display:none}.expanded .stage-bottom{bottom:12px}.expanded .stage-caption{top:12px}}
@media(prefers-reduced-motion:reduce){.study-choice{transition:none}}
.study-picker.variants{grid-template-columns:repeat(3,minmax(0,1fr))}.thumb-balance-seesaw::before{content:'◉ ━ ▲ ━ ◉';font:22px monospace}.thumb-balance-orbit::before{content:'◎';font:48px serif}.thumb-marble-tower::before{content:'●';font:18px serif;line-height:1;border-bottom:4px double currentColor;box-shadow:0 10px 0 -1px currentColor,0 -10px 0 -1px currentColor;padding:0 10px}.thumb-switchyard::before{content:'━╋━';font:32px monospace}.variants .study-thumb i{display:none}.thumb-city-future i{border-radius:0!important;background:currentColor;border-top:4px solid #d2b38c;transform:skewY(-15deg)!important;width:14px!important;height:28px!important}.thumb-city-future i:first-child{height:38px!important}
@media(max-width:760px){.study-picker.variants{grid-template-columns:repeat(2,minmax(0,1fr))}.variants .choice-label{white-space:normal;font-size:12px}.lab-groups button{padding-inline:12px}}
.thumb-business-city::before{content:'▥ ▤ ▥';font:26px monospace;transform:skewY(-12deg)}.thumb-harbor-city::before{content:'▥ ≋ ▥';font:26px monospace;transform:skewY(-12deg)}
.stage-bottom.city-bottom{justify-content:flex-end}
</style>
