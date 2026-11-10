<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { locale, t } from '../../i18n.js'
import { useSession } from '../../composables/useSession.js'
import { startReservation, confirmReservation, recoverReservation } from '../../api/reservations.js'
import UiButton from '../ui/UiButton.vue'
import UiDialog from '../ui/UiDialog.vue'
import TextField from '../ui/TextField.vue'
import { guestWishImage, publicCollectionImage } from '../../api/lists.js'
import WishCard from '../wishes/WishCard.vue'
const props = defineProps({ wish: { type: Object, required: true }, listId: { type: String, default: '' }, token: { type: String, default: '' }, username: { type: String, default: '' } })
const emit = defineEmits(['updated'])
const { user } = useSession()
const open = ref(false), mode = ref('start'), email = ref(''), code = ref(''), challenge = ref(''), busy = ref(false), failure = ref(''), success = ref(''), refreshNeeded = ref(false)
watch(open, value => { if (!value && refreshNeeded.value) { refreshNeeded.value = false; emit('updated') } })
function show(which) { mode.value = which; email.value = user.value?.email || ''; code.value = ''; challenge.value = ''; failure.value = ''; success.value = ''; open.value = true }
function payload() { return { wish_id: props.wish.id, list_id: props.listId, list_token: props.token, username: props.username, email: email.value, locale: locale.value } }
async function submit() {
  if (busy.value) return
  busy.value = true; failure.value = ''
  try {
    if (mode.value === 'start') { const result = await startReservation(payload()); challenge.value = result.challenge; mode.value = 'verify' }
    else if (mode.value === 'verify') { await confirmReservation({ ...payload(), challenge: challenge.value, code: code.value }); success.value = t('reservation.confirmed'); mode.value = 'done'; refreshNeeded.value = true }
    else if (mode.value === 'recover') { await recoverReservation(payload()); success.value = t('reservation.recoverySent'); mode.value = 'done' }
  } catch (error) { failure.value = t(`reservation.error.${['invalid_code', 'invalid_input', 'reservation_unavailable', 'reservation_limit', 'rate_limited', 'email_unavailable'].includes(error.code) ? error.code : 'unavailable'}`) }
  finally { busy.value = false }
}
const container = ref(null), image = ref('')
const wish = computed(() => ({ ...props.wish, image_url: image.value }))
let observer, request, mounted = false, version = 0
function reset() { version++; observer?.disconnect(); request?.abort(); if (image.value) URL.revokeObjectURL(image.value); image.value = '' }
function observe() {
  reset()
  if (!mounted || !props.wish.has_image) return
  observer = new IntersectionObserver(async entries => {
    if (!entries.some(entry => entry.isIntersecting)) return
    observer.disconnect(); const current = version
    request = new AbortController()
    try { const signal = AbortSignal.any([request.signal, AbortSignal.timeout(10000)]); const blob = await (props.username ? publicCollectionImage(props.username, props.wish.id, signal) : guestWishImage(props.listId, props.token, props.wish.id, signal)); if (current === version) image.value = URL.createObjectURL(blob) }
    catch { /* The wish remains readable when the image is unavailable or access changed. */ }
  }, { rootMargin: '200px' })
  observer.observe(container.value)
}
onMounted(() => { mounted = true; observe() })
watch(() => [props.listId, props.token, props.username, props.wish.id, props.wish.has_image], observe)
onBeforeUnmount(reset)
</script>
<template>
  <div ref="container" class="shared-wish">
    <WishCard :wish="wish" read-only />
    <div class="reservation-actions"><template v-if="wish.reserved"><span class="reservation-status">{{ t('reservation.reserved') }}</span><UiButton v-if="wish.guest_recoverable" variant="secondary" @click="show('recover')">{{ t('reservation.needLink') }}</UiButton></template><UiButton v-else @click="show('start')">{{ t('reservation.reserve') }}</UiButton></div>
    <UiDialog v-model:open="open" :busy="busy" :title="t(mode === 'recover' ? 'reservation.needLink' : 'reservation.reserve')">
      <form v-if="mode !== 'done'" class="reservation-form" @submit.prevent="submit">
        <p class="reservation-hint">{{ wish.title }}</p>
        <TextField v-if="mode !== 'verify'" :id="`email-${wish.id}`" v-model="email" type="email" required autocomplete="email" :label="t('reservation.email')" :disabled="busy || (mode === 'start' && !!user)" />
        <template v-else><p class="reservation-hint">{{ t('reservation.codeSent') }} {{ email }}</p><TextField :id="`code-${wish.id}`" v-model="code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required :label="t('reservation.code')" :disabled="busy" /></template>
        <p v-if="failure" class="reservation-error" role="alert">{{ failure }}</p>
        <UiButton type="submit" :loading="busy">{{ t(mode === 'start' ? 'reservation.sendCode' : mode === 'verify' ? 'reservation.confirm' : 'reservation.sendLink') }}</UiButton>
        <UiButton v-if="mode === 'verify'" variant="secondary" :disabled="busy" @click="mode = 'start'">{{ t('reservation.back') }}</UiButton>
      </form>
      <p v-else role="status">{{ success }}</p>
    </UiDialog>
  </div>
</template>
<style scoped>.shared-wish { min-width: 0; }.reservation-actions { display: flex; gap: var(--wx-space-3); align-items: center; flex-wrap: wrap; margin-top: var(--wx-space-2); }.reservation-status { color: var(--wx-text-muted); font-weight: 600; }.reservation-form { display: grid; gap: var(--wx-space-4); }.reservation-hint { color: var(--wx-text-muted); }.reservation-error { color: var(--wx-error); }</style>
