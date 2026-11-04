<script setup>
import { t } from '../../i18n.js'
defineProps({ lists: { type: Array, default: () => [] }, disabled: Boolean })
const selected = defineModel({ type: Array, default: () => [] })
</script>
<template>
  <div class="list-checkboxes"><label v-for="list in lists" :key="list.id" class="list-check-row"><input v-model="selected" type="checkbox" :value="list.id" :disabled="disabled" /><span><strong>{{ list.title }}</strong><small>{{ t(`lists.visibility.${list.visibility}`) }}</small></span></label></div>
</template>
