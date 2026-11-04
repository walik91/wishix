<script setup>
import { onMounted, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { publicProfile } from '../api/lists.js'
import { sharedListPath } from '../sharedListPath.js'
import ListCard from '../components/wishes/ListCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
const props = defineProps({ username: { type: String, required: true } })
const username = ref(''), lists = ref([]), loading = ref(true), error = ref('')
watchEffect(() => { document.title = `${username.value || t('lists.publicProfile')} · ${t('appName')}` })
async function load() { loading.value = true; error.value = ''; try { const data = await publicProfile(props.username); username.value = data.username; lists.value = data.lists } catch (failure) { error.value = failure.status === 404 ? 'list_not_found' : 'unavailable' } finally { loading.value = false } }
onMounted(load)
</script>
<template><div class="lists-page public-profile-page"><UiSkeleton v-if="loading" class="list-form-skeleton" /><UiCard v-else-if="error" class="list-empty"><EmptyState icon="user" :title="t('lists.profileUnavailable')"><UiButton v-if="error !== 'list_not_found'" @click="load">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard><template v-else><header class="list-page-heading"><div><p class="list-eyebrow">{{ t('lists.publicProfile') }}</p><h1>{{ username }}</h1><p class="list-muted">{{ t('lists.publicLists') }}</p></div></header><div v-if="lists.length" class="list-grid"><ListCard v-for="(list, index) in lists" :key="index" :list="list" :href="sharedListPath(username, list.share_path)" :manage="false" /></div><UiCard v-else class="list-empty"><EmptyState icon="list" :title="t('lists.noPublicLists')" /></UiCard></template></div></template>
<style scoped>.public-profile-page { padding-block: var(--wx-space-8); }</style>
