<script setup>
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
const props = defineProps({ username: { type: String, default: '' } })
const initial = computed(() => Array.from(props.username.trim())[0]?.toLocaleUpperCase())
</script>

<template>
  <span class="avatar" aria-hidden="true"><span v-if="initial">{{ initial }}</span><AppIcon v-else name="user" /></span>
</template>

<style scoped>
.avatar { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 40px; width: 40px; height: 40px; border: 2px solid var(--wx-surface); border-radius: 50%; color: var(--wx-primary-pressed); background: var(--wx-lavender); font-size: var(--wx-text-md); font-weight: 700; }
</style>
