<script setup>
import AppIcon from '../ui/AppIcon.vue'
import UiCard from '../ui/UiCard.vue'
</script>

<!-- A decorative template, never a saved wish or a loading indicator. -->
<template>
  <UiCard class="wish-template" aria-hidden="true"><span class="wish-image"><AppIcon name="heart" /></span><span class="wish-lines"><span></span><span></span><span></span></span></UiCard>
</template>

<style scoped>
.wish-template { display: flex; align-items: center; gap: var(--wx-space-4); padding: var(--wx-space-3); }
.wish-image { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 64px; height: 64px; border-radius: var(--wx-radius-sm); color: var(--wx-secondary); background: var(--wx-background); }
.wish-image .icon { width: 28px; height: 28px; }
.wish-lines { display: grid; gap: var(--wx-space-2); width: 132px; }
.wish-lines > span { height: 8px; border-radius: var(--wx-radius-pill); background: var(--wx-border); }
.wish-lines > span:nth-child(2) { width: 75%; }
.wish-lines > span:nth-child(3) { width: 45%; margin-top: var(--wx-space-1); background: var(--wx-lavender); }
</style>
