<script setup>
defineProps({ href: String })
</script>

<template>
  <component :is="href ? 'a' : 'div'" class="card" :href="href || undefined"><slot /></component>
</template>

<style scoped>
.card { min-width: 0; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: var(--wx-shadow-card); }
</style>
