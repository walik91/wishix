<template>
  <span class="badge"><slot /></span>
</template>

<style scoped>
.badge { display: inline-flex; align-items: center; width: fit-content; padding: var(--wx-space-1) var(--wx-space-3); border-radius: var(--wx-radius-pill); color: var(--wx-text-muted); background: var(--wx-background); font-size: var(--wx-text-xs); font-weight: 500; line-height: 1.5; }
</style>
