<template>
  <div class="card"><slot /></div>
</template>

<style scoped>
.card { min-width: 0; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: var(--wx-shadow-card); }
</style>
