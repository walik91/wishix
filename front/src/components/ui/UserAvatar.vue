<script setup>
import { computed, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
const props = defineProps({ username: { type: String, default: '' }, src: { type: String, default: '' } })
const initial = computed(() => Array.from(props.username.trim())[0]?.toLocaleUpperCase())
const failed = ref(false)
watch(() => props.src, () => { failed.value = false })
</script>

<template>
  <span class="avatar" aria-hidden="true"><img v-if="src && !failed" :src="src" alt="" @error="failed = true" /><span v-else-if="initial">{{ initial }}</span><AppIcon v-else name="user" /></span>
</template>

<style scoped>
.avatar { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 40px; width: 40px; height: 40px; border: 2px solid var(--wx-surface); border-radius: 50%; color: var(--wx-primary-pressed); background: var(--wx-lavender); font-size: var(--wx-text-md); font-weight: 700; }
.avatar img { width: 100%; height: 100%; object-fit: cover; border-radius: inherit; }
</style>
