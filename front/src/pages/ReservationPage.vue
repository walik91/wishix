<script setup>
import { onMounted, ref } from 'vue'
import { locale, t } from '../i18n.js'
import { manageReservation } from '../api/reservations.js'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiDialog from '../components/ui/UiDialog.vue'
const token = window.location.hash.slice(1)
if (window.location.hash) window.history.replaceState(null, '', window.location.pathname)
const booking = ref(null), loading = ref(true), busy = ref(false), error = ref(''), cancelled = ref(false), cancelOpen = ref(false)
function showDate(value) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'long', timeStyle: 'short' }).format(new Date(value)) }
async function perform(action = '') {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    const result = await manageReservation(token, action)
    if (action === 'cancel') { cancelled.value = true; cancelOpen.value = false; booking.value = null }
    else booking.value = result.reservation
  } catch (failure) { error.value = t(`reservation.error.${['reservation_expired', 'reservation_unavailable', 'reservation_action_invalid', 'rate_limited'].includes(failure.code) ? failure.code : 'unavailable'}`) }
  finally { loading.value = false; busy.value = false }
}
onMounted(() => { document.title = `${t('reservation.manage')} · ${t('appName')}`; perform() })
</script>
<template>
  <main class="reservation-page"><UiCard class="reservation-panel">
    <h1>{{ t('reservation.manage') }}</h1>
    <p v-if="loading" role="status">{{ t('reservation.loading') }}</p>
    <p v-else-if="cancelled" role="status">{{ t('reservation.cancelled') }}</p>
    <template v-else-if="booking">
      <h2>{{ booking.wish_title }}</h2><p v-if="booking.list_title">{{ booking.list_title }}</p>
      <p>{{ t(booking.status === 'purchased' ? 'reservation.purchased' : 'reservation.reserved') }}</p>
      <p v-if="booking.expires_at">{{ t('reservation.expires') }} <time :datetime="booking.expires_at">{{ showDate(booking.expires_at) }}</time></p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <div class="actions">
        <UiButton v-if="booking.status === 'reserved'" :loading="busy" @click="perform('purchase')">{{ t('reservation.markPurchased') }}</UiButton>
        <UiButton v-if="booking.status === 'reserved' && booking.can_extend" variant="secondary" :disabled="busy" @click="perform('extend')">{{ t('reservation.extend') }}</UiButton>
        <UiButton v-if="booking.status === 'purchased'" variant="secondary" :disabled="busy" @click="perform('undo_purchase')">{{ t('reservation.undoKeep') }}</UiButton>
        <UiButton variant="secondary" :disabled="busy" @click="cancelOpen = true">{{ t(booking.status === 'purchased' ? 'reservation.undoCancel' : 'reservation.cancel') }}</UiButton>
      </div>
    </template>
    <p v-else-if="error" class="error" role="alert">{{ error }}</p>
    <UiDialog v-model:open="cancelOpen" :busy="busy" :title="t('reservation.cancel')"><p>{{ t('reservation.cancelConfirm') }}</p><div class="actions"><UiButton variant="destructive" :loading="busy" @click="perform('cancel')">{{ t('reservation.cancel') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="cancelOpen = false">{{ t('reservation.back') }}</UiButton></div></UiDialog>
  </UiCard></main>
</template>
<style scoped>.reservation-page { width: min(100%, 600px); margin: var(--wx-space-8) auto; }.reservation-panel { padding: var(--wx-space-6); }.reservation-panel p { line-height: 1.5; }.actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); margin-top: var(--wx-space-5); }.error { color: var(--wx-error); }</style>
