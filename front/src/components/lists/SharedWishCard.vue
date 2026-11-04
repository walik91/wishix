<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { sharedWishImage, publicCollectionImage } from '../../api/lists.js'
import WishCard from '../wishes/WishCard.vue'
const props = defineProps({ wish: { type: Object, required: true }, token: { type: String, default: '' }, username: { type: String, default: '' } })
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
    try { const signal = AbortSignal.any([request.signal, AbortSignal.timeout(10000)]); const blob = await (props.username ? publicCollectionImage(props.username, props.wish.id, signal) : sharedWishImage(props.token, props.wish.id, signal)); if (current === version) image.value = URL.createObjectURL(blob) }
    catch { /* The wish remains readable when the image is unavailable or access changed. */ }
  }, { rootMargin: '200px' })
  observer.observe(container.value)
}
onMounted(() => { mounted = true; observe() })
watch(() => [props.token, props.username, props.wish.id, props.wish.has_image], observe)
onBeforeUnmount(reset)
</script>
<template><div ref="container" class="shared-wish"><WishCard :wish="wish" read-only /></div></template>
<style scoped>.shared-wish { min-width: 0; }</style>
