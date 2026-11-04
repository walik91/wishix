<script setup>
import { nextTick, ref } from 'vue'
import { t } from '../../i18n.js'
import UiButton from '../ui/UiButton.vue'
import TextField from '../ui/TextField.vue'

defineProps({ busy: Boolean, remaining: { type: Number, default: 0 } })
const emit = defineEmits(['verify', 'resend'])
const code = ref('')
const invalid = ref(false)
const field = ref(null)

async function submit() {
  invalid.value = !/^[0-9]{6}$/.test(code.value)
  if (invalid.value) { await nextTick(); field.value.focus() }
  else emit('verify', code.value)
}
</script>

<template>
  <div class="verification">
    <form class="code-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="code" ref="field" v-model="code" name="code" autocomplete="one-time-code" inputmode="numeric" maxlength="6" required :label="t('auth.code')" :hint="t('auth.codeHint')" :error="invalid ? t('auth.codeInvalid') : ''" />
      <slot />
      <UiButton type="submit" :loading="busy">{{ t('auth.confirm') }}</UiButton>
    </form>
    <UiButton variant="secondary" :disabled="busy || remaining > 0" @click="$emit('resend')">{{ t('auth.resend') }}<span v-if="remaining > 0">({{ remaining }} {{ t('auth.seconds') }})</span></UiButton>
  </div>
</template>

<style scoped>
.verification, .code-form { display: grid; gap: var(--wx-space-5); }
</style>
