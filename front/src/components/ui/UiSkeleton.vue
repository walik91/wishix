<template>
  <span class="skeleton" aria-hidden="true"></span>
</template>

<style scoped>
.skeleton { display: block; height: 14px; border-radius: var(--wx-radius-sm); background: var(--wx-border); animation: pulse 1.6s ease-in-out infinite alternate; }
@keyframes pulse { to { opacity: .45; } }
</style>
