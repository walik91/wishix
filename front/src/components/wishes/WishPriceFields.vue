<script setup>
import { computed, nextTick, ref } from 'vue'
import { t } from '../../i18n.js'
import { currencyPattern } from '../../wishPrice.js'
import TextField from '../ui/TextField.vue'

defineProps({ errors: { type: Object, required: true } })
const emit = defineEmits(['price-input', 'currency-input', 'currency-selected'])
const price = defineModel('price', { type: String, required: true })
const currency = defineModel('currency', { type: String, required: true })
const currencies = ['MDL', 'RON', 'EUR', 'USD', 'GBP', 'UAH', 'RUB', 'CHF', 'CAD', 'AUD', 'JPY']
const enteringCode = ref(false)
const codeField = ref(null)
const extraCurrency = computed(() => !enteringCode.value && currencyPattern.test(currency.value) && !currencies.includes(currency.value) ? currency.value : '')

function updateCurrency(code) {
  currency.value = code
  emit('currency-input')
  if (currencyPattern.test(code)) emit('currency-selected', code)
}

async function selectCurrency(event) {
  const code = event.target.value
  enteringCode.value = code === 'other'
  updateCurrency(enteringCode.value ? '' : code)
  if (enteringCode.value) {
    await nextTick()
    codeField.value?.focus()
  }
}
</script>

<template>
  <fieldset class="price-group">
    <legend>{{ t('wishes.priceLabel') }}</legend>
    <div class="price-fields">
      <TextField id="wish-price" v-model="price" :label="t('wishes.priceAmountLabel')" :hint="t('wishes.priceHint')" :error="errors.price ? t(`wishes.error.${errors.price}`) : ''" inputmode="decimal" placeholder="0.00" maxlength="13" @input="emit('price-input')" />
      <div class="currency-field">
        <label for="wish-currency">{{ t('wishes.currencyLabel') }}</label>
        <select id="wish-currency" :value="enteringCode ? 'other' : currency" :aria-invalid="!enteringCode && errors.currency ? 'true' : undefined" :aria-describedby="!enteringCode && errors.currency ? 'wish-currency-error' : undefined" @change="selectCurrency">
          <option value="">{{ t('wishes.currencyPlaceholder') }}</option>
          <option v-for="code in currencies" :key="code" :value="code">{{ code }}</option>
          <option v-if="extraCurrency" :value="extraCurrency">{{ extraCurrency }}</option>
          <option value="other">{{ t('wishes.currencyOther') }}</option>
        </select>
        <p v-if="!enteringCode && errors.currency" id="wish-currency-error" class="field-error">{{ t(`wishes.error.${errors.currency}`) }}</p>
        <TextField v-if="enteringCode" id="wish-currency-code" ref="codeField" :model-value="currency" :label="t('wishes.currencyCodeLabel')" :error="errors.currency ? t(`wishes.error.${errors.currency}`) : ''" maxlength="3" autocomplete="off" autocapitalize="characters" spellcheck="false" @update:model-value="updateCurrency($event.toUpperCase())" />
      </div>
    </div>
  </fieldset>
</template>

<style scoped>
.price-group { min-width: 0; margin: 0; padding: 0; border: 0; }
legend { margin-bottom: var(--wx-space-3); padding: 0; font-weight: 600; }
.price-fields { display: grid; gap: var(--wx-space-3); min-width: 0; align-items: start; }
.currency-field { display: grid; gap: var(--wx-space-2); min-width: 0; }
label { font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
select { width: 100%; min-width: 0; min-height: 48px; padding: var(--wx-space-3); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-text); background: var(--wx-surface); font-size: var(--wx-text-md); }
select:hover { border-color: var(--wx-secondary); }
select:focus-visible { border-color: var(--wx-primary); box-shadow: var(--wx-focus-ring); outline: none; }
select[aria-invalid="true"] { border-color: var(--wx-error); }
.field-error { margin: 0; color: var(--wx-error); font-size: var(--wx-text-xs); line-height: 1.5; }
@media (min-width: 48rem) {
  .price-fields { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); }
}
</style>
