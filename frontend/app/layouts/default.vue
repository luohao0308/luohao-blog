<template>
  <div class="flex min-h-screen flex-col bg-white text-slate-900 antialiased">
    <header class="border-b border-slate-200">
      <div class="mx-auto flex max-w-4xl items-center justify-between px-6 py-4">
        <NuxtLink to="/" class="text-lg font-semibold tracking-tight">luohao-blog</NuxtLink>
        <nav class="flex gap-6 text-sm text-slate-600">
          <NuxtLink to="/" class="transition-colors hover:text-slate-900">首页</NuxtLink>
          <NuxtLink to="/archives" class="transition-colors hover:text-slate-900">归档</NuxtLink>
          <NuxtLink to="/about" class="transition-colors hover:text-slate-900">关于</NuxtLink>
        </nav>
      </div>
    </header>
    <main class="mx-auto w-full max-w-4xl flex-1 px-6 py-10">
      <slot />
    </main>
    <footer class="border-t border-slate-200 py-6 text-center text-sm text-slate-500">
      © luohao · 全栈 & AI Agent
    </footer>
  </div>
</template>
