<script setup>
import { ref } from 'vue'
import { t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
import TextField from '../ui/TextField.vue'

defineProps({
  error: { type: String, default: '' },
  autocomplete: { type: String, default: 'current-password' },
  placeholder: { type: String, default: '' },
})
const value = defineModel({ type: String, default: '' })
const visible = ref(false)
const field = ref(null)
defineExpose({ focus: () => field.value?.focus() })
</script>

<template>
  <TextField id="password" ref="field" v-model="value" name="password" :type="visible ? 'text' : 'password'" :autocomplete="autocomplete" required :label="t('login.password')" :placeholder="placeholder || t('login.passwordPlaceholder')" :error="error">
    <button class="password-toggle" type="button" :aria-label="t(visible ? 'login.hidePassword' : 'login.showPassword')" :aria-pressed="visible" aria-controls="password" @click="visible = !visible">
      <AppIcon :name="visible ? 'eyeOff' : 'eye'" />
    </button>
  </TextField>
</template>

<style scoped>
.password-toggle { display: grid; place-items: center; align-self: center; min-width: 44px; min-height: 44px; margin-right: var(--wx-space-1); padding: var(--wx-space-2); border: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: transparent; cursor: pointer; }
.password-toggle:hover { color: var(--wx-primary); background: var(--wx-lavender); }
</style>
